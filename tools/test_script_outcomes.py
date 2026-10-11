#!/usr/bin/env python3
"""Script availability and failure recovery through CLI, MCP and real PTYs."""
import json
import os
import pathlib
import queue
import subprocess
import sys
import tempfile
import threading

from test_failure_terminal import FAKE_SSH
from test_footer import activate
from test_terminal import ROOT, Session


SCRIPT_READ = r'''
if remote[:3] == ["scontrol", "write", "batch_script"] or remote[:2] == ["sacct", "--batch-script"]:
    historical = remote[0] == "sacct"
    if historical:
        assert "--noheader" in remote
    with open(os.environ["PERCH_TEST_READS"], "a") as record:
        record.write(str(remote)+"\n")
    if state == "scriptfail":
        print("partial script must not escape")
        print("\x1b[31mScript access denied\x1b[0m", file=sys.stderr)
        sys.exit(1)
    if state == "empty": sys.exit(0)
    if state == "none":
        print("NONE" if historical else "")
        sys.exit(0)
    print("#!/bin/sh\necho readable-script")
    sys.exit(0)
'''


def check_mcp(env, state):
    p = subprocess.Popen([str(ROOT/'bin/perch'),'mcp','--host','perch-test.invalid','--no-config'],
                         stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE,
                         text=True,env={**os.environ,**env})
    replies = queue.Queue()
    def receive():
        for line in p.stdout:
            try: replies.put(json.loads(line))
            except ValueError: replies.put(AssertionError('Non-protocol stdout'))
        replies.put(EOFError('MCP ended early'))
    threading.Thread(target=receive,daemon=True).start()
    sequence = 0
    def request(method, params, notification=False):
        nonlocal sequence
        sequence += 1
        message = {'jsonrpc':'2.0','method':method,'params':params}
        if not notification: message['id'] = sequence
        p.stdin.write(json.dumps(message)+'\n'); p.stdin.flush()
        if notification: return
        while True:
            reply = replies.get(timeout=10)
            if isinstance(reply, Exception): raise reply
            if reply.get('id') == sequence: return reply
    try:
        reply=request('initialize',{'protocolVersion':'2025-11-25','capabilities':{},'clientInfo':{'name':'script-outcomes','version':'1'}})
        assert 'result' in reply, reply
        request('notifications/initialized',{},notification=True)
        for outcome, expected in [('none','no batch script is available'),('scriptfail','Script access denied'),('ok','readable-script')]:
            state.write_text(outcome)
            reply=request('tools/call',{'name':'perch_script','arguments':{'job_id':'9000_0'}})
            result=reply['result']
            assert bool(result.get('isError')) == (outcome != 'ok'), result
            assert expected in json.dumps(result), result
            assert 'partial script' not in json.dumps(result)
        p.stdin.close(); assert p.wait(timeout=10)==0
        assert not p.stderr.read()
    finally:
        if p.poll() is None: p.kill(); p.wait(timeout=5)
        p.stdout.close(); p.stderr.close()


def main():
    with tempfile.TemporaryDirectory(prefix='perch-script-outcomes-') as directory:
        folder=pathlib.Path(directory)
        state=folder/'state'; state.write_text('ok')
        ssh=folder/'ssh'
        ssh.write_text('#!'+sys.executable+'\n'+FAKE_SSH.replace('assert remote[:2]', SCRIPT_READ+'\nassert remote[:2]'))
        ssh.chmod(0o700)
        env={'PATH':str(folder)+':/usr/bin:/bin','PERCH_TEST_STATE':str(state),
             'PERCH_TEST_READS':str(folder/'reads'),'PERCH_TEST_FIXTURES':str(ROOT/'testdata/hummingbird')}
        for job in ['9100_41','9000_0']:
            for outcome in ['empty','none','scriptfail','ok']:
                state.write_text(outcome)
                p=subprocess.run([str(ROOT/'bin/perch'),'script',job,'--json','--no-config','--host','perch-test.invalid'],
                                 env={**os.environ,**env},capture_output=True,text=True,timeout=15)
                if outcome=='ok':
                    assert p.returncode==0, p.stderr
                    assert json.loads(p.stdout)['data']['text']=='#!/bin/sh\necho readable-script\n'
                else:
                    assert p.returncode!=0 and not p.stdout, (p.returncode,p.stdout,p.stderr)
                    expected='Script access denied' if outcome=='scriptfail' else 'no batch script is available'
                    assert expected in p.stderr and '\x1b' not in p.stderr, p.stderr
        check_mcp(env,state)
        for width,height in [(48,18),(80,24),(140,40)]:
            state.write_text('none')
            s=Session(['--host','perch-test.invalid','--no-config','--reduced-motion'],width,height,env=env)
            try:
                s.wait('simulation-sweep'); s.key('2'); s.wait('single-simulation')
                s.key('s'); s.wait('no batch script is available')
                s.screenshot(f'script-unavailable-{width}')
                state.write_text('ok'); s.pump(2.2); activate(s,'r retry'); s.wait('readable-script')
                state.write_text('scriptfail'); s.pump(2.2); s.key('r'); s.wait('Script access'); s.wait('denied')
                assert 'readable-script' not in s.text and 'partial script' not in s.text
                s.resize(48,18); s.wait('Script access'); s.wait('denied')
                s.screenshot(f'script-read-failure-{width}')
                state.write_text('ok'); s.pump(2.2); activate(s,'r retry'); s.wait('readable-script')
                s.key('\x1b'); s.wait('single-simulation'); s.close()
            except BaseException:
                print(s.text); s.screenshot('script-outcome-failure'); s.abort(); raise
    print('Script outcomes passed: production CLI/MCP and three-size PTYs; live/historical command selection, unavailable/empty/failed/successful reads, clean diagnostics, no partial output, retry/recovery, resize, back and cleanup. Synthetic SSH only.')


if __name__=='__main__': main()
