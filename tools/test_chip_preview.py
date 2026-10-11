#!/usr/bin/env python3
"""Offline animation review with actual PTY playback; emulator images only."""
import re

from test_terminal import ROOT, Session


def frames(s):
    return {state: int(frame) for state, frame in re.findall(
        r'(idle|running|queued|complete|attention|reconnecting) #(\d+)', s.text)}


def main():
    s = Session(['--static'], 120, 34, binary=ROOT/'bin/chip-preview')
    try:
        s.wait('paused')
        assert len(frames(s)) == 6 and set(frames(s).values()) == {0}
        s.key('\x1b[C')
        s.wait('running #01')
        assert set(frames(s).values()) == {1}
        s.screenshot('chip-cadence-aligned')
        s.key(' ')
        s.wait('playing')
        s.pump(.4)
        s.key(' ')
        s.wait('paused')
        observed = frames(s)
        assert 1 < observed['idle'] < 32, observed
        assert 0 <= (observed['running']-1) - 4*(observed['idle']-1) < 4, observed
        assert len({v for k, v in observed.items() if k != 'running'}) == 1
        s.pump(.2)
        assert frames(s) == observed, 'paused clocks continued'
        s.screenshot('chip-cadence-playing-paused')
        s.key('\x1b[D')
        s.pump(.1)
        assert len(set(frames(s).values())) == 1, 'manual step did not realign'
        s.close()
    except BaseException:
        print(s.text)
        s.abort()
        raise
    s = Session(['--sample', '--state', 'running'], 48, 22, binary=ROOT/'bin/chip-preview')
    try:
        s.wait('fixed pose')
        assert '20 ms' in s.text
        s.key(' ')
        s.wait('playing')
        s.pump(.2)
        s.key(' ')
        s.wait('fixed pose')
        assert frames(s)['running'] > 0
        s.screenshot('chip-cadence-running-sample')
        s.close()
    except BaseException:
        print(s.text)
        s.abort()
        raise
    print('Chip preview PTY passed: all-state relative cadence, pause, aligned stepping, '
          'single running sample, and terminal restoration. Native font acceptance remains separate.')


if __name__ == '__main__':
    main()
