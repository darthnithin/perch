#!/usr/bin/env python3
"""Responsive help, reverse navigation and context retention through a PTY."""
from test_terminal import Session
from test_mouse import wheel


def main():
    s = Session(['--demo', '--no-config', '--reduced-motion'], 120, 38)
    try:
        s.wait('parameter-sweep')
        s.key('/find parameter\r')
        s.key('j')
        s.wait('› 9100_10')
        s.key('?')
        s.wait('Work on Hummingbird')
        s.wait('Esc returns · filter kept')
        assert '/receipts' in s.text and '/module name/version' in s.text
        assert 'Make yourself at home' in s.text and 'Selected job' in s.text
        assert s.screen.display[5].index('Move around') < s.screen.display[5].index('Work on Hummingbird')
        s.screenshot('help-polished-wide')
        s.resize(80, 24)
        s.wait('Move around')
        s.screenshot('help-polished-medium')
        wheel(s, 8, 7)
        assert 'Move around' not in s.text, 'help wheel did not scroll'
        s.resize(48, 18)
        s.key('\x1b[F')
        s.wait('scenario')
        assert 'Quit and restore' in s.text
        s.screenshot('help-polished-compact-end')
        s.key('\x1b[H')
        s.wait('Move around')
        s.screenshot('help-polished-compact-top')
        s.resize(120, 38)
        s.wait('Work on Hummingbird')
        s.key('\x1b')
        s.wait('Filter: parameter')
        s.wait('› 9100_10')
        s.key('?')
        s.wait('Keyboard & commands')
        for content in ['Offline demo', 'Allocation', 'Recent history', 'Filter: parameter']:
            s.key('\x1b[Z')
            s.wait(content)
            assert 'Keyboard & commands' not in s.text
        s.wait('› 9100_10')
        s.key('?')
        s.wait('Keyboard & commands')
        s.key('l')
        s.wait('Standard output')
        assert 'Keyboard & commands' not in s.text
        s.key('\x1b')
        s.key('\t')
        s.wait('Recent history')
        s.close()
    except BaseException:
        print(s.text)
        s.screenshot('help-polish-failure')
        s.abort()
        raise
    print('Help PTY passed: wide/medium/minimum layouts, mouse wheel, Home/End, '
          'resize recovery, backward/forward tabs, retained filter/selection, terminal cleanup.')


if __name__ == '__main__':
    main()
