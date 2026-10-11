#!/usr/bin/env python3
"""Opt-in native Linux release smoke test in a fresh bounded Slurm job.

Stages a binary and two standard-library harnesses in a private home directory;
installs nothing. Never accepts an existing job or retries submission. An
ambiguous submission leaves the stage intact for inspection using its comment.
"""
import argparse
import hashlib
import io
import json
import os
from pathlib import Path, PurePosixPath
import re
import secrets
import shlex
import subprocess
import tarfile
import time

ROOT = Path(__file__).resolve().parents[1]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--host', required=True)
    parser.add_argument('--binary', type=Path, required=True)
    parser.add_argument('--version', required=True)
    parser.add_argument('--report', type=Path, required=True)
    parser.add_argument('--submit', action='store_true')
    args = parser.parse_args()
    if not args.submit:
        parser.error('--submit is required')
    if not re.fullmatch(r'[A-Za-z0-9_.@-]+', args.host) or args.host.startswith('-'):
        parser.error('invalid SSH destination')
    if not re.fullmatch(r'[A-Za-z0-9._-]+', args.version):
        parser.error('invalid version')
    from test_release_runtime import architecture
    assert architecture(args.binary) == {'format': 'ELF', 'architecture': 'amd64'}
    binary_hash = hashlib.sha256(args.binary.read_bytes()).hexdigest()
    os.umask(0o077)
    args.report.parent.mkdir(parents=True, exist_ok=True)
    with args.report.open('x') as out:
        out.write('{}\n')
    token = 'perch-release-' + secrets.token_hex(12)
    report = {'host': args.host, 'version': args.version, 'binary_sha256': binary_hash,
              'comment': token, 'stage': 'preparing',
              'scope': 'New offline demo/CLI/MCP/PTY smoke job; one CPU, 256 MiB, three minutes; no installation'}

    def save():
        temp = args.report.with_suffix('.pending')
        temp.write_text(json.dumps(report, indent=2) + '\n')
        temp.replace(args.report)

    def remote(argv, stdin=None):
        result = subprocess.run(['ssh', '-T', '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=8',
                                 '-o', 'ServerAliveInterval=5', '-o', 'ServerAliveCountMax=2',
                                 '--', args.host, shlex.join(argv)], input=stdin,
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=35)
        if result.returncode:
            raise RuntimeError(result.stderr.decode(errors='replace')[-2000:])
        return result.stdout.decode()

    def clean(argv):
        script = 'set -e; for v in ${!SBATCH_@} ${!SQUEUE_@}; do unset "$v"; done; unset SLURM_CLUSTERS; exec "$@"'
        return remote(['env', '-u', 'BASH_ENV', '-u', 'ENV', '/bin/bash', '--noprofile',
                       '--norc', '-c', script, 'perch-release-check', *argv])

    save()
    try:
        assert remote(['uname', '-m']).strip() == 'x86_64'
        home = remote(['printenv', 'HOME']).strip()
        assert home.startswith('/') and '\n' not in home
        stage = remote(['mktemp', '-d', home + '/.perch-release-check.XXXXXXXX']).strip()
        assert PurePosixPath(stage).parent == PurePosixPath(home)
        assert PurePosixPath(stage).name.startswith('.perch-release-check.')
        report['remote_stage'] = stage
        save()
        payload = io.BytesIO()
        files = {'perch': args.binary.read_bytes(),
                 'test_release_runtime.py': (ROOT/'tools/test_release_runtime.py').read_bytes(),
                 'test_mcp.py': (ROOT/'tools/test_mcp.py').read_bytes(),
                 'run.sh': ('#!/bin/sh\nset -eu\nexport PATH=/usr/bin:/bin\nexec /usr/bin/python3 '
                            'test_release_runtime.py ./perch --version ' + shlex.quote(args.version) + '\n').encode()}
        with tarfile.open(fileobj=payload, mode='w') as tar:
            for name, content in files.items():
                entry = tarfile.TarInfo(name)
                entry.size = len(content)
                entry.mode = 0o700 if name in ('perch', 'run.sh') else 0o600
                tar.addfile(entry, io.BytesIO(content))
        remote(['tar', '-xf', '-', '-C', stage], payload.getvalue())
        assert remote(['sha256sum', stage + '/perch']).split()[0] == binary_hash
        report['stage'] = 'submission attempted'
        save()
        receipt = clean(['sbatch', '--parsable', '--job-name=perch-release-check', '--comment=' + token,
                         '--nodes=1', '--ntasks=1', '--cpus-per-task=1', '--mem=256M', '--time=3',
                         '--export=NONE', '--chdir=' + stage, '--output=' + stage + '/result.json',
                         '--error=' + stage + '/stderr.txt', stage + '/run.sh']).strip()
        assert re.fullmatch(r'[1-9][0-9]*(;[A-Za-z0-9_-]+)?', receipt), receipt
        job = receipt.split(';')[0]
        report.update(job_id=job, stage='submitted')
        save()
        deadline = time.monotonic() + 300
        while True:
            state = clean(['squeue', '--noheader', '--local', '--jobs=' + job, '--format=%i|%k|%T']).strip()
            if not state:
                break
            for line in state.splitlines():
                fields = line.strip().split('|')
                assert fields[0] == job and fields[1] == token, 'probe identity changed'
            report['last_queue_observation'] = state
            save()
            if time.monotonic() > deadline:
                raise TimeoutError('Observation deadline; job may still be queued. No resubmission or stage removal.')
            time.sleep(3)
        report.update(stage='collecting', queue_absent=True)
        save()
        accounting = remote(['sacct', '--noheader', '--parsable2', '--jobs=' + job,
                             '--format=JobIDRaw,State,ExitCode,Elapsed']).strip()
        report['accounting'] = accounting
        report['stderr'] = remote(['cat', stage + '/stderr.txt'])
        report['runtime'] = json.loads(remote(['cat', stage + '/result.json']))
        assert report['runtime']['binary_sha256'] == binary_hash
        assert report['runtime']['host_system'] == 'Linux'
        assert report['runtime']['host_machine'] == 'x86_64'
        assert any(line.split('|')[:3] == [job, 'COMPLETED', '0:0'] for line in accounting.splitlines()), accounting
        assert not report['stderr'], report['stderr']
        report['stage'] = 'passed'
        save()
        # Remove only the exact files staged by this run, after terminal success.
        remote(['rm', '--', *[stage + '/' + name for name in [*files, 'result.json', 'stderr.txt']]])
        # Python may have created its own bounded bytecode cache.
        remote(['/bin/sh', '-c', 'if [ -d "$1/__pycache__" ]; then rm -f -- "$1"/__pycache__/test_release_runtime.*.pyc; rmdir -- "$1/__pycache__"; fi', 'cleanup', stage])
        remote(['rmdir', '--', stage])
        report['remote_stage_removed'] = True
        save()
        print(json.dumps(report, indent=2))
    except BaseException as error:
        report['error'] = str(error)
        save()
        raise


if __name__ == '__main__':
    main()
