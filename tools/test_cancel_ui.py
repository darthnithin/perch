#!/usr/bin/env python3
"""Production cancellation reviews via PTY; synthetic SSH, no network/mutations."""
import json
import os
from pathlib import Path
import sys
import tempfile

from test_terminal import Session, ROOT
from test_failure_terminal import FAKE_SSH
from test_mouse import click, target
from test_footer import activate, footer
from test_hover import move, env as pointer_env

EXTRA = r'''
if remote[0]=='env':
    import json,time
    root=pathlib.Path(os.environ['PERCH_CANCEL_UI_TEST'])
    mode=(root/'mode').read_text()
    command=remote[11:]
    assert remote[10]=='perch-cancel',remote
    if command[0]=='squeue':
        count=int((root/'count').read_text())+1;(root/'count').write_text(str(count))
        if mode=='slow-preview':time.sleep(2)
        data=json.loads((pathlib.Path(os.environ['PERCH_TEST_FIXTURES'])/'queue.json').read_text())
        if mode=='requeued' and count>1:data['jobs'][0]['restart_cnt']=1
        print(json.dumps(data));sys.exit(0)
    assert command[0]=='scancel',command
    with (root/'sent').open('a') as out:out.write(json.dumps(command)+'\n')
    if mode=='slow-send':time.sleep(2)
    if mode=='unknown':sys.exit(255)
    if mode=='nonzero':sys.exit(1)
    sys.exit(0)
'''


