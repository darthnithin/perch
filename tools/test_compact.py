#!/usr/bin/env python3
"""Compact tabs, scroll discovery and resize recovery through a real PTY."""
from test_terminal import Session
from test_mouse import target, click, wheel


def main():
    s = Session(['--demo', '--no-config', '--reduced-motion'], 48, 18)
    try:
        s.wait('Your jobs')
        for name, content in [('History', 'Recent history'),
                              ('Partitions', 'Allocation'),
                              ('Connection', 'Offline demo'), ('Jobs', 'Your jobs')]:
            x, y = target(s, name)
            assert y == 2, 'tab label was not visible in navigation'
            click(s, x + len(name) - 1, y)
            s.wait(content)
        click(s, *target(s, 'Partitions'))
        s.wait('1–4 /')
        s.wait('measured utilization.')
        s.screenshot('compact-partitions-after')
        s.key('\x1b[F')
        s.wait('↑')
        assert '↓' not in s.screen.display[9], 'end still advertises more below'
        s.screenshot('compact-partitions-bottom')
        # Growing only the height leaves the text unchanged. The old offset
        # must still be clamped so no partition disappears above the viewport.
        s.resize(48, 60)
        s.wait('128x24')
        s.wait('256x44')
        assert '1–' not in s.text, 'scroll counter shown when all content fits'
        s.resize(48, 18)
        click(s, *target(s, 'Connection'))
        s.wait('Offline demo')
        s.wait('1–4 /')
        assert 'q quit' in s.screen.display[-1], 'compact recovery footer clipped'
        s.screenshot('compact-connection-after')
        wheel(s, 8, 6)
        s.wait('↑')
        s.screenshot('compact-connection-scrolled')
        s.resize(100, 32)
        s.wait('Offline demo')
        s.screenshot('connection-medium-after')
        s.key('1')
        s.wait('parameter-sweep')
        s.close()
    except BaseException:
        s.screenshot('compact-failure')
        print(s.text)
        s.abort()
        raise
    print('Compact PTY: all four tab clicks, word wrapping, scroll indicators, '
          'End, mouse wheel, height-only resize recovery and terminal cleanup passed.')


if __name__ == '__main__':
    main()
