#!/usr/bin/env python3
"""Submitted-script search through production reads, mouse targets and real PTYs."""
import pathlib
import sys
import tempfile
from test_failure_terminal import FAKE_SSH
from test_footer import activate
from test_hover import env as hover_env
from test_terminal import ROOT, Session

SCRIPT_READ = r'''
if remote[:3] == ["scontrol", "write", "batch_script"] or remote[:2] == ["sacct", "--batch-script"]:
    if state == "scriptfail":
        print("Script access denied", file=sys.stderr)
        sys.exit(1)
    sys.stdout.write(pathlib.Path(os.environ["PERCH_TEST_SCRIPT"]).read_text())
    sys.exit(0)
'''


def main():
    with tempfile.TemporaryDirectory(prefix='perch-script-search-') as directory:
        folder=pathlib.Path(directory)
        state,data=folder/'state',folder/'script'
        state.write_text('ok')
        original='#!/bin/sh\n# [ERROR.*] first\n'+''.join(f'# setup stage {i:03d}\n' for i in range(50))+'# [error.*] middle\n'+''.join(f'# setup stage {i:03d}\n' for i in range(50,100))+'# [Error.*] last\n'
        ssh=folder/'ssh'
        ssh.write_text('#!'+sys.executable+'\n'+FAKE_SSH.replace('assert remote[:2]',SCRIPT_READ+'\nassert remote[:2]'))
        ssh.chmod(0o700)
        env={**hover_env('ghostty'),'PATH':str(folder)+':/usr/bin:/bin',
             'PERCH_TEST_STATE':str(state),'PERCH_TEST_SCRIPT':str(data),
             'PERCH_TEST_FIXTURES':str(ROOT/'testdata/hummingbird')}
        for width,height in [(140,40),(80,24),(48,18)]:
            data.write_text(original);state.write_text('ok')
            s=Session(['--host','perch-test.invalid','--no-config'],width,height,env=env)
            try:
                s.wait('simulation-sweep');s.wait('● connected')
                s.key('/find simulation\r');s.key('s');s.wait('[ERROR.*] first')
                activate(s,'f search');s.wait('Search submitted script')
                s.key('[error.*]\r');s.wait('esc clear');s.wait('Match 1 / 3');s.wait('[ERROR.*] first')
                assert 'Filter: simulation' in s.text
                s.screenshot(f'script-search-first-{width}')
                activate(s,'] next');s.wait('Match 2 / 3');s.wait('[error.*] middle')
                activate(s,'[ prev');s.wait('Match 1 / 3')
                activate(s,'[ prev');s.wait('Match 3 / 3');s.wait('[Error.*] last')
                s.resize(48,18);s.wait('[Error.*] last');s.wait('Match 3 / 3')
                activate(s,'f edit');s.key('\x15absent');s.wait('No matches in submitted script')
                s.screenshot(f'script-search-empty-{width}')
                activate(s,'esc cancel');s.wait('Match 3 / 3');s.wait('[Error.*] last')
                s.resize(width,height)
                # Unchanged refresh keeps the selected hit; a changed script does not.
                s.pump(2.2);s.key('r');s.wait('Match 3 / 3')
                data.write_text(original+'# [error.*] added\n')
                s.pump(2.2);s.key('r');s.wait('4 matches · submitted script')
                activate(s,'] next');s.wait('Match 1 / 4');s.wait('[ERROR.*] first')
                s.key('\x1b[F');s.wait('[error.*] added');s.wait('Match 1 / 4')
                # Failed reread clears both script and find; retry yields a fresh reader.
                state.write_text('scriptfail');s.pump(2.2);s.key('r');s.wait('Script access');s.wait('denied')
                assert '[error.*]' not in s.text and 'Match 1 /' not in s.text
                state.write_text('ok');s.pump(2.2);activate(s,'r retry');s.wait('[ERROR.*] first')
                if width==80:
                    data.write_text('#!/bin/sh\n'+'x'*(250<<10)+'\n# final-needle\n')
                    s.pump(2.2);s.key('r');activate(s,'f search');s.key('final-needle\r')
                    s.wait('find final-needle');s.wait('esc clear');s.wait('Match 1 / 1');s.wait('final-needle');s.screenshot('script-search-long-line')
                    activate(s,'esc clear')
                activate(s,'f search');s.key('absent\r');s.wait('esc clear');s.wait('No matches in submitted script')
                activate(s,'esc clear');s.wait('f search')
                s.key('\x1b');s.wait('Filter: simulation');s.wait('simulation-sweep');assert 'Submitted script' not in s.text;s.close()
            except BaseException:
                print(s.text);s.screenshot(f'script-search-failure-{width}');s.abort();raise
    print('Script search PTY passed: production adapter, three sizes, literal/case-insensitive search, clickable editing/next/previous, wraparound, cancel, resize, refresh/replacement, failure/retry, long unbroken line, preserved job filter, animation enabled and terminal cleanup. Synthetic SSH only.')


if __name__=='__main__': main()
