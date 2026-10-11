#!/usr/bin/env python3
"""Offline shell CLI checks: no network, queue changes and a real foreground PTY."""
import json
import os
from pathlib import Path
import pty
import select
import shlex
import signal
import subprocess
import sys
import tempfile
import time

ROOT = Path(__file__).resolve().parents[1]
FAKE = r'''
import json, os, shlex, signal, sys, time
from pathlib import Path
root=Path(os.environ['PERCH_SHELL_TEST'])
args=sys.argv[1:]
if '-O' in args: sys.exit(0)
command=shlex.split(args[-1])
with (root/'calls').open('a') as out: out.write(json.dumps(command)+'\n')
if command[0]=='id': print('researcher')
elif command[0]=='squeue':
    count=int((root/'count').read_text())+1
    (root/'count').write_text(str(count))
    data=json.loads((root/'queue').read_text())
    mode=(root/'mode').read_text()
    if count>1 and mode=='requeued': data['jobs'][0]['restart_cnt']=1
    if mode=='foreign': data['jobs'][0]['user_name']='someone-else'
    if mode=='finished': data['jobs'][0]['job_state']=['COMPLETED']
    print(json.dumps(data))
elif command[:3]==['scontrol','show','hostnames']: print('node-14')
elif command[:2]==['timeout','--version']: print('timeout (GNU coreutils) 8.32')
elif command[0]=='env':
    assert '-tt' in args and 'RequestTTY=force' in args and 'RequestTTY=no' not in args, args
    assert os.isatty(0) and os.isatty(1)
    # This fake validates the handoff contract; the separate opt-in live harness
    # checks actual OpenSSH raw mode, Ctrl-C and Slurm step teardown.
    (root/'interactive-pid').write_text(str(os.getpid()))
    def stop(*_):
        (root/'child-terminated').write_text('yes')
        sys.exit(143)
    signal.signal(signal.SIGTERM,stop)
    print('SYNTHETIC_SHELL_READY',flush=True)
    for line in sys.stdin:
        if line.strip()=='exit': break
    print('SYNTHETIC_SHELL_CLOSED',flush=True)
else: raise AssertionError(command)
'''


def main():
    with tempfile.TemporaryDirectory(prefix='perch-shell-test-') as directory:
        root=Path(directory); (root/'home').mkdir(mode=0o700)
        ssh=root/'ssh'; ssh.write_text('#!'+sys.executable+'\n'+FAKE); ssh.chmod(0o700)
        data=json.loads((ROOT/'testdata/hummingbird/queue.json').read_text())
        data['jobs']=data['jobs'][:1]
        (root/'queue').write_text(json.dumps(data))
        env=os.environ|{'HOME':str(root/'home'),'PATH':str(root)+':/usr/bin:/bin','PERCH_SHELL_TEST':str(root)}
        args=[str(ROOT/'bin/perch'),'shell','9100_39','--host','fixture.invalid','--no-config']
        def reset(mode='ok'):
            (root/'mode').write_text(mode); (root/'count').write_text('0'); (root/'calls').write_text('')
        def calls(): return [json.loads(x) for x in (root/'calls').read_text().splitlines()]
        reset()
        result=subprocess.run([*args,'--json'],capture_output=True,text=True,env=env,timeout=10)
        assert result.returncode==0,result.stderr
        preview=json.loads(result.stdout)
        assert preview['mode']=='preview' and preview['plan']['allocation_id']==9101
        assert not any(x[0]=='env' for x in calls()),calls()
        reset()
        result=subprocess.run([*args,'--open'],capture_output=True,text=True,env=env,timeout=10)
        assert result.returncode and 'interactive terminal' in result.stderr and not calls()
        for mode in ['foreign','finished']:
            reset(mode)
            result=subprocess.run(args,capture_output=True,text=True,env=env,timeout=10)
            assert result.returncode and 'RUNNING' in result.stderr,result
            assert not any(x[0]=='env' for x in calls())

        def terminal(mode, terminate=False):
            reset(mode)
            master,slave=pty.openpty()
            process=subprocess.Popen([*args,'--open'],stdin=slave,stdout=slave,stderr=slave,env=env,start_new_session=True)
            raw=bytearray()
            def pump():
                if select.select([master],[],[],.05)[0]: raw.extend(os.read(master,65536))
            def wait(marker):
                end=time.monotonic()+12
                while marker not in raw:
                    pump()
                    assert process.poll() is None and time.monotonic()<end,raw.decode(errors='replace')
            try:
                if mode=='requeued':
                    while process.poll() is None: pump()
                    pump()
                    assert process.returncode and b'allocation changed since preview' in raw,raw
                    assert not any(x[0]=='env' for x in calls())
                else:
                    wait(b'SYNTHETIC_SHELL_READY')
                    assert int((root/'count').read_text())==2
                    if terminate: process.terminate()
                    else: os.write(master,b'exit\n')
                    deadline=time.monotonic()+10
                    while process.poll() is None:
                        pump(); assert time.monotonic()<deadline,raw
                    pump()
                    if terminate:
                        assert process.returncode and (root/'child-terminated').read_text()=='yes'
                    else:
                        assert process.returncode==0 and b'Shell closed.' in raw,raw
                    child=int((root/'interactive-pid').read_text())
                    try: os.kill(child,0)
                    except ProcessLookupError: pass
                    else: raise AssertionError('shell child survived exit')
            finally:
                if process.poll() is None: process.kill(); process.wait(timeout=5)
                os.close(master);os.close(slave)
        terminal('requeued')
        terminal('ok')
        terminal('ok',terminate=True)
        if '--render' in sys.argv:
            from test_terminal import Session
            for width in [100,48]:
                reset()
                session=Session(args[1:],width,25,env=env)
                try:
                    while session.process.poll() is None: session.pump(.1)
                    session.pump(.2)
                    assert session.process.returncode==0,session.text
                    session.screenshot('shell-preview-'+str(width))
                finally: session.abort()
    print('Shell CLI: preview is read-only; ownership/state/requeue guarded; explicit foreground entry and SIGTERM child cleanup passed through a real PTY.')


if __name__=='__main__': main()
