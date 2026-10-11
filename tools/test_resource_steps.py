#!/usr/bin/env python3
"""Recorded step resources through the adapter and real PTY; no network."""
import json
from pathlib import Path
import shutil
import sys
import tempfile

from test_failure_terminal import FAKE_SSH
from test_terminal import ROOT, Session


def main():
    with tempfile.TemporaryDirectory(prefix='perch-resource-steps-') as tmp:
        folder = Path(tmp)
        state = folder/'state'
        state.write_text('ok')
        ssh = folder/'ssh'
        ssh.write_text('#!'+sys.executable+'\n'+FAKE_SSH)
        ssh.chmod(0o700)
        fixture = ROOT/'testdata/hummingbird'
        queue = json.loads((fixture/'queue.json').read_text())
        queue['jobs'] = []
        (folder/'queue.json').write_text(json.dumps(queue))
        shutil.copyfile(fixture/'accounting-multistep.json', folder/'accounting.json')
        shutil.copyfile(fixture/'partitions.json', folder/'partitions.json')
        env = {'PATH': str(folder)+':/usr/bin:/bin', 'PERCH_TEST_STATE': str(state),
               'PERCH_TEST_FIXTURES': str(folder)}
        s = Session(['--host', 'perch-test.invalid', '--no-config', '--reduced-motion'], 100, 38, env=env)
        try:
            s.wait('● connected')
            s.key('2')
            s.wait('mpi-fixture')
            s.key('u')
            s.wait('8.1%')
            s.key('\x1b[F')
            s.wait('Recorded steps')
            s.wait('9400.batch')
            s.wait('9400.extern')
            s.wait('9400.0')
            s.wait('0.837 s')
            s.wait('0.000 s')
            s.wait('0.460 s')
            assert 'unavailable' in s.text, 'missing external-step RSS became zero'
            assert '29.7 MiB' in s.text and '26.0 MiB' in s.text
            s.screenshot('resource-steps-wide')
            s.resize(48, 18)
            s.key('\x1b[F')
            s.wait('26.0 MiB')
            s.key('k')
            s.wait('9400.0')
            assert 'q quit' in s.screen.display[-1]
            s.screenshot('resource-steps-compact')
            s.key('\x1b[H')
            s.wait('mpi-fixture')
            s.key('\x1b')
            s.wait('History')
            s.close()
        except BaseException:
            print(s.text)
            s.screenshot('resource-steps-failure')
            s.abort()
            raise
    print('Resource steps PTY passed: real multistep fixture, aggregate CPU ratio, '
          'separate step CPU/RSS, missing versus recorded zero, scroll/resize/back and cleanup.')


if __name__ == '__main__':
    main()
