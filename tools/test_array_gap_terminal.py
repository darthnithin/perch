#!/usr/bin/env python3
"""Missing array records in the real PTY; synthetic SSH only, no cluster calls."""
import json
import os
import subprocess
import pathlib
import sys
import tempfile

from test_failure_terminal import FAKE_SSH
from test_terminal import Session, ROOT


def main():
    with tempfile.TemporaryDirectory(prefix='perch-array-gap-') as directory:
        root = pathlib.Path(directory)
        state = root / 'state'
        state.write_text('ok')
        fixtures = root / 'fixtures'
        fixtures.mkdir()
        queue = json.loads((ROOT / 'testdata/hummingbird/queue.json').read_text())
        job = queue['jobs'][0]
        job['array_task_id'] = {'set': False, 'number': 0}
        job['array_task_string'] = '0-9'
        job['job_state'] = ['PENDING']
        job['start_time'] = {'set': False, 'number': 0}
        job['nodes'] = ''
        job['state_reason'] = 'Priority'
        queue['jobs'] = [job]
        queued = json.dumps(queue)
        queue_path = fixtures / 'queue.json'
        queue_path.write_text(queued)
        accounting = json.loads((ROOT / 'testdata/hummingbird/accounting.json').read_text())
        accounting['jobs'] = []
        (fixtures / 'accounting.json').write_text(json.dumps(accounting))
        (fixtures / 'partitions.json').write_text((ROOT / 'testdata/hummingbird/partitions.json').read_text())
        ssh = root / 'ssh'
        ssh.write_text('#!' + sys.executable + '\n' + FAKE_SSH)
        ssh.chmod(0o700)
        env = {'PATH': str(root) + ':/usr/bin:/bin', 'PERCH_TEST_STATE': str(state),
               'PERCH_TEST_FIXTURES': str(fixtures)}
        session = Session(['--host', 'perch-test.invalid', '--no-config', '--reduced-motion'], 140, 40, env=env)
        try:
            session.wait('10 queued')
            queue['jobs'] = []
            queue_path.write_text(json.dumps(queue))
            session.pump(2.2)
            session.key('r')
            session.wait('9100 [?]')
            session.wait('At least 10 tasks')
            assert '0 queued' in session.text
            session.screenshot('array-awaiting-accounting-wide')
            session.key('/job 9100\r')
            session.wait('Individual task details')
            session.key('l')
            session.wait('Choose a confirmed task')
            assert 'Standard output' not in session.text
            session.resize(48, 18)
            session.wait('Awaiting accounting')
            session.wait('At least 10 tasks')
            session.screenshot('array-awaiting-accounting-narrow')
            session.key('\x1b[F')
            session.wait('read its output')
            session.screenshot('array-awaiting-accounting-narrow-scrolled')
            session.resize(140, 40)
            session.key('1')
            queue_path.write_text(queued)
            session.pump(2.2)
            session.key('r')
            session.wait('10 queued')
            assert '9100 [?]' not in session.text, 'gap remained after records returned'
            session.close()
        except BaseException:
            session.screenshot('array-gap-failure')
            print(session.text)
            session.abort()
            raise
        # Overlap within one collapsed expression must not inflate either
        # the visible queue total or the retained minimum after it disappears.
        job['array_task_string'] = '0-9,5-14,0-12:2'
        job['name'] = 'overlap-sweep'
        queue['jobs'] = [job]
        queue_path.write_text(json.dumps(queue))
        result = subprocess.run(
            [str(ROOT / 'bin/perch'), 'arrays', '--host', 'perch-test.invalid', '--no-config', '--json'],
            env=os.environ | env, capture_output=True, text=True, timeout=10)
        assert result.returncode == 0, result.stderr
        summary = json.loads(result.stdout)['data'][0]
        assert summary['observed_tasks'] == 15 and summary['pending'] == 15, summary
        session = Session(['--host', 'perch-test.invalid', '--no-config', '--reduced-motion'], 140, 40, env=env)
        try:
            session.wait('15 queued')
            session.wait('0 / 15 observed tasks finished')
            session.screenshot('array-overlap-distinct-tasks')
            queue['jobs'] = []
            queue_path.write_text(json.dumps(queue))
            session.pump(2.2)
            session.key('r')
            session.wait('At least 15 tasks')
            session.close()
        except BaseException:
            session.screenshot('array-overlap-failure')
            print(session.text)
            session.abort()
            raise
    print('Array gap PTY/CLI: retained rows, action guards, resize/recovery and distinct overlapping task counts passed.')


if __name__ == '__main__':
    main()
