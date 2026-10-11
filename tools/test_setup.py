#!/usr/bin/env python3
"""First-run destination choice, explicit persistence and real workspace entry."""
import json
import argparse
import os
from pathlib import Path
import subprocess
import sys
import tempfile

from test_failure_terminal import FAKE_SSH
from test_mouse import target, click
from test_terminal import ROOT, OUT, Session


def live(host, binary=None):
    s = Session(['setup', '--host', host, '--no-config', '--reduced-motion'], 100, 34, binary=binary)
    report = {'host':host, 'scope':'Read-only setup to live SSH workspace; no saved preference changes or job submissions.'}
    try:
        s.wait('Connect to Hummingbird')
        s.wait('This session only')
        s.screenshot('setup-hummingbird-choice')
        click(s, *target(s, 'Open workspace'))
        s.wait('● connected', timeout=40)
        s.key('4')
        s.wait('Scheduler:')
        s.wait('SSH client')
        s.screenshot('setup-hummingbird-connected')
        s.key('1')
        s.wait('● connected')
        s.close()
        report.update(connected=True, connection_tab=True, jobs_return=True, terminal_restored=True)
    except BaseException:
        s.screenshot('setup-hummingbird-failure'); s.abort(); raise
    (OUT/'setup-hummingbird-results.json').write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps(report, indent=2))


def main(binary=None):
    binary = binary or ROOT/'bin/perch'
    with tempfile.TemporaryDirectory(prefix='perch-setup-') as tmp:
        root = Path(tmp)
        calls, state = root/'ssh-calls', root/'state'
        state.write_text('ok')
        ssh = root/'ssh'
        ssh.write_text('#!'+sys.executable+'\nimport os,sys,json\n'
                       'with open(os.environ["PERCH_SETUP_CALLS"],"a") as f: f.write(json.dumps(sys.argv[1:])+"\\n")\n'+FAKE_SSH.replace(
                           'if "perch-test.invalid" not in sys.argv:',
                           'if not {"alice@perch-test.invalid", "explicit-alias", "one-session", "save-recovery"}.intersection(sys.argv):'))
        ssh.chmod(0o700)
        env = {**os.environ, 'HOME':str(root), 'XDG_CONFIG_HOME':str(root/'config'),
               'TERM':'xterm-256color', 'COLORTERM':'truecolor',
               'PATH':str(root)+':/usr/bin:/bin', 'PERCH_SETUP_CALLS':str(calls),
               'PERCH_TEST_STATE':str(state), 'PERCH_TEST_FIXTURES':str(ROOT/'testdata/hummingbird')}

        def cli(*args):
            return subprocess.run([str(binary), *args], env=env, capture_output=True, text=True, timeout=10)

        path = Path(cli('config','path').stdout.strip())
        assert cli('config','set','theme','graphite').returncode == 0
        s = Session([], 80, 24, env=env, binary=binary)
        try:
            s.wait('Connect to Hummingbird')
            assert not calls.exists(), 'SSH started before a destination was chosen'
            s.key('\r')
            s.wait('Enter user@host')
            s.key('alice@perch-test.invalid')
            click(s, *target(s, 'Remember this destination'))
            s.wait('[x] Remember')
            s.wait('Space toggles')
            s.screenshot('setup-remember-80')
            click(s, *target(s, 'Save & open workspace'))
            s.wait('● connected')
            s.wait('simulation-sweep')
            s.screenshot('setup-connected-80')
            s.close()
        except BaseException:
            print(s.text); s.screenshot('setup-failure'); s.abort(); raise
        saved = json.loads(path.read_text())
        assert saved == {'schema_version':1, 'host':'alice@perch-test.invalid', 'theme':'graphite'}, saved
        assert path.stat().st_mode & 0o777 == 0o600
        assert path.parent.stat().st_mode & 0o777 == 0o700
        assert all('alice@perch-test.invalid' in json.loads(line) for line in calls.read_text().splitlines())

        # Saved destinations and explicit flags bypass onboarding; forcing setup
        # lets the user review that destination again without modifying it.
        for args in [[], ['--host','explicit-alias']]:
            s = Session(args, 80, 24, env=env, binary=binary)
            try:
                s.wait('● connected')
                assert 'Connect to Hummingbird' not in s.text
                s.close()
            except BaseException: s.abort(); raise
        before = calls.read_text()
        s = Session(['setup'], 48, 18, env=env, binary=binary)
        try:
            s.wait('alice@perch-test.invalid')
            s.wait('Remember this destination')
            s.screenshot('setup-compact-48')
            s.resize(40, 12)
            s.wait('Resize to continue')
            s.key('\r')
            assert calls.read_text() == before
            s.resize(120, 38)
            s.wait('Connect to Hummingbird')
            s.screenshot('setup-wide-120')
            s.key('\x15cancelled-alias')
            s.wait('cancelled-alias')
            assert 'alice@perch-test.invalid' not in s.text, 'could not replace existing host'
            s.close(exit_key='\x1b')
        except BaseException: print(s.text); s.abort(); raise
        assert calls.read_text() == before and json.loads(path.read_text()) == saved

        # no-config has no save toggle; choosing a different SSH alias is local
        # to this invocation, and the same real workspace still opens.
        s = Session(['setup','--no-config','--no-mouse','--host','one-session'], 48, 18, env=env, binary=binary)
        try:
            s.wait('This session only')
            assert '[ ] Remember' not in s.text and b'\x1b[?1003h' not in s.raw
            s.key('\t\r')
            s.wait('● connected')
            s.close()
        except BaseException: print(s.text); s.abort(); raise
        assert json.loads(path.read_text()) == saved

        # A failed save cannot silently connect; turning Remember off recovers.
        s = Session(['setup','--host','save-recovery'], 80, 24, env=env, binary=binary)
        try:
            s.wait('save-recovery')
            click(s,*target(s,'Remember this destination'))
            path.chmod(0o644)
            before = calls.read_text()
            click(s,*target(s,'Save & open workspace'))
            s.wait('Save failed')
            assert calls.read_text() == before
            s.screenshot('setup-save-error-80')
            click(s,*target(s,'Remember this destination'))
            click(s,*target(s,'Open workspace'))
            s.wait('● connected')
            s.close()
        except BaseException: print(s.text); s.abort(); raise
        finally: path.chmod(0o600)
        assert json.loads(path.read_text()) == saved
        result = cli('setup')
        assert result.returncode != 0 and 'interactive terminal' in result.stderr
        assert cli('setup','--demo').returncode != 0
        assert cli('setup','--json').returncode != 0
        assert cli('--demo','jobs','--json').returncode == 0
    print('Setup PTY passed: no network before acceptance, exact SSH destination, '
          'opt-in private persistence, preserved theme, saved/explicit bypass, '
          'cancel, 48-column/resize, no-config/no-mouse, save failure recovery, '
          'noninteractive rejection and terminal restoration. SSH was synthetic.')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', type=Path, help='Test an extracted candidate instead of bin/perch')
    parser.add_argument('--live-host', help='Opt-in read-only setup and connection to this SSH destination')
    args = parser.parse_args()
    if args.live_host:
        live(args.live_host, args.binary)
    else:
        main(args.binary)
