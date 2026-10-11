#!/usr/bin/env python3
"""Pod metrics through the production CLI and real PTY, synthetic data only."""
import json
import subprocess
from test_terminal import Session, ROOT
from test_footer import activate
from test_mouse import wheel


def main():
    args = [str(ROOT/'bin/perch'), 'nrp', 'metrics', 'retrying-replacement', '--demo']
    result = subprocess.run(args+['--json'], capture_output=True, text=True, check=True)
    data = json.loads(result.stdout)
    assert data['source'] == 'synthetic-demo'
    sample = data['data']
    assert sample['containers'][0]['cpu_cores'] == '0.215'
    assert sample['containers'][0]['memory_working_set_bytes'] == '153092096'
    assert sample['window'] == '30s' and not sample['stale']
    text = subprocess.run(args, capture_output=True, text=True, check=True).stdout
    assert 'MEMORY WORKING SET' in text and '146Mi' in text
    for width, height in [(140,40),(80,24),(48,18)]:
        s = Session(['nrp','ui','--demo'],width,height)
        try:
            s.wait('Namespace Jobs');s.key('2');s.key('/find retrying-replacement\r')
            if width < 64: s.key('\r');s.wait('Object details')
            activate(s,'u resources');s.wait('0.215 cores');s.wait('146Mi working set')
            s.screenshot(f'nrp-metrics-{width}')
            s.resize(48,18);s.wait('146Mi working set')
            before=s.text;wheel(s,10,5);assert before!=s.text
            s.key('\x1b[F');s.wait('GPU')
            s.resize(48,18);s.key('\x1b[H');s.wait('0.215 cores')
            s.pump(2.1);activate(s,'r read');s.wait('146Mi working set')
            activate(s,'esc back');s.wait('Filter: retrying-replacement')
            s.key('/metrics\r');s.wait('0.215 cores');s.key('\x1b')
            s.key('1');s.key('/find retrying\r');s.key('u');s.wait('Choose a Pod')
            s.key('j\r');s.wait('0.215 cores')
            s.key('\x1b');s.wait('Choose a Pod');s.key('\x1b');s.wait('Filter: retrying')
            s.close()
        except BaseException:
            print(s.text);s.screenshot('nrp-metrics-failure');s.abort();raise
    print('NRP metrics CLI/PTY passed: exact units, three sizes, mouse entry/scroll/read/back, Pod and Job origins, resize, filtered return and terminal cleanup. Synthetic data only.')


if __name__ == '__main__': main()
