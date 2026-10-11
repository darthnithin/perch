#!/usr/bin/env python3
"""Portable, standard-library-only acceptance of an extracted release binary.

Exercises offline CLI, MCP, and a real PTY. Does not render pixels or establish
native terminal/font acceptance. HOME/config/cache stay in a temporary directory.
"""
import argparse
import errno
import fcntl
import hashlib
import json
import os
from pathlib import Path
import platform
import pty
import re
import select
import signal
import struct
import subprocess
import sys
import tempfile
import termios
import time

ANSI = re.compile(rb'\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b\[[0-?]*[ -/]*[@-~]')


def architecture(binary):
    with binary.open('rb') as f:
        header = f.read(20)
    if header.startswith(b'\x7fELF'):
        machine = struct.unpack('<H' if header[5] == 1 else '>H', header[18:20])[0]
        return {'format': 'ELF', 'architecture': {62: 'amd64', 183: 'arm64'}.get(machine, str(machine))}
    if header[:4] == b'\xcf\xfa\xed\xfe':
        machine = struct.unpack('<I', header[4:8])[0]
        return {'format': 'Mach-O', 'architecture': {0x1000007: 'amd64', 0x100000C: 'arm64'}.get(machine, str(machine))}
    raise AssertionError('unexpected binary format')


