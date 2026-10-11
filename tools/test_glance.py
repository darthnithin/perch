#!/usr/bin/env python3
"""Real PTY inspector entry, bounded summary, resizing and scrolling."""
from test_terminal import Session
from test_mouse import target, click, wheel


def main():
    s=Session(['--demo','--reduced-motion'],140,40)
    try:
        s.wait('8 / 40 observed tasks finished')
        s.wait('7 succeeded');s.wait('1 failed');s.wait('28 queued')
        s.wait('full details')
        s.screenshot('glance-running-wide')
        click(s,*target(s,'full details'))
        s.wait('Time limit');s.wait('↑↓ scroll')
        s.resize(48,18)
        s.wait('parameter-sweep')
        before=s.text
        wheel(s,8,6)
        assert s.text!=before,'compact inspector did not scroll'
        s.key('\x1b[F');s.wait('resources')
        s.screenshot('inspector-minimum-scrolled')
        s.key('\x1b');s.resize(140,40)
        s.wait('› 9100_11');s.wait('full details')
        s.key('/job 9231\r');s.wait('Waiting for priority')
        s.key('\x1b');s.wait('› 9231');s.wait('full details')
        s.screenshot('glance-queued-wide')
        # The side panel remains actionable at the minimum supported height.
        s.resize(96,18);s.wait('full details')
        s.screenshot('glance-short')
        click(s,*target(s,'full details'));s.wait('↑↓ scroll')
        s.key('\x1b');s.key('2');s.resize(140,40)
        s.wait('Recent history');s.wait('full details')
        s.screenshot('glance-history-wide')
        s.close()
    except BaseException:
        s.screenshot('glance-failure');print(s.text);s.abort();raise
    s=Session(['--demo','--reduced-motion','--no-color'],140,40)
    try:
        s.wait('8 / 40 observed tasks finished')
        s.wait('7 succeeded');s.wait('1 failed');s.wait('28 queued')
        s.screenshot('array-progress-no-color');s.close()
    except BaseException:
        s.screenshot('array-progress-no-color-failure');print(s.text);s.abort();raise
    print('Glance PTY: priority status/progress, clickable inspector, 48x18 scrolling, 96x18 entry and selection-preserving return passed.')

if __name__=='__main__':main()
