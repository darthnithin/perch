#!/usr/bin/env python3
"""Send real SGR mouse input through Perch's PTY at wide and compact sizes."""
import json
from test_terminal import Session, OUT


def target(s, text):
    for y, line in enumerate(s.screen.display):
        x = line.find(text)
        if x >= 0:
            return x, y
    raise AssertionError(f'Missing pointer target {text!r}:\n{s.text}')


def click(s, x, y):
    s.key(f'\x1b[<0;{x+1};{y+1}M\x1b[<0;{x+1};{y+1}m')


def wheel(s, x, y, down=True):
    s.key(f'\x1b[<{65 if down else 64};{x+1};{y+1}M')


def main():
    s = Session(['--demo', '--reduced-motion'], 140, 44)
    try:
        s.wait('parameter-sweep')
        assert b'\x1b[?1003h' in s.raw, 'cell-motion capture not enabled'
        click(s, *target(s, '9100_10'))
        s.wait('› 9100_10')
        click(s, *target(s, '2  History'))
        s.wait('Recent history')
        click(s, *target(s, '3  Partitions'))
        s.wait('Allocation is scheduler capacity')
        click(s, *target(s, '4  Connection'))
        s.wait('Connection')
        click(s, *target(s, '1  Jobs'))
        s.wait('› 9100_10')
        wheel(s, *target(s, '› 9100_10'))
        s.wait('› 9231')
        click(s, *target(s, 'Find a command'))
        s.key('/find embedding\r')
        s.wait('Filter: embedding')
        assert 'parameter-sweep' not in s.text
        s.key('\x1b')
        s.resize(80, 24)
        click(s, *target(s, '9100_10'))
        s.wait('› 9100_10')
        s.key('\r')
        s.wait('9100_10')
        s.key('\x1b'); s.key('?')
        s.wait('Move around')
        before = s.text
        wheel(s, 8, 7)
        assert s.text != before and 'Move around' not in s.text, 'help did not scroll'
        s.key('\x1b'); s.key('\x0b')
        s.wait('Type to find a command')
        wheel(s, 8, 7)
        click(s, *target(s, 'History'))
        s.wait('Recent history')
        # Smallest supported size still retains row context and record footer.
        click(s, *target(s, '1  Jobs'))
        s.resize(48, 18)
        s.wait('active records')
        wheel(s, *target(s, '› 9100_10'))
        s.wait('› 9231')
        s.screenshot('mouse-minimum')
        s.resize(140, 44)
        s.wait('My jobs')
        s.screenshot('mouse-wide')
        s.close()
        assert b'\x1b[?1003l' in s.raw, 'mouse capture not disabled on exit'
        assert b'\x1b[?1006l' in s.raw, 'SGR mouse mode not disabled on exit'
    except BaseException:
        s.screenshot('mouse-failure'); s.abort(); raise

    s = Session(['--demo', '--no-mouse', '--reduced-motion'], 80, 24)
    try:
        s.wait('parameter-sweep')
        assert b'\x1b[?1002h' not in s.raw and b'\x1b[?1003h' not in s.raw
        s.key('2'); s.wait('Recent history')
        s.close()
    except BaseException:
        s.abort(); raise
    result = {'mouse': 'passed', 'checks': ['SGR tab and job clicks', 'wheel selects jobs',
        'command input focus and execution', 'resize targets', 'help scrolling',
        'palette wheel and click', '48x18 rows', '--no-mouse', 'mouse and terminal cleanup'],
        'scope': 'Real PTY input and output; native Terminal.app mouse interaction still needs user review.'}
    (OUT/'mouse-results.json').write_text(json.dumps(result, indent=2)+'\n')
    print(json.dumps(result, indent=2))

if __name__ == '__main__':
    main()
