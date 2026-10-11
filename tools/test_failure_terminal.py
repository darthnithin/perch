#!/usr/bin/env python3
"""Exercise failure/recovery in the real TUI with a local, fake ssh executable.

No cluster or network connection is made. The adapter parses scrubbed Slurm
fixtures through its normal framing; this verifies UI/transport integration,
not real network loss, password entry or MFA acceptance.
"""
import json
import pathlib
import sys
import tempfile

from test_terminal import Session, ROOT, OUT

FAKE_SSH = r'''
import os, pathlib, re, shlex, sys
if "perch-test.invalid" not in sys.argv:
    sys.exit("Test ssh refuses any other host")
if "-O" in sys.argv: sys.exit(0)
state = pathlib.Path(os.environ["PERCH_TEST_STATE"]).read_text().strip()
if state in ("network", "auth"):
    print("Permission denied (publickey)." if state == "auth" else "Connection refused", file=sys.stderr)
    sys.exit(255)
if sys.argv[-1] == "true": sys.exit(0)
remote = shlex.split(sys.argv[-1])
if remote == ["id", "-un"]:
    print("researcher")
    sys.exit(0)
assert remote[:2] == ["sh", "-c"], remote
lines = remote[2].splitlines()
fixtures = pathlib.Path(os.environ["PERCH_TEST_FIXTURES"])
def reply(args):
    if args == ["id", "-un"]: return "researcher\n", "", 0
    if args == ["squeue", "--version"]: return "slurm 25.11.4\n", "", 0
    if args == ["scontrol", "show", "config"]:
        return "ClusterName = hummingbird\nSchedulerType = sched/backfill\n", "", 0
    if args == ["sh", "-c", "command -v sstat"]: return "/test/sstat\n", "", 0
    if args[0] == "squeue" and state == "partial": return "", "Synthetic queue failure", 1
    name = {"squeue":"queue", "sacct":"accounting", "sinfo":"partitions"}.get(args[0])
    assert name is not None, args
    return (fixtures / (name+".json")).read_text(), "", 0
for i, line in enumerate(lines):
    match = re.fullmatch(r"printf '%s\\n' '(PERCH_[a-f0-9]+:\d+):OUT'", line)
    if not match: continue
    marker = match[1]
    command = shlex.split(lines[i+1].split(' 2>')[0])
    out, err, code = reply(command)
    sys.stdout.write(f"{marker}:OUT\n{out}\n{marker}:ERR\n{err}\n{marker}:EXIT:{code}\n")
'''

def main():
    with tempfile.TemporaryDirectory(prefix='perch-failure-test-') as directory:
        root = pathlib.Path(directory)
        state = root/'state'
        state.write_text('ok')
        ssh = root/'ssh'
        ssh.write_text('#!'+sys.executable+'\n'+FAKE_SSH)
        ssh.chmod(0o700)
        env = {'PATH':str(root)+':/usr/bin:/bin', 'PERCH_TEST_STATE':str(state),
               'PERCH_TEST_FIXTURES':str(ROOT/'testdata/hummingbird')}
        s = Session(['--host','perch-test.invalid','--reduced-motion'], 140, 40, env=env)
        try:
            s.wait('simulation-sweep'); s.wait('● connected')
            s.key('/find simulation\r'); s.wait('Filter: simulation')
            state.write_text('network'); s.pump(2.2); s.key('r')
            s.wait('● disconnected'); s.wait('c reconnect')
            assert 'simulation-sweep' in s.text, 'failure erased retained jobs'
            s.resize(48,24); s.wait('Filter: simulation')
            assert 'disconnected' in s.text and 'c reconnect' in s.text
            s.screenshot('disconnected-filter-narrow')
            s.resize(140,40); s.screenshot('disconnected-filter-wide')
            state.write_text('ok')
            s.wait('● connected', timeout=20)  # automatic bounded-backoff retry
            assert 'Filter: simulation' in s.text, 'automatic recovery lost filter'
            state.write_text('auth'); s.key('c')
            s.wait('Sign-in did not complete'); s.wait('● disconnected')
            state.write_text('ok'); s.key('c'); s.wait('● connected')
            assert 'simulation-sweep' in s.text and 'Filter: simulation' in s.text
            state.write_text('partial'); s.pump(2.2); s.key('r')
            s.wait('partial refresh'); s.wait('Synthetic queue failure')
            assert 'simulation-sweep' in s.text, 'partial refresh erased jobs'
            s.screenshot('partial-refresh-filter')
            s.key('4'); s.wait('SSH client'); s.wait('Synthetic queue failure')
            state.write_text('ok'); s.pump(2.2); s.key('r'); s.wait('● connected')
            s.key('1'); s.wait('Filter: simulation')
            assert s.raw.count(b'\x1b[?1049h') >= 3, 'interactive login did not resume the TUI'
            s.close()
        except BaseException:
            s.screenshot('failure-recovery-failure'); s.abort(); raise

        state.write_text('auth')
        s = Session(['--host','perch-test.invalid','--reduced-motion'], 80,24,env=env)
        try:
            s.wait('Unable to load your jobs'); s.wait('● disconnected')
            s.screenshot('initial-auth-failure')
            state.write_text('ok'); s.key('c'); s.wait('simulation-sweep'); s.wait('● connected')
            s.close()
        except BaseException:
            s.screenshot('initial-auth-failure-test'); s.abort(); raise
    result = {'failure_recovery':'passed', 'checks':['retained jobs during network failure',
        'filter and connection state visible at 48 columns', 'automatic recovery',
        'failed then successful interactive login', 'partial queue failure',
        'connection detail navigation', 'initial authentication failure and recovery',
        'alternate screen re-entry and terminal cleanup'],
        'scope':'Real PTY and production adapter with fake SSH; no network, password or MFA test.'}
    (OUT/'failure-results.json').write_text(json.dumps(result,indent=2)+'\n')
    print(json.dumps(result,indent=2))

if __name__ == '__main__': main()
