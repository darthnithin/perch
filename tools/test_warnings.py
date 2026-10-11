#!/usr/bin/env python3
"""Time-warning editing, persistence and flag precedence in isolated processes."""
import json
import os
import pathlib
import subprocess
import sys
import tempfile

from test_terminal import ROOT, Session
from test_footer import activate
from test_hover import env, move
from test_mouse import target
from test_failure_terminal import FAKE_SSH


def main():
    for width, height in [(140, 40), (80, 24), (48, 18)]:
        s = Session(['--demo', '--no-config', '--reduced-motion'], width, height, env=env('ghostty'))
        try:
            s.wait('parameter-sweep'); s.key('/find parameter\r')
            s.key('/warnings\r'); s.wait('Time warnings'); s.wait('> 5m')
            s.key('\x15bogus\r'); s.wait('time warnings need')
            s.key('\x15off'); activate(s, 'esc back'); s.wait('Filter: parameter')
            s.key('/warnings\r'); s.wait('> 5m')
            s.key('\x151m,15m,5m')
            move(s, *target(s, '1m,15m,5m'))
            s.screenshot('warnings-editor-'+str(width))
            activate(s, 'enter apply'); s.wait('Time warnings: 15m,5m,1m')
            s.key('/warnings\r'); s.wait('> 15m,5m,1m')
            s.resize(48, 18); s.wait('enter apply'); s.wait('1–5 thresholds')
            s.key('\x15off\r'); s.wait('Time warnings: off')
            s.resize(width, height)
            s.key('d'); s.key('d'); s.key('/activity\r')
            s.wait('of 1'); assert 'Time limit near' not in s.text, 'off still warned'
            s.key('/warnings\r'); s.wait('> off')
            s.key('\x1b'); s.wait('Activity')
            s.key('3'); s.close()
        except BaseException:
            print(s.text); s.screenshot('warnings-failure-'+str(width)); s.abort(); raise

    with tempfile.TemporaryDirectory(prefix='perch-warnings-') as directory:
        root = pathlib.Path(directory)
        state = root/'state'; state.write_text('ok')
        ssh = root/'ssh'; ssh.write_text('#!'+sys.executable+'\n'+FAKE_SSH); ssh.chmod(0o700)
        isolated = {**os.environ, 'HOME':str(root/'home'), 'XDG_CONFIG_HOME':str(root/'config'),
                    'PATH':str(root)+':/usr/bin:/bin', 'PERCH_TEST_STATE':str(state),
                    'PERCH_TEST_FIXTURES':str(ROOT/'testdata/hummingbird')}

        def cli(*args, success=True):
            result = subprocess.run([str(ROOT/'bin/perch'), *args], env=isolated, capture_output=True, text=True, timeout=15)
            assert (result.returncode == 0) == success, (args, result.stdout, result.stderr)
            return result.stdout

        cli('config','set','time-warnings','10m,1m')
        cli('config','set','host','perch-test.invalid')
        cli('config','set','time-warnings','1m,1m',success=False)
        cli('--demo','--time-warnings=0m','jobs',success=False)
        for arguments, expected in [([], '10m,1m'), (['--time-warnings=off'], 'off'), (['--no-config'], '5m')]:
            s = Session(['--host','perch-test.invalid','--reduced-motion',*arguments],80,24,env=isolated)
            try:
                s.wait('● connected'); s.key('/warnings\r'); s.wait('> '+expected)
                if not arguments:
                    s.key('\x1515m,5m,1m')
                    activate(s,'ctrl+s save'); s.wait('Time warnings saved: 15m,5m,1m')
                    saved=json.loads(cli('config','show','--json'))
                    assert saved['time_warnings']=='15m,5m,1m' and saved['host']=='perch-test.invalid'
                    s.key('/warnings\r'); s.wait('> 15m,5m,1m')
                s.key('\x1b'); s.close()
            except BaseException:
                print(s.text); s.abort(); raise
        saved=json.loads(cli('config','show','--json'))
        assert saved['time_warnings']=='15m,5m,1m', 'launch override persisted'
        path=pathlib.Path(cli('config','path').strip())
        assert path.stat().st_mode & 0o777 == 0o600
        cli('config','unset','time-warnings')
        assert 'time_warnings' not in json.loads(cli('config','show','--json'))
    print('Warnings PTY/CLI passed: compact/wide editing, validation, cancel, pointer apply/save, resize, off suppresses future activity, explicit persistence, private files, saved/flag/no-config precedence, terminal cleanup. Fake SSH only; no desktop notification sent.')


if __name__ == '__main__':
    main()
