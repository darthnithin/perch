#!/usr/bin/env python3
"""Nautilus hover via real mouse reports; no native pointer appearance claim."""
import os
import signal
from test_terminal import Session
from test_mouse import target, click
from test_hover import move, shape, env


def latest(s, name):
    sequence = b'\x1b]22;'+name.encode()+b'\x07'
    assert bytes(s.raw)[s.raw.rfind(b'\x1b]22;'):].startswith(sequence), (name, bytes(s.raw[-200:]))


def main():
    s=Session(['nrp','ui','--demo'],120,38,env=env('ghostty'))
    try:
        s.wait('Jobs current · Pods current')
        assert b'\x1b[?1003h' in s.raw and b'\x1b[?1004h' in s.raw
        before=s.text
        x,y=target(s,'2 Pods');move(s,x,y);latest(s,'pointer')
        assert before==s.text,'hover navigated or changed content'
        assert s.screen.buffer[y][x].underscore,'hover not visibly indicated'
        count=s.raw.count(b'\x1b]22;pointer');move(s,x+1,y)
        assert count==s.raw.count(b'\x1b]22;pointer'),'same shape re-emitted'
        s.screenshot('nrp-hover-tab')
        click(s,x,y);s.wait('Namespace Pods')
        before=s.text
        x,y=target(s,'retrying-replacement');move(s,x,y);latest(s,'pointer')
        assert s.text==before and s.screen.buffer[y][x].bg=='302923','row hover mutated selection or lacks highlight'
        s.screenshot('nrp-hover-pod')
        click(s,8,s.height-5) # clicking the row must restore keyboard focus
        click(s,x,y);s.wait('› retrying-replacement')
        s.key('l');s.wait('Containers · retrying-replacement');latest(s,'default')
        before=s.text;x,y=target(s,'log-helper');move(s,x,y);latest(s,'pointer')
        assert s.text==before and s.screen.buffer[y][x].bg=='302923','container hover changed choice or lacks highlight'
        s.screenshot('nrp-hover-container')
        click(s,8,s.height-5) # Enter after picker click must open logs, not submit input
        click(s,x,y);s.wait('› log-helper');s.key('\r');s.wait('log-helper · current')
        move(s,8,7);latest(s,'default')
        s.key('1');s.wait('Namespace Jobs');s.key('/find retrying\r');s.wait('1–1 of 1')
        s.resize(48,18);latest(s,'default')
        x,y=target(s,'retrying');move(s,x,y);latest(s,'pointer')
        s.screenshot('nrp-hover-compact')
        s.key('\x1b[O');latest(s,'default') # focus lost
        move(s,8,s.height-5);latest(s,'text')
        click(s,8,s.height-5);s.key('/pods\r');s.wait('Namespace Pods');latest(s,'default')
        move(s,12,2);latest(s,'pointer');s.resize(40,12);s.wait('Resize to at least');latest(s,'default')
        s.resize(80,24);s.wait('Namespace Pods');move(s,12,2);latest(s,'pointer');s.close()
        assert b'\x1b[?1003l' in s.raw and b'\x1b[?1004l' in s.raw
        assert s.raw.endswith(b'\x1b]22;default\x07'),'exit did not reset pointer'
    except BaseException:
        print(s.text);s.screenshot('nrp-hover-failure');s.abort();raise
    s=Session(['nrp','ui','--demo'],80,24,env=env('iTerm.app'))
    try:
        s.wait('Namespace Jobs');move(s,12,2);latest(s,'hand2')
        os.kill(s.process.pid,signal.SIGTERM);s.process.wait(timeout=10);s.pump(.2)
        latest(s,'xterm')
        assert s.raw.rfind(b'\x1b]22;xterm\x07') > s.raw.rfind(b'\x1b[?1049l'),'signal reset before renderer shutdown'
        s.abort()
    except BaseException:s.abort();raise
    for settings,args in [(env('Apple_Terminal'),[]),(env('ghostty'),['--no-mouse']),({**env('ghostty'),'TMUX':'synthetic'},[])]:
        s=Session(['nrp','ui','--demo',*args],80,24,env=settings)
        try:
            s.wait('Namespace Jobs');move(s,12,2)
            if args:
                assert b'\x1b[?1003h' not in s.raw
                assert not s.screen.buffer[2][12].underscore
            else:
                assert s.screen.buffer[2][12].underscore,'fallback highlight missing'
            s.close();assert b'\x1b]22;' not in s.raw,'unsupported or disabled OSC'
        except BaseException:s.abort();raise
    print('NRP hover PTY passed: CSS/X11, tabs/rows/container targets, no selection or read on hover, compact layout, focus/resize/keyboard reset, no-mouse/unsupported/multiplexer fallback, clean/signal exit.')


if __name__=='__main__':main()
