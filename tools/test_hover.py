#!/usr/bin/env python3
"""Actual mouse-motion reports and OSC 22 output; emulator is not native acceptance."""
import os
import signal
import sys
import tempfile
from pathlib import Path
from test_failure_terminal import FAKE_SSH
from test_terminal import ROOT
from test_terminal import Session
from test_mouse import target, click


def move(s,x,y): s.key(f'\x1b[<35;{x+1};{y+1}M')
def shape(s,name):
    expected=b'\x1b]22;'+name.encode()+b'\x07'
    assert expected in s.raw, (name, bytes(s.raw[-300:]))
def env(program): return {'TERM_PROGRAM':program,'TMUX':'','STY':''}


def main():
    s=Session(['--demo','--no-config','--reduced-motion'],100,40,env=env('ghostty'))
    try:
        s.wait('parameter-sweep')
        assert b'\x1b[?1003h' in s.raw
        shape(s,'default')
        assert not any(s.screen.buffer[1][x].underscore for x in range(s.width)), 'keyboard-mode query invented a title underline'
        before=s.text
        move(s,*target(s,'2  History'));shape(s,'pointer')
        assert s.text==before, 'hover changed visible text or navigated'
        count=s.raw.count(b'\x1b]22;pointer')
        x,y=target(s,'2  History');move(s,x+1,y)
        assert s.raw.count(b'\x1b]22;pointer')==count,'redundant pointer output'
        s.screenshot('hover-tab')
        click(s,x,y);s.wait('Recent history')
        move(s,*target(s,'Find a command'));shape(s,'text')
        move(s,0,0);assert s.raw.rfind(b'\x1b]22;default')>s.raw.rfind(b'\x1b]22;text')
        move(s,*target(s,'1  Jobs'));shape(s,'pointer')
        s.resize(48,24)
        assert s.raw.rfind(b'\x1b]22;default')>s.raw.rfind(b'\x1b]22;pointer')
        move(s,*target(s,'1 Jobs'));click(s,*target(s,'1 Jobs'));s.wait('active records')
        move(s,*target(s,'9100_10'));s.screenshot('hover-job-compact')
        # Keyboard navigation dismisses hover, rather than leaving an unrelated hand.
        s.key('?');s.wait('Move around')
        assert s.raw.rfind(b'\x1b]22;default')>s.raw.rfind(b'\x1b]22;pointer')
        s.key('\x1b');move(s,*target(s,'2 History'));shape(s,'pointer')
        s.close()
        assert b'\x1b[?1003l' in s.raw
        assert s.raw.endswith(b'\x1b]22;default\x07'), 'pointer not reset after terminal shutdown'
    except BaseException: print(s.text);s.abort();raise
    # iTerm uses X11 names, not CSS; SIGTERM also reaches final reset.
    s=Session(['--demo','--no-config','--reduced-motion'],80,24,env=env('iTerm.app'))
    try:
        s.wait('parameter-sweep');move(s,*target(s,'2  History'));shape(s,'hand2')
        os.kill(s.process.pid,signal.SIGTERM);s.process.wait(timeout=10);s.pump(.2)
        assert s.raw.endswith(b'\x1b]22;xterm\x07'),'signal exit did not reset iTerm pointer'
        s.abort()
    except BaseException: s.abort();raise
    for program,args in [('Apple_Terminal',[]),('ghostty',['--no-mouse'])]:
        s=Session(['--demo','--no-config','--reduced-motion',*args],80,24,env=env(program))
        try:
            s.wait('parameter-sweep');move(s,*target(s,'2  History'));s.close()
            assert b'\x1b]22;' not in s.raw,'unsupported/disabled pointer emitted OSC 22'
        except BaseException:s.abort();raise
    with tempfile.TemporaryDirectory(prefix='perch-hover-reconnect-') as tmp:
        root=Path(tmp);state=root/'state';state.write_text('ok')
        ssh=root/'ssh';ssh.write_text('#!'+sys.executable+'\n'+FAKE_SSH.replace('if sys.argv[-1] == "true": sys.exit(0)', 'if sys.argv[-1] == "true": print("PERCH_LOGIN_STARTED", flush=True); sys.exit(0)'));ssh.chmod(0o700)
        settings={**env('ghostty'),'PATH':str(root)+':/usr/bin:/bin','PERCH_TEST_STATE':str(state),
                  'PERCH_TEST_FIXTURES':str(ROOT/'testdata/hummingbird')}
        s=Session(['--host','perch-test.invalid','--no-config','--reduced-motion'],100,40,env=settings)
        try:
            s.wait('● connected');move(s,*target(s,'2  History'));shape(s,'pointer')
            start=len(s.raw);s.key('c');s.wait('● connected');s.pump(.5)
            output=bytes(s.raw[start:])
            assert b'\x1b[?1049l' in output and b'\x1b[?1049h' in output,'reconnect did not suspend/resume'
            assert output.index(b'\x1b]22;default\x07') < output.index(b'PERCH_LOGIN_STARTED'), 'SSH inherited hand cursor'
            move(s,*target(s,'2  History'));shape(s,'pointer');s.close()
        except BaseException:s.abort();raise
    print('Hover PTY passed: CSS/X11 vocabularies, nonmutating hover/click, deduplication, keyboard/resize, unsupported/no-mouse fallback, clean and signal exit reset.')

if __name__=='__main__':main()
