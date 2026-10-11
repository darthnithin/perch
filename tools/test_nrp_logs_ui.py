#!/usr/bin/env python3
"""Job-to-Pod-to-container log navigation and mode isolation in a real PTY."""
from test_terminal import Session
from test_mouse import click, wheel


def main():
    s = Session(['nrp', 'ui', '--demo'], 120, 38)
    try:
        s.wait('Namespace Jobs')
        s.wait('retrying')
        s.key('/find retrying\r')
        s.wait('1–1 of 1')
        s.key('\r')
        s.wait('Owned Pods')
        s.key('l')
        s.wait('Choose a Pod')
        s.wait('retrying-old')
        s.screenshot('nrp-log-pods')
        click(s, 8, 8)
        s.key('\r')
        s.wait('Containers · retrying-replacement')
        s.wait('compute · app')
        s.wait('log-helper · sidecar')
        s.wait('prepare · init')
        s.screenshot('nrp-log-containers')
        s.key('\r')
        s.wait('Current instance of compute')
        s.wait('current · snapshot')
        s.screenshot('nrp-log-current')
        s.key('p')
        s.wait('Previous instance of compute')
        s.wait('previous · snapshot')
        assert 'Current instance' not in s.text
        s.resize(48, 18)
        s.wait('previous · snapshot')
        s.key('\x1b[F')
        s.wait('termination state')
        assert 'q quit' in s.screen.display[-1]
        s.screenshot('nrp-log-previous-compact')
        wheel(s, 8, 7, down=False)
        s.key('\x1b[H')
        s.wait('Previous instance')
        s.resize(80, 24)
        s.key('?')
        s.wait('Keyboard & commands')
        s.key('\x1b')
        s.wait('previous · snapshot')
        s.key('\x1b')
        s.wait('Containers')
        # Results is waiting for an image; its log read must fail explicitly.
        s.key('j\r')
        s.wait('read failed')
        s.wait('Unable to read container logs')
        assert 'Previous instance' not in s.text and 'Current instance' not in s.text
        s.screenshot('nrp-log-unavailable')
        s.key('\x1b')
        s.wait('Containers')
        s.key('\x1b')
        s.wait('Choose a Pod')
        assert '› retrying-replacement' in s.text
        s.key('\x1b')
        s.wait('Object details')
        s.wait('retrying')
        s.key('\x1b')
        s.wait('Filter: retrying')
        s.key('2')
        s.key('/find retrying-replacement\r')
        s.key('/logs\r')
        s.wait('Containers')
        s.key('\r')
        s.wait('Current instance of compute')
        s.key('1')
        s.wait('Namespace Jobs')
        s.wait('Filter: retrying')
        assert 'Current instance' not in s.text
        s.close()
    except BaseException:
        print(s.text)
        s.screenshot('nrp-log-ui-failure')
        s.abort()
        raise
    print('NRP logs PTY passed: owned Pod and explicit container pickers, mouse selection, '
          'current/previous isolation, unavailable output, scrolling/resize/help, '
          'back navigation, retained workload filters and terminal cleanup.')


if __name__ == '__main__':
    main()
