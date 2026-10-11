#!/usr/bin/env python3
"""Animated real-PTY navigation with 5,000 task records via private fake SSH."""
import copy
import json
from pathlib import Path
import sys
import tempfile
import time

from test_failure_terminal import FAKE_SSH
from test_terminal import ROOT, Session


def main():
    with tempfile.TemporaryDirectory(prefix='perch-large-array-') as tmp:
        root=Path(tmp);fixtures=root/'fixtures';fixtures.mkdir()
        state=root/'state';state.write_text('ok')
        ssh=root/'ssh';ssh.write_text('#!'+sys.executable+'\n'+FAKE_SSH);ssh.chmod(0o700)
        env={'PATH':str(root)+':/usr/bin:/bin','PERCH_TEST_STATE':str(state),'PERCH_TEST_FIXTURES':str(fixtures)}
        queue=json.loads((ROOT/'testdata/hummingbird/queue.json').read_text())
        history=json.loads((ROOT/'testdata/hummingbird/accounting.json').read_text())
        qbase,hbase=queue['jobs'][0],history['jobs'][0]
        queue['jobs']=[];history['jobs']=[]
        now=int(time.time())
        for task in range(5000):
            if task<4:
                j=copy.deepcopy(qbase);j.update(job_id=9001+task,name=f'running-{task}',array_task_string='')
                j['array_job_id']['number']=9000;j['array_task_id']['number']=task
                j['submit_time']['number']=now-3600;j['start_time']['number']=now-300
                queue['jobs'].append(j)
            else:
                j=copy.deepcopy(hbase);j.update(job_id=9001+task,name=f'completed-{task}',steps=[])
                j['array']['job_id']=9000;j['array']['task_id']['number']=task;j['array']['task']=''
                j['time']['submission']=now-3600;j['state']['current']=['COMPLETED']
                history['jobs'].append(j)
        def write(name,value):
            temp=fixtures/(name+'.tmp');temp.write_text(json.dumps(value));temp.replace(fixtures/(name+'.json'))
        write('queue',queue);write('accounting',history)
        (fixtures/'partitions.json').write_text((ROOT/'testdata/hummingbird/partitions.json').read_text())
        s=Session(['--host','perch-test.invalid','--no-config'],160,40,env=env)
        timings={}
        try:
            s.wait('4996 / 5000 observed tasks finished',timeout=20)
            s.wait('4 running')
            s.screenshot('large-array-animated-jobs')
            started=time.monotonic();s.key('2');s.wait('of 4996 records')
            timings['history_seconds']=round(time.monotonic()-started,3)
            started=time.monotonic();s.key('/sort name asc\r');s.wait('Sorted: name ↑');s.wait('› 9000_4999')
            timings['sort_seconds']=round(time.monotonic()-started,3)
            s.screenshot('large-array-sorted-history')
            s.key('/sort default\r');s.wait('› 9000_4999')
            assert 'Sorted:' not in s.text
            started=time.monotonic();s.key('/find completed-4999\r');s.wait('9000_4999');s.wait('of 1 records')
            timings['filter_seconds']=round(time.monotonic()-started,3)
            s.key('\r');s.wait('Exit code');s.key('\x1b[F');s.wait('4996 succeeded')
            s.key('\x1b');s.resize(80,24);s.wait('9000_4999');s.wait('Filter: completed-4999')
            s.screenshot('large-array-filtered-compact')
            s.resize(160,40);s.key('1');s.wait('running-0')
            q=queue['jobs'][0];q['name']='requeued-task';q['job_state']=['PENDING'];q['restart_cnt']=1
            q['start_time']['set']=False;q['state_reason']='Priority'
            write('queue',queue)
            s.pump(2.2);s.key('r');s.wait('requeued-task');s.wait('3 running');s.wait('1 queued')
            s.wait('4996 / 5000 observed tasks finished')
            s.screenshot('large-array-refreshed')
            s.key('2');s.wait('9000_4999');s.wait('Filter: completed-4999')
            s.close()
        except BaseException:
            print(s.text);s.screenshot('large-array-failure');s.abort();raise
    print('Large-array PTY passed: 5000 parsed tasks, animation enabled, History/sort/reset/filter/inspector/resize, refreshed counts/name, retained selection and tab filter, terminal cleanup. Synthetic SSH only.')
    print(json.dumps(timings,sort_keys=True))


if __name__=='__main__':main()
