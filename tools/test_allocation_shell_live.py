#!/usr/bin/env python3
"""Opt-in, bounded allocation-shell experiment. Creates only its own small job.

Requires --submit and a fresh --report path. Never attaches to an existing user
workload. Saves the receipt before testing and cancels only a matching owned
probe job. Unknown submission outcomes retain their directory for reconciliation.
"""
import argparse
import errno
import fcntl
import hashlib
import json
import os
from pathlib import Path
import pty
import re
import select
import shlex
import signal
import struct
import subprocess
import termios
import time

ROOT = Path(__file__).resolve().parents[1]


class AllocationWaitExpired(TimeoutError):
    """The scheduler did not provide an allocation; shell behavior is untested."""


def tui_session(host, job, nodes=None, gpu=False):
    # Optional visual harness uses the project's .venv, never Terminal.app.
    from test_terminal import Session
    from test_footer import activate,footer
    from test_hover import move,env
    session=Session(['--host',host,'--no-config','--reduced-motion'],120,36,env=env('ghostty'))
    def wait_raw(marker,start):
        deadline=time.monotonic()+20
        while marker not in session.raw[start:]:
            session.pump()
            assert session.process.poll() is None and time.monotonic()<deadline,bytes(session.raw[-2000:])
    visits=[]
    nodes=nodes or [None]
    try:
        session.wait('● connected',timeout=30)
        session.key('/find '+job+'\r');session.wait('Filter: '+job)
        activate(session,'enter inspect');session.wait('o shell')
        for index,node in enumerate(nodes):
            activate(session,'o shell');session.wait('enter open shell',timeout=30)
            if node is not None:
                # Every new preview starts at the first allocated node.
                session.wait('Node       '+nodes[0],timeout=30)
                for expected in nodes[1:index+1]:
                    activate(session,'n node')
                    session.wait('Node       '+expected,timeout=30)
            suffix='-node-'+str(index+1) if len(nodes)>1 else ''
            session.screenshot('shell-ui-hummingbird-preview'+suffix)
            start=len(session.raw)
            move(session,*footer(session,'enter open shell'));activate(session,'enter open shell')
            wait_raw(b'bash-',start)
            prefix=bytes(session.raw[start:])
            assert b'\x1b[?1049l' in prefix and b'\x1b[?1003l' in prefix and b'\x1b]22;default\x07' in prefix
            session.key('perch_tty=no; if test -t 0 && test -t 1; then perch_tty=yes; fi; printf "\\nPERCH_TUI job=%s step=%s node=%s tty=%s\\n" "$SLURM_JOB_ID" "$SLURM_STEP_ID" "$(hostname -s)" "$perch_tty"\r')
            wait_raw(b'\r\nPERCH_TUI job=',start)
            match=re.search(rb'\r\nPERCH_TUI job=(\d+) step=(\d+) node=([\w.-]+) tty=yes\r?\n',session.raw[start:])
            assert match and match[1].decode()==job,bytes(session.raw[-2000:])
            actual_node=match[3].decode()
            if node is not None: assert actual_node==node, (actual_node,node)
            gpu_observation = None
            if gpu:
                # Observe only GPU names/visibility in this owned probe's CPU
                # step. Do not inspect processes or assume that visibility
                # proves device ownership, isolation or CUDA usability.
                gpu_start=len(session.raw)
                session.key('printf "\\nPERCH_GPU_ENV cuda=%s step=%s\\n" "${CUDA_VISIBLE_DEVICES-unset}" "${SLURM_STEP_GPUS-unset}"; timeout 5s nvidia-smi --query-gpu=name --format=csv,noheader; perch_gpu_result=$?; printf "\\nPERCH_GPU_END %s\\n" "$perch_gpu_result"\r')
                wait_raw(b'\r\nPERCH_GPU_END ',gpu_start)
                deadline=time.monotonic()+5
                while not re.search(rb'\r\nPERCH_GPU_END [0-9]+\r?\n',session.raw[gpu_start:]):
                    session.pump()
                    assert session.process.poll() is None and time.monotonic()<deadline, 'GPU observation did not finish'
                payload=bytes(session.raw[gpu_start:])
                gpu_observation=parse_gpu_observation(payload)
            session.key('sleep 30\r');session.pump(.3);session.key('\x03')
            session.key('printf "\\nPERCH_TUI_INTERRUPT_OK\\n"\r')
            wait_raw(b'\r\nPERCH_TUI_INTERRUPT_OK\r\n',start)
            session.key('exit\r');session.wait('Shell closed',timeout=20)
            session.wait(job+' · perch-shell-probe')
            session.screenshot('shell-ui-hummingbird-returned'+suffix)
            visit={'node':actual_node,'step_id':match[2].decode(),'stdin_stdout_tty':True,'ctrl_c_returns_to_shell':True}
            if gpu_observation is not None: visit['gpu_observation']=gpu_observation
            visits.append(visit)
        session.key('\x1b');session.wait('Filter: '+job)
        session.close()
        return {'job_id':job,'visits':visits,'mouse_node_choice':len(nodes)>1,
                'mouse_preview_and_open':True,'pointer_reset_before_shell':True,
                'returned_to_same_inspector':True,'filter_retained':True,
                'terminal_attributes_restored':True}
    except BaseException:
        session.screenshot('shell-ui-hummingbird-failure')
        session.abort();raise


