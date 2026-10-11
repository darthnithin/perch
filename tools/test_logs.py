#!/usr/bin/env python3
"""Log reading/follow mode, wrapped scroll position and script navigation."""
from test_terminal import Session
from test_mouse import wheel


def main():
    s = Session(['--demo', '--no-config', '--reduced-motion'], 80, 24)
    try:
        s.wait('parameter-sweep')
        s.key('l')
        s.wait('following')
        s.wait('up to 256 KiB retained')
        s.wait('Waiting for the next batch')
        s.screenshot('logs-follow-medium')
        s.key('\x1b[H')
        s.wait('paused · End latest')
        s.wait('[demo]')
        s.screenshot('logs-paused-medium')
        s.resize(140, 44)
        s.wait('paused · End latest')
        s.resize(48, 18)
        s.wait('paused · End latest')
        s.screenshot('logs-paused-compact')
        s.key('\x1b[F')
        s.wait('following')
        s.wait('next batch')
        s.screenshot('logs-follow-compact')
        s.resize(80, 24)
        s.wait('following')
        s.wait('next batch')
        wheel(s, 8, 8, down=False)
        s.wait('paused · End latest')
        for _ in range(8):
            wheel(s, 8, 8)
        s.wait('following')
        s.key('e')
        s.wait('Standard error')
        s.wait('following')
        s.key('s')
        s.wait('Submitted script')
        s.wait('#!/bin/bash')
        assert 'following' not in s.text and 'paused' not in s.text
        s.resize(48, 18)
        s.wait('#!/bin/bash')
        assert '↓' in s.text, 'compact script has no scroll indicator'
        assert 'Find a command' not in s.text, 'inactive input occupies reading space'
        s.screenshot('script-scroll-compact')
        s.key('\x1b[F')
        s.wait('python train.py')
        s.key('\x1b')
        s.wait('parameter-sweep')
        s.close()
    except BaseException:
        print(s.text)
        s.screenshot('logs-polish-failure')
        s.abort()
        raise
    print('Logs PTY passed: initial follow, Home pauses, End resumes, resize '
          'retains intent, wheel pause/resume, stderr reset, script positions, cleanup.')


if __name__ == '__main__':
    main()
