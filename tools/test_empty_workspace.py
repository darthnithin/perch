#!/usr/bin/env python3
"""Empty workspace discovery with fixture-backed SSH and real keyboard/mouse input."""
import json
from pathlib import Path
import shutil
import sys
import tempfile

from test_failure_terminal import FAKE_SSH
from test_mouse import click
from test_terminal import ROOT, Session


def body_target(s, text):
    for y, row in enumerate(s.screen.display):
        if y > 4 and text in row:
            return row.index(text)+2, y
    raise AssertionError('Missing body action '+text+'\n'+s.text)


def main():
    with tempfile.TemporaryDirectory(prefix='perch-empty-') as tmp:
        folder = Path(tmp)
        state = folder/'state'
        state.write_text('ok')
        ssh = folder/'ssh'
        ssh.write_text('#!'+sys.executable+'\n'+FAKE_SSH)
        ssh.chmod(0o700)
        fixtures = ROOT/'testdata/hummingbird'
        for name in ('queue', 'accounting'):
            data = json.loads((fixtures/(name+'.json')).read_text())
            data['jobs'] = []
            (folder/(name+'.json')).write_text(json.dumps(data))
        shutil.copyfile(fixtures/'partitions.json', folder/'partitions.json')
        env = {'PATH':str(folder)+':/usr/bin:/bin', 'PERCH_TEST_STATE':str(state),
               'PERCH_TEST_FIXTURES':str(folder)}
        s = Session(['--host','perch-test.invalid','--no-config','--reduced-motion'], 140, 40, env=env)
        try:
            s.wait('● connected')
            s.wait('No active jobs.')
            assert 'Select a job' not in s.text and '/software' in s.text
            s.screenshot('empty-workspace-wide')
            click(s, *body_target(s, 'n Prepare a job'))
            s.wait('Cluster directory')
            s.key('/scratch/researcher/project')
            s.key('\x1b')
            s.wait('No active jobs.')
            s.key('n')
            s.wait('/scratch/researcher/project')
            s.key('\x1b')
            s.wait('No active jobs.')
            click(s, *body_target(s, '2 Recent history'))
            s.wait('No recent completed jobs.')
            s.key('1')
            s.wait('No active jobs.')
            click(s, *body_target(s, '3 Cluster partitions'))
            s.wait('Allocation is scheduler capacity')
            s.key('1')
            s.resize(48, 18)
            s.wait('n New job')
            assert '3 Partitions' in s.text and 'q quit' in s.screen.display[-1]
            s.screenshot('empty-workspace-compact')
            click(s, *body_target(s, 'n New job'))
            s.wait('Cluster directory')
            s.key('\x1b')
            s.wait('n New job')
            s.key('/find missing\r')
            s.wait('No jobs match this filter.')
            assert 'n New job' not in s.text
            s.key('\x1b')
            s.wait('No active jobs.')
            state.write_text('network')
            s.pump(2.2)
            s.key('r')
            s.wait('● disconnected')
            s.wait('No jobs at last refresh.')
            assert 'No active jobs.' not in s.text
            s.screenshot('empty-workspace-stale')
            s.close()
        except BaseException:
            print(s.text)
            s.abort()
            raise
    print('Empty workspace PTY passed: preparation without submission, retained draft, '
          'history/partitions pointer actions, compact resize, filter distinction, stale status and cleanup.')


if __name__ == '__main__':
    main()
