#!/usr/bin/env python3
"""Opt-in production CLI cancellation checks, using only fresh bounded probes.

No existing job-ID argument. Held/running ordinary jobs and arrays, plus a
mixed running/pending array with a one-task concurrency limit.
Each array cancels task 0, verifies sibling 1, then confirms whole-array scope.
Writes intent and receipts; never retries a submission or cancellation.
"""
import argparse
import json
import os
from pathlib import Path
import re
import secrets
import shlex
import signal
import subprocess
import time

ROOT=Path(__file__).resolve().parents[1]


def number(value):
    if isinstance(value,dict):return value.get('number') if value.get('set') and not value.get('infinite') else None
    return value


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--host',required=True)
    parser.add_argument('--report',type=Path,required=True)
    parser.add_argument('--submit',action='store_true')
    parser.add_argument('--tui',action='store_true',help='Use real PTY reviews and pointer confirmation; excludes grouped single-task selection (CLI only)')
    parser.add_argument('--case',choices=['held-job','running-job','held-array','running-array','mixed-array','all'],default='all')
    args=parser.parse_args()
    if not args.submit:parser.error('--submit is required')
    if args.tui and args.case=='held-array':parser.error('Use the CLI for an individual unmaterialized task; the TUI offers whole-array review for grouped rows')
    if not args.host or args.host.startswith('-') or any(c.isspace() for c in args.host):parser.error('invalid host')
    os.umask(0o077);args.report.parent.mkdir(parents=True,exist_ok=True)
    with args.report.open('x') as out:out.write('{}\n')
    report={'host':args.host,'interface':'tui' if args.tui else 'cli','cases':[],'scope':'Only freshly submitted probes; at most 2 CPUs / 128 MiB, two-minute limits; no output files'}
    def save():
        tmp=args.report.with_suffix('.tmp');tmp.write_text(json.dumps(report,indent=2)+'\n');tmp.replace(args.report)
    def remote(argv):
        r=subprocess.run(['ssh','-T','-o','BatchMode=yes','-o','ConnectTimeout=8','-o','ServerAliveInterval=5','-o','ServerAliveCountMax=2','--',args.host,shlex.join(argv)],capture_output=True,text=True,timeout=25)
        if r.returncode:raise RuntimeError(r.stderr[-2000:] or f'SSH exit {r.returncode}')
        return r.stdout.strip()
    def clean(argv):
        script='set -e; for v in ${!SBATCH_@} ${!SCANCEL_@} ${!SQUEUE_@}; do unset "$v"; done; unset SLURM_CLUSTERS; exec "$@"'
        return remote(['env','-u','BASH_ENV','-u','ENV','/bin/bash','--noprofile','--norc','-c',script,'perch-cancel-test',*argv])
    user=remote(['id','-un'])
    if not re.fullmatch(r'[A-Za-z_][A-Za-z0-9_.-]*',user):raise RuntimeError('invalid user')
    report['user']=user;save()
    def interrupt(*_):raise InterruptedError('interrupted; cleaning owned probe')
    signal.signal(signal.SIGINT,interrupt);signal.signal(signal.SIGTERM,interrupt)
    cases=['held-job','running-job','held-array','running-array','mixed-array'] if args.case=='all' else [args.case]
    if args.tui and args.case=='all':cases=['held-job','running-array','mixed-array']
    for case in cases:
        array=case.endswith('array');mixed=case=='mixed-array';running=case.startswith('running') or mixed
        token='perch-cancel-cli-'+secrets.token_hex(12)
        record={'case':case,'token':token,'stage':'preparing','commands':[]};report['cases'].append(record);save()
        job_id=None;attempted=False
        def rows():
            data=json.loads(clean(['squeue','--json=v0.0.44','--local','--user='+user]))
            warnings=[w for w in data.get('warnings',[]) if not (not data.get('jobs') and w.get('description')=='Zero jobs to dump')]
            if data.get('errors') or warnings:raise RuntimeError('queue query incomplete')
            own=[j for j in data['jobs'] if j.get('comment')==token]
            for j in own:
                parent=number(j.get('array_job_id')) if array else j.get('job_id')
                if j.get('user_name')!=user or j.get('name')!='perch-cancel-cli' or (job_id and parent!=int(job_id)):raise RuntimeError('probe identity changed')
            return own
        def wait_for(predicate,seconds=35):
            deadline=time.monotonic()+seconds
            while True:
                current=rows()
                if predicate(current):return current
                if time.monotonic()>deadline:raise RuntimeError('observation deadline; no resubmission')
                time.sleep(1)
        def cli(selector,*options):
            if args.tui:
                from test_terminal import Session
                from test_mouse import click,target
                actual=[str(ROOT/'bin/perch'),'--host='+args.host,'--no-config','--reduced-motion']
                observation={'argv':actual,'target':selector,'whole_array':'--array' in options,'confirm':'--confirm' in options,'stage':'starting'}
                record['commands'].append(observation);save()
                s=Session(actual[1:],100,30)
                try:
                    s.wait('● connected',timeout=45);s.wait('perch-cancel-cli',timeout=30)
                    s.key('/job '+selector+'\r')
                    s.key('/cancel-array\r' if '--array' in options else '/cancel\r')
                    scope='array' if '--array' in options else 'task' if '_' in selector else 'job'
                    s.wait('ctrl+x cancel '+scope,timeout=30)
                    assert '#'+selector in s.text,s.text
                    assert user+' @ hummingbird' in s.text,s.text
                    frame='cancel-ui-hummingbird-'+case+'-'+selector+'-'+str(len(record['commands']))
                    s.screenshot(frame+'-review')
                    observation['preview_screen']=s.text;observation['preview_render']='output/terminal/'+frame+'-review.png';save()
                    if scope=='array':
                        match=re.search(r'Whole array · (\d+) tasks',s.text);assert match,s.text
                        observed=int(match[1])
                    else:
                        assert 'One '+scope in s.text,s.text
                        observed=1
                    states=[state for state in ['1 RUNNING','1 PENDING'] if state in s.text]
                    result={'mode':'preview','preview':{'scope':scope,'observed_tasks':observed,'states':states}}
                    if '--confirm' in options:
                        click(s,*target(s,'Cancel '+scope+' · Ctrl+X'))
                        s.wait('Cancellation requested',timeout=30)
                        s.screenshot(frame+'-receipt')
                        observation['receipt_screen']=s.text;observation['receipt_render']='output/terminal/'+frame+'-receipt.png'
                        result={'status':'requested'}
                        s.key('\r')
                    else:s.key('\x1b')
                    s.close();observation.update(stage='returned',exit=0);save()
                    return result
                except BaseException:
                    observation['failure_screen']=s.text;save();s.abort();raise
            cmd=[str(ROOT/'bin/perch'),'cancel',selector,'--host='+args.host,'--no-config','--json',*options]
            observation={'argv':cmd,'stage':'starting'};record['commands'].append(observation);save()
            r=subprocess.run(cmd,capture_output=True,text=True,timeout=55)
            observation.update(stage='returned',exit=r.returncode,stdout=r.stdout,stderr=r.stderr);save()
            if r.returncode:raise RuntimeError('production CLI failed: '+r.stderr)
            return json.loads(r.stdout)
        try:
            record['stage']='submitting';save();attempted=True
            receipt=clean(['sbatch','--parsable',*([] if running else ['--hold']),*(['--array=0-1%1' if mixed else '--array=0-1'] if array else []),'--nodes=1','--ntasks=1','--cpus-per-task=1','--mem=64M','--time=2','--chdir=/tmp','--output=/dev/null','--error=/dev/null','--export=NONE','--job-name=perch-cancel-cli','--comment='+token,'--wrap='+('exec sleep 90' if running else '/bin/true')])
            if not re.fullmatch(r'[1-9][0-9]*(?:;[A-Za-z0-9_.-]+)?',receipt):raise RuntimeError('unknown submission receipt; inspect by comment')
            job_id=receipt.split(';')[0];record['job_id']=job_id;record['stage']='submitted';save()
            if mixed:
                current=wait_for(lambda r:len(r)==2 and sorted(j['job_state'] for j in r)==[['PENDING'],['RUNNING']])
            else:
                current=wait_for(lambda r:len(r)==(2 if array and running else 1) and all(j['job_state']==(['RUNNING'] if running else ['PENDING']) for j in r))
            # Capture the scope fields needed for diagnosing schema mismatches.
            keys=['job_id','array_job_id','array_task_id','array_task_string','user_name','name','cluster','submit_time','start_time','restart_cnt','job_state','het_job_id','het_job_offset','het_job_id_set','federation_origin','federation_siblings_active','federation_siblings_viable']
            record['before']=[{k:j.get(k) for k in keys} for j in current];save()
            selector=job_id+'_0' if array and not mixed else job_id
            options=['--array'] if mixed else []
            preview=cli(selector,*options);assert preview['mode']=='preview' and preview['preview']['scope']==('array' if mixed else 'task' if array else 'job')
            if mixed:assert preview['preview']['observed_tasks']==2 and preview['preview']['states']==['1 RUNNING','1 PENDING'],preview
            assert rows(),'preview removed probe'
            result=cli(selector,*options,'--confirm');assert result['status']=='requested',result
            if array and not mixed:
                remaining=wait_for(lambda r:len(r)==1 and (number(r[0].get('array_task_id'))==1 or r[0].get('array_task_string')=='1'))
                record['sibling_remained']=[{k:j.get(k) for k in keys} for j in remaining];save()
                preview=cli(job_id,'--array');assert preview['preview']['observed_tasks']==1
                result=cli(job_id,'--array','--confirm');assert result['status']=='requested'
            wait_for(lambda r:not r)
            record['queue_absent_after_production_action']=True;record['stage']='passed';save()
        except BaseException as exc:
            record['error']=str(exc)
            try:save()
            except OSError:pass
            raise
        finally:
            if attempted:
                try:
                    current=rows()
                    if job_id is None and current:
                        ids={number(j.get('array_job_id')) if array else j.get('job_id') for j in current}
                        if len(ids)!=1 or None in ids:raise RuntimeError('ambiguous receipt; cannot clean')
                        job_id=str(ids.pop());record['recovered_job_id']=job_id
                    if current:
                        record['cleanup_attempted']=True
                        try:save()
                        except OSError:pass  # Disk failure must not skip owned cleanup.
                        clean(['scancel','--ctld','--user='+user,'--',job_id])
                    wait_for(lambda r:not r)
                    record['cleanup_queue_absent']=True
                    if job_id:record['accounting']=clean(['sacct','--jobs='+job_id,'--array','--user='+user,'--noheader','--parsable2','--format=JobID,State,ExitCode,Elapsed'])
                    save()
                except BaseException as exc:
                    record['cleanup_error']=str(exc);save();raise
        print('PASS:',case,job_id,flush=True)
    report['status']='passed';save()


if __name__=='__main__':main()
