#!/usr/bin/env python3
"""Private opt-in diagnostics through actual CLI, MCP and TUI processes."""
import base64
import json
import os
import pathlib
import selectors
import subprocess
import sys
import tempfile
from test_terminal import Session, ROOT
from test_failure_terminal import FAKE_SSH


def main():
    with tempfile.TemporaryDirectory(prefix='perch-diagnostics-') as directory:
        root=pathlib.Path(directory)
        home=root/'home'; home.mkdir(mode=0o700)
        state=root/'state'; state.write_text('ok')
        ssh=root/'ssh'; ssh.write_text('#!'+sys.executable+'\n'+FAKE_SSH);ssh.chmod(0o700)
        fixtures=root/'fixtures'; fixtures.mkdir()
        marker='UNPUBLISHED-private-job-name'
        for name in ['queue','accounting','partitions']:
            data=json.loads((ROOT/'testdata/hummingbird'/f'{name}.json').read_text())
            if name=='queue': data['jobs'][0]['name']=marker
            (fixtures/f'{name}.json').write_text(json.dumps(data))
        env=os.environ | {'HOME':str(home),'XDG_CACHE_HOME':str(home/'cache'),
            'PATH':str(root)+':/usr/bin:/bin','PERCH_TEST_STATE':str(state),
            'PERCH_TEST_FIXTURES':str(fixtures),'TERM':'xterm-256color','COLORTERM':'truecolor'}
        binary=str(ROOT/'bin/perch')
        def cli(*args,success=True):
            r=subprocess.run([binary,*args,'--no-config','--host','perch-test.invalid'],env=env,capture_output=True,text=True,timeout=10)
            assert (r.returncode==0)==success,(args,r.stderr)
            return r
        cli('jobs','--json')
        assert not list(home.rglob('events.jsonl')), 'implicit capture'
        cli('jobs','--debug-raw',success=False)
        assert not list(home.rglob('events.jsonl')), 'raw without debug created capture'
        r=cli('jobs','--json','--debug')
        assert marker in r.stdout and 'metadata only' in r.stderr
        file=next(home.rglob('events.jsonl'))
        def events(path=None): return [json.loads(line) for line in (path or file).read_text().splitlines()]
        assert marker not in file.read_text(), 'default debug contains raw names'
        assert not any(e['raw_captured'] for e in events())
        assert file.stat().st_mode&0o777==0o600 and file.parent.stat().st_mode&0o777==0o700
        r=cli('jobs','--json','--debug','--debug-raw')
        rawfile=file.parent.parent/'raw'/'events.jsonl'
        assert rawfile.exists() and marker not in file.read_text()
        raw=next(e for e in reversed(events(rawfile)) if e['raw_captured'] and e['operation']=='squeue')
        assert json.loads(base64.b64decode(raw['stdout_base64']))['jobs'][0]['name']==marker
        assert raw['requested_parser']=='v0.0.44'
        # Preserve malformed scheduler bytes for parser debugging without changing exit status.
        (fixtures/'queue.json').write_text('{malformed-private-response')
        cli('jobs','--debug','--debug-raw',success=False)
        raw=next(e for e in reversed(events(rawfile)) if e['raw_captured'] and e['operation']=='squeue')
        assert base64.b64decode(raw['stdout_base64'])==b'{malformed-private-response'
        (fixtures/'queue.json').write_text((ROOT/'testdata/hummingbird/queue.json').read_text())
        # The wrapper must still allow interactive sign-in to suspend and resume the TUI.
        s=Session(['--host','perch-test.invalid','--no-config','--debug','--reduced-motion'],120,40,env=env)
        try:
            s.wait('● connected');s.key('4');s.wait('Diagnostics: metadata only')
            s.screenshot('diagnostics-connection')
            s.key('c');s.wait('● connected');s.pump(.4)
            assert s.raw.count(b'\x1b[?1049h')>=2, 'debug wrapper lost login capability'
            s.close()
        except BaseException:
            s.abort();raise
        # MCP stdout remains protocol-only even though stderr announces capture.
        p=subprocess.Popen([binary,'mcp','--host','perch-test.invalid','--no-config','--debug'],env=env,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
        selector=selectors.DefaultSelector();selector.register(p.stdout,selectors.EVENT_READ)
        def send(body,reply=True):
            p.stdin.write(json.dumps(body)+'\n');p.stdin.flush()
            if reply:
                assert selector.select(10), 'MCP timed out'
                return json.loads(p.stdout.readline())
        try:
            response=send({'jsonrpc':'2.0','id':1,'method':'initialize','params':{'protocolVersion':'2025-11-25','capabilities':{},'clientInfo':{'name':'debug-test','version':'1'}}})
            assert 'result' in response
            send({'jsonrpc':'2.0','method':'notifications/initialized','params':{}},False)
            response=send({'jsonrpc':'2.0','id':2,'method':'tools/call','params':{'name':'perch_jobs','arguments':{}}})
            assert response['result']['structuredContent']['source']=='hummingbird'
            p.stdin.close(); assert p.wait(5)==0
            assert 'metadata only' in p.stderr.read()
        finally:
            selector.close()
            if p.poll() is None:p.terminate();p.wait(5)
        # An unwritable/unsafe sink fails before a query; no JSON success claim.
        file.chmod(0o644)
        r=cli('jobs','--debug','--json',success=False)
        assert not r.stdout and 'cannot start diagnostics' in r.stderr
    print('Diagnostics: opt-in, private files, metadata redaction, raw parser evidence, TUI reconnect and MCP protocol isolation passed.')
if __name__=='__main__':main()
