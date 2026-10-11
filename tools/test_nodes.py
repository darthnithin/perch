#!/usr/bin/env python3
"""Node allocations through the production CLI and keyboard/mouse PTY."""
import json,subprocess
from test_terminal import Session,ROOT
from test_footer import activate
from test_mouse import click,target,wheel

def main():
    result=subprocess.run([str(ROOT/'bin/perch'),'nodes','--demo','--no-config','--json'],capture_output=True,text=True,check=True,timeout=10)
    data=json.loads(result.stdout)
    assert data['source']=='demo' and len(data['data'])==20 and 'history_days' not in data
    n=data['data'][0]
    assert n['name']=='node-06' and n['unallocated_memory_bytes']==0 and n['idle_cpus']==15 and 'PLANNED' in n['states']
    gpu=next(n for n in data['data'] if n['name']=='node-24')
    assert gpu['gpus']==[{'type':'p100','configured':4,'allocated':4,'unallocated':0}]
    text=subprocess.run([str(ROOT/'bin/perch'),'nodes','--demo','--no-config'],capture_output=True,text=True,check=True).stdout
    assert 'MEM UNALLOCATED' in text and 'not utilization' in text
    assert 'GPU p100: 4 / 4 allocated; 0 unallocated' in text
    for w,h in [(140,40),(80,24),(48,18)]:
        s=Session(['--demo','--no-config','--reduced-motion'],w,h)
        try:
            s.wait('parameter-sweep');s.key('3');s.wait('n nodes')
            activate(s,'n nodes');s.wait('Node allocations');s.wait('0 B unallocated')
            s.screenshot(f'nodes-{w}')
            activate(s,'g GPU');s.wait('GPU nodes · 1 of 20');s.wait('p100 · 4 / 4 allocated')
            assert 'node-06' not in s.text
            s.screenshot(f'nodes-gpu-{w}')
            activate(s,'r refresh');s.wait('GPU nodes · 1 of 20');s.wait('p100 · 4 / 4 allocated')
            activate(s,'g all nodes');s.wait('node-06')
            before=s.text;wheel(s,10,6);assert before!=s.text
            s.key('\x1b[F');s.wait('Observed:');s.screenshot(f'nodes-bottom-{w}')
            activate(s,'r refresh');s.wait('node-06')
            s.resize(48,18);s.wait('0 B unallocated')
            s.key('l');s.wait('Choose a job in Jobs or History first.')
            activate(s,'esc back');s.wait('Partitions');s.wait('n nodes')
            s.resize(140,40);s.key('\x0b');s.key('node cpu');s.wait('Node CPU, memory and GPU allocations')
            click(s,*target(s,'Node CPU, memory and GPU allocations'));s.wait('node-06')
            s.key('\x1b');s.key('/nodes\r');s.wait('Node allocations');s.wait('node-06')
            s.close()
        except BaseException:
            print(s.text);s.screenshot('nodes-failure');s.abort();raise
    print('Node CLI/PTY passed: CPU/memory/GPU assignments, clickable GPU-only filter, filtered refresh, partition footer click, refresh/back, palette/slash entry, scroll/resize, hidden-action guard and terminal cleanup at 140/80/48 columns.')
if __name__=='__main__':main()