def parse_gpu_observation(payload):
    match=re.search(rb'\r\nPERCH_GPU_ENV cuda=([^\r\n]*) step=([^\r\n]*)\r?\n(.*?)\r\nPERCH_GPU_END ([0-9]+)\r?\n',payload,re.S)
    if not match: raise AssertionError('GPU observation markers are incomplete')
    output=match[3].decode(errors='replace').strip()
    if len(output)>4096: raise AssertionError('GPU observation exceeded bound')
    return {'step_requests_gpus':False,'cuda_visible_devices':match[1].decode(),
            'slurm_step_gpus':match[2].decode(),'nvidia_smi_exit':int(match[4]),
            'nvidia_smi_output':output,
            'scope':'Visibility only in a CPU-only shell step; not proof of GPU ownership, isolation or CUDA execution'}


def probe_resource_args(nodes, gpu):
    if gpu and nodes != 1: raise ValueError('GPU probe requires exactly one node')
    args=['--nodes',str(nodes),'--tasks-per-node','1','--cpus','1','--mem','512M' if gpu else '128M','--time','3m']
    if gpu: args += ['--partition','96x24gpu4','--gpus','1']
    return args


def shell_session(ssh, remote, job, disconnect=False, perch=False, expiry=False):
    master, slave = pty.openpty()
    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', 24, 100, 0, 0))
    process = subprocess.Popen([*ssh, *([remote] if remote is not None else [])], stdin=slave, stdout=slave,
                               stderr=slave, start_new_session=True)
    raw = bytearray()
    def pump():
        if select.select([master], [], [], .1)[0]:
            try: raw.extend(os.read(master, 65536))
            except OSError as error:
                if error.errno != errno.EIO: raise
        assert len(raw) < 128*1024, 'probe output exceeded bound'
    def wait(needle, timeout=20):
        deadline = time.monotonic()+timeout
        while needle not in raw:
            pump()
            if process.poll() is not None or time.monotonic() >= deadline:
                raise AssertionError('shell did not reach '+repr(needle)+': '+raw.decode(errors='replace')[-4000:])
    try:
        wait(b'bash-' if perch else b'PERCH_PROBE> ')
        # Test descriptors in the interactive shell. Command substitution would
        # replace stdout with a pipe and incorrectly report a non-TTY session.
        os.write(master, b'perch_tty=no; if test -t 0 && test -t 1; then perch_tty=yes; fi; printf "\\nPERCH_REPORT job=%s step=%s node=%s origin=%s tty=%s\\n" "$SLURM_JOB_ID" "$SLURM_STEP_ID" "$(hostname -s)" "${PERCH_ORIGIN-unset}" "$perch_tty"; grep Cpus_allowed_list /proc/self/status\n')
        wait(b'\r\nPERCH_REPORT job=')
        wait(b'Cpus_allowed_list:\t')
        match = re.search(rb'\r\nPERCH_REPORT job=(\d+) step=(\d+) node=([\w.-]+) origin=([\w-]+) tty=(\w+)\r?\n', raw)
        assert match and match[1].decode() == job and match[4] == (b'unset' if perch else b'login-step') and match[5] == b'yes', bytes(raw[-4000:])
        result = {'job_id': job, 'step_id': match[2].decode(), 'node': match[3].decode(),
                  'environment_origin': match[4].decode(), 'stdin_stdout_tty': True}
        affinity = re.search(rb'Cpus_allowed_list:\t([^\r\n]+)', raw)
        result['cpu_affinity'] = affinity[1].decode()
        if disconnect:
            process.terminate()
        elif not expiry:
            os.write(master, b'sleep 30\n')
            # Let Bash start the child before sending the terminal interrupt.
            for _ in range(3): pump()
            os.write(master, b'\x03')
            os.write(master, b'printf "\\nPERCH_AFTER_INTERRUPT\\n"\n')
            wait(b'\r\nPERCH_AFTER_INTERRUPT\r\n')
            result['ctrl_c_returns_to_shell'] = True
            os.write(master, b'exit\n')
        deadline = time.monotonic()+(90 if expiry else 12)
        while process.poll() is None and time.monotonic() < deadline: pump()
        assert process.poll() is not None, 'SSH session did not exit'
        result['ssh_exit'] = process.returncode
        if not disconnect and not expiry: assert process.returncode == 0, result
        if expiry: assert process.returncode != 0, result
        return result
    finally:
        if process.poll() is None:
            process.terminate()
            deadline = time.monotonic()+8
            while process.poll() is None and time.monotonic()<deadline: pump()
            if process.poll() is None:
                os.killpg(process.pid,signal.SIGKILL)
                process.wait(timeout=5)
        os.close(master); os.close(slave)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--host', required=True)
    parser.add_argument('--report', type=Path, required=True)
    parser.add_argument('--submit', action='store_true')
    parser.add_argument('--wait-seconds', type=int, default=70, help='bounded queue wait, 30–900 seconds; expiry never resubmits')
    parser.add_argument('--nodes', type=int, choices=[1,2], default=1, help='two nodes additionally verify CLI selection and TUI node cycling')
    parser.add_argument('--perch-shell', action='store_true', help='exercise the production CLI including SIGTERM and step expiry')
    parser.add_argument('--perch-tui', action='store_true', help='exercise production TUI mouse entry, foreground shell and return; requires .venv')
    parser.add_argument('--gpu', action='store_true', help='one GPU on Hummingbird 96x24gpu4; requires --nodes=1 --perch-tui; observes the existing CPU-only shell policy')
    args = parser.parse_args()
    if not 30 <= args.wait_seconds <= 900: parser.error('--wait-seconds must be between 30 and 900')
    if not args.submit: parser.error('explicit --submit is required')
    if args.nodes==2 and not args.perch_tui: parser.error('--nodes=2 requires --perch-tui')
    if args.gpu and (args.nodes!=1 or not args.perch_tui): parser.error('--gpu requires --nodes=1 --perch-tui')
    if args.perch_shell and args.perch_tui: parser.error('choose one production entry point per probe')
    if args.report.exists(): parser.error('report already exists; inspect it before another probe')
    if args.host.startswith('-') or any(c.isspace() for c in args.host): parser.error('invalid host')
    os.umask(0o077)
    ssh = ['ssh', '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=8', '-o', 'ServerAliveInterval=5', '-o', 'ServerAliveCountMax=2']
    def remote(command, timeout=20):
        result = subprocess.run([*ssh, '-T', '--', args.host, command], capture_output=True, text=True, timeout=timeout)
        if result.returncode: raise RuntimeError(result.stderr[-3000:])
        return result.stdout.strip()
    scope = 'owned synthetic one-node allocation, 1 CPU, 512 MiB, 1 GPU on 96x24gpu4, three minutes; CPU-only shell step' if args.gpu else f'owned synthetic {args.nodes}-node allocation, 1 CPU and 128 MiB per node, three minutes; no GPU'
    report = {'started_at':time.strftime('%Y-%m-%dT%H:%M:%SZ',time.gmtime()), 'queue_wait_limit_seconds':args.wait_seconds, 'host': args.host, 'stage': 'preparing', 'scope': scope}
    if args.gpu:
        report['gpu_acceptance_scope']='CPU-only shell round trip inside a GPU allocation; GPU name/visibility observation only. CUDA execution and GPU sharing are not certified.'
    report['binary_sha256'] = hashlib.sha256((ROOT/'bin/perch').read_bytes()).hexdigest()
    def save():
        args.report.parent.mkdir(parents=True, exist_ok=True)
        args.report.write_text(json.dumps(report, indent=2)+'\n')
        print(report['stage'], flush=True)
    user = remote('id -un')
    workdir = remote('umask 077; mktemp -d "$HOME/.perch-shell-probe.XXXXXXXX"')
    assert workdir.startswith('/') and re.fullmatch(r'\.perch-shell-probe\.[A-Za-z0-9]+', Path(workdir).name)
    report['remote_workdir'] = workdir
    save()
    job = None
    token = None
    def queue():
        text = remote(shlex.join(['squeue', '--jobs='+job, '--noheader', '--format=%i|%u|%T|%j|%N|%k|%R']))
        if not text: return None
        fields = text.split('|')
        assert len(fields) == 7 and fields[0] == job and fields[1] == user and fields[3] == 'perch-shell-probe' and fields[5] == token, 'probe identity changed; refusing action'
        return fields
    try:
        report['stage'] = 'submitting-once'; save()
        result = subprocess.run([str(ROOT/'bin/perch'), 'run', '--host', args.host, '--no-config', '--name', 'perch-shell-probe', '--workdir', workdir,
                                 *probe_resource_args(args.nodes,args.gpu), '--submit', '--json', '--',
                                 '/bin/bash', '-c', 'export PERCH_ORIGIN=batch-only; exec sleep 150'], capture_output=True, text=True, timeout=55)
        report['submission_stdout'] = result.stdout
        report['submission_stderr'] = result.stderr
        save()
        assert result.returncode == 0, 'submission outcome requires receipt/journal review; never automatically retry'
        receipt = json.loads(result.stdout)['receipt']
        job, token = receipt['job_id'], receipt['submission_token']
        assert re.fullmatch(r'[1-9][0-9]*', job)
        report['job_id'], report['token'] = job, token
        report['stage'] = 'waiting-for-probe-allocation'; save()
        deadline = time.monotonic()+args.wait_seconds
        while True:
            row = queue()
            assert row is not None, 'probe left queue before shell experiment'
            if report.get('last_queue_state')!=row[2] or report.get('last_queue_reason')!=row[6]:
                report['last_queue_state'],report['last_queue_reason']=row[2],row[6]
                save()
            if row[2] == 'RUNNING': break
            if time.monotonic() >= deadline:
                raise AllocationWaitExpired(f'No allocation within {args.wait_seconds}s; last state {row[2]}, reason {row[6]}. No shell was opened.')
            time.sleep(2)
        nodes=remote(shlex.join(['scontrol','show','hostnames',row[4]])).splitlines()
        assert len(nodes)==args.nodes and len(set(nodes))==args.nodes
        assert all(re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9.-]*',n) for n in nodes)
        report['allocated_nodes']=nodes
        if args.gpu:
            allocation=remote(shlex.join(['scontrol','--oneliner','show','job',job]))
            allocated=re.search(r'(?:^|\s)AllocTRES=(\S+)',allocation)
            assert allocated and 'gres/gpu=1' in allocated[1].split(','), 'owned GPU allocation not confirmed'
            report['allocated_tres']=allocated[1]
        node=nodes[0]
        if args.nodes==2:
            report['stage']='testing-CLI-node-selection';save()
            report['node_previews']=[]
            for candidate in nodes:
                preview=subprocess.run([str(ROOT/'bin/perch'),'shell',job,'--host',args.host,'--no-config','--node',candidate,'--time','1m','--json'],capture_output=True,text=True,timeout=30)
                assert preview.returncode==0,preview.stderr
                plan=json.loads(preview.stdout)['plan']
                assert plan['node']==candidate and plan['allocated_nodes']==nodes and plan['allocation_id']==int(job)
                report['node_previews'].append(plan)
            forbidden=subprocess.run([str(ROOT/'bin/perch'),'shell',job,'--host',args.host,'--no-config','--node','perch-not-allocated.invalid','--json'],capture_output=True,text=True,timeout=30)
            assert forbidden.returncode!=0 and 'does not belong' in forbidden.stderr,forbidden.stderr
            report['outside_node_refused']=True
            launch=[str(ROOT/'bin/perch'),'shell',job,'--host',args.host,'--no-config','--node',nodes[1],'--time','1m','--open']
            report['cli_second_node']=shell_session(launch,None,job,perch=True)
            assert report['cli_second_node']['node']==nodes[1]
            assert queue()[2]=='RUNNING','CLI shell exit changed parent allocation'
            save()
        if args.perch_tui:
            report['stage']='testing-TUI-shell-round-trip';save()
            report['tui']=tui_session(args.host,job,nodes,gpu=args.gpu)
            assert queue()[2]=='RUNNING','TUI shell exit changed parent allocation'
            report['stage']='shell-probe-passed';save()
            return
        flags = ['srun', '--jobid='+job, '--overlap', '--exact', '--nodes=1', '--ntasks=1', '--cpus-per-task=1', '--mem=64M', '--gres=none', '--mpi=none', '--immediate=5', '--time=1', '--nodelist='+node, '--chdir='+workdir, '--pty', '/bin/bash', '--noprofile', '--norc', '-i']
        command = shlex.join(['env', 'PERCH_ORIGIN=login-step', 'PS1=PERCH_PROBE> ', *flags])
        report['step_arguments'] = flags
        report['stage'] = 'testing-interactive-step'; save()
        launch = [*ssh, '-tt', '--', args.host]
        if args.perch_shell:
            preview = subprocess.run([str(ROOT/'bin/perch'), 'shell', job, '--host', args.host, '--no-config', '--time', '1m', '--json'], capture_output=True, text=True, timeout=30)
            assert preview.returncode == 0, preview.stderr
            report['perch_preview'] = json.loads(preview.stdout)
            assert report['perch_preview']['plan']['allocation_id'] == int(job)
            launch = [str(ROOT/'bin/perch'), 'shell', job, '--host', args.host, '--no-config', '--time', '1m', '--open']
            command = None
        report['normal_exit'] = shell_session(launch, command, job, perch=args.perch_shell)
        assert queue()[2] == 'RUNNING', 'shell exit changed parent allocation'
        report['stage'] = 'testing-SSH-disconnect'; save()
        report['disconnect'] = shell_session(launch, command, job, disconnect=True, perch=args.perch_shell)
        assert queue()[2] == 'RUNNING', 'SSH disconnect changed parent allocation'
        step = job+'.'+report['disconnect']['step_id']
        deadline = time.monotonic()+12
        while True:
            state = remote(shlex.join(['sacct', '--jobs='+step, '--noheader', '--parsable2', '--format=JobIDRaw,State,ExitCode']))
            report['disconnected_step_state'] = state
            if state and all('RUNNING' not in line and 'PENDING' not in line for line in state.splitlines()): break
            assert time.monotonic() < deadline, 'disconnected step not confirmed terminal; parent cleanup will cancel it'
            time.sleep(1)
        if args.perch_shell:
            report['stage'] = 'testing-one-minute-step-expiry'; save()
            report['expiry'] = shell_session(launch, command, job, perch=True, expiry=True)
            assert queue()[2] == 'RUNNING', 'step expiry changed parent allocation'
            step = job+'.'+report['expiry']['step_id']
            report['expired_step_state'] = remote(shlex.join(['sacct', '--jobs='+step, '--noheader', '--parsable2', '--format=JobIDRaw,State,ExitCode']))
            deadline=time.monotonic()+20
            while not report['expired_step_state'] or 'RUNNING' in report['expired_step_state']:
                assert time.monotonic()<deadline,report['expired_step_state']
                time.sleep(1)
                report['expired_step_state'] = remote(shlex.join(['sacct', '--jobs='+step, '--noheader', '--parsable2', '--format=JobIDRaw,State,ExitCode']))
            assert any(s in report['expired_step_state'] for s in ['TIMEOUT','CANCELLED','FAILED']), report['expired_step_state']
        report['stage'] = 'shell-probe-passed'; save()
    except BaseException as error:
        report['stage'] = 'not-exercised' if isinstance(error,AllocationWaitExpired) else 'failed'
        report['error'] = str(error)[-6000:]
        save()
        raise
    finally:
        if job is not None:
            row = queue()
            if row is not None:
                remote(shlex.join(['scancel', job]))
                deadline = time.monotonic()+70
                while queue() is not None:
                    if time.monotonic() >= deadline: raise RuntimeError('probe cancellation not yet confirmed; preserve directory')
                    time.sleep(1)
            report['queue_absence_verified'] = True
            report['accounting'] = remote(shlex.join(['sacct', '--jobs='+job, '--noheader', '--parsable2', '--format=JobIDRaw,State,ExitCode,Elapsed,AllocCPUS,ReqMem,NodeList']))
            remote('rm -rf -- '+shlex.quote(workdir)+'; test ! -e '+shlex.quote(workdir))
            report['remote_directory_removed'] = True
            save()


if __name__ == '__main__': main()
