#!/usr/bin/env python3
"""Reports preserve a scrolled cluster panel through actual PTY input."""
from test_terminal import Session
from test_footer import activate


def main():
    for tab in ('3', '4'):
        for width in (48, 80, 140):
            s = Session(['--demo', '--no-config', '--reduced-motion'], width, 18)
            try:
                s.wait('parameter-sweep')
                s.key(tab)
                s.key('\x1b[F')
                before = s.screen.display[5:9]
                assert '↑' in s.screen.display[9], s.text
                if tab == '3':
                    activate(s, 'n nodes')
                else:
                    s.key('/nodes\r')
                s.wait('node-06')
                activate(s, 'r refresh')
                s.wait('node-06')
                s.resize(80 if width == 48 else 48, 18)
                s.wait('Node allocations')
                # A report-to-report transition keeps the original caller.
                s.key('/priority\r')
                s.wait('Queue priority')
                s.resize(width, 18)
                activate(s, 'esc back')
                assert s.screen.display[5:9] == before, s.text
                s.screenshot(f'report-return-tab{tab}-{width}')
                if tab == '3':
                    # Return at a different width as well, retaining GPU context.
                    activate(s, 'n nodes')
                    s.wait('node-06')
                    s.resize(48, 18)
                    s.key('\x1b')
                    s.wait('gpu:p100:4')
                s.close()
            except BaseException:
                print(s.text)
                s.screenshot('report-return-failure')
                s.abort()
                raise
    print('Report return PTY passed: Partitions/Connection scroll, mouse entry/back, refresh, chained reports, resize and cleanup at 48/80/140 columns.')


if __name__ == '__main__':
    main()
