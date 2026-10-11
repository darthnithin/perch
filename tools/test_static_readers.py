#!/usr/bin/env python3
"""Large script/module readers through production adapters and real PTYs."""
import pathlib
import sys
import tempfile

from test_failure_terminal import FAKE_SSH
from test_terminal import ROOT, Session


REPLIES = '''
if remote[:3] == ["scontrol", "write", "batch_script"] or (remote[:2] == ["bash", "-lc"] and "perch-module" in remote):
    sys.stdout.write(pathlib.Path(os.environ["PERCH_TEST_TEXT"]).read_text())
    sys.exit(0)
'''


def main():
    with tempfile.TemporaryDirectory(prefix='perch-static-readers-') as directory:
        folder = pathlib.Path(directory)
        state, data = folder/'state', folder/'text'
        state.write_text('ok')
        # Just below the script limit, with one pathological unbroken line.
        text = '#!/bin/bash\n# first-marker\n' + 'x' * ((256 << 10)-128) + '\n# last-marker\n'
        data.write_text(text)
        ssh = folder/'ssh'
        ssh.write_text('#!'+sys.executable+'\n'+FAKE_SSH.replace('assert remote[:2]', REPLIES+'\nassert remote[:2]'))
        ssh.chmod(0o700)
        env = {'PATH':str(folder)+':/usr/bin:/bin', 'PERCH_TEST_STATE':str(state),
               'PERCH_TEST_TEXT':str(data), 'PERCH_TEST_FIXTURES':str(ROOT/'testdata/hummingbird')}
        s = Session(['--host','perch-test.invalid','--no-config'],80,24,env=env)
        try:
            s.wait('simulation-sweep'); s.wait('● connected')
            s.key('/find simulation\r'); s.wait('Filter: simulation')
            for reader in ['script', 'module']:
                s.key('s' if reader == 'script' else '/module cuda/12.8.1\r')
                s.wait('#!/bin/bash')
                for width, height in [(80,24),(48,18),(140,40)]:
                    s.pump(.4)
                    start = len(s.raw)
                    s.resize(width,height)
                    s.pump(.4)
                    (ROOT/'output/terminal'/f'static-{reader}-resize-{width}.ansi').write_bytes(s.raw[start:])
                    assert s.screen.display[1].strip().startswith('perch'), 'resize damaged title'
                    assert '╭' in s.screen.display[4] and '╮' in s.screen.display[4], 'resize damaged reader border'
                    assert sum('1 Jobs' in line or '1  Jobs' in line for line in s.screen.display) == 1, 'resize duplicated navigation'
                    s.key('\x1b[F'); s.wait('# last-marker',timeout=3)
                    if reader == 'module': s.wait('Use in your cluster shell')
                    s.screenshot(f'static-{reader}-tail-{width}')
                    s.key('\x1b[H'); s.wait('#!/bin/bash',timeout=3)
                    if reader == 'module': s.wait('module load cuda/12.8.1')
                    if reader == 'script' and height <= 24:
                        assert 'Find a command' not in s.text
                        s.key('/'); s.wait('> /')
                        s.key('\x1b'); s.wait('#!/bin/bash')
                    s.screenshot(f'static-{reader}-head-{width}')
                s.key('\x1b'); s.wait('Filter: simulation'); s.wait('simulation-sweep')
            s.close()
        except BaseException:
            print(s.text); s.screenshot('static-readers-failure'); s.abort(); raise
    print('Static readers PTY passed: near-256 KiB unbroken line through production script/module adapters; Home/End, three widths, return to filtered jobs, animation enabled and terminal cleanup. Synthetic SSH, no native-window acceptance.')


if __name__ == '__main__': main()
