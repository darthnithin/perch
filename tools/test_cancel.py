#!/usr/bin/env python3
"""Exercise the production cancellation CLI through a synthetic system SSH.

No network or actual scheduler. Verify preview, exact mutation argv, refusals,
unknown outcomes, exit status, JSON, and no retries across process boundaries.
"""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time

ROOT = Path(__file__).resolve().parents[1]
FAKE = r'''
import json, os, shlex, sys, time
from pathlib import Path
root=Path(os.environ['PERCH_CANCEL_TEST'])
args=sys.argv[1:]
if '-O' in args: sys.exit(0)
assert 'fixture.invalid' in args, args
command=shlex.split(args[-1])
with (root/'calls').open('a') as out: out.write(json.dumps(command)+'\n')
mode=(root/'mode').read_text()
if command==['id','-un']: print('researcher');sys.exit(0)
assert command[0]=='env' and command[10]=='perch-cancel',command
command=command[11:]
if command[0]=='squeue':
    count=int((root/'count').read_text())+1;(root/'count').write_text(str(count))
    assert '--local' in command and '--user=researcher' in command
    data=json.loads((root/'queue').read_text())
    if mode=='foreign': data['jobs'][0]['user_name']='someone-else'
    if count>1 and mode=='requeued':data['jobs'][0]['restart_cnt']=1
    if count>1 and mode=='completed':data['jobs']=[]
    print(json.dumps(data))
elif command[0]=='scancel':
    assert '--ctld' in command and '--clusters=hummingbird' in command
    assert '--user=researcher' in command and command[-2]=='--'
    if mode=='timeout':time.sleep(5)
    if mode=='network':print('Connection lost',file=sys.stderr);sys.exit(255)
    if mode=='nonzero':print('Unsuccessful request',file=sys.stderr);sys.exit(1)
else:raise AssertionError(command)
'''


def main():
    with tempfile.TemporaryDirectory(prefix='perch-cancel-test-') as directory:
        root=Path(directory);(root/'home').mkdir(mode=0o700)
        ssh=root/'ssh';ssh.write_text('#!'+sys.executable+'\n'+FAKE);ssh.chmod(0o700)
        data=json.loads((ROOT/'testdata/hummingbird/queue.json').read_text())
        data['jobs']=data['jobs'][:1];job=data['jobs'][0]
        job['job_id']=9100  # Important: a task allocation can equal its parent.
        job['het_job_id']={'set':False,'infinite':False,'number':0}
        job['het_job_offset']={'set':True,'number':0}
        for key in ['het_job_id_set','federation_origin','federation_siblings_active','federation_siblings_viable']:job[key]=''
        (root/'queue').write_text(json.dumps(data))
        env=os.environ|{'HOME':str(root/'home'),'PATH':str(root)+':/usr/bin:/bin','PERCH_CANCEL_TEST':str(root)}
        base=[str(ROOT/'bin/perch'),'cancel','--host=fixture.invalid','--no-config']
        def run(*args,mode='ok'):
            (root/'mode').write_text(mode);(root/'count').write_text('0');(root/'calls').write_text('')
            result=subprocess.run([*base,*args],capture_output=True,text=True,env=env,timeout=10)
            calls=[json.loads(x) for x in (root/'calls').read_text().splitlines()]
            mutations=[a[11:] for a in calls if len(a)>11 and a[11]=='scancel']
            return result,calls,mutations
        r,_,m=run('9100_39','--json');assert r.returncode==0 and not m,r
        p=json.loads(r.stdout);assert p['mode']=='preview' and p['preview']['scope']=='task'
        r,_,m=run('9100_39');assert r.returncode==0 and 'siblings continue' in r.stdout and 'Preview only' in r.stdout and not m,r
        r,_,m=run('9100');assert r.returncode and '--array' in r.stderr and not m,r
        r,_,m=run('9100_39','--confirm','--json');assert r.returncode==0 and json.loads(r.stdout)['status']=='requested' and len(m)==1,r
        assert m[0]==['scancel','--ctld','--clusters=hummingbird','--user=researcher','--state=RUNNING','--','9100_39'],m
        r,_,m=run('9100','--array','--confirm','--json');assert r.returncode==0 and len(m)==1 and m[0][-1]=='9100' and not any(a.startswith('--state=') for a in m[0]),r
        for mode in ['requeued','completed','foreign']:
            r,_,m=run('9100_39','--confirm','--json',mode=mode);assert r.returncode and not m,(mode,r,m)
        for mode,status in [('timeout','unknown'),('network','unknown'),('nonzero','unconfirmed')]:
            r,_,m=run('9100_39','--confirm','--json','--timeout=800ms',mode=mode)
            assert r.returncode and len(m)==1 and json.loads(r.stdout)['status']==status,(mode,r,m)
        for args in [('9100_39','--array','--confirm'),('9100.0','--confirm'),('9100','--confirm','--signal=KILL')]:
            r,c,m=run(*args);assert r.returncode and not c,(args,r,c)
        if '--render' in sys.argv:
            from test_terminal import Session
            for width,height in [(80,30),(48,36)]:
                (root/'mode').write_text('ok');(root/'count').write_text('0');(root/'calls').write_text('')
                s=Session(['cancel','9100_39','--host=fixture.invalid','--no-config'],width,height,env=env)
                try:
                    deadline=time.monotonic()+10
                    while s.process.poll() is None and time.monotonic()<deadline:s.pump()
                    s.pump()
                    assert s.process.poll()==0,s.text
                    assert 'siblings continue' in s.text and 'Preview only.' in s.text,s.text
                    s.screenshot('cancellation-preview-'+str(width),scale=2)
                    import termios
                    assert termios.tcgetattr(s.slave)==s.before,'CLI changed terminal attributes'
                finally:s.abort()
            print('PASS: 80/48-column PTY previews, wrapping and terminal attributes; emulator renders saved')
        print('PASS: cancellation CLI preview, explicit scope, exact argv, fresh checks, unknown receipts, and one-shot mutations')


if __name__=='__main__':main()