def terminal_check(binary, env, mouse, nrp=False, pointer='css'):
    env = dict(env, TERM_PROGRAM='iTerm.app' if pointer == 'x11' else 'ghostty', TMUX='', STY='')
    master, slave = pty.openpty()
    before = termios.tcgetattr(slave)
    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', 40, 140, 0, 0))
    args = [str(binary), '--demo', '--no-config', '--reduced-motion']
    if nrp:
        args = [str(binary), 'nrp', 'ui', '--demo']
    if not mouse:
        args += ['--no-mouse']
    process = subprocess.Popen(args, stdin=slave, stdout=slave, stderr=slave,
                               start_new_session=True, env=env)
    raw = bytearray()
    answered = set()

    def pump(duration=.1):
        deadline = time.monotonic() + duration
        while time.monotonic() < deadline:
            if not select.select([master], [], [], max(0, min(.05, deadline-time.monotonic())))[0]:
                continue
            try:
                data = os.read(master, 65536)
            except OSError as e:
                if e.errno == errno.EIO:
                    break
                raise
            if not data:
                break
            start = max(0, len(raw)-32)
            raw.extend(data)
            assert len(raw) <= 8*1024*1024, 'terminal output exceeded smoke-test bound'
            for query, response in [(b'\x1b[6n', b'\x1b[1;1R'),
                                    (b'\x1b]11;?', b'\x1b]11;rgb:1c1c/1b1b/1a1a\x1b\\'),
                                    (b'\x1b]10;?', b'\x1b]10;rgb:ecec/e6e6/dfdf\x1b\\')]:
                position = raw.find(query, start)
                if position >= 0 and (query, position) not in answered:
                    answered.add((query, position))
                    os.write(master, response)

    def wait(text, since=0):
        deadline = time.monotonic()+10
        while text.encode() not in ANSI.sub(b'', bytes(raw[since:])):
            pump()
            assert process.poll() is None, 'TUI exited before '+text
            if time.monotonic() > deadline:
                raise AssertionError('TUI did not render '+repr(text)+'; recent demo output: '+repr(ANSI.sub(b'',bytes(raw[-5000:]))))

    def key(value, expected=None):
        since = len(raw)
        os.write(master, value.encode())
        pump(.15)
        if expected:
            wait(expected, since)

    def move(x, y, shape):
        since = len(raw)
        key(f'\x1b[<35;{x+1};{y+1}M')
        sequence = b'\x1b]22;'+shape.encode()+b'\x07'
        deadline = time.monotonic()+5
        while sequence not in raw[since:]:
            pump()
            assert process.poll() is None, 'TUI exited during hover'
            assert time.monotonic() < deadline, 'missing pointer shape '+shape

    try:
        wait('Namespace Jobs' if nrp else 'parameter-sweep')
        assert b'\x1b[?1049h' in raw, 'alternate screen not entered'
        if mouse:
            # Both workspaces put their first tab on row 2 and input at h-5.
            # These are real SGR motion reports, not direct model calls.
            move(3, 2, 'hand2' if pointer == 'x11' else 'pointer')
            move(8, 35, 'xterm' if pointer == 'x11' else 'text')
            if pointer != 'x11':
                move(0, 0, 'default')
        if nrp:
            key('/quotas\r', 'Used / limit')
            key('\x1b', 'Jobs')
            # This portable smoke test reads new output, not an emulated screen.
            # Bubble Tea can retain the unchanged "Namespace " prefix and
            # repaint only "Pods". Full visible-screen assertions live in the
            # pyte-based NRP suite; require newly emitted distinctive text here.
            key('2', 'Pods')
            key('/find retrying-replacement\r')
            key('u', '0.215 cores')
            key('\x1b')
            key('l', 'Containers')
            key('\r', 'Current instance of compute')
            key('p', 'Previous')
        else:
            key('/sort cpus\r', 'Sorted: cpus')
            key('/sort memory asc\r', 'REQ MEM')
            key('/sort default\r', 'ELAPSED')
            key('/actions\r', 'Job actions')
            key('\x1b')
            key('/warnings\r', 'Time warnings')
            key('\x1b')
            key('/activity\r', 'No new activity yet.')
            key('d', 'Finished')
            key('\x1b')
            key('/shell\r', 'A new shell alongside your workload')
            key('\x1b')
            key('l', 'Dataset ready')
            key('f', 'Search retained output')
            # One paste event avoids matching a fragmented series of per-letter
            # redraws in this raw-output harness. Full-screen typing checks are
            # covered separately by the terminal-emulator suite.
            key('\x1b[200~Dataset\x1b[201~', 'Match 1 / 1')
            key('\r', 'previous')
            key('\x1b') # Clear search; remain in output.
            key('\x1b[F', 'following')
            key('\x1b')
            key('s', 'Submitted script')
            key('f', 'Search submitted script')
            key('\x1b[200~python\x1b[201~', 'Match 1 / 1')
            key('\r', 'previous')
            key('\x1b')
            key('\x1b')
            key('2', 'Recent history')
            key('3', 'Allocation is scheduler capacity')
            key('n', 'Node allocations')
            key('g', 'p100')
            key('g', 'node-06')
            key('\x1b', 'Partitions')
            key('/priority\r', 'Queue priority')
            wait('configured weight 0')
            key('\x1b', 'Partitions')
            key('1')
            key('/software cuda\r', 'cuda/12.8.1')
            key('\r', 'module load cuda/12.8.1')
            key('\x1b', 'login environment')
            key('\x1b')
            key('/run\r', 'Prepare a job')
            key('\r', 'enter the command to run')
            # Error focus is on the command; fill it, then the directory.
            key('printf release-check\x1b[Z/scratch/perch-preview\r', 'Review your job')
            key('\x1b')
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', 18, 48, 0, 0))
        os.kill(process.pid, signal.SIGWINCH)
        pump(.2)
        if nrp:
            key('\x1b[F', 'termination state')
        else:
            key('/cancel\r', 'ctrl+x cancel task')
            since = len(raw)
            key('\r')
            assert b'Demo' not in ANSI.sub(b'', bytes(raw[since:])), 'Enter confirmed cancellation'
            key('\x18', 'Demo')
            key('\r')
            key('/last-cancel\r', 'Demo')
            key('\r')
        key('?', 'Keyboard & commands' if nrp else 'Move around')
        key('\x1b')
        key('q')
        deadline = time.monotonic()+5
        while process.poll() is None and time.monotonic() < deadline:
            pump()
        assert process.poll() == 0, 'TUI failed to exit cleanly'
        pump()
        after = termios.tcgetattr(slave)
        pending = getattr(termios, 'PENDIN', 0)
        before[3] &= ~pending
        after[3] &= ~pending
        assert before == after, 'terminal attributes not restored'
        assert b'\x1b[?1049l' in raw and b'\x1b[?25h' in raw, 'screen/cursor not restored'
        if mouse:
            assert b'\x1b[?1003h' in raw and b'\x1b[?1006h' in raw, 'mouse mode not enabled'
            assert b'\x1b[?1003l' in raw and b'\x1b[?1006l' in raw, 'mouse mode not restored'
            reset = b'\x1b]22;'+(b'xterm' if pointer == 'x11' else b'default')+b'\x07'
            assert raw.endswith(reset), 'pointer not restored after terminal shutdown'
        else:
            assert b'\x1b[?1002h' not in raw and b'\x1b[?1003h' not in raw, 'no-mouse enabled capture'
            assert b'\x1b]22;' not in raw, 'no-mouse changed pointer shape'
    finally:
        if process.poll() is None:
            process.kill()
            process.wait(timeout=5)
        os.close(master)
        os.close(slave)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('binary', type=Path)
    parser.add_argument('--version', required=True)
    args = parser.parse_args()
    binary = args.binary.resolve()
    with tempfile.TemporaryDirectory(prefix='perch-release-runtime-') as directory:
        env = dict(os.environ, HOME=directory, XDG_CONFIG_HOME=directory+'/config',
                   XDG_CACHE_HOME=directory+'/cache', TERM='xterm-256color', COLORTERM='truecolor')
        env.pop('NO_COLOR', None)
        result = subprocess.run([str(binary), 'version'], env=env, capture_output=True, text=True, timeout=10)
        assert result.returncode == 0 and result.stdout.strip() == 'perch '+args.version, result.stderr
        cli = {}
        for command in ['jobs', 'history', 'arrays', 'partitions', 'waits', 'nodes', 'priority']:
            result = subprocess.run([str(binary), command, '--demo', '--no-config', '--json'], env=env,
                                    capture_output=True, text=True, timeout=10)
            assert result.returncode == 0 and not result.stderr, (command, result.stderr)
            response = json.loads(result.stdout)
            assert response['source'] == 'demo' and response['data'], command
            if command == 'priority':
                assert response['priority_policy']['fairshare_weight'] == 0
            if command == 'jobs':
                gpu_job = next(j for j in response['data'] if j['id'] == '9231')
                assert gpu_job['gpu_requests'] == [{'scope':'node','type':'','count':1}]
            cli[command] = 'passed'
        result = subprocess.run([str(binary), 'run', '--host=offline.invalid', '--no-config',
                                 '--workdir=/tmp/perch-release', '--array=0-3%2', '--json',
                                 '--', 'echo', 'release check'], env=env,
                                capture_output=True, text=True, timeout=10)
        assert result.returncode == 0 and not result.stderr, result.stderr
        preview = json.loads(result.stdout)
        assert preview['mode'] == 'preview' and preview['plan'] and not preview.get('receipt'), preview
        cli['run_offline_array_preview'] = 'passed'
        result = subprocess.run([str(binary), 'nrp', 'prepare', '--name', 'release-check',
                                 '--namespace', 'offline-check', '--image', 'python:3.13-slim',
                                 '--cpu', '500m', '--memory', '512Mi', '--ephemeral-storage', '1Gi',
                                 '--time', '5m', '--manifest', '--', 'python', '-c', "print('$(HOME)')"],
                                env=env, capture_output=True, text=True, timeout=10)
        assert result.returncode == 0 and not result.stderr, result.stderr
        manifest = json.loads(result.stdout)
        assert manifest['kind']=='Job' and manifest['metadata']['namespace']=='offline-check'
        assert manifest['spec']['activeDeadlineSeconds']==300 and manifest['spec']['backoffLimit']==0
        container = manifest['spec']['template']['spec']['containers'][0]
        assert container['resources']['requests']==container['resources']['limits']
        assert container['args'][1]=="print('$$(HOME)')"
        cli['nrp_offline_manifest'] = 'passed'
        result = subprocess.run([sys.executable, str(Path(__file__).with_name('test_mcp.py')),
                                 '--binary', str(binary)], env=env, capture_output=True, text=True, timeout=30)
        assert result.returncode == 0, result.stdout + result.stderr
        terminal_check(binary, env, True)
        terminal_check(binary, env, True, pointer='x11')
        terminal_check(binary, env, False)
        nrp_cli = {}
        for command in [['jobs'], ['pods'], ['quotas'], ['metrics', 'retrying-replacement'], ['job', 'indexed'], ['pod', 'retrying-replacement'],
                        ['logs', 'retrying-replacement', '--container', 'compute'],
                        ['logs', 'retrying-replacement', '--container', 'compute', '--previous']]:
            result = subprocess.run([str(binary), 'nrp', *command, '--demo', '--json'], env=env,
                                    capture_output=True, text=True, timeout=10)
            assert result.returncode == 0 and not result.stderr, (command, result.stderr)
            response = json.loads(result.stdout)
            assert response['provider'] == 'nautilus' and response['source'] == 'synthetic-demo'
            assert response['data'], command
            if command[0] == 'logs':
                previous = '--previous' in command
                assert response['data']['previous'] == previous
                assert ('Previous' if previous else 'Current')+' instance of compute' in response['data']['text']
            nrp_cli[' '.join(command)] = 'passed'
        terminal_check(binary, env, True, nrp=True)
        terminal_check(binary, env, True, nrp=True, pointer='x11')
        terminal_check(binary, env, False, nrp=True)
    print(json.dumps({'binary': architecture(binary), 'binary_sha256': hashlib.sha256(binary.read_bytes()).hexdigest(), 'host_system': platform.system(),
                      'host_machine': platform.machine(), 'version': args.version,
                      'cli': cli, 'mcp_wire': 'passed', 'pty_mouse': 'passed', 'pty_no_mouse': 'passed',
                      'pointer_protocols': {'hummingbird_css_x11': 'passed', 'nrp_css_x11': 'passed'},
                      'nrp_cli': nrp_cli, 'nrp_pty_mouse': 'passed', 'nrp_pty_no_mouse': 'passed',
                      'checks': ['output', 'retained-output search and follow restoration', 'submitted-script search and return', 'job sorting, resource columns and reset', 'history', 'partitions', 'software', 'module selection and return', 'job draft correction', '48x18 resize', 'help',
                                 'job actions', 'time-warning preferences', 'observed activity',
                                 'node allocations and GPU filter', 'scoped GPU requests', 'priority factors and observed fair-share policy', 'namespace quotas', 'offline NRP Job manifest',
                                 'Pod CPU/memory metrics',
                                 'allocation shell review', 'compact cancellation review and demo receipt',
                                 'session receipt reopening', 'offline array launch preview',
                                 'exit status', 'termios restoration', 'alternate screen', 'cursor', 'mouse cleanup',
                                 'hover pointer and text shapes', 'pointer cleanup after shutdown'],
                      'visual_acceptance': 'not tested'}, indent=2))


if __name__ == '__main__':
    main()
