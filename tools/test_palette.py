#!/usr/bin/env python3
"""Contextual command discovery through real keyboard and SGR mouse input."""
from test_terminal import Session
from test_mouse import target, click


def menu(s, query):
    s.key('\x0b')
    s.key(query)


def check(width, height):
    s = Session(['--demo', '--no-config', '--reduced-motion'], width, height)
    try:
        s.wait('parameter-sweep')
        s.key('3')
        s.wait('Allocation')
        menu(s, 'logs')
        s.wait('Choose a job in Jobs or History.')
        assert 'enter run' not in s.screen.display[-1]
        s.key('\r')
        click(s, *target(s, 'Read job output'))
        s.wait('Commands')
        s.wait('Choose a job')
        assert 'Standard output' not in s.text
        s.screenshot(f'palette-context-unavailable-{width}')
        s.key('\x1b')
        s.wait('Allocation')
        # A visible tab click can leave the menu and restore the jobs context.
        menu(s, 'logs')
        click(s, *target(s, 'Jobs'))
        s.wait('parameter-sweep')
        s.key('/find embedding\r')
        s.wait('9231')
        menu(s, 'shell')
        s.wait('Choose one running job or array task.')
        s.key('\r')
        s.wait('Commands')
        assert 'Open shell' not in s.text
        s.key('\x1b')
        s.key('/find parameter\r')
        s.key('j')
        s.wait('› 9100_10')
        menu(s, 'logs')
        s.wait('Selected job · 9100_10')
        assert 'enter run' in s.screen.display[-1]
        s.screenshot(f'palette-context-job-{width}')
        click(s, *target(s, 'Read job output'))
        s.wait('Standard output')
        s.key('\x1b')
        s.wait('› 9100_10')
        s.wait('Filter: parameter')
        menu(s, 'inspect')
        s.wait('Inspect selected job')
        s.key('\r')
        s.wait('Node')
        s.wait('9100_10')
        s.key('\x1b')
        menu(s, 'filter')
        s.key('\r')
        s.key('embedding\r')
        s.wait('Filter: embedding')
        s.wait('9231')
        menu(s, 'zzzz-no-match')
        s.wait('No matching commands.')
        assert 'enter run' not in s.screen.display[-1]
        s.key('\r')
        s.wait('No matching commands.')
        s.key('\x1b')
        s.key('\x1b')
        menu(s, '')
        # First selection stays visible with the whole shortcut at minimum size.
        s.wait('/run')
        s.screenshot(f'palette-context-menu-{width}')
        s.key('\x1b')
        s.close()
    except BaseException:
        print(s.text)
        s.screenshot(f'palette-context-failure-{width}')
        s.abort()
        raise


def main():
    for size in [(120, 38), (80, 24), (48, 18)]:
        check(*size)
    print('Command palette PTY: unavailable actions retain menu/context; keyboard '
          'and mouse agree; target IDs, pending-shell refusal, inspect/filter '
          'discovery, empty search, compact shortcuts and terminal restoration passed.')


if __name__ == '__main__':
    main()
