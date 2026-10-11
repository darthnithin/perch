#!/usr/bin/env python3
"""Retained logs through production adapter + fake SSH + real PTY; no network."""
import json
import pathlib
import sys
import tempfile

from test_failure_terminal import FAKE_SSH
from test_terminal import ROOT, Session


LOG_READ = r'''
if len(remote) >= 7 and remote[3] == "perch-log":
    import json
    old, offset = remote[5], int(remote[6])
    with open(os.environ["PERCH_TEST_LOG_READS"], "a") as record:
        record.write(json.dumps({"identity": old, "offset": offset, "state": state})+"\n")
    if state == "logfail":
        print("Synthetic log permission denied", file=sys.stderr)
        sys.exit(1)
    data = pathlib.Path(os.environ["PERCH_TEST_LOG"]).read_bytes()
    reset = old != "1:2" or offset > len(data)
    start = max(0, len(data)-262144) if reset else offset
    sys.stdout.write(f"1:2 {len(data)} {start} {int(reset)}\n")
    sys.stdout.flush()
    sys.stdout.buffer.write(data[start:start+262144])
    sys.exit(0)
'''


def main():
    with tempfile.TemporaryDirectory(prefix='perch-log-retention-') as directory:
        folder = pathlib.Path(directory)
        state, data, reads = (folder/name for name in ['state', 'log', 'reads'])
        state.write_text('ok')
        original = ''.join(f'iteration {i:03d}: checkpoint written\n' for i in range(60))
        data.write_text(original)
        ssh = folder/'ssh'
        ssh.write_text('#!'+sys.executable+'\n'+FAKE_SSH.replace(
            'lines = remote[2].splitlines()', LOG_READ+'\nlines = remote[2].splitlines()'))
        ssh.chmod(0o700)
        env = {'PATH': str(folder)+':/usr/bin:/bin', 'PERCH_TEST_STATE': str(state),
               'PERCH_TEST_LOG': str(data), 'PERCH_TEST_LOG_READS': str(reads),
               'PERCH_TEST_FIXTURES': str(ROOT/'testdata/hummingbird')}
        s = Session(['--host', 'perch-test.invalid', '--no-config', '--reduced-motion'], 80, 24, env=env)
        try:
            s.wait('simulation-sweep')
            s.wait('● connected')
            s.key('l')
            s.wait('iteration 059')
            s.key('\x1b[H')
            s.wait('paused · End latest')
            s.wait('iteration 000')
            state.write_text('logfail')
            s.pump(2.2)
            s.key('r')
            s.wait('stale · r retry')
            s.wait('Last read failed:')
            assert 'iteration 000' in s.text and 'Unable to read this view' not in s.text
            assert 'Synthetic log permission denied' in s.text
            record = json.loads(reads.read_text().splitlines()[-1])
            assert record['identity'] == '1:2' and record['offset'] == len(original.encode())
            s.screenshot('logs-stale-medium')
            s.resize(48, 18)
            s.wait('stale · r retry')
            s.wait('iteration 000')
            s.screenshot('logs-stale-compact')
            s.resize(80, 24)
            data.write_text(original+'iteration 060: recovery checkpoint\n')
            state.write_text('ok')
            s.pump(2.2)
            s.key('r')
            s.wait('paused · End latest')
            assert 'iteration 000' in s.text and 'Last read failed' not in s.text
            s.key('\x1b[F')
            s.wait('iteration 060')
            s.wait('following')
            s.screenshot('logs-recovered-follow')
            # A confirmed truncation supersedes the retained old file.
            state.write_text('logfail')
            s.pump(2.2)
            s.key('r')
            s.wait('stale · r retry')
            assert 'iteration 060' in s.text
            data.write_text('')
            state.write_text('ok')
            s.pump(2.2)
            s.key('r')
            s.wait('exists and is empty')
            s.wait('following')
            assert 'iteration 060' not in s.text
            s.close()
        except BaseException:
            print(s.text)
            s.screenshot('logs-retention-failure')
            s.abort()
            raise
    print('Log retention PTY passed: failed read retains output and cursor, paused position, '
          'stale/error labels, minimum-size layout, incremental recovery, follow intent, '
          'confirmed truncation and terminal restoration; synthetic SSH only.')


if __name__ == '__main__':
    main()
