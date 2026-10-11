#!/usr/bin/env python3
"""Clickable footer round trips through real SGR reports; demo only."""
from test_terminal import Session
from test_mouse import target, click
from test_hover import move, env


def footer(s, label):
    x = s.screen.display[-1].find(label)
    assert x >= 0, (label, s.screen.display[-1])
    return x, s.height-1


def activate(s, label):
    click(s, *footer(s, label))


def main():
    for width, height in [(140, 40), (48, 18)]:
        s = Session(['--demo', '--no-config', '--reduced-motion'], width, height, env=env('ghostty'))
        try:
            s.wait('parameter-sweep')
            click(s, *target(s, 'Find a command'))
            s.key('/find parameter')
            activate(s, 'enter run')
            s.wait('Filter: parameter')
            activate(s, '↓')
            s.wait('9100_10')
            move(s, *footer(s, 'enter inspect'))
            x, y = footer(s, 'enter inspect')
            assert s.screen.buffer[y][x].underscore, 'footer hover lacks visible feedback'
            assert s.raw[s.raw.rfind(b'\x1b]22;'):].startswith(b'\x1b]22;pointer\x07'), 'footer did not request hand pointer'
            s.screenshot(f'footer-jobs-{width}')
            activate(s, 'enter inspect')
            s.wait('9100_10 · parameter-sweep')
            move(s, *footer(s, 'esc back'))
            s.screenshot(f'footer-inspector-{width}')
            activate(s, 'l output')
            s.wait('stdout')
            activate(s, 'e stdout/stderr')
            s.wait('stderr')
            activate(s, 'esc back')
            s.wait('› 9100_10')
            s.wait('Filter: parameter')
            click(s, *target(s, 'Find a command'))
            s.key('/help')
            activate(s, 'enter run')
            s.wait('Move around')
            activate(s, 'esc back')
            s.wait('› 9100_10')
            s.wait('Filter: parameter')
            s.close()
        except BaseException:
            s.screenshot('footer-failure'); print(s.text); s.abort(); raise

    s = Session(['--demo', '--no-config', '--reduced-motion'], 48, 24, env=env('ghostty'))
    try:
        s.wait('parameter-sweep')
        click(s, *target(s, 'Find a command')); s.key('/run'); activate(s, 'enter run')
        s.wait('Prepare a job')
        s.key('/scratch/example')
        activate(s, 'tab next'); s.key('printf hello')
        activate(s, 'enter review'); s.wait('Review your job')
        activate(s, 'v validate'); s.wait('Validated')
        s.screenshot('footer-launch-compact')
        # This is the existing demo submission flow. No remote execution.
        activate(s, 'ctrl+s submit'); s.wait('Demo complete')
        activate(s, 'esc close'); s.wait('parameter-sweep')
        s.close()
    except BaseException:
        s.screenshot('footer-launch-failure'); print(s.text); s.abort(); raise
    print('Footer PTY passed: wide/minimum selection, hover, inspect, output, streams, back, retained filter, command submission/help and compact demo launch review/validate/submit/close.')


if __name__ == '__main__':
    main()
