#!/usr/bin/env python3
"""Monitor an explicitly submitted testdata/monitor-smoke.sh job in a real PTY.

Usage: .venv/bin/python tools/test_live_terminal.py USER@HOST JOB_ID
This harness never submits or cancels a job. Captures contain only its filtered
synthetic workload; do not point it at private workloads.
"""
import json
import re
import sys
import time
from test_terminal import Session, OUT

host, job_id = sys.argv[1:]
assert job_id.isdigit(), 'Expected a numeric smoke job ID'
s = Session(['--host', host, '--reduced-motion'], height=50)
checks = []
try:
    s.wait('perch-smoke', timeout=45)
    s.key('/find '+job_id+'\r')
    s.wait('perch-smoke')
    s.screenshot('live-jobs')
    checks.append('job visible through SSH')
    s.key('l'); s.wait('PERCH_SMOKE_BEGIN')
    before = re.findall(r'PERCH_SMOKE_TICK \d+', s.text)
    deadline = time.monotonic()+20
    while re.findall(r'PERCH_SMOKE_TICK \d+', s.text) == before and time.monotonic()<deadline: s.pump(.2)
    assert re.findall(r'PERCH_SMOKE_TICK \d+', s.text) != before, 'No incremental log update'
    s.wait('PERCH_SMOKE_TICK')
    s.screenshot('live-output')
    checks.append('stdout and incremental polling')
    s.key('e'); s.wait('PERCH_SMOKE_STDERR')
    checks.append('separate stderr')
    s.key('e'); s.wait('PERCH_SMOKE_BEGIN')
    s.key('\x1b'); s.key('s'); s.wait('bytearray')
    checks.append('submitted script')
    s.key('\x1b'); s.key('u'); s.wait('Live step measurements')
    s.wait('3 samples', timeout=40)
    assert 'largest task peak RSS' in s.text, 'RSS was not measured'
    s.screenshot('live-resources')
    checks.append('three live samples and rendered resource charts')
    s.key('\x1b'); s.key('c')
    s.wait('perch-smoke', timeout=45)
    s.key('l'); s.wait('PERCH_SMOKE_BEGIN')
    checks.append('in-TUI SSH reconnect and resumed log view')
    s.key('\x1b'); s.key('\x1b')
    s.wait('Finished · perch-smoke', timeout=330)
    checks.append('accounting-confirmed completion transition')
    s.key('/job '+job_id+'\r'); s.wait(job_id+' · perch-smoke')
    s.key('\r')
    s.wait('done')
    s.screenshot('live-completed')
    s.key('l'); s.wait('PERCH_SMOKE_COMPLETE')
    checks.append('completed job output retained')
    assert s.raw.count(b'\x1b[?1049h') >= 2, 'TUI did not re-enter after interactive SSH'
    assert b'\x1b[?1049l' in s.raw, 'TUI did not suspend for interactive SSH'
    s.close()
    checks.append('terminal modes, cursor and alternate screen restored')
except BaseException:
    s.screenshot('live-failure'); s.abort(); raise
result = {'job_id':job_id, 'checks':checks, 'capture':'VT-emulator rendering of actual PTY output'}
(OUT/'live-results.json').write_text(json.dumps(result, indent=2)+'\n')
print(json.dumps(result, indent=2))
