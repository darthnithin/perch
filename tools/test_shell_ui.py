#!/usr/bin/env python3
"""Allocation shell UI round trips through real PTYs and synthetic SSH only."""
import json
import os
import signal
import termios
import time
from pathlib import Path
import sys
import tempfile

from test_terminal import Session,ROOT
from test_failure_terminal import FAKE_SSH
from test_footer import activate,footer
from test_mouse import click,target
from test_hover import move,env as pointer_env

EXTRA = r'''
root=pathlib.Path(os.environ['PERCH_SHELL_TEST'])
if remote[0]=='squeue':
    import json,time
    count=int((root/'count').read_text())+1
    (root/'count').write_text(str(count))
    mode=(root/'mode').read_text()
    if mode=='slow':time.sleep(2)
    data=json.loads((pathlib.Path(os.environ['PERCH_TEST_FIXTURES'])/'queue.json').read_text())
    if mode=='requeued' and count>1: data['jobs'][0]['restart_cnt']=1
    print(json.dumps(data));sys.exit(0)
if remote[:3]==['scontrol','show','hostnames']:
    print('node-14\nnode-15');sys.exit(0)
if remote[:2]==['timeout','--version']:
    print('timeout (GNU coreutils) 8.32');sys.exit(0)
if remote[0]=='env':
    import termios,tty
    assert '-tt' in sys.argv and 'RequestTTY=force' in sys.argv and 'RequestTTY=no' not in sys.argv
    assert os.isatty(0) and os.isatty(1)
    with (root/'opened').open('a') as out: out.write('opened\n')
    import json
    (root/'last-command').write_text(json.dumps(remote))
    (root/'child-pid').write_text(str(os.getpid()))
    if (root/'mode').read_text()=='exit-error':sys.exit(12)
    before=termios.tcgetattr(0)
    try:
        tty.setraw(0)
        os.write(1,b'\r\nPERCH_SHELL_READY\r\nprivate-shell-output\r\n')
        line=b''
        while True:
            c=os.read(0,1)
            if not c:break
            if c==b'\x03':
                line=b'';os.write(1,b'\r\nPERCH_SHELL_INTERRUPTED\r\n');continue
            if c in [b'\r',b'\n']:
                if line==b'exit':break
                line=b''
            else:line+=c
    finally:termios.tcsetattr(0,termios.TCSANOW,before)
    sys.exit(0)
'''


