#!/usr/bin/env python3
"""Actual process restart/kill recovery with fake SSH and an isolated HOME.

No network connection or real submission. The real CLI persists intent before
the fake scheduler receives it, then an independent CLI/TUI reads it offline.
"""
import json
import os
import pathlib
import shutil
import signal
import subprocess
import sys
import tempfile
import time

from test_terminal import Session, ROOT, OUT

FAKE = r'''
import json, os, pathlib, shlex, sys, time
assert "perch-receipt.invalid" in sys.argv
if "-O" in sys.argv: sys.exit(0)
args = shlex.split(sys.argv[-1])
if args[:2] == ["sh", "-c"]: sys.exit(255) # TUI probe: offline
assert args[0] == "bash", args
if args[1] == "-c": sys.exit(0) # directory check
sys.stdin.read()
if "--test-only" in args: sys.exit(0)
assert "--parsable" in args, args
token = next(a.removeprefix("--comment=") for a in args if a.startswith("--comment="))
state = pathlib.Path(os.environ["PERCH_RECEIPT_TEST"])
home = pathlib.Path(os.environ["HOME"])
journals = list(home.rglob("journal.json"))
assert len(journals) == 1
entries = json.loads(journals[0].read_text())["entries"]
assert any(e["receipt"]["submission_token"] == token and not e["receipt"]["job_id"] for e in entries)
with (state/"attempts").open("a") as f: f.write(token+"\n")
mode = (state/"mode").read_text()
if mode == "lost": sys.exit(255)
if mode == "kill":
    socket = next(a.removeprefix("ControlPath=") for a in sys.argv if a.startswith("ControlPath="))
    (state/"started").write_text(socket)
    time.sleep(30)
    sys.exit(99)
print("424242;synthetic")
'''

def main():
    with tempfile.TemporaryDirectory(prefix='perch-receipts-test-') as directory:
        root = pathlib.Path(directory)
        home = root/'home'; home.mkdir(mode=0o700)
        ssh = root/'ssh'; ssh.write_text('#!'+sys.executable+'\n'+FAKE); ssh.chmod(0o700)
        env = os.environ | {'HOME':str(home), 'XDG_STATE_HOME':str(home/'state'),
            'PATH':str(root)+':/usr/bin:/bin', 'PERCH_RECEIPT_TEST':str(root),
            'TERM':'xterm-256color','COLORTERM':'truecolor'}
        binary = str(ROOT/'bin/perch')
        args = [binary,'run','--debug','--debug-raw','--host','perch-receipt.invalid','--name','recovery-smoke',
            '--workdir','/synthetic','--submit','--json','--','printf','private-command-marker']
        for mode in ['accepted','lost']:
            (root/'mode').write_text(mode)
            r = subprocess.run(args, env=env, capture_output=True, text=True, timeout=10)
            if mode == 'accepted':
                assert r.returncode == 0, r.stderr
                assert json.loads(r.stdout)['receipt']['job_id'] == '424242'
            else:
                assert r.returncode != 0 and 'before retrying' in r.stderr, r.stderr
        (root/'mode').write_text('kill')
        p = subprocess.Popen(args, env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)
        try:
            deadline = time.monotonic()+10
            while not (root/'started').exists() and time.monotonic()<deadline:
                assert p.poll() is None, 'submission process exited before kill point'
                time.sleep(.02)
            assert (root/'started').exists(), 'submission did not reach fake scheduler'
            os.killpg(p.pid, signal.SIGKILL)
            p.communicate(timeout=5)
        finally:
            if p.poll() is None:
                os.killpg(p.pid, signal.SIGKILL); p.communicate(timeout=5)
            if (root/'started').exists():
                socket_dir = pathlib.Path((root/'started').read_text()).parent
                assert socket_dir.parent == pathlib.Path('/tmp') and socket_dir.name.startswith('perch-')
                shutil.rmtree(socket_dir)
        r = subprocess.run([binary,'receipts','--json'],env=env,capture_output=True,text=True,timeout=5)
        assert r.returncode == 0,r.stderr
        records=json.loads(r.stdout)['entries']
        assert len(records)==3 and sum(e['status']=='accepted' for e in records)==1,records
        assert len((root/'attempts').read_text().splitlines())==3,'unexpected submission retry'
        journals=list(home.rglob('journal.json')); assert len(journals)==1
        assert 'private-command-marker' not in journals[0].read_text()
        for capture in home.rglob('events*.jsonl'):
            assert 'private-command-marker' not in capture.read_text()
            assert all(not e['raw_captured'] for e in map(json.loads,capture.read_text().splitlines()))
        assert journals[0].stat().st_mode & 0o777 == 0o600
        s=Session(['--host','perch-receipt.invalid','--reduced-motion'],120,40,env=env)
        try:
            s.wait('Unable to load your jobs')
            s.key('/receipts\r'); s.wait('accepted · #424242'); s.wait('unconfirmed')
            s.screenshot('receipts-recovered-offline')
            s.resize(48,18); s.key('\x1b[F'); s.wait('not saved')
            s.screenshot('receipts-minimum')
            s.key('\x1b'); s.close()
        except BaseException:
            s.screenshot('receipts-failure'); s.abort(); raise
    result={'receipts':'passed','checks':['durable intent precedes transport','accepted receipt after restart',
        'lost-response receipt','SIGKILL recovery','no automatic retry','private metadata-only state',
        'offline TUI after restart','48x18 scrolling','terminal cleanup'],
        'scope':'Real processes and PTY with fake SSH; no cluster job submitted.'}
    (OUT/'receipts-results.json').write_text(json.dumps(result,indent=2)+'\n')
    print(json.dumps(result,indent=2))

if __name__=='__main__': main()
