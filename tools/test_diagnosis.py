#!/usr/bin/env python3
"""Reach a log-based hint through real inspector navigation, with synthetic SSH."""
import json
from pathlib import Path
import shutil
import sys
import tempfile

from test_failure_terminal import FAKE_SSH
from test_log_retention import LOG_READ
from test_terminal import ROOT, Session


def main():
    with tempfile.TemporaryDirectory(prefix='perch-diagnosis-') as tmp:
        f = Path(tmp)
        state, log, reads = (f/name for name in ('state','log','reads'))
        state.write_text('ok')
        log.write_text('CUDA out of memory\n')
        ssh = f/'ssh'
        ssh.write_text('#!'+sys.executable+'\n'+FAKE_SSH.replace(
            'lines = remote[2].splitlines()', LOG_READ+'\nlines = remote[2].splitlines()'))
        ssh.chmod(0o700)
        fixtures = ROOT/'testdata/hummingbird'
        queue = json.loads((fixtures/'queue.json').read_text())
        j = queue['jobs'][0]
        queue['jobs'] = [j]
        j.update(job_id=4242, name='failed-diagnosis', job_state=['FAILED'], array_task_string='')
        for key in ('array_job_id','array_task_id'):
            j[key] = {'set':False,'infinite':False,'number':0}
        (f/'queue.json').write_text(json.dumps(queue))
        history = json.loads((fixtures/'accounting.json').read_text())
        history['jobs'] = []
        (f/'accounting.json').write_text(json.dumps(history))
        shutil.copyfile(fixtures/'partitions.json', f/'partitions.json')
        env = {'PATH':str(f)+':/usr/bin:/bin', 'PERCH_TEST_STATE':str(state),
               'PERCH_TEST_FIXTURES':str(f), 'PERCH_TEST_LOG':str(log), 'PERCH_TEST_LOG_READS':str(reads)}
        s = Session(['--host','perch-test.invalid','--no-config','--reduced-motion'], 100, 36, env=env)
        try:
            s.wait('● connected')
            s.key('2')
            s.wait('failed-diagnosis')
            s.key('l')
            s.wait('CUDA out of memory')
            s.key('\x1b')
            s.key('\r')
            s.key('\x1b[F')
            s.wait('Possible GPU memory exhaustion')
            s.wait('Log hint, not a confirmed cause.')
            s.wait('stdout read')
            s.screenshot('diagnosis-inspector-wide')
            s.resize(48, 24)
            s.key('\x1b[F')
            s.wait('stdout read')
            s.wait('not a confirmed cause')
            s.screenshot('diagnosis-inspector-compact')
            # A later successful empty read must supersede the previous hint.
            log.write_text('')
            s.key('l')
            s.wait('exists and is empty')
            s.key('\x1b')
            s.key('\r')
            s.key('\x1b[F')
            s.wait('Job failed')
            assert 'Possible GPU' not in s.text
            s.close()
        except BaseException:
            print(s.text)
            s.abort()
            raise
    print('Diagnosis PTY passed: log read → inspector evidence, source/time/caveat, compact view, '
          'successful truncation clears hint, terminal cleanup; no cluster workload.')


if __name__ == '__main__':
    main()
