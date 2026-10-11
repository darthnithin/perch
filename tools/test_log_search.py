#!/usr/bin/env python3
"""Search retained output through production log reads and real PTYs."""
import pathlib
import sys
import tempfile
from test_failure_terminal import FAKE_SSH
from test_log_retention import LOG_READ
from test_terminal import ROOT, Session
from test_footer import activate
from test_hover import env as hover_env


def main():
    with tempfile.TemporaryDirectory(prefix='perch-log-search-') as directory:
        folder=pathlib.Path(directory)
        state,data,reads=(folder/name for name in ['state','log','reads'])
        state.write_text('ok')
        original='[ERROR.*] first\n'+''.join(f'iteration {i:03d}: checkpoint written\n' for i in range(50))+'[error.*] middle\n'+''.join(f'iteration {i:03d}: checkpoint written\n' for i in range(50,100))+'[Error.*] last\n'
        ssh=folder/'ssh'
        ssh.write_text('#!'+sys.executable+'\n'+FAKE_SSH.replace('lines = remote[2].splitlines()',LOG_READ+'\nlines = remote[2].splitlines()'))
        ssh.chmod(0o700)
        env={**hover_env('ghostty'),'PATH':str(folder)+':/usr/bin:/bin',
             'PERCH_TEST_STATE':str(state),'PERCH_TEST_LOG':str(data),
             'PERCH_TEST_LOG_READS':str(reads),'PERCH_TEST_FIXTURES':str(ROOT/'testdata/hummingbird')}
        for width,height in [(140,40),(80,24),(48,18)]:
            data.write_text(original);state.write_text('ok')
            s=Session(['--host','perch-test.invalid','--no-config','--reduced-motion'],width,height,env=env)
            try:
                s.wait('simulation-sweep');s.wait('● connected')
                s.key('/find simulation\r');s.key('l');s.wait('[Error.*] last')
                activate(s,'f search');s.wait('Search retained output')
                s.key('[error.*]\r');s.wait('Match 1 / 3');s.wait('[ERROR.*] first')
                assert 'Filter: simulation' in s.text
                s.screenshot(f'log-search-first-{width}')
                activate(s,'] next');s.wait('Match 2 / 3');s.wait('[error.*] middle')
                activate(s,'[ prev');s.wait('Match 1 / 3')
                activate(s,'[ prev');s.wait('Match 3 / 3');s.wait('[Error.*] last')
                s.resize(48,18);s.wait('[Error.*] last');s.wait('Match 3 / 3')
                activate(s,'f edit');s.key('\x15absent');s.wait('No matches in retained output')
                s.screenshot(f'log-search-empty-{width}')
                activate(s,'esc cancel');s.wait('Match 3 / 3');s.wait('[Error.*] last')
                s.resize(width,height)
                # An incremental read updates counts without resuming follow.
                data.write_text(original+'[error.*] appended\n')
                s.pump(2.2);s.key('r');s.wait('Match 3 / 4')
                assert 'paused' in s.text
                activate(s,'] next');s.wait('[error.*] appended')
                # Read failure keeps the existing searchable text.
                state.write_text('logfail');s.pump(2.2);s.key('r');s.wait('stale · r retry')
                activate(s,'[ prev');s.wait('[Error.*] last')
                # A confirmed file truncation invalidates the old match ordinal.
                state.write_text('ok');data.write_text('[error.*] rotated\n')
                s.pump(2.2);s.key('r');s.wait('1 match · retained output')
                assert '[Error.*] last' not in s.text
                activate(s,'] next');s.wait('Match 1 / 1')
                s.screenshot(f'log-search-rotated-{width}')
                s.key('\x1b[F');s.wait('following');s.wait('up to 256 KiB retained')
                if width==80:
                    data.write_text('[error.*] rotated\n'+'x'*(72*3640)+'\nfinal-needle\n')
                    s.pump(2.2);s.key('r');s.wait('final-needle')
                    activate(s,'f search');s.key('final-needle\r');s.wait('Match 1 / 1')
                    s.wait('find final-needle');s.wait('esc clear')
                    s.wait('final-needle');s.screenshot('log-search-long-line')
                    s.key('\x1b[F');s.wait('following')
                s.key('e');s.wait('Standard error');assert 'Match 1 /' not in s.text
                s.key('\x1b');s.wait('Filter: simulation');s.close()
            except BaseException:
                print(s.text);s.screenshot(f'log-search-failure-{width}');s.abort();raise
    print('Log search PTY passed: three sizes, literal/case-insensitive matches, clickable search/next/previous, wraparound, editing cancellation, resize, incremental append, stale reads, rotation, End-follow, stream reset, preserved job filter and terminal cleanup. Synthetic SSH only.')


if __name__=='__main__': main()
