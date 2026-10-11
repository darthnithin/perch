#!/usr/bin/env python3
"""Selected-job action discovery, keyboard/pointer and review-only activation."""
import argparse
from pathlib import Path
from test_terminal import Session
from test_mouse import click, target
from test_hover import move, env
from test_footer import activate, footer


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', type=Path)
    args = parser.parse_args()
    for width, height in [(140, 40), (80, 24), (48, 18)]:
        s = Session(['--demo', '--no-config', '--reduced-motion'], width, height, env=env('ghostty'), binary=args.binary)
        try:
            s.wait('parameter-sweep')
            s.key('/find parameter\r'); s.key('j'); s.wait('› 9100_10')
            move(s, *footer(s, 'a actions'))
            assert 'Job actions' not in s.text
            activate(s, 'a actions'); s.wait('Job actions · #9100_10')
            s.wait('Read output')
            assert 'Prepare a new job' not in s.text
            assert 'Find a command' not in s.text
            if width == 140:
                s.resize(48, 18); s.wait('Resources and efficiency')
                s.key('\x1b[B' * 7); s.wait('Cancel whole array')
                s.screenshot('job-actions-resized-last')
                s.key('\x1b[A' * 7); s.wait('Read output')
                s.resize(width, height); s.wait('Job actions · #9100_10')
            s.screenshot('job-actions-menu-'+str(width))
            click(s, *target(s, 'Read output')); s.wait('Standard output')
            s.key('\x1b[H'); s.wait('paused')
            s.key('a'); s.wait('Job actions'); s.key('\x1b'); s.wait('paused')
            s.key('\x1b'); s.wait('› 9100_10'); s.wait('Filter: parameter')
            activate(s, 'a actions'); s.key('cancel whole'); s.wait('Cancel whole array')
            s.screenshot('job-actions-array-'+str(width))
            click(s, *target(s, 'Cancel whole array')); s.wait('ctrl+x cancel array')
            assert 'Demo · no cancellation sent' not in s.text, 'menu confirmed mutation'
            s.key('\r'); s.wait('Cancellation review'); s.key('\x1b')
            s.wait('› 9100_10'); s.wait('Filter: parameter')
            s.key('/find embedding\r'); s.key('/actions\r'); s.wait('Job actions · #9231')
            s.key('shell'); s.wait('Choose one running job or array task.')
            s.key('\r'); click(s, *target(s, 'Open a shell'))
            assert 'Allocation shell' not in s.text
            s.screenshot('job-actions-unavailable-'+str(width))
            s.key('\x1b'); s.key('\x0b'); s.wait('Commands'); s.wait('Prepare a new job')
            s.key('\x1b'); s.key('3'); s.key('a'); s.wait('Choose a job in Jobs or History.')
            assert 'Job actions' not in s.text
            s.close()
        except BaseException:
            print(s.text); s.screenshot('job-actions-failure-'+str(width)); s.abort(); raise
    print('Job actions PTY passed: wide/compact discoverability, pointer hover/click, explicit job scope, output and paused-scroll return, array review without mutation, unavailable actions, global palette and terminal cleanup.')


if __name__ == '__main__':
    main()
