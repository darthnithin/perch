#!/usr/bin/env python3
"""Production CLI and report navigation with offline priority fixtures."""
import json, subprocess
from test_terminal import Session,ROOT
from test_mouse import click,target,wheel
from test_footer import activate


def main():
    r=subprocess.run([str(ROOT/'bin/perch'),'priority','--demo','--no-config','--json'],capture_output=True,text=True,check=True)
    data=json.loads(r.stdout)
    assert data['source']=='demo' and len(data['data'])==2 and 'history_days' not in data
    assert data['data'][0]['score']==10010148
    assert data['priority_policy']['fairshare_weight']==0
    plain=subprocess.run([str(ROOT/'bin/perch'),'priority','--demo','--no-config'],capture_output=True,text=True,check=True).stdout
    assert 'WEIGHTED SCORE' in plain and '10010148' in plain and 'not queue positions' in plain
    assert 'Fair-share does not contribute' in plain
    for width,height in [(140,40),(80,24),(48,18)]:
        s=Session(['--demo','--no-config','--reduced-motion'],width,height)
        try:
            s.wait('parameter-sweep')
            s.key('2')
            s.key('/find parameter-sweep\r')
            s.key('/priority\r')
            s.wait('Queue priority')
            s.wait('10010148')
            s.screenshot(f'priority-{width}')
            s.key('l')
            s.wait('Choose a job in Jobs or History first.')
            assert 'Queue priority' in s.text
            s.key('\x1b[F')
            s.wait('Observed:')
            s.wait('weight 0')
            if height == 18:
                s.wait('Fair-share does not contribute')
                assert 'Find a command' not in s.text
                s.key('/');s.wait('> /');s.key('\x1b');s.key('\x1b[F');s.wait('weight 0')
            s.screenshot(f'priority-bottom-{width}')
            activate(s,'r refresh')
            s.wait('10010148')
            wheel(s,10,6)
            s.resize(48,18)
            s.key('\x1b[H')
            s.wait('10010148')
            activate(s,'esc back')
            s.wait('Recent history')
            assert 'parameter-sweep' in s.text and '1–' in s.text
            s.resize(140,40)
            s.key('\x0b')
            s.key('priority')
            s.wait('Queue priority factors')
            click(s,*target(s,'Queue priority factors'))
            s.wait('10010148')
            s.key('3')
            s.wait('Allocation is scheduler capacity')
            s.close()
        except BaseException:
            print(s.text);s.screenshot('priority-failure');s.abort();raise
    print('Priority CLI/PTY passed: weighted data, 140/80/48 layouts, scrolling, pointer refresh/back, history filter return, hidden job-action guard, palette, resize and terminal cleanup.')


if __name__=='__main__': main()
