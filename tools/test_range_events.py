#!/usr/bin/env python3
"""Range outcome activity through real parser/transport/UI, synthetic SSH only."""
import copy
import json
import pathlib
import sys
import tempfile

from test_terminal import ROOT, Session
from test_failure_terminal import FAKE_SSH


def main():
    with tempfile.TemporaryDirectory(prefix='perch-range-events-') as tmp:
        root=pathlib.Path(tmp)
        fixtures=root/'fixtures';fixtures.mkdir()
        state=root/'state';state.write_text('ok')
        copied=root/'clipboard'
        clip=root/'pbcopy';clip.write_text('#!'+sys.executable+'\nimport sys\nfrom pathlib import Path\nPath('+repr(str(copied))+').write_text(sys.stdin.read())\n');clip.chmod(0o700)
        ssh=root/'ssh';ssh.write_text('#!'+sys.executable+'\n'+FAKE_SSH);ssh.chmod(0o700)
        accounting=json.loads((ROOT/'testdata/hummingbird/accounting.json').read_text())
        failed=copy.deepcopy(accounting['jobs'][0])
        failed['job_id']=9000;failed['name']='range-outcomes'
        failed['array']['task_id']['set']=False;failed['array']['task']='0-10'
        failed['state']['current']=['FAILED'];failed['steps']=[]
        accounting['jobs']=[]
        accounting['errors']=[{}] # initial unavailable history is not an empty success
        queue=json.loads((ROOT/'testdata/hummingbird/queue.json').read_text())
        q=queue['jobs'][0];queue['jobs']=[q]
        q.update(job_id=9000,name='range-outcomes',job_state=['PENDING'],nodes='',array_task_string='0-10',restart_cnt=0)
        q['array_job_id']['number']=9000;q['array_task_id']['set']=False
        q['submit_time']['number']=failed['time']['submission'];q['start_time']['set']=False
        def save():
            (fixtures/'queue.json').write_text(json.dumps(queue))
            (fixtures/'accounting.json').write_text(json.dumps(accounting))
        save()
        (fixtures/'partitions.json').write_text((ROOT/'testdata/hummingbird/partitions.json').read_text())
        env={'PATH':str(root)+':/usr/bin:/bin','PERCH_TEST_STATE':str(state),'PERCH_TEST_FIXTURES':str(fixtures)}
        s=Session(['--host','perch-test.invalid','--no-config','--reduced-motion'],140,40,env=env)
        try:
            s.wait('range-outcomes');s.wait('partial refresh')
            s.key('/activity\r');s.wait('No new activity yet.')
            q['array_task_string']='0-9';accounting['jobs']=[failed];accounting['errors']=[];save()
            s.key('c');s.wait('Failed');s.wait('1 task')
            assert '10 tasks' not in s.text and '11 tasks' not in s.text
            s.screenshot('range-activity-partial')
            queue['jobs']=[];save()
            s.key('c');s.pump(2)
            assert '10 tasks' not in s.text,'stale accounting triggered a range alert'
            s.key('\x1b');s.wait('My jobs');s.wait('1 failed')
            s.key('y');s.wait('Copied failed tasks: --array=10')
            assert copied.read_text()=='--array=10','stale accounting reached the clipboard'
            s.screenshot('range-awaiting-accounting')
            s.key('/activity\r');s.wait('Activity')
            # Reconnect preserves the 60-second accounting cadence. Wait for
            # that interval, then request a normal eligible refresh.
            s.pump(61);s.key('r');s.wait('10 tasks',timeout=20);s.wait('of 2')
            s.screenshot('range-activity-complete')
            s.resize(48,24);s.wait('10 tasks');s.wait('1 task')
            s.screenshot('range-activity-narrow')
            s.key('\r');s.wait('9000_[0-10]');s.key('\x1b');s.wait('Activity')
            s.key('c');s.pump(2);s.wait('of 2')
            assert 'of 3' not in s.text,'reconnect duplicated an outcome'
            s.close()
        except BaseException:
            print(s.text);s.screenshot('range-activity-failure');s.abort();raise
    print('Range activity PTY passed: startup silence, 1 then 10 observed outcomes, stale-accounting copy exclusion, wide/narrow counts, exact-attempt inspection/return, reconnect deduplication, terminal cleanup. Synthetic SSH; no desktop notification sent.')


if __name__=='__main__':main()
