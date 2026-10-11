#!/usr/bin/env python3
"""Nautilus workspace in a real PTY with synthetic fixtures, no cluster access."""
from test_terminal import Session
from test_mouse import click, wheel


def main():
    s = Session(['nrp', 'ui', '--demo'], 120, 38)
    try:
        s.wait('Namespace Jobs')
        s.wait('indexed')
        s.wait('Jobs current')
        assert 'Failed' in s.text and 'Completing' in s.text
        assert s.screen.display[5].index('STATE') == s.screen.display[7].index('Failed')
        s.screenshot('nrp-jobs-wide')
        s.key('/find retrying\r')
        s.wait('1–1 of 1')
        s.key('\r')
        s.wait('retrying-old')
        s.wait('retrying-replacement')
        assert 'Active' in s.text and 'Failed Pods: 1' in s.text
        s.screenshot('nrp-job-detail')
        s.key('\x1b')
        s.wait('Filter: retrying')
        click(s, 12, 2)
        s.wait('Namespace Pods')
        s.key('/find retrying-replacement\r')
        s.wait('1–1 of 1')
        s.key('\r')
        s.wait('Current: Running')
        s.wait('Previous: Terminated')
        s.wait('OOMKilled')
        s.screenshot('nrp-pod-detail')
        s.resize(48, 18)
        s.key('\x1b[F')
        s.wait('Pod conditions')
        assert 'q quit' in s.screen.display[-1]
        s.screenshot('nrp-detail-compact')
        s.key('1')
        s.wait('Filter: retrying')
        s.wait('q quit')
        s.screenshot('nrp-jobs-compact')
        s.key('?')
        s.wait('Keyboard & commands')
        wheel(s, 8, 7)
        s.key('\x1b[F')
        s.wait('available yet')
        s.key('\x1b[Z')
        s.wait('Connection')
        s.key('\x1b[H')
        s.wait('Offline demo')
        s.resize(80, 24)
        s.wait('synthetic data')
        s.screenshot('nrp-connection-medium')
        s.key('c')
        s.wait('no sign-in needed')
        s.key('\x1b')
        s.wait('Filter: retrying')
        s.key('3')
        s.wait('collaborators')
        s.key('\x1b')
        s.wait('Filter: retrying')
        s.key('\x1b')
        s.wait('8 · Enter to inspect')
        s.resize(40, 12)
        s.wait('Resize to at least')
        s.resize(120, 38)
        s.wait('Namespace Jobs')
        s.close()
    except BaseException:
        print(s.text)
        s.screenshot('nrp-ui-failure')
        s.abort()
        raise
    s = Session(['nrp', 'ui', '--demo', '--no-mouse'], 80, 24)
    try:
        s.wait('Namespace Jobs')
        click(s, 12, 2)
        assert 'Namespace Jobs' in s.text
        s.key('2')
        s.wait('Namespace Pods')
        s.close()
    except BaseException:
        s.abort()
        raise
    print('NRP UI PTY passed: Job/Pod inspection, owner relationships, prior container '
          'failure, filters, keyboard/mouse navigation, scrolling, compact/resize, '
          'offline reconnect, no-mouse and terminal cleanup.')


if __name__ == '__main__':
    main()
