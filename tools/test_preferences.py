#!/usr/bin/env python3
"""Persist preferences across real processes with isolated HOME and fake SSH."""
import json
import os
import pathlib
import subprocess
import sys
import tempfile
import time

from test_failure_terminal import FAKE_SSH
from test_mouse import click, target
from PIL import Image
from test_terminal import ROOT, OUT, Session


def main():
    with tempfile.TemporaryDirectory(prefix='perch-preferences-') as directory:
        root = pathlib.Path(directory)
        state = root / 'state'
        state.write_text('ok')
        ssh = root / 'ssh'
        ssh.write_text('#!' + sys.executable + '\n' + FAKE_SSH)
        ssh.chmod(0o700)
        fixtures = root / 'fixtures'
        fixtures.mkdir()
        for name in ('queue', 'accounting', 'partitions'):
            data = json.loads((ROOT / 'testdata/hummingbird' / (name + '.json')).read_text())
            if name == 'queue':
                now = int(time.time())
                for job in data['jobs']:
                    for field, seconds in [('submit_time', -3600), ('eligible_time', -3600), ('start_time', -900), ('end_time', 6300)]:
                        job[field] = {'set': True, 'infinite': False, 'number': now + seconds}
            (fixtures / (name + '.json')).write_text(json.dumps(data))
        env = {**os.environ, 'HOME': str(root), 'XDG_CONFIG_HOME': str(root / 'config'),
               'PATH': str(root) + ':/usr/bin:/bin', 'TERM': 'xterm-256color', 'COLORTERM': 'truecolor',
               'PERCH_TEST_STATE': str(state), 'PERCH_TEST_FIXTURES': str(fixtures)}

        def cli(*args, success=True):
            result = subprocess.run([str(ROOT / 'bin/perch'), *args], env=env, text=True,
                                    capture_output=True, timeout=10)
            assert (result.returncode == 0) == success, (args, result.stdout, result.stderr)
            return result

        path = pathlib.Path(cli('config', 'path').stdout.strip())
        cli('config', 'show')
        assert not path.parent.exists(), 'read-only config created state'
        for key, value in [('host', 'perch-test.invalid'), ('theme', 'graphite'),
                           ('mouse', 'false'), ('reduced-motion', 'true')]:
            cli('config', 'set', key, value)
        saved = json.loads(cli('config', 'show', '--json').stdout)
        assert saved['host'] == 'perch-test.invalid' and saved['mouse'] is False
        assert path.stat().st_mode & 0o777 == 0o600
        assert path.parent.stat().st_mode & 0o777 == 0o700
        assert json.loads(cli('jobs', '--json').stdout)['data'], 'saved CLI destination did not return jobs'
        result = json.loads(cli('run', '--workdir', '/tmp', '--json', '--', 'true').stdout)
        assert result['host'] == 'perch-test.invalid' and result['mode'] == 'preview'

        s = Session([], 120, 36, env=env)
        try:
            s.wait('● connected')
            s.wait('simulation-sweep')
            assert b'\x1b[?1002h' not in s.raw and b'\x1b[?1003h' not in s.raw, 'saved mouse preference ignored'
            assert b'2;238;183;153m' in s.raw, 'saved graphite theme ignored'
            s.screenshot('preferences-saved-graphite')
            assert Image.open(OUT / 'preferences-saved-graphite.png').getpixel((0, 0)) == (24, 28, 32), 'emulator ignored graphite canvas'
            s.close()
        except BaseException:
            s.abort()
            raise
        s = Session(['--theme=warm', '--no-mouse=false'], 120, 36, env=env)
        try:
            s.wait('● connected')
            s.wait('simulation-sweep')
            assert b'\x1b[?1003h' in s.raw, 'explicit false did not override saved mouse preference'
            assert b'2;230;173;143m' in s.raw, 'explicit theme did not override saved value'
            s.screenshot('preferences-warm-override')
            s.close()
        except BaseException:
            s.abort()
            raise
        assert json.loads(cli('config', 'show', '--json').stdout) == saved, 'launch flags silently persisted'
        s = Session(['--theme=warm', '--no-mouse=false'], 120, 36, env=env)
        try:
            s.wait('● connected')
            s.key('/settings\r')
            s.wait('Make yourself at home')
            s.screenshot('settings-warm-preview')
            click(s, *target(s, 'Palette'))
            click(s, *target(s, 'Chip motion'))
            s.wait('graphite')
            s.wait('Animated')
            def chip_region():
                return '\n'.join(line[65:] for line in s.screen.display[7:17])
            observed_frames = set()
            for _ in range(6):
                s.pump(.09)
                observed_frames.add(chip_region())
            assert len(observed_frames) > 1, 'Chip preview did not animate after enabling motion'
            click(s, *target(s, 'Chip motion'))
            s.wait('Still')
            frozen = chip_region()
            s.pump(.3)
            assert chip_region() == frozen, 'reduced motion did not freeze the preview'
            click(s, *target(s, 'Chip motion'))
            s.wait('Animated')
            s.screenshot('settings-graphite-animated')
            s.key('\x1b')
            s.wait('simulation-sweep')
            assert json.loads(cli('config', 'show', '--json').stdout) == saved, 'cancel persisted preview'
            s.key('/settings\r')
            s.wait('Still')
            s.resize(48, 18)
            s.wait('Save defaults')
            click(s, *target(s, 'Chip motion'))
            click(s, *target(s, 'Mouse'))
            s.wait('Native selection')
            s.screenshot('settings-minimum')
            click(s, *target(s, 'Save defaults'))
            s.wait('Appearance saved')
            s.close()
        except BaseException:
            s.screenshot('settings-failure')
            print(s.text)
            s.abort()
            raise
        appearance = json.loads(cli('config', 'show', '--json').stdout)
        assert appearance == {'schema_version': 1, 'host': 'perch-test.invalid', 'theme': 'warm',
                              'reduced_motion': False, 'mouse': False, 'color_mode': 'auto'}, appearance
        s = Session([], 80, 24, env=env)
        try:
            s.wait('● connected')
            s.key('/settings\r')
            s.wait('warm'); s.wait('Animated'); s.wait('Native selection')
            assert b'\x1b[?1002h' not in s.raw and b'\x1b[?1003h' not in s.raw, 'saved capture choice lost on restart'
            s.key('\x1b')
            s.close()
        except BaseException:
            s.abort()
            raise
        path.write_text('{broken')
        cli('jobs', success=False)
        cli('jobs', '--no-config', '--host=perch-test.invalid')
        cli('version')
        cli('config', 'path')
        cli('config', 'reset')
        assert json.loads(cli('config', 'show', '--json').stdout) == {'schema_version': 1}
        s = Session(['--demo', '--no-config', '--reduced-motion'], 80, 24, env=env)
        try:
            s.wait('parameter-sweep')
            s.key('\x0bappearance\r')
            s.wait('Use for this demo')
            s.key(' ')
            s.wait('graphite')
            s.key('s')
            s.wait('Appearance applied for this session')
            s.key('/settings\r')
            s.close()  # quitting directly from settings also restores the terminal
            assert b'\x1b]111' in s.raw and b'\x1b]110' in s.raw, 'palette preview left terminal colors set'
        except BaseException:
            s.abort()
            raise
        assert json.loads(cli('config', 'show', '--json').stdout) == {'schema_version': 1}, 'demo appearance was persisted'
    print('Preferences: private persistence, CLI/launch destination, live TUI preview/cancel/save/restart, compact mouse controls, overrides, recovery and cleanup passed.')


if __name__ == '__main__':
    main()
