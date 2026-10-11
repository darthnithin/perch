#!/usr/bin/env python3
"""NRP clickable footer and contextual input through real PTYs; demo only."""
from test_terminal import Session
from test_mouse import target, click
from test_hover import move, env
from test_footer import footer, activate


def main():
    for width, height in [(120, 38), (48, 18)]:
        s = Session(['nrp', 'ui', '--demo'], width, height, env=env('ghostty'))
        try:
            s.wait('Filter workloads')
            click(s, *target(s, 'Filter workloads'))
            s.key('/find retrying')
            assert 'enter run' in s.screen.display[-1] and 'inspect' not in s.screen.display[-1]
            activate(s, 'enter run'); s.wait('1–1 of 1')
            move(s, *footer(s, 'enter inspect'))
            x, y = footer(s, 'enter inspect')
            assert s.screen.buffer[y][x].underscore, 'no footer hover feedback'
            assert s.raw[s.raw.rfind(b'\x1b]22;'):].startswith(b'\x1b]22;pointer\x07')
            s.screenshot(f'nrp-footer-jobs-{width}')
            activate(s, 'enter inspect'); s.wait('Object details')
            s.wait('Esc returns')
            activate(s, 'l logs'); s.wait('Choose a Pod')
            activate(s, '↓')
            activate(s, 'enter open'); s.wait('Containers')
            activate(s, 'enter open'); s.wait('Current instance')
            activate(s, 'p '); s.wait('Previous instance')
            assert 'Current instance' not in s.text, 'old output survived mode switch'
            s.wait('Snapshot · up to 256 KiB / 2000 lines')
            move(s, *footer(s, 'esc back'))
            s.screenshot(f'nrp-footer-logs-{width}')
            activate(s, 'esc back'); s.wait('Containers')
            activate(s, 'esc back'); s.wait('Choose a Pod')
            activate(s, 'esc back'); s.wait('Object details')
            activate(s, 'esc back'); s.wait('Filter: retrying'); s.wait('1–1 of 1')
            activate(s, '? help'); s.wait('Keyboard & commands')
            activate(s, 'esc back'); s.wait('Namespace Jobs')
            click(s, *target(s, '4 Connect'))
            assert 'c connect' in s.screen.display[-1]
            s.screenshot(f'nrp-footer-connection-{width}')
            activate(s, 'esc back'); s.wait('Namespace Jobs'); s.wait('Filter: retrying')
            # Assert the footer itself exits before close() checks restoration.
            activate(s, 'q quit')
            assert s.process.wait(timeout=5) == 0, 'footer quit did not exit'
            s.close()
        except BaseException:
            print(s.text); s.screenshot('nrp-footer-failure'); s.abort(); raise
    print('NRP footer PTY passed: focused command hints/submission, hover, inspect, Pod/container selection, current/previous logs, back-stack/filter preservation, help, connection return and click-to-quit cleanup at wide/minimum sizes.')


if __name__ == '__main__':
    main()
