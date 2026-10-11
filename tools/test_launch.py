#!/usr/bin/env python3
"""Exercise the launch UI through a PTY. Demo only; never submits remotely."""
import json
from test_terminal import Session, OUT
from test_footer import activate
from test_mouse import click


def main():
    s = Session(['--demo', '--reduced-motion'], 120, 40)
    try:
        s.wait('parameter-sweep')
        s.key('/run\r'); s.wait('Prepare a job')
        s.key('\r'); s.wait('enter the command to run')
        # Validation moves to the missing command. Return to the directory to
        # fill both required inputs, then review without losing either value.
        s.key('\x1b[Z/scratch/example\tpython3 train.py\r')
        s.wait('Review your job'); s.screenshot('launch-review')
        # Submission has no effect before validation.
        s.key('\x13'); s.wait('Validate this request before submitting')
        s.key('v'); s.wait('Validated'); s.screenshot('launch-ready')
        s.key('\x13'); s.wait('Demo complete'); s.screenshot('launch-demo-result')
        s.key('\x13'); s.wait('Demo complete')
        s.key('n'); s.resize(80, 24); s.wait('Prepare a job')
        s.screenshot('launch-form-compact')
        s.key('/scratch/keep-me\tprintf hello\r'); s.wait('Review your job')
        s.key('e'); s.wait('Shell command')
        s.key('\x1b'); s.key('/run\r'); s.wait('/scratch/keep-me')
        # Tab through all fields in the minimum supported screen.
        s.resize(48, 18)
        for label in ['Job name', 'CPUs', 'Memory', 'Time limit', 'Partition', 'GPUs', 'Modules', 'Array tasks', 'After success']:
            s.key('\t'); s.wait(label)
        s.screenshot('launch-form-minimum')
        s.key('123'); s.key('\x1b[Z'); s.key('0-9%2'); s.key('\r')
        s.resize(120, 40); s.wait('10 array tasks'); s.wait('After success: 123')
        s.screenshot('launch-array-review')
        s.key('e'); s.resize(48, 18); s.key('\t\t'); s.wait('Nodes')
        s.key('\x01\x0b2\t'); s.wait('Tasks per node')
        s.key('\x01\x0b3\t'); s.wait('Launcher')
        s.wait('auto / direct / srun / mpirun (Open MPI)')
        s.screenshot('launch-topology-minimum')
        s.key('\r'); s.resize(120, 40); s.wait('6 tasks (3/node)'); s.wait('Launcher: srun')
        s.screenshot('launch-multitask-review')
        s.key('\x1b'); s.close()
    except BaseException:
        s.screenshot('launch-failure'); s.abort(); raise
    for width, height in [(48,18),(80,24),(140,40)]:
        s = Session(['--demo','--no-config','--reduced-motion'], width, height)
        try:
            s.wait('parameter-sweep'); s.key('/run\r')
            s.key('relative/path\tpython3 train.py\r')
            s.wait('› Cluster directory'); s.wait('Correct this field')
            assert 'absolute cluster path' in s.text
            s.screenshot(f'launch-correction-path-{width}')
            # The highlighted input is a real pointer target, including after
            # error expansion borrows space from the inactive progress bar.
            row = next(i for i,line in enumerate(s.text.splitlines()) if 'relative/path' in line)
            click(s,8,row)
            s.key('\x01\x0b/scratch/preserve-draft\t\t\t\x01\x0b0\t')
            activate(s,'enter review')
            s.wait('› CPUs per task'); s.wait('CPUs per task must be 1–4096')
            s.screenshot(f'launch-correction-cpu-{width}')
            s.resize(48,18); s.wait('CPUs per task must be 1–4096')
            s.key('\x01\x0b2\r'); s.wait('Review your job')
            s.resize(140,40); s.wait('/scratch/preserve-draft'); s.wait('python3 train.py')
            s.key('e'); s.key('\t'*8+'\x01\x0b2\t\t\x01\x0bdirect\r')
            s.wait('› Launcher'); s.wait('choose srun or mpirun')
            s.resize(width,height); s.screenshot(f'launch-correction-topology-{width}')
            s.key('\x01\x0bauto\r'); s.resize(140,40)
            s.wait('2 tasks (1/node)'); s.wait('Launcher: srun')
            s.key('\x1b'); s.close()
        except BaseException:
            print(s.text); s.screenshot('launch-correction-failure'); s.abort(); raise
    result = {'launch_demo':'passed', 'checks':['form input', 'preview without submission',
        'validation gate', 'explicit simulated submission', 'duplicate key ignored',
        'edit', 'draft retention', '80x24 and 48x18 fields', 'array and dependency review', 'multi-node task review',
        'field-directed validation, pointer correction and resize at 48/80/140 columns', 'terminal cleanup'],
        'scope':'Demo PTY. Live submission is an opt-in, separate test.'}
    (OUT/'launch-results.json').write_text(json.dumps(result, indent=2)+'\n')
    print(json.dumps(result, indent=2))

if __name__ == '__main__':
    main()
