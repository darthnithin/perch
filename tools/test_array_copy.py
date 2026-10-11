#!/usr/bin/env python3
"""Array recovery-copy acceptance with synthetic SSH and a private fake clipboard."""
import copy
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time

from test_failure_terminal import FAKE_SSH
from test_terminal import ROOT, Session

CLIPBOARD=r'''
import os,pathlib,sys,time
root=pathlib.Path(os.environ['PERCH_TEST_CLIPBOARD'])
mode=(root/'mode').read_text().strip()
with (root/'calls').open('a') as f: f.write(f'{mode} {os.getpid()}\n')
if mode=='fail': sys.exit(1)
if mode=='timeout': time.sleep(20);sys.exit(1)
(root/'copied').write_text(sys.stdin.read())
'''


def await_calls(session, path, count):
    deadline=time.monotonic()+5
    while not path.exists() or len(path.read_text().splitlines())<count:
        assert time.monotonic()<deadline, 'clipboard helper was not invoked'
        session.pump(.05)
    assert len(path.read_text().splitlines())==count, 'duplicate clipboard invocation'


def main():
    with tempfile.TemporaryDirectory(prefix='perch-array-copy-') as tmp:
        root=Path(tmp);fixtures=root/'fixtures';fixtures.mkdir()
        state=root/'state';state.write_text('ok')
        mode=root/'mode';mode.write_text('ok')
        copied=root/'copied';calls=root/'calls'
        for name in ['pbcopy','wl-copy','xclip']:
            helper=root/name;helper.write_text('#!'+sys.executable+'\n'+CLIPBOARD);helper.chmod(0o700)
        audit="import os,pathlib,sys\nwith pathlib.Path(os.environ['PERCH_TEST_CLIPBOARD'],'ssh-calls').open('a') as f: f.write(repr(sys.argv)+'\\n')\n"
        ssh=root/'ssh';ssh.write_text('#!'+sys.executable+'\n'+audit+FAKE_SSH);ssh.chmod(0o700)
        template=json.loads((ROOT/'testdata/hummingbird/accounting.json').read_text())
        base=template['jobs'][0]
        history=[]
        for task,status in [(42,'FAILED'),(5,'OUT_OF_MEMORY'),(3,'FAILED'),(4,'TIMEOUT'),(17,'FAILED'),(10,'CANCELLED'),(11,'COMPLETED')]:
            j=copy.deepcopy(base);j['job_id']=9001+task;j['array']['task_id']['number']=task
            j['name']='recovery-array';j['state']['current']=[status];j['steps']=[]
            history.append(j)
        # Duplicate scheduler records and stale failed attempts must not add IDs.
        history.append(copy.deepcopy(history[2]))
        template['jobs']=history
        (fixtures/'accounting.json').write_text(json.dumps(template))
        queue=json.loads((ROOT/'testdata/hummingbird/queue.json').read_text())
        retry=copy.deepcopy(queue['jobs'][0]);retry['job_id']=9018;retry['array_job_id']['number']=9000
        retry['array_task_id']['number']=17;retry['name']='recovery-array';retry['restart_cnt']=1
        retry['submit_time']['number']=base['time']['submission']+60
        retry['start_time']['number']=int(time.time())-300
        # A pending portion contributes to progress, never to failed IDs.
        pending=copy.deepcopy(retry);pending['job_id']=9000;pending['array_task_id']['set']=False
        pending['array_task_string']='50-99';pending['job_state']=['PENDING'];pending['restart_cnt']=0
        pending['submit_time']['number']=base['time']['submission'];pending['start_time']['set']=False
        standalone=copy.deepcopy(retry);standalone['job_id']=9999;standalone['name']='standalone'
        standalone['array_job_id']['set']=False;standalone['array_task_id']['set']=False
        queue['jobs']=[retry,pending,standalone]
        (fixtures/'queue.json').write_text(json.dumps(queue))
        (fixtures/'partitions.json').write_text((ROOT/'testdata/hummingbird/partitions.json').read_text())
        env={'PATH':str(root)+':/usr/bin:/bin','PERCH_TEST_STATE':str(state),'PERCH_TEST_FIXTURES':str(fixtures),
             'PERCH_TEST_CLIPBOARD':str(root),'WAYLAND_DISPLAY':'perch-test','DISPLAY':':perch-test'}
        r=subprocess.run([str(ROOT/'bin/perch'),'arrays','--host','perch-test.invalid','--no-config','--json'],env={**os.environ,**env},text=True,capture_output=True,timeout=20)
        assert r.returncode==0,r.stderr
        a=json.loads(r.stdout)['data'][0]
        assert (a['observed_tasks'],a['failed'],a['completed'],a['cancelled'],a['running'],a['pending'])==(57,4,1,1,1,50),a
        assert a['failed_task_ids']==[3,4,5,42] and a['failed_array_spec']=='--array=3-5,42',a
        s=Session(['--host','perch-test.invalid','--no-config','--reduced-motion'],140,40,env=env)
        try:
            s.wait('● connected');s.key('/job 9000_3\r');s.wait('recovery-array')
            s.key('\x1b[F');s.wait('6 / 57 observed tasks finished');s.wait('4 failed')
            s.wait('1 succeeded');s.wait('1 cancelled')
            s.key('y');s.wait('Copied failed tasks: --array=3-5,42')
            await_calls(s,calls,1)
            assert copied.read_text()=='--array=3-5,42'
            s.screenshot('array-copy-success')
            # Explicit slash command uses the same selected array and destination.
            copied.unlink()
            s.key('/failed\r');await_calls(s,calls,2)
            deadline=time.monotonic()+5
            while not copied.exists():
                assert time.monotonic()<deadline, 'slash command did not finish copying'
                s.pump(.05)
            s.wait('Copied failed tasks: --array=3-5,42')
            assert copied.read_text()=='--array=3-5,42'
            mode.write_text('fail');s.key('y');s.wait('Clipboard unavailable')
            await_calls(s,calls,3)
            assert 'Copied failed tasks:' not in s.text
            s.resize(48,24);s.wait('--array=3-5,42');s.screenshot('array-copy-fallback')
            # Establish a different notice before testing the timeout result.
            mode.write_text('ok');s.key('y');s.wait('Copied failed tasks:')
            await_calls(s,calls,4)
            mode.write_text('timeout');started=time.monotonic();s.key('y')
            await_calls(s,calls,5)
            s.wait('Copying failed tasks…')
            s.key('yyy');s.key('/failed\r')
            assert len(calls.read_text().splitlines())==5, 'overlapping copy helpers started'
            s.wait('Clipboard unavailable',timeout=5)
            elapsed=time.monotonic()-started
            assert 2.5<elapsed<5, ('clipboard timeout was not bounded',elapsed)
            timeout_pid=int(calls.read_text().splitlines()[-1].split()[1])
            try: os.kill(timeout_pid,0)
            except ProcessLookupError: pass
            else: raise AssertionError('timed-out clipboard helper is still running')
            # The timeout does not wedge later copy requests.
            mode.write_text('ok');s.key('y');s.wait('Copied failed tasks:')
            await_calls(s,calls,6)
            # An unrelated standalone selection never calls the clipboard helper.
            s.resize(120,40);s.key('/job 9999\r');s.wait('standalone')
            before=calls.read_text();s.key('y');s.wait('Select an array task first.')
            assert calls.read_text()==before
            # Closing the UI cancels a pending helper instead of leaving it behind.
            s.key('/job 9000_3\r');s.wait('recovery-array')
            mode.write_text('timeout');s.key('y');await_calls(s,calls,7)
            s.wait('Copying failed tasks…')
            quit_pid=int(calls.read_text().splitlines()[-1].split()[1])
            s.close()
            deadline=time.monotonic()+2
            while True:
                try: os.kill(quit_pid,0)
                except ProcessLookupError: break
                assert time.monotonic()<deadline, 'clipboard helper survived application exit'
                time.sleep(.05)
        except BaseException: print(s.text);s.screenshot('array-copy-failure');s.abort();raise
        # The fake SSH accepts read-only probes/queries only; any sbatch/scancel
        # call would fail its command allowlist. No real clipboard was touched.
        remote_calls=(root/'ssh-calls').read_text()
        assert not any(command in remote_calls for command in ['sbatch','scancel','srun']),remote_calls

        # A terminal collapsed range and an overlapping running retry are one
        # array. The old concrete record establishes the retry's family even
        # when Slurm changes its submission time.
        collapsed=copy.deepcopy(base)
        collapsed['job_id']=9000;collapsed['name']='collapsed-failures'
        collapsed['array']['task_id']['set']=False
        collapsed['array']['task']='0-20'
        collapsed['state']['current']=['FAILED'];collapsed['steps']=[]
        old=copy.deepcopy(history[4])  # failed task 17, before the current retry
        template['jobs']=[collapsed,old]
        (fixtures/'accounting.json').write_text(json.dumps(template))
        retry['name']='collapsed-failures'
        queue['jobs']=[retry]
        (fixtures/'queue.json').write_text(json.dumps(queue))
        r=subprocess.run([str(ROOT/'bin/perch'),'arrays','--host','perch-test.invalid','--no-config','--json'],env={**os.environ,**env},text=True,capture_output=True,timeout=20)
        assert r.returncode==0,r.stderr
        a=json.loads(r.stdout)['data'][0]
        assert (a['observed_tasks'],a['failed'],a['running'])==(21,20,1),a
        assert a['failed_array_spec']=='--array=0-16,18-20',a
        mode.write_text('ok')
        s=Session(['--host','perch-test.invalid','--no-config','--reduced-motion'],140,40,env=env)
        try:
            s.wait('● connected');s.wait('collapsed-failures');s.wait('20 failed')
            assert '20 failed' in s.screen.display[2], 'title-bar total differs from reconciled failures'
            s.key('y');s.wait('Copied failed tasks: --array=0-16,18-20')
            assert copied.read_text()=='--array=0-16,18-20'
            s.screenshot('array-copy-collapsed-wide')
            s.resize(48,24);s.key('\r');s.key('\x1b[F');s.wait('20 failed')
            s.key('y');s.wait('--array=0-16,18-20');s.screenshot('array-copy-collapsed-narrow')
            s.close()
        except BaseException: print(s.text);s.screenshot('array-copy-collapsed-failure');s.abort();raise
        # Same-attempt accounting can retain a larger failed range while the
        # scheduler currently reports a pending subset. Copy only the remainder.
        pending['array_task_string']='0-19'
        pending['name']='overlapping-ranges'
        pending['nodes']=''
        queue['jobs']=[pending]
        (fixtures/'queue.json').write_text(json.dumps(queue))
        r=subprocess.run([str(ROOT/'bin/perch'),'arrays','--host','perch-test.invalid','--no-config','--json'],env={**os.environ,**env},text=True,capture_output=True,timeout=20)
        assert r.returncode==0,r.stderr
        a=json.loads(r.stdout)['data'][0]
        assert (a['observed_tasks'],a['failed'],a['pending'])==(21,1,20),a
        assert a['failed_array_spec']=='--array=20',a
        s=Session(['--host','perch-test.invalid','--no-config','--reduced-motion'],140,40,env=env)
        try:
            s.wait('● connected');s.wait('overlapping-ranges');s.wait('1 failed')
            s.key('y');s.wait('Copied failed tasks: --array=20')
            assert copied.read_text()=='--array=20'
            s.screenshot('array-copy-overlap-wide')
            s.resize(48,24);s.key('\r');s.key('\x1b[F');s.wait('1 failed')
            s.key('y');s.wait('--array=20')
            assert copied.read_text()=='--array=20'
            s.screenshot('array-copy-overlap-narrow')
            s.close()
        except BaseException: print(s.text);s.abort();raise
        # Two historical ranges with equal attempt evidence and conflicting
        # terminal outcomes must not pick the lexically first range's state.
        completed=copy.deepcopy(collapsed)
        completed['array']['task']='20-25'
        completed['state']['current']=['COMPLETED']
        for rows in [[collapsed,completed],[completed,collapsed]]:
            template['jobs']=rows
            (fixtures/'accounting.json').write_text(json.dumps(template))
            r=subprocess.run([str(ROOT/'bin/perch'),'arrays','--host','perch-test.invalid','--no-config','--json'],env={**os.environ,**env},text=True,capture_output=True,timeout=20)
            assert r.returncode!=0 and 'conflicting accounting outcomes' in r.stderr,(r.stdout,r.stderr)
        before_calls=calls.read_text()
        s=Session(['--host','perch-test.invalid','--no-config','--reduced-motion'],140,40,env=env)
        try:
            s.wait('● connected');s.wait('Array summary unavailable')
            s.key('y');s.wait('Cannot copy failed tasks:')
            assert calls.read_text()==before_calls,'ambiguous summary wrote to clipboard'
            s.screenshot('array-copy-overlap-conflict')
            s.close()
        except BaseException: print(s.text);s.abort();raise
        # Different range IDs cannot hide missing submission evidence. The
        # accounting component refuses an ambiguous overlap in either order.
        unknown=copy.deepcopy(completed)
        unknown['time']['submission']=0
        for rows in [[collapsed,unknown],[unknown,collapsed]]:
            template['jobs']=rows
            (fixtures/'accounting.json').write_text(json.dumps(template))
            r=subprocess.run([str(ROOT/'bin/perch'),'arrays','--host','perch-test.invalid','--no-config','--json'],env={**os.environ,**env},text=True,capture_output=True,timeout=20)
            assert r.returncode!=0 and 'missing submission times' in r.stderr,(r.stdout,r.stderr)
        # A lone missing-time accounting range parses, but overlaps the current
        # queue's known submission. Family filtering must not erase ambiguity.
        unknown['array']['task']='0-20'
        template['jobs']=[unknown]
        (fixtures/'accounting.json').write_text(json.dumps(template))
        r=subprocess.run([str(ROOT/'bin/perch'),'arrays','--host','perch-test.invalid','--no-config','--json'],env={**os.environ,**env},text=True,capture_output=True,timeout=20)
        assert r.returncode!=0 and 'ambiguous array identity' in r.stderr,(r.stdout,r.stderr)
        s=Session(['--host','perch-test.invalid','--no-config','--reduced-motion'],140,40,env=env)
        try:
            s.wait('● connected');s.wait('Array summary unavailable')
            s.key('y');s.wait('Cannot copy failed tasks:')
            assert calls.read_text()==before_calls,'missing identity wrote to clipboard'
            s.screenshot('array-copy-identity-wide')
            s.resize(48,24);s.key('\r');s.key('\x1b[F');s.wait('Array summary unavailable')
            s.key('y');s.pump(.3)
            assert calls.read_text()==before_calls,'compact missing identity wrote to clipboard'
            s.screenshot('array-copy-identity-narrow')
            s.close()
        except BaseException: print(s.text);s.abort();raise
    print('Array copy PTY/CLI passed: distinct progress, retry/duplicate/cancel exclusions, sorted compressed IDs, ambiguous submission refusal, copy success/failure/timeout/recovery, duplicate suppression, compact fallback, standalone guard, no submission and exit cleanup.')

if __name__=='__main__': main()