def main():
    for width,height in [(140,40),(80,24),(48,18)]:
        s=Session(['--demo','--no-config','--reduced-motion'],width,height,env=pointer_env('ghostty'))
        try:
            s.wait('Find a command');s.key('/find parameter\r');s.wait('Filter: parameter')
            s.key('/cancel\r');s.wait('ctrl+x cancel task')
            s.key('\r');assert 'Cancellation review' in s.text
            move(s,*target(s,'Cancel task · Ctrl+X'));s.screenshot('cancel-ui-review-'+str(width))
            assert 'Sibling tasks continue' in s.text,s.text
            activate(s,'esc back');s.wait('Filter: parameter')
            s.key('/cancel-array\r');s.wait('ctrl+x cancel array');s.screenshot('cancel-ui-array-'+str(width))
            s.key('\x18');s.wait('Demo · no cancellation sent')
            activate(s,'enter back');s.key('/last-cancel\r');s.wait('Demo · no cancellation sent')
            s.screenshot('cancel-ui-receipt-'+str(width))
            s.key('\x18');assert 'Demo · no cancellation sent' in s.text
            activate(s,'enter back');s.wait('Filter: parameter');s.close()
        except BaseException:print(s.text);s.abort();raise

    with tempfile.TemporaryDirectory(prefix='perch-cancel-ui-') as directory:
        root=Path(directory);(root/'home').mkdir(mode=0o700)
        state=root/'state';state.write_text('ok')
        mode=root/'mode';mode.write_text('ok')
        count=root/'count';count.write_text('0')
        sent=root/'sent';sent.write_text('')
        fixtures=root/'fixtures';fixtures.mkdir()
        for name in ['queue','accounting','partitions']:
            data=json.loads((ROOT/'testdata/hummingbird'/f'{name}.json').read_text())
            if name=='queue':
                data['jobs']=data['jobs'][:1];j=data['jobs'][0]
                for k in ['het_job_id','het_job_offset']:j[k]={'set':True,'number':0}
                for k in ['het_job_id_set','federation_origin','federation_siblings_active','federation_siblings_viable']:j[k]=''
            (fixtures/f'{name}.json').write_text(json.dumps(data))
        ssh=root/'ssh';ssh.write_text('#!'+sys.executable+'\n'+FAKE_SSH.replace('assert remote[:2] == ["sh", "-c"], remote',EXTRA+'\nassert remote[:2] == ["sh", "-c"], remote'));ssh.chmod(0o700)
        env=pointer_env('ghostty')|{'HOME':str(root/'home'),'PATH':str(root)+':/usr/bin:/bin','PERCH_TEST_STATE':str(state),'PERCH_TEST_FIXTURES':str(fixtures),'PERCH_CANCEL_UI_TEST':str(root)}
        s=Session(['--host','perch-test.invalid','--no-config','--reduced-motion'],120,36,env=env)
        try:
            s.wait('● connected');s.key('/find simulation\r');s.wait('Filter: simulation')
            activate(s,'enter inspect');s.key('/cancel\r');s.wait('ctrl+x cancel task')
            assert not sent.read_text(),'preview mutated'
            s.screenshot('cancel-ui-live-review')
            activate(s,'esc back');s.wait('9100_39 · simulation-sweep')
            mode.write_text('slow-preview');count.write_text('0')
            s.key('/cancel\r');s.wait('Checking the current job')
            activate(s,'esc back');s.pump(2.5)
            assert 'Cancellation review' not in s.text and not sent.read_text(),'late preview reopened or mutated'
            # Requeue between review and confirmation must send no mutation.
            mode.write_text('requeued');count.write_text('0')
            s.key('/cancel\r');s.wait('ctrl+x cancel task');s.key('\x18');s.wait('Cancellation not sent')
            assert not sent.read_text(),'changed attempt was cancelled'
            s.screenshot('cancel-ui-changed-attempt')
            activate(s,'esc back')
            # Once sending, repeated keys/clicks and Escape cannot send or dismiss.
            mode.write_text('slow-send');count.write_text('0')
            s.key('/cancel\r');s.wait('ctrl+x cancel task')
            click(s,*target(s,'Cancel task · Ctrl+X'));s.wait('Waiting for a receipt')
            s.key('\x18\x18\r\x1b');s.key('3');assert 'Cancellation review' in s.text
            s.wait('Cancellation requested');assert len(sent.read_text().splitlines())==1
            args=json.loads(sent.read_text().splitlines()[0]);assert args[-1]=='9100_39' and '--clusters=hummingbird' in args
            s.screenshot('cancel-ui-requested')
            activate(s,'enter back');s.wait('9100_39 · simulation-sweep')
            s.key('\x1b');s.wait('Filter: simulation');assert 'running' in s.text,'receipt fabricated terminal state'
            # Connection uncertainty is retained and reopening the receipt cannot retry.
            mode.write_text('unknown');count.write_text('0')
            s.key('/cancel\r');s.wait('ctrl+x cancel task');activate(s,'ctrl+x cancel task');s.wait('Outcome unknown')
            assert len(sent.read_text().splitlines())==2
            s.resize(48,18);s.screenshot('cancel-ui-unknown-48')
            activate(s,'enter back');s.key('/last-cancel\r');s.wait('Outcome unknown');s.key('\x18')
            assert len(sent.read_text().splitlines())==2
            activate(s,'enter back');s.close()
            assert b'Perch cancellation' in s.raw and b'unknown' in s.raw,'exit lost receipt'
        except BaseException:print(s.text);s.abort();raise
        # Forced shutdown retains uncertainty outside the alternate screen.
        mode.write_text('slow-send');count.write_text('0');sent.write_text('')
        s=Session(['--host','perch-test.invalid','--no-config','--reduced-motion'],80,24,env=env)
        try:
            s.wait('● connected');s.wait('simulation-sweep');s.key('/cancel\r');s.wait('ctrl+x cancel task')
            s.key('\x18');s.wait('Waiting for a receipt');s.key('\x03');s.pump()
            s.close()
            assert b'Perch cancellation' in s.raw and b'unknown' in s.raw,'interrupt lost uncertain receipt'
        except BaseException:print(s.text);s.abort();raise
    print('PASS: cancellation UI at 140/80/48 columns; explicit scope, hover, keyboard/mouse, no Enter confirmation, context, changed attempt, duplicates, uncertain receipt and terminal restoration')


if __name__=='__main__':main()
