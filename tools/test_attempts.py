#!/usr/bin/env python3
"""Requeue/accounting-lag regression through real parser, SSH adapter and PTY."""
import copy
import json
import os
import pathlib
import subprocess
import sys
import tempfile
import time
from test_terminal import Session, ROOT
from test_failure_terminal import FAKE_SSH


def main():
    with tempfile.TemporaryDirectory(prefix='perch-attempts-') as directory:
        root=pathlib.Path(directory)
        home=root/'home';home.mkdir(mode=0o700)
        state=root/'state';state.write_text('ok')
        ssh=root/'ssh';ssh.write_text('#!'+sys.executable+'\n'+FAKE_SSH);ssh.chmod(0o700)
        fixtures=root/'fixtures';fixtures.mkdir()
        queue=json.loads((ROOT/'testdata/hummingbird/queue.json').read_text())
        accounting=json.loads((ROOT/'testdata/hummingbird/accounting.json').read_text())
        (fixtures/'partitions.json').write_text((ROOT/'testdata/hummingbird/partitions.json').read_text())
        env=os.environ | {'HOME':str(home),'PATH':str(root)+':/usr/bin:/bin',
            'PERCH_TEST_STATE':str(state),'PERCH_TEST_FIXTURES':str(fixtures),
            'TERM':'xterm-256color','COLORTERM':'truecolor'}
        now=int(time.time())
        q=queue['jobs'][0];queue['jobs']=[q]
        q.update(job_id=4242,name='current-attempt',restart_cnt=1)
        q['array_job_id']={'set':False,'infinite':False,'number':0}
        q['array_task_id']={'set':False,'infinite':False,'number':0}
        q['array_task_string']=''
        q['submit_time']={'set':True,'infinite':False,'number':now-3600}
        q['start_time']={'set':True,'infinite':False,'number':now-120}
        old=accounting['jobs'][0]
        old.update(job_id=4242,name='previous-attempt',restart_cnt=0)
        old['array']['job_id']=0
        old['array']['task_id']={'set':False,'infinite':False,'number':0}
        old['array']['task']=''
        old['time']['submission']=now-3600
        (fixtures/'queue.json').write_text(json.dumps(queue))
        (fixtures/'accounting.json').write_text(json.dumps(accounting))
        s=Session(['--host','perch-test.invalid','--no-config','--reduced-motion'],120,36,env=env)
        try:
            s.wait('● connected');s.wait('current-attempt')
            s.key('/job 4242\r');s.wait('Restarts')
            queue['jobs']=[]
            (fixtures/'queue.json').write_text(json.dumps(queue))
            s.pump(2.2);s.key('r');s.wait('Outcome not yet confirmed')
            assert 'current-attempt' in s.text and 'previous-attempt' not in s.text
            s.screenshot('requeued-awaiting-accounting')
            s.key('\x1b');s.wait('current-attempt')
            s.close()
        except BaseException:
            s.abort();raise
        latest=copy.deepcopy(old)
        latest.update(restart_cnt=1,name='current-attempt')
        for history in [[latest,old],[old,latest]]:
            accounting['jobs']=history
            (fixtures/'accounting.json').write_text(json.dumps(accounting))
            r=subprocess.run([str(ROOT/'bin/perch'),'job','4242','--host','perch-test.invalid','--no-config','--json'],env=env,capture_output=True,text=True,timeout=10)
            assert r.returncode==0,r.stderr
            job=json.loads(r.stdout)['data']
            assert job['name']=='current-attempt' and job['restart_count']==1,job
        # Contradictory terminal outcomes for exactly the same attempt must
        # never become an arbitrary success/failure in the CLI or workspace.
        conflict=copy.deepcopy(latest)
        conflict['state']['current']=['FAILED']
        for rows in [[latest,conflict],[conflict,latest]]:
            accounting['jobs']=rows
            (fixtures/'accounting.json').write_text(json.dumps(accounting))
            r=subprocess.run([str(ROOT/'bin/perch'),'job','4242','--host','perch-test.invalid','--no-config','--json'],env=env,capture_output=True,text=True,timeout=10)
            assert r.returncode!=0 and 'conflicting accounting outcomes' in r.stderr,(r.stdout,r.stderr)
        missing=copy.deepcopy(latest)
        missing['time']['submission']=0
        for rows in [[latest,missing],[missing,latest]]:
            accounting['jobs']=rows
            (fixtures/'accounting.json').write_text(json.dumps(accounting))
            for command in ['job','logs','script','resources']:
                r=subprocess.run([str(ROOT/'bin/perch'),command,'4242','--host','perch-test.invalid','--no-config','--json'],env=env,capture_output=True,text=True,timeout=10)
                assert r.returncode!=0 and 'ambiguous accounting identity' in r.stderr,(command,r.stdout,r.stderr)
        accounting['jobs']=[latest,conflict]
        (fixtures/'accounting.json').write_text(json.dumps(accounting))
        queue['jobs']=[q]
        (fixtures/'queue.json').write_text(json.dumps(queue))
        s=Session(['--host','perch-test.invalid','--no-config','--reduced-motion'],140,40,env=env)
        try:
            s.wait('current-attempt');s.wait('partial refresh')
            s.key('4');s.wait('conflicting accounting outcomes')
            s.wait('invalid response')
            s.screenshot('accounting-conflicting-outcomes')
            accounting['jobs']=[latest,old]
            (fixtures/'accounting.json').write_text(json.dumps(accounting))
            s.key('c');s.wait('● connected')
            s.key('1');s.wait('current-attempt')
            s.close()
        except BaseException:
            print(s.text);s.abort();raise
        # Real scrubbed Slurm requeue records, with the current task made pending
        # to reconstruct the observed intermediate phase.
        history=json.loads((ROOT/'testdata/hummingbird/mixed-requeue.json').read_text())
        retry=next(j for j in history['jobs'] if j['restart_cnt']==1)
        final=copy.deepcopy(history)
        retry['state']['current']=['PENDING']
        retry['time']['start']=0;retry['time']['end']=0;retry['time']['elapsed']=0
        queue=json.loads((ROOT/'testdata/hummingbird/queue.json').read_text())
        q=queue['jobs'][0];queue['jobs']=[q]
        q.update(job_id=retry['job_id'],name='synthetic-requeue',restart_cnt=1,
                 user_name='researcher',cluster='hummingbird',job_state=['PENDING'],
                 array_task_string='',state_reason='Priority',nodes='')
        q['memory_per_node']={'set':True,'infinite':False,'number':128}
        q['time_limit']={'set':True,'infinite':False,'number':2}
        q['array_job_id']={'set':True,'infinite':False,'number':9500}
        q['array_task_id']={'set':True,'infinite':False,'number':0}
        q['submit_time']={'set':True,'infinite':False,'number':retry['time']['submission']}
        q['start_time']={'set':False,'infinite':False,'number':0}
        (fixtures/'queue.json').write_text(json.dumps(queue))
        (fixtures/'accounting.json').write_text(json.dumps(history))
        s=Session(['--host','perch-test.invalid','--no-config','--reduced-motion'],140,46,env=env)
        try:
            s.wait('● connected');s.wait('1 / 2 observed tasks finished')
            s.screenshot('mixed-array-requeue')
            r=subprocess.run([str(ROOT/'bin/perch'),'arrays','--host','perch-test.invalid','--no-config','--json'],env=env,capture_output=True,text=True,timeout=10)
            assert r.returncode==0,r.stderr
            a=json.loads(r.stdout)['data'][0]
            assert a['observed_tasks']==2 and a['completed']==1 and a['pending']==1 and a['unresolved']==0,a
            queue['jobs']=[]
            (fixtures/'queue.json').write_text(json.dumps(queue))
            (fixtures/'accounting.json').write_text(json.dumps(final))
            s.pump(2.2);s.key('r');s.wait('Outcome not yet confirmed')
            # Accounting has its own 60-second cadence. A fresh process checks
            # the completed records without accelerating production polling.
            s.close()
            s=Session(['--host','perch-test.invalid','--no-config','--reduced-motion'],140,46,env=env)
            s.wait('● connected');s.wait('No active jobs.')
            s.key('2');s.wait('synthetic-requeue');s.wait('2 / 2 observed tasks finished')
            s.close()
        except BaseException:
            s.screenshot('mixed-array-requeue-failure');print(s.text);s.abort();raise
    print('Attempt identity PTY/CLI: accounting lag stays UNKNOWN, restart count visible, duplicate row order cannot select an older attempt; contradictory same-attempt outcomes fail accounting and recover; missing duplicate identity blocks job/log/script/resource reads in either order; mixed array requeue keeps its sibling and reaches 2/2.')
if __name__=='__main__':main()
