#!/usr/bin/env python3
"""Opt-in Slurm cancellation-scope probe using only a newly submitted array.

Never accepts an existing job ID. The default holds two /bin/true tasks without
compute use. --running uses two bounded 1-CPU/64-MiB sleep tasks. Unique ownership
and comment checks precede each cancel; finally removes remaining probe tasks.
Unknown receipts are not retried; a report is written before every mutation.
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


def number(value):
    if isinstance(value, dict):
        return value.get('number') if value.get('set') and not value.get('infinite') else None
    return value


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--host', required=True)
    parser.add_argument('--report', type=Path, required=True)
    parser.add_argument('--submit', action='store_true')
    parser.add_argument('--running', action='store_true', help='Use two short running tasks instead of a held array (2 CPUs / 128 MiB total, 2-minute limit)')
    args = parser.parse_args()
    if not args.submit: parser.error('explicit --submit is required')
    if not args.host or args.host.startswith('-') or any(c.isspace() for c in args.host): parser.error('invalid SSH destination')
    os.umask(0o077)
    args.report.parent.mkdir(parents=True, exist_ok=True)
    # Exclusive creation prevents an ambiguous earlier submission being repeated.
    with args.report.open('x') as f: f.write('{}\n')
    token = 'perch-cancel-probe-'+secrets.token_hex(12)
    name = 'perch-cancel-probe'
    scope = 'two 1-CPU/64-MiB tasks, two-minute limit' if args.running else 'two held /bin/true array tasks'
    report = {'host':args.host, 'token':token, 'scope':scope+'; no existing workload accepted', 'stage':'preparing'}
    def save():
        tmp = args.report.with_suffix(args.report.suffix+'.tmp')
        tmp.write_text(json.dumps(report, indent=2)+'\n')
        tmp.replace(args.report)
    save()
    def remote(argv):
        command = shlex.join(argv)
        result = subprocess.run(['ssh','-T','-o','BatchMode=yes','-o','ConnectTimeout=8',
                                 '-o','ServerAliveInterval=5','-o','ServerAliveCountMax=2',
                                 '--',args.host,command], capture_output=True, text=True, timeout=25)
        if result.returncode: raise RuntimeError(result.stderr[-3000:] or f'remote exit {result.returncode}')
        return result.stdout.strip()
    def clean(program, argv):
        prefix = 'SBATCH_' if program == 'sbatch' else 'SCANCEL_'
        source = 'set -e; for v in ${!'+prefix+'@}; do unset "$v"; done; unset SLURM_CLUSTERS; exec "$@"'
        return remote(['env','-u','BASH_ENV','-u','ENV','/bin/bash','--noprofile','--norc','-c',source,'perch-cancel-probe',program,*argv])
    user = remote(['id','-un'])
    if not re.fullmatch(r'[A-Za-z_][A-Za-z0-9_.-]*', user): raise RuntimeError('unexpected remote username')
    report['user'], report['scancel_version'] = user, remote(['scancel','--version'])
    save()
    job_id = None
    attempted = False
    def queue():
        data = json.loads(remote(['squeue','--json=v0.0.44','--user='+user]+(['--jobs='+job_id] if job_id else [])))
        if data.get('errors'): raise RuntimeError('queue returned parser errors')
        return data['jobs']
    def own_rows():
        rows = [j for j in queue() if j.get('comment') == token]
        for j in rows:
            if j.get('user_name') != user or j.get('name') != name:
                raise RuntimeError('probe identity mismatch')
            if job_id is not None and number(j.get('array_job_id')) != int(job_id):
                raise RuntimeError('probe array identity mismatch')
        return rows
    def snapshot(rows):
        return [{k:j.get(k) for k in ('job_id','array_job_id','array_task_id','array_task_string','job_state','state_reason','submit_time','restart_cnt')} for j in rows]
    def stop(_signum, _frame): raise InterruptedError('probe interrupted; cleaning its own array')
    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)
    try:
        report['stage'] = 'submitting'; save(); attempted = True
        receipt = clean('sbatch', ['--parsable',*([] if args.running else ['--hold']),'--array=0-1','--nodes=1','--ntasks=1',
                                  '--cpus-per-task=1','--mem=64M','--time=2','--chdir=/tmp',
                                  '--output=/dev/null','--error=/dev/null','--export=NONE',
                                  '--job-name='+name,'--comment='+token,'--wrap='+('exec sleep 90' if args.running else '/bin/true')])
        if not re.fullmatch(r'[1-9][0-9]*(?:;[A-Za-z0-9_.-]+)?', receipt): raise RuntimeError('unknown submission receipt; do not resubmit')
        job_id = receipt.split(';')[0]
        report['job_id'], report['stage'] = job_id, 'submitted'; save()
        rows = own_rows()
        if args.running:
            deadline = time.monotonic()+45
            while len(rows)!=2 or any(j.get('job_state')!=['RUNNING'] for j in rows):
                if time.monotonic() >= deadline: raise RuntimeError('probe did not get two running tasks; no resubmission')
                time.sleep(1)
                rows=own_rows()
            if {number(j.get('array_task_id')) for j in rows}!={0,1}: raise RuntimeError('unexpected running task IDs')
        elif len(rows) != 1 or rows[0].get('array_task_string') != '0-1' or rows[0].get('job_state') != ['PENDING']:
            raise RuntimeError('expected one collapsed, pending two-task record')
        report['before_task_cancel'] = snapshot(rows)
        report['stage'] = 'cancelling-task-zero'; save()
        cancel_args=['--ctld','--user='+user,'--state='+('RUNNING' if args.running else 'PENDING'),'--',job_id+'_0']
        report['task_cancel_arguments']=cancel_args; save()
        clean('scancel', cancel_args)
        report['stage'] = 'task-cancel-returned'; save()
        remaining = own_rows()
        if args.running:
            deadline=time.monotonic()+30
            while any(number(j.get('array_task_id'))==0 for j in remaining):
                if time.monotonic()>=deadline: raise RuntimeError('task zero still visible after cancellation deadline')
                time.sleep(1)
                remaining=own_rows()
        report['after_task_cancel'] = snapshot(remaining)
        # Observe exact raw records; the conclusion is recorded separately from
        # assumptions about whether scancel splits an unmaterialized task.
        report['remaining_task_ids'] = [number(j.get('array_task_id')) for j in remaining]
        report['task_one_remained'] = len(remaining)==1 and (number(remaining[0].get('array_task_id'))==1 or remaining[0].get('array_task_string')=='1')
        report['stage'] = 'scope-observed'; save()
    except BaseException as exc:
        report['error'] = f'{type(exc).__name__}: {exc}'
        save()
        raise
    finally:
        if attempted:
            try:
                remaining = own_rows()
                if job_id is None and remaining:
                    identities = {number(j.get('array_job_id')) for j in remaining}
                    if len(identities) != 1 or None in identities: raise RuntimeError('ambiguous probe identity; manual reconciliation required')
                    job_id = str(identities.pop())
                    report['recovered_job_id'] = job_id
                if remaining:
                    report['cleanup_started'] = True; save()
                    clean('scancel', ['--ctld','--user='+user,'--',job_id])
                deadline = time.monotonic()+30
                while own_rows():
                    if time.monotonic() >= deadline: raise RuntimeError('probe still in queue after cleanup deadline')
                    time.sleep(1)
                report['cleanup_queue_absence_verified'] = True
                if job_id:
                    report['accounting'] = remote(['sacct','--jobs='+job_id,'--array','--noheader','--parsable2',
                                                   '--format=JobID,State,ExitCode,Elapsed'])
                if job_id is None:
                    report['submission_outcome'] = 'unknown; no matching current queue rows; do not resubmit automatically'
                save()
            except BaseException as exc:
                report['cleanup_error'] = f'{type(exc).__name__}: {exc}'
                save()
                raise
    print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
