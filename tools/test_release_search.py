#!/usr/bin/env python3
"""Screen-based search acceptance for a selected packaged executable (demo only)."""
import argparse
import hashlib
import json
from pathlib import Path
from test_terminal import Session
from test_mouse import target, click


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('binary',type=Path)
    args=parser.parse_args()
    binary=args.binary.resolve()
    for width,height in [(140,40),(80,24),(48,18)]:
        session=Session(['--demo','--no-config','--reduced-motion'],width,height,binary=binary)
        try:
            session.wait('parameter-sweep');session.key('l');session.wait('following')
            # The initial viewport follows the tail; Dataset is above the fold
            # at small sizes. Search must reveal it rather than require it first.
            session.key('f');session.wait('Search retained output')
            session.key('Dataset\r');session.wait('Match 1 / 1');session.wait('esc clear')
            session.wait('Dataset ready')
            session.screenshot(f'release-search-{binary.parent.name}-{width}')
            session.key('\x1b');session.wait('Recent output')
            session.key('\x1b[F');session.wait('following');session.key('\x1b')
            session.key('s');session.wait('Submitted script')
            session.key('f');session.wait('Search submitted script')
            session.key('python\r');session.wait('Match 1 / 1');session.wait('esc clear')
            session.wait('python train.py');session.screenshot(f'release-script-search-{binary.parent.name}-{width}')
            session.key('\x1b');session.key('\x1b')
            session.key('/sort memory\r');session.wait('Sorted: memory ↓');session.wait('› 9100_11')
            if width==48:session.wait('4.0 GiB requested')
            else:
                session.wait('REQ MEM ↓');click(session,*target(session,'REQ MEM'));session.wait('REQ MEM ↑')
            session.screenshot(f'release-sort-{binary.parent.name}-{width}')
            session.key('/sort default\r');session.wait('› 9100_11')
            assert 'Sorted:' not in session.text
            session.close()
        except BaseException:
            print(session.text);session.abort();raise
    print(json.dumps({'binary_sha256':hashlib.sha256(binary.read_bytes()).hexdigest(),
                      'widths':[140,80,48], 'input':'ordinary per-character typing',
                      'checks':['typed output and script queries','committed match count','match revealed above tail','Escape clears and returns','End follows output','memory sort and heading toggle','selected job retained','default order restored','terminal cleanup'],
                      'result':'passed','native_visual_acceptance':'not tested'},indent=2))


if __name__=='__main__':main()
