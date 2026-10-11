#!/usr/bin/env python3
"""Actual CLI, keyboard/mouse navigation and small-terminal queue-wait view."""
import json
import subprocess
from test_terminal import Session, ROOT
from test_mouse import click, target, wheel

def main():
    result = subprocess.run([str(ROOT/'bin/perch'),'waits','--demo','--no-config','--json'],capture_output=True,text=True,check=True,timeout=10)
    data=json.loads(result.stdout)
    assert data['history_days']==7 and data['source']=='demo'
    cpu=next(p for p in data['data']['partitions'] if p['partition']=='128x24')
    assert cpu['submit_to_start']['samples']>=8
    assert cpu['submit_to_start']['median_seconds']==3600
    assert cpu['eligible_to_start']['median_seconds']==2400
    s=Session(['--demo','--no-config','--reduced-motion'],120,36)
    try:
        s.wait('parameter-sweep')
        s.key('/waits\r')
        s.wait('Queue wait history')
        s.wait('40m')
        s.screenshot('waits-wide')
        s.resize(48,18)
        s.wait('Queue wait history')
        s.wait('Eligible')
        assert '1h 00m' in s.text and '40m 00s' in s.text, s.text
        s.screenshot('waits-minimum')
        before=s.text
        wheel(s,10,6)
        assert s.text!=before, 'wheel did not scroll'
        s.key('\x1b[F')
        s.wait('predictions')
        s.screenshot('waits-minimum-scrolled')
        s.key('\x1b')
        s.wait('Your jobs')
        s.resize(120,36)
        s.key('\x0b')
        s.key('queue wait')
        s.wait('Queue wait history by partition')
        click(s,*target(s,'Queue wait history by partition'))
        s.wait('Accounting snapshot')
        click(s,*target(s,'3  Partitions'))
        s.wait('Allocation is scheduler capacity')
        s.close()
    except Exception:
        s.abort(); raise
    print('Wait history CLI, palette, pointer scrolling, resize, escape and cleanup passed.')
if __name__=='__main__': main()
