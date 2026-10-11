#!/usr/bin/env python3
"""Explicit opt-in, one bounded srun compute job; never retry submission."""
import argparse
import json
import re
import shlex
import subprocess
import time
import uuid
from test_terminal import ROOT, Session
from test_mouse import click, target

SCRIPT='''#!/bin/sh
set -eu
srun --ntasks=1 python3 - <<'COMPUTE'
import hashlib, time
block = bytearray(32 * 1024 * 1024)
stop = time.monotonic() + 110
while time.monotonic() < stop:
    hashlib.sha256(block).digest()
COMPUTE
'''

def main():
    p=argparse.ArgumentParser();p.add_argument('host');a=p.parse_args()
    out=ROOT/'output/validation'
    token='perch-live-steps-'+uuid.uuid4().hex
    job=None
    receipt={'comment':token,'host':a.host,'job_id':None}
    def save(): (out/'live-steps-live-receipt.json').write_text(json.dumps(receipt,indent=2)+'\n')
    def remote(argv,script=None):
        return subprocess.run(['ssh','-o','BatchMode=yes','-o','ConnectTimeout=10','--',a.host,shlex.join(argv)],input=script,text=True,capture_output=True,timeout=45)
    def queued():
        r=remote(['squeue','--me','--json=v0.0.44'])
        assert r.returncode==0,r.stderr
        d=json.loads(r.stdout);assert not d.get('errors')
        return d['jobs']
    def resources():
        r=subprocess.run([str(ROOT/'bin/perch'),'resources',job,'--host',a.host,'--no-config','--json'],text=True,capture_output=True,timeout=45)
        assert r.returncode==0,r.stderr
        return json.loads(r.stdout)
    save()
    try:
        shell='umask 077; for key in ${!SBATCH_@}; do unset "$key" || exit 70; done; exec sbatch "$@"'
        r=remote(['bash','-lc',shell,'perch-live-steps-test','--parsable','--nodes=1','--ntasks=1','--cpus-per-task=1','--mem=128M','--time=00:03:00','--job-name=perch-step-smoke','--comment='+token,'--output=/dev/null','--error=/dev/null'],SCRIPT)
        match=re.fullmatch(r'([1-9][0-9]*)(?:;[A-Za-z0-9_.-]+)?\s*',r.stdout)
        if match: job=match[1];receipt['job_id']=job;save()
        assert r.returncode==0 and job,'Submission unconfirmed; do not resubmit: '+token
        print('Submitted bounded resource check:',job,flush=True)
        deadline=time.monotonic()+90
        while True:
            records=[x for x in queued() if str(x['job_id'])==job and x.get('comment')==token]
            if records and 'RUNNING' in records[0]['job_state']: break
            assert time.monotonic()<deadline,'Job did not start within acceptance window'
            time.sleep(10)
        data=resources()
        deadline=time.monotonic()+55
        while not any(x['id']==job+'.0' and (x['peak_rss_bytes'] or 0)>16*1048576 for x in data['data']['live_steps']):
            assert time.monotonic()<deadline,'Compute-step memory was not observed'
            time.sleep(10);data=resources()
        assert all(x['average_task_cpu_seconds'] is None or 0 <= x['average_task_cpu_seconds'] < 600 for x in data['data']['live_steps']), 'Unavailable CPU sentinel became a measurement'
        (out/'live-steps-live-resources.json').write_text(json.dumps(data,indent=2)+'\n')
        raw=remote(['sstat','--allsteps','--jobs='+job,'--noheader','--parsable2','--format=JobID,MaxRSS,AveCPU'])
        assert raw.returncode==0,raw.stderr
        (out/'live-steps-live-sstat.txt').write_text(raw.stdout)
        ids={x['id'] for x in data['data']['live_steps']}
        assert job+'.batch' in ids and job+'.0' in ids
        print('CLI observed batch and compute-step measurements.',flush=True)
        s=Session(['--host',a.host,'--no-config','--reduced-motion'],100,40)
        try:
            s.wait('● connected',timeout=30);s.key('/job '+job+'\r');s.wait('perch-step-smoke');s.key('u')
            s.wait('Step '+job+'.0',timeout=30);s.wait('largest task peak RSS')
            s.wait('2 samples',timeout=25)
            s.screenshot('live-steps-hummingbird-compute')
            s.key(']');s.wait('Step '+job+'.batch')
            s.screenshot('live-steps-hummingbird-batch')
            s.resize(48,24);click(s,*target(s,'[ previous'));s.wait('Step '+job+'.0')
            s.wait('largest task peak RSS')
            s.screenshot('live-steps-hummingbird-compact');s.close()
        except BaseException: s.screenshot('live-steps-hummingbird-failure');s.abort();raise
        receipt['cli_and_pty_passed']=True;save()
        print('Live compute/batch chart switching and compact mouse checks passed.',flush=True)
    finally:
        if not job:
            matches=[x for x in queued() if x.get('comment')==token]
            if len(matches)==1: job=str(matches[0]['job_id']);receipt['job_id']=job;save()
        if job:
            remaining=[x for x in queued() if str(x['job_id'])==job]
            if remaining:
                r=remote(['scancel',job]);assert r.returncode==0,r.stderr
            deadline=time.monotonic()+20
            while any(str(x['job_id'])==job for x in queued()):
                assert time.monotonic()<deadline,'Test job still queued: '+job
                time.sleep(2)
            receipt['verified_absent']=True;save()
            print('Verified test job absent from queue:',job,flush=True)

if __name__=='__main__': main()
