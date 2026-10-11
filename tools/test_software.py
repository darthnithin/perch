#!/usr/bin/env python3
"""Browse module setup through the real TUI. Demo by default; optional read-only SSH."""
import argparse
from test_terminal import Session
from test_footer import activate
from test_hover import env, move, shape
from test_mouse import target, click, wheel


def demo():
    for width, height in [(48,18),(80,24),(140,40)]:
        s = Session(['--demo','--no-config','--reduced-motion'],width,height,env=env('ghostty'))
        try:
            s.wait('parameter-sweep'); s.key('/find parameter\r'); s.key('j')
            s.key('/software\r'); s.wait('5 modules')
            s.key('jjj'); s.wait('openmpi5/5.0.10')
            s.screenshot(f'software-catalog-{width}')
            activate(s,'enter setup'); s.wait('module load openmpi5/5.0.10')
            s.key('\x1b[F'); s.wait('Use in your cluster shell')
            s.key('\x1b[H'); s.wait('module load openmpi5/5.0.10')
            s.screenshot(f'software-module-{width}')
            activate(s,'esc back'); s.wait('5 modules'); s.wait('› openmpi5/5.0.10')
            s.key('\x1b[H'); s.wait('› cuda/12.8.1')
            before=s.text
            move(s,*target(s,'cuda/12.8.1')); shape(s,'pointer')
            assert before==s.text, 'hover opened a module or changed selection'
            click(s,*target(s,'cuda/12.8.1')); s.wait('module load cuda/12.8.1')
            s.key('\x1b'); s.resize(48,18); s.wait('› cuda/12.8.1')
            wheel(s,*target(s,'cuda/12.8.1'))
            assert '› cuda/12.8.1' not in s.text, 'wheel did not move catalog selection'
            s.key('\x1b[F'); s.wait('› singularity')
            s.key('r'); s.wait('› singularity')
            s.key('\r'); s.wait('Unable to read this view')
            s.key('\x1b'); s.wait('› singularity')
            s.key('f'); s.key('no-such-demo-module\r'); s.wait('No matching modules')
            s.key('\r'); s.wait('No matching modules')
            s.key('\x1b'); s.wait('Filter: parameter'); s.wait('› 9100_10')
            s.close()
        except BaseException:
            print(s.text); s.screenshot('software-catalog-failure'); s.abort(); raise
    print('Software browser PTY passed: three sizes, keyboard and footer entry, hover without activation, click, wheel, resize, refresh selection, module failure/back, empty results and caller filter/selection. Demo only.')


def live(host):
    s = Session(['--host',host,'--no-config','--reduced-motion'],100,32)
    try:
        s.wait('● connected',timeout=30)
        s.key('/software cuda\r'); s.wait('cuda/',timeout=30)
        s.screenshot('software-catalog-hummingbird')
        activate(s,'enter setup'); s.wait('After loading prerequisites',timeout=30)
        s.wait('module load cuda/')
        s.screenshot('software-module-hummingbird')
        s.key('\x1b'); s.wait('cuda/')
        s.resize(48,18); s.wait('cuda/'); s.screenshot('software-catalog-hummingbird-48')
        s.key('\x1b'); s.wait('My jobs'); s.close()
    except BaseException:
        print(s.text); s.screenshot('software-catalog-live-failure'); s.abort(); raise
    print('Hummingbird software browse/setup/back passed over SSH in a real PTY. Read-only; no module was loaded or job submitted. Native appearance remains untested.')


if __name__ == '__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--host')
    args=parser.parse_args()
    if args.host: live(args.host)
    else: demo()
