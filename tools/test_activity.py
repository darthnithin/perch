#!/usr/bin/env python3
"""Observed activity: real PTY rendering, exact-attempt navigation and controls."""
from test_terminal import Session
from test_mouse import click, target, wheel
from test_hover import env, move


def main():
    for width, height in [(140, 40), (80, 24), (48, 18)]:
        s = Session(['--demo', '--no-config', '--reduced-motion'], width, height, env=env('ghostty'))
        try:
            s.wait('parameter-sweep')
            s.key('/find embedding\r')
            s.key('/activity\r'); s.wait('No new activity yet.')
            s.screenshot('activity-empty-'+str(width))
            s.key('d'); s.wait('Finished'); s.wait('#9100_8')
            s.screenshot('activity-finished-'+str(width))
            move(s, *target(s, 'Finished'))
            assert 'Activity' in s.text, 'hover opened entry'
            click(s, *target(s, 'Finished')); s.wait('9100_8 · parameter-sweep')
            s.key('l'); s.wait('following'); s.key('f'); s.key('demo\r'); s.wait('Match 1 /')
            s.key('\x1b'); s.wait('Standard output'); assert 'Activity' not in s.text
            s.key('\x1b'); s.wait('Activity'); s.wait('Finished')
            s.key('d'); s.wait('of 5')
            s.key('\x1b[H'); s.wait('Time limit near')
            s.screenshot('activity-warnings-'+str(width))
            s.key('\r'); s.wait('9100_9 · parameter-sweep')
            s.key('\x1b'); s.wait('Activity')
            s.key('/cancel\r'); s.wait('Open an activity entry')
            assert 'Cancellation review' not in s.text
            s.key('\x1b[F'); s.wait('Finished')
            wheel(s, 10, 6, down=False); s.wait('Time limit near')
            s.key('\x1b'); s.wait('Filter: embedding')
            s.key('\x0b'); s.key('activity'); s.wait('Activity this session')
            s.key('\r'); s.wait('Observed this session')
            s.resize(48,18); s.wait('of 5')
            s.key('\x1b[F'); s.wait('Finished')
            s.resize(width,height); s.wait('Activity')
            s.key('3'); s.wait('Partitions')
            s.close()
        except BaseException:
            print(s.text); s.screenshot('activity-failure-'+str(width)); s.abort(); raise
    print('Activity PTY passed: startup silence, observed finish/time warnings, keyboard and pointer, filter return, hidden-action guard, palette, resize, terminal restoration.')


if __name__ == '__main__':
    main()