def main():
    for width,height in [(140,40),(48,18)]:
        s=Session(['--demo','--no-config','--reduced-motion'],width,height,env=pointer_env('ghostty'))
        try:
            s.wait('Find a command');s.key('/find parameter\r');s.wait('Filter: parameter')
            activate(s,'enter inspect');s.wait('o shell')
            activate(s,'o shell');s.wait('enter open shell')
            move(s,*footer(s,'enter open shell'))
            x,y=footer(s,'enter open shell')
            assert s.screen.buffer[y][x].underscore
            s.screenshot('shell-ui-demo-'+str(width))
            s.key('\x1b[F');s.wait('Exit or Ctrl-D')
            s.screenshot('shell-ui-bottom-'+str(width))
            activate(s,'esc back');s.wait('o shell')
            s.key('o');s.wait('enter open shell');activate(s,'enter open shell')
            s.wait('Demo preview only');s.wait('o shell')
            s.key('\x1b');s.wait('Filter: parameter');s.close()
        except BaseException:print(s.text);s.abort();raise

    with tempfile.TemporaryDirectory(prefix='perch-shell-ui-') as directory:
        root=Path(directory);(root/'home').mkdir(mode=0o700)
        state=root/'state';state.write_text('ok')
        mode=root/'mode';mode.write_text('ok')
        count=root/'count';count.write_text('0')
        opened=root/'opened';opened.write_text('')
        fixtures=root/'fixtures';fixtures.mkdir()
        for name in ['queue','accounting','partitions']:
            data=json.loads((ROOT/'testdata/hummingbird'/f'{name}.json').read_text())
            if name=='queue':
                data['jobs']=data['jobs'][:1]
                data['jobs'][0]['node_count']={'set':True,'infinite':False,'number':2}
                data['jobs'][0]['nodes']='node-[14-15]'
            (fixtures/f'{name}.json').write_text(json.dumps(data))
        ssh=root/'ssh'
        ssh.write_text('#!'+sys.executable+'\n'+FAKE_SSH.replace('assert remote[:2] == ["sh", "-c"], remote',EXTRA+'\nassert remote[:2] == ["sh", "-c"], remote'))
        ssh.chmod(0o700)
        env=pointer_env('ghostty')|{'HOME':str(root/'home'),'PATH':str(root)+':/usr/bin:/bin',
            'PERCH_TEST_STATE':str(state),'PERCH_TEST_FIXTURES':str(fixtures),'PERCH_SHELL_TEST':str(root)}
        s=Session(['--host','perch-test.invalid','--no-config','--reduced-motion','--debug','--debug-raw'],120,36,env=env)
        try:
            s.wait('● connected');s.wait('simulation-sweep')
            s.key('/find simulation\r');s.wait('Filter: simulation')
            activate(s,'enter inspect');s.wait('9100_39 · simulation-sweep')
            activate(s,'o shell');s.wait('enter open shell');s.wait('node-14')
            activate(s,'n node');s.wait('node-15');s.wait('enter open shell')
            s.screenshot('shell-ui-live-preview')
            assert not opened.read_text(),'preview opened a shell'
            before=len(s.raw)
            move(s,*target(s,'Open shell  ↵'));click(s,*target(s,'Open shell  ↵'))
            deadline=__import__('time').monotonic()+10
            while b'PERCH_SHELL_READY' not in s.raw[before:]:
                s.pump();assert __import__('time').monotonic()<deadline,s.text
            prefix=bytes(s.raw[before:]).split(b'PERCH_SHELL_READY')[0]
            assert b'\x1b[?1049l' in prefix and b'\x1b[?1003l' in prefix
            assert b'\x1b]22;default\x07' in prefix,'pointer not reset before shell'
            s.key('\x03');assert b'PERCH_SHELL_INTERRUPTED' in s.raw
            s.key('exit\r');s.wait('Shell closed');s.wait('9100_39 · simulation-sweep')
            assert s.raw.count(b'\x1b[?1049h')>=2
            assert opened.read_text().count('opened')==1
            launched=json.loads((root/'last-command').read_text())
            assert '--nodelist=node-15' in launched and '--jobid=9101' in launched,launched
            s.screenshot('shell-ui-returned')
            s.key('\x1b');s.wait('Filter: simulation')

            # Requeue during the preview must fail revalidation without entry.
            mode.write_text('requeued');count.write_text('0')
            s.key('o');s.wait('enter open shell');activate(s,'enter open shell')
            s.wait('Shell not opened');s.wait('allocation changed')
            assert opened.read_text().count('opened')==1
            s.resize(48,18);s.wait('esc back');s.screenshot('shell-ui-stale-48')
            activate(s,'esc back');s.wait('Filter: simulation')
            s.resize(120,36)

            # Cancelling a slow read-only preparation never resumes it later.
            mode.write_text('slow');count.write_text('0')
            s.key('o');s.wait('Checking your running allocation');s.key('\x1b')
            mode.write_text('ok');s.pump(2.3)
            assert 'Allocation shell' not in s.text and opened.read_text().count('opened')==1

            mode.write_text('exit-error');count.write_text('0')
            s.key('o');s.wait('enter open shell');activate(s,'enter open shell')
            s.wait('Shell ended');s.wait('Filter: simulation')
            assert opened.read_text().count('opened')==2
            s.close()
            for p in (root/'home').rglob('*'):
                if p.is_file():assert b'private-shell-output' not in p.read_bytes(),p
        except BaseException:s.screenshot('shell-ui-failure');print(s.text);s.abort();raise

        mode.write_text('ok');count.write_text('0')
        s=Session(['--host','perch-test.invalid','--no-config','--reduced-motion'],100,32,env=env)
        try:
            s.wait('● connected');s.wait('simulation-sweep')
            s.key('o');s.wait('enter open shell');activate(s,'enter open shell')
            deadline=time.monotonic()+10
            while b'PERCH_SHELL_READY' not in s.raw:
                s.pump();assert time.monotonic()<deadline,s.text
            child=int((root/'child-pid').read_text())
            os.kill(s.process.pid,signal.SIGTERM)
            deadline=time.monotonic()+10
            while s.process.poll() is None:
                s.pump();assert time.monotonic()<deadline,'termination stalled in foreground shell'
            s.pump(.2)
            try:os.kill(child,0)
            except ProcessLookupError:pass
            else:raise AssertionError('foreground child survived TUI termination')
            after=termios.tcgetattr(s.slave)
            pending=getattr(termios,'PENDIN',0)
            after[3]&=~pending;s.before[3]&=~pending
            assert after==s.before,'TTY modes were not restored'
        finally:s.abort()
    print('Shell UI: wide/48-column demo, node choice, mouse preview/open/back, real foreground handoff, pointer/mouse restoration, Ctrl-C, exact return context, requeue refusal, cancellation, error return, SIGTERM child/TTY cleanup and diagnostic exclusion passed. SSH was synthetic; no cluster actions.')


if __name__=='__main__':main()
