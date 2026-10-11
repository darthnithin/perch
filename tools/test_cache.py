#!/usr/bin/env python3
"""Opt-in cache restart/recovery through the real parser and PTY; synthetic SSH only."""
import json
import os
import pathlib
import subprocess
import sys
import tempfile
import time
from test_terminal import Session, ROOT
from test_failure_terminal import FAKE_SSH


def main():
    with tempfile.TemporaryDirectory(prefix='perch-cache-') as tmp:
        root=pathlib.Path(tmp);home=root/'home';home.mkdir(mode=0o700)
        state=root/'state';state.write_text('ok')
        ssh=root/'ssh';ssh.write_text('#!'+sys.executable+'\n'+FAKE_SSH);ssh.chmod(0o700)
        fixtures=root/'fixtures';fixtures.mkdir()
        for name in ['queue','accounting','partitions']:
            data=json.loads((ROOT/'testdata/hummingbird'/(name+'.json')).read_text())
            if name=='queue':
                now=int(time.time())
                for i,j in enumerate(data['jobs']):
                    for key,value in [('submit_time',now-1000),('eligible_time',now-900),('start_time',now-600+i*30),('end_time',now+6600+i*30)]:
                        j[key]['number']=value
            (fixtures/(name+'.json')).write_text(json.dumps(data))
        settings={**os.environ,'HOME':str(home),'XDG_CACHE_HOME':str(home/'cache'),
                  'TERM':'xterm-256color','COLORTERM':'truecolor','PATH':str(root)+':/usr/bin:/bin','PERCH_TEST_STATE':str(state),'PERCH_TEST_FIXTURES':str(fixtures)}
        args=['--host','perch-test.invalid','--no-config','--reduced-motion']
        # Neither ordinary sessions nor rejected command flags create a cache.
        for command in [['--demo','--cache'],['jobs','--cache','--json']]:
            p=subprocess.run([str(ROOT/'bin/perch'),*args,*command],env=settings,capture_output=True,text=True,timeout=10)
            assert p.returncode and '--cache requires a live TUI' in p.stderr
        s=Session(args,100,32,env=settings)
        try:s.wait('● connected');s.close()
        except BaseException:s.abort();raise
        assert not list(home.rglob('view.json'))
        s=Session([*args,'--cache'],120,38,env=settings)
        try:
            s.wait('● connected');s.key('4');s.wait('Cache saved')
            cache=next(home.rglob('view.json'))
            assert cache.stat().st_mode&0o777==0o600 and cache.parent.stat().st_mode&0o777==0o700
            saved=json.loads(cache.read_text())
            assert saved['Scope']=={'Host':'perch-test.invalid','User':'researcher','Cluster':'hummingbird'}
            for j in saved['Snapshot']['Jobs']+saved['History']:
                assert not j['Stdout'] and not j['Stderr'] and not j['WorkDir'] and not j['Steps']
            s.close()
        except BaseException:print(s.text);s.abort();raise
        before=cache.read_bytes();state.write_text('network')
        s=Session([*args,'--cache'],120,38,env=settings)
        try:
            s.wait('cached · disconnected');s.wait('simulation-sweep')
            s.key('/find simulation\r');s.wait('Filter: simulation')
            s.key('\r');s.wait('simulation-sweep')
            s.screenshot('cache-offline-inspector')
            s.key('l');s.wait('Saved job');s.wait('before reading')
            assert cache.read_bytes()==before,'offline view rewrote its timestamp'
            s.key('\x1b');s.resize(48,24);s.wait('cached · disconnected');s.screenshot('cache-offline-compact')
            # Reconnect, then discard saved rows once current data arrives.
            state.write_text('ok');s.key('c');s.wait('● connected');s.wait('Filter: simulation')
            assert 'cached ·' not in s.text
            s.resize(120,38);s.screenshot('cache-reconnected')
            s.key('4');s.wait('Cache saved');s.close()
        except BaseException:print(s.text);s.screenshot('cache-test-failure');s.abort();raise
        # A saved alias cannot bring its old account's rows into a new identity.
        saved=json.loads(cache.read_text());saved['Scope']['User']='previous-user';saved['Snapshot']['User']='previous-user'
        for j in saved['Snapshot']['Jobs']+saved['History']:j['User']='previous-user';j['Name']='old-account-private-name'
        cache.write_text(json.dumps(saved))
        state.write_text('partial') # identity probe succeeds, queue read fails
        s=Session([*args,'--cache'],100,32,env=settings)
        try:
            s.wait('personal jobs are unavailable');assert 'old-account-private-name' not in s.text
            s.key('4');s.wait('identity changed');s.close()
        except BaseException:print(s.text);s.abort();raise
        # Expired data is never resurrected after a failed probe.
        saved['Snapshot']['At']='2000-01-01T00:00:00Z';cache.write_text(json.dumps(saved));state.write_text('network')
        s=Session([*args,'--cache'],80,24,env=settings)
        try:
            s.wait('Unable to load your jobs');assert 'old-account-private-name' not in s.text
            s.key('4');s.key('\x1b[F');s.wait('expired');s.close()
        except BaseException:print(s.text);s.abort();raise
    print('Snapshot cache PTY passed: opt-in, CLI/demo refusal, private whitelist, restart with failed SSH, cached navigation/detail guards, compact status, filter-preserving reconnect, scope mismatch, expiry and cleanup. Synthetic SSH only.')


if __name__=='__main__':main()
