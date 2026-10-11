#!/usr/bin/env python3
"""Opt-in delayed-start job, inspect through real Perch, cancel in finally.

No submission retry. A unique comment supports exact recovery of a lost receipt.
"""
import argparse
import json
import re
import shlex
import subprocess
import uuid

from test_terminal import ROOT, Session


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('host', help='Explicit authorized Hummingbird SSH destination')
    args = parser.parse_args()
    out = ROOT/'output/validation'
    token = 'perch-estimate-'+uuid.uuid4().hex
    job_id = None

    def remote(argv, script=None):
        return subprocess.run(['ssh', '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=10',
                               '--', args.host, shlex.join(argv)], input=script,
                              text=True, capture_output=True, timeout=40)

    def queued(ids=None):
        cmd = ['squeue', '--me', '--json=v0.0.44']
        if ids:
            cmd += ['--jobs='+ids]
        r = remote(cmd)
        assert r.returncode == 0, r.stderr
        data = json.loads(r.stdout)
        assert not data.get('errors'), 'scheduler returned query errors'
        return data

    def cli(command):
        r = subprocess.run([str(ROOT/'bin/perch'), command, job_id, '--host', args.host,
                            '--no-config', '--json'], text=True, capture_output=True, timeout=45)
        assert r.returncode == 0, r.stderr
        return json.loads(r.stdout)

    receipt = {'comment': token, 'host': args.host, 'job_id': None}
    (out/'pending-estimate-live-receipt.json').write_text(json.dumps(receipt, indent=2)+'\n')
    try:
        shell = 'umask 077; for key in ${!SBATCH_@}; do unset "$key" || exit 70; done; exec sbatch "$@"'
        r = remote(['bash', '-lc', shell, 'perch-estimate-test', '--parsable', '--begin=now+30minutes',
                    '--nodes=1', '--ntasks=1', '--cpus-per-task=1', '--mem=64M', '--time=00:01:00',
                    '--job-name=perch-estimate-smoke', '--comment='+token,
                    '--output=/dev/null', '--error=/dev/null'], '#!/bin/sh\ntrue\n')
        match = re.fullmatch(r'([1-9][0-9]*)(?:;[A-Za-z0-9_.-]+)?\s*', r.stdout)
        if match:
            job_id = match[1]
            receipt['job_id'] = job_id
            (out/'pending-estimate-live-receipt.json').write_text(json.dumps(receipt, indent=2)+'\n')
        assert r.returncode == 0 and job_id, 'Submission unconfirmed; do not resubmit. Comment: '+token
        raw = queued(job_id)
        assert len(raw['jobs']) == 1 and raw['jobs'][0]['comment'] == token
        observed = raw['jobs'][0]
        assert 'PENDING' in observed['job_state']
        expected = observed['start_time']
        data = cli('job')
        j = data['data']
        assert j['state'] == 'PENDING' and j['started_at'] is None
        assert bool(j['estimated_start_at']) == (expected['set'] and not expected['infinite'] and expected['number'] > 0)
        (out/'pending-estimate-live-job.json').write_text(json.dumps(data, indent=2)+'\n')
        s = Session(['--host', args.host, '--no-config', '--reduced-motion'], 120, 38)
        try:
            s.wait('● connected', timeout=30)
            s.key('/job '+job_id+'\r')
            s.wait('perch-estimate-smoke')
            s.wait('Scheduler: BeginTime')
            s.wait('Tentative start:' if j['estimated_start_at'] else 'No start estimate reported')
            s.screenshot('pending-estimate-live')
            s.resize(48, 24)
            s.wait('Waiting for its earliest start')
            s.screenshot('pending-estimate-live-compact')
            s.close()
        except BaseException:
            s.abort()
            raise
        print(json.dumps({'pending_inspection':'passed', 'job_id':job_id,
                          'started_at':j['started_at'], 'estimated_start_at':j['estimated_start_at']}))
    finally:
        # Recover only this invocation's exact token if acceptance was ambiguous.
        if not job_id:
            matches = [j for j in queued()['jobs'] if j.get('comment') == token]
            if len(matches) == 1:
                job_id = str(matches[0]['job_id'])
                receipt['job_id'] = job_id
                (out/'pending-estimate-live-receipt.json').write_text(json.dumps(receipt, indent=2)+'\n')
            else:
                print('No unique queued receipt recovered; submission was not retried. Comment:', token)
        if job_id:
            cancelled = remote(['scancel', job_id])
            assert cancelled.returncode == 0, 'Cleanup cancellation failed for '+job_id+': '+cancelled.stderr
            assert not queued(job_id)['jobs'], 'Test job remains queued after cancellation: '+job_id
            receipt['cancelled'] = True
            (out/'pending-estimate-live-receipt.json').write_text(json.dumps(receipt, indent=2)+'\n')
            print('Cancelled and verified absent from queue:', job_id)


if __name__ == '__main__':
    main()
