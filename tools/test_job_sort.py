#!/usr/bin/env python3
"""Job sorting and shared column hit targets through real keyboard/mouse PTYs."""
from test_terminal import Session
from test_mouse import target, click
from test_hover import env as hover_env


def main():
    for width,height in [(140,40),(80,24),(48,18)]:
        s=Session(['--demo','--no-config'],width,height,env=hover_env('ghostty'))
        try:
            s.wait('parameter-sweep');s.wait('9100_11')
            click(s,*target(s,'NAME'));s.wait('Sorted: name ↑');s.wait('› 9100_11')
            click(s,*target(s,'NAME'));s.wait('Sorted: name ↓');s.wait('NAME ↓')
            s.key('/sort cpus\r');s.wait('Sorted: cpus ↓');s.wait('› 9100_11')
            s.key('k');s.wait('› 9231')
            s.screenshot(f'job-sort-cpus-{width}')
            if width==48: assert '8 CPUs' in s.text
            else: assert 'CPUS ↓' in s.text
            s.key('/find parameter\r');s.wait('Filter: parameter');s.wait('› 9100_11')
            s.key('/sort\r');s.wait('Sort jobs');s.wait('/sort default')
            s.key(' memory');s.wait('/sort memory')
            s.screenshot(f'job-sort-chooser-{width}')
            click(s,*target(s,'/sort memory'));s.wait('Sorted: memory ↓');s.wait('› 9100_11')
            s.wait('Filter: parameter')
            if width==48: s.wait('4.0 GiB requested')
            else:
                s.wait('REQ MEM ↓');click(s,*target(s,'REQ MEM'));s.wait('REQ MEM ↑')
                click(s,*target(s,'REQ MEM'));s.wait('REQ MEM ↓')
            s.screenshot(f'job-sort-memory-{width}')
            s.resize(48,18);s.wait('4.0 GiB requested');s.wait('Sorted: memory ↓');s.wait('Filter: parameter')
            s.key('2');s.wait('Recent history');assert 'Sorted: memory' not in s.text
            s.key('/sort elapsed asc\r');s.wait('Sorted: elapsed ↑')
            s.key('1');s.wait('Sorted: memory ↓');s.wait('› 9100_11')
            s.key('/sort nope\r');s.wait('Use /sort default')
            assert 'Sorted: memory ↓' not in s.text # Error occupies the hint, sort remains unchanged.
            s.key('\x1b');s.wait('Sorted: memory ↓')
            s.key('/sort default\r');s.wait('› 9100_11');assert 'Sorted:' not in s.text
            s.key('2');s.wait('Sorted: elapsed ↑')
            s.key('4');s.wait('Offline demo');s.key('/sort name\r');s.wait('Sorted: name ↑');s.wait('parameter-sweep')
            s.close()
        except BaseException:
            print(s.text);s.screenshot(f'job-sort-failure-{width}');s.abort();raise
    print('Job sort PTY passed: three sizes, clickable/reversible headings, resource columns and compact values, chooser, retained selection/filter, resize, independent Jobs/History orders, invalid input, reset and terminal cleanup. Demo data only.')


if __name__=='__main__': main()
