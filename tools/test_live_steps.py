#!/usr/bin/env python3
"""All-step resource samples, independent charts and missing-step gaps; fake SSH."""
import json
from pathlib import Path
import subprocess
import sys
import tempfile
from test_failure_terminal import FAKE_SSH
from test_terminal import ROOT, Session
from test_mouse import click, target

STAT = r'''
if remote and remote[0] == 'sstat':
    assert remote == ['sstat','--allsteps','--jobs=9101','--noheader','--parsable2','--format=JobID,MaxRSS,AveCPU'], remote
    with open(os.environ['PERCH_TEST_READS'],'a') as f: f.write('sample\n')
    print(pathlib.Path(os.environ['PERCH_TEST_SAMPLE']).read_text(),end='')
    sys.exit(0)
'''

def main():
    with tempfile.TemporaryDirectory(prefix='perch-live-steps-') as tmp:
        root=Path(tmp)
        state, sample, reads = [root/x for x in ['state','sample','reads']]
        state.write_text('ok')
        sample.write_text('9101.batch|1024K|00:00.000\n9101.0|32M|00:12.500\n9101.extern||213503982334-14:25:51|\n')
        ssh=root/'ssh'
        # Single Run commands go directly through SSH, unlike batched queries.
        ssh.write_text('#!'+sys.executable+'\n'+FAKE_SSH.replace('assert remote[:2]',STAT+'\nassert remote[:2]'))
        ssh.chmod(0o700)
        env={'PATH':str(root)+':/usr/bin:/bin','PERCH_TEST_STATE':str(state),
             'PERCH_TEST_SAMPLE':str(sample),'PERCH_TEST_READS':str(reads),
             'PERCH_TEST_FIXTURES':str(ROOT/'testdata/hummingbird')}
        import os
        cli=subprocess.run([str(ROOT/'bin/perch'),'resources','9100_39','--host','perch-test.invalid','--no-config','--json'],env={**os.environ,**env},capture_output=True,text=True,timeout=20)
        assert cli.returncode == 0, cli.stderr
        data=json.loads(cli.stdout)['data']
        assert data['live_peak_rss_bytes']==1048576
        assert data['live_steps'][1]['peak_rss_bytes']==32*1048576
        assert data['live_steps'][1]['average_task_cpu_seconds']==12.5
        assert data['live_steps'][2]['peak_rss_bytes'] is None
        assert data['live_steps'][2]['average_task_cpu_seconds'] is None
        human=subprocess.run([str(ROOT/'bin/perch'),'resources','9100_39','--host','perch-test.invalid','--no-config'],env={**os.environ,**env},capture_output=True,text=True,timeout=20)
        assert human.returncode==0 and 'Live steps' in human.stdout and '32.0 MiB' in human.stdout and '12.500 s' in human.stdout, human.stdout+human.stderr
        s=Session(['--host','perch-test.invalid','--no-config','--reduced-motion'],100,40,env=env)
        try:
            s.wait('● connected')
            s.key('/job 9100_39\r')
            s.key('u')
            s.wait('Step 9101.0')
            s.wait('32.0 MiB')
            s.wait('12.5 s')
            before=reads.read_text()
            s.key(']')
            s.wait('Step 9101.batch')
            s.wait('1.0 MiB')
            click(s,*target(s,'] next'))
            s.wait('Step 9101.extern')
            s.wait('RSS unavailable')
            click(s,*target(s,'[ previous'))
            s.wait('Step 9101.batch')
            s.key('[')
            s.wait('Step 9101.0')
            assert reads.read_text()==before, 'step switching queried scheduler'
            sample.write_text('9101.batch|2M|00:01\n')
            s.pump(2.2); s.key('r')
            s.wait('RSS unavailable')
            assert 'Step 9101.0' in s.text and '2.0 MiB' not in s.text
            s.screenshot('live-steps-gap')
            s.key(']'); s.wait('Step 9101.batch'); s.wait('2.0 MiB')
            s.screenshot('live-steps-batch')
            s.resize(48,24)
            s.wait('Step 9101.batch')
            click(s,*target(s,'[ previous')); s.wait('Step 9101.0')
            s.screenshot('live-steps-compact')
            s.key('\x1b[F'); s.wait('samples')
            s.key('\x1b'); s.wait('active records')
            s.close()
        except BaseException:
            print(s.text); s.screenshot('live-steps-failure'); s.abort(); raise
    s=Session(['--demo','--no-config','--reduced-motion'],100,40)
    try:
        s.wait('parameter-sweep');s.key('u');s.wait('Step 9112.0');s.wait('1.1 GiB')
        s.key(']');s.wait('Step 9112.batch');s.wait('4.0 MiB')
        s.key('[');s.wait('Step 9112.0');s.screenshot('live-steps-demo');s.close()
    except BaseException: s.abort();raise
    print('Live steps: CLI scope/nulls, keyboard/mouse charts, disappearance gap, local switching, resize/back and terminal cleanup passed.')

if __name__=='__main__': main()
