#!/usr/bin/env python3
"""Verify color settings against real PTY bytes with a private fake SSH host."""
import json
import os
import pathlib
import re
import subprocess
import sys
import tempfile

from test_failure_terminal import FAKE_SSH
from test_mouse import click, target
from test_terminal import ROOT, Session

RGB = re.compile(rb'\x1b\[[0-9;]*(?:38|48);2;')
INDEXED = re.compile(rb'\x1b\[[0-9;]*(?:38|48);5;')


def main():
    with tempfile.TemporaryDirectory(prefix='perch-colors-') as directory:
        root = pathlib.Path(directory)
        (root / 'state').write_text('ok')
        ssh = root / 'ssh'
        ssh.write_text('#!' + sys.executable + '\n' + FAKE_SSH)
        ssh.chmod(0o700)
        env = {**os.environ, 'HOME': str(root), 'XDG_CONFIG_HOME': str(root / 'config'),
               'PATH': str(root) + ':/usr/bin:/bin', 'TERM': 'xterm-256color',
               'COLORTERM': '', 'TERM_PROGRAM': '', 'TMUX': '',
               'PERCH_TEST_STATE': str(root / 'state'),
               'PERCH_TEST_FIXTURES': str(ROOT / 'testdata/hummingbird')}
        for key in ('NO_COLOR', 'CLICOLOR_FORCE', 'WT_SESSION', 'GOOGLE_CLOUD_SHELL'):
            env.pop(key, None)
        args = ['--host', 'perch-test.invalid', '--reduced-motion']
        s = Session(args, 100, 32, env=env)
        try:
            s.wait('connected')
            assert INDEXED.search(s.raw) and not RGB.search(s.raw), 'Auto did not use 256 colors'
            s.key('/settings\r')
            s.wait('Color mode')
            start = len(s.raw)
            click(s, *target(s, 'Color mode'))
            s.wait('True color')
            s.pump(.3)
            assert RGB.search(s.raw[start:]), 'preview did not emit RGB'
            s.screenshot('settings-truecolor-preview')
            start = len(s.raw)
            s.key('\x1b')
            s.pump(.3)
            assert INDEXED.search(s.raw[start:]), 'cancel did not restore 256 colors'
            s.key('/settings\r')
            click(s, *target(s, 'Color mode'))
            s.wait('True color')
            s.resize(48, 18)
            s.wait('Save defaults')
            s.screenshot('settings-color-mode-compact')
            click(s, *target(s, 'Save defaults'))
            s.wait('Appearance saved')
            s.close()
        except BaseException:
            s.abort()
            raise
        saved = subprocess.run([str(ROOT / 'bin/perch'), 'config', 'show', '--json'],
                               env=env, capture_output=True, text=True, check=True)
        assert json.loads(saved.stdout)['color_mode'] == 'truecolor'
        for extra, rgb in (([], True), (['--color-mode=256'], False),
                           (['--color-mode=auto'], False)):
            s = Session(args + extra, 100, 32, env=env)
            try:
                s.wait('connected')
                # Ignore the initial detection frame; force a new screen after startup.
                start = len(s.raw)
                s.key('/settings\r')
                s.wait('Color mode')
                s.pump(.3)
                assert bool(RGB.search(s.raw[start:])) == rgb, ('wrong persisted/override profile', extra)
                if not rgb:
                    assert INDEXED.search(s.raw[start:])
                s.close()
            except BaseException:
                s.abort()
                raise
        print('PASS: RGB preview, cancel, compact save, persistence and explicit overrides in real PTY output')


if __name__ == '__main__':
    main()
