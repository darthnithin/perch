#!/usr/bin/env python3
"""Built CLI acceptance against a local synthetic TLS API; never contacts NRP.

Uses a temporary certificate, kubeconfigs, home and fake kubectl. When kubectl
is installed, also tests its real config-only path against the synthetic file.
"""
import base64
import argparse
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
import pty
import termios
from pathlib import Path
import shutil
import ssl
import subprocess
import sys
import tempfile
import threading
import time
from urllib.parse import urlsplit, parse_qs

ROOT = Path(__file__).resolve().parents[1]
BINARY = ROOT / 'bin/perch'


def main():
    global BINARY
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', type=Path, default=BINARY)
    parser.add_argument('--ui', action='store_true')
    args = parser.parse_args()
    BINARY = args.binary.resolve()
    kubectl = shutil.which('kubectl')
    openssl = shutil.which('openssl')
    assert openssl, 'openssl is required for this local TLS acceptance harness'
    with tempfile.TemporaryDirectory(prefix='perch-nrp-') as tmp:
        folder = Path(tmp)
        cert, key = folder/'cert.pem', folder/'key.pem'
        conf = folder/'openssl.cnf'
        conf.write_text('[req]\nprompt=no\ndistinguished_name=dn\nx509_extensions=v3\n'
                        '[dn]\nCN=perch-local-test\n[v3]\nsubjectAltName=IP:127.0.0.1\n'
                        'basicConstraints=critical,CA:TRUE\n'
                        'keyUsage=critical,digitalSignature,keyEncipherment,keyCertSign\n')
        subprocess.run([openssl, 'req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '1',
                        '-config', str(conf), '-keyout', str(key), '-out', str(cert)],
                       check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        requests = []
        denied = set()
        unauthenticated = set()
        token = 'synthetic-local-acceptance-token'
        validation = {'code':201, 'bodies':[]}

        class API(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_POST(self):
                requests.append(self.path)
                assert self.path == '/apis/batch/v1/namespaces/research/jobs?dryRun=All&fieldValidation=Strict', self.path
                assert self.headers.get('Authorization') == 'Bearer '+token
                assert self.headers.get('Content-Type') == 'application/json'
                payload = self.rfile.read(int(self.headers['Content-Length']))
                validation['bodies'].append(payload)
                self.send_response(validation['code'])
                self.end_headers()
                self.wfile.write(payload if validation['code']==201 else b'private-server-output-must-not-escape')

            def do_GET(self):
                requests.append(self.path)
                if self.headers.get('Authorization') != 'Bearer '+token:
                    self.send_error(401)
                    return
                paths = {'/apis/batch/v1/namespaces/research/jobs?limit=250': 'jobs',
                         '/api/v1/namespaces/research/pods?limit=250': 'pods',
                         '/api/v1/namespaces/research/resourcequotas?limit=250': 'resourcequotas'}
                resource = paths.get(self.path)
                if self.path == '/apis/metrics.k8s.io': resource = 'metrics-api'
                if self.path == '/apis/metrics.k8s.io/v1/namespaces/research/pods/retrying-replacement': resource = 'metrics'
                endpoint = urlsplit(self.path)
                pod_path = '/api/v1/namespaces/research/pods/retrying-replacement'
                if endpoint.path == pod_path and not endpoint.query:
                    resource = 'pod'
                elif endpoint.path == pod_path+'/log':
                    resource = 'logs'
                    query = parse_qs(endpoint.query)
                    assert query == {'container': ['compute'], 'follow': ['false'],
                                     'previous': query.get('previous'), 'limitBytes': ['262144'],
                                     'tailLines': ['2000']}
                    assert query['previous'] in [['true'], ['false']]
                if resource in unauthenticated:
                    self.send_error(401)
                    return
                if resource is None or resource in denied:
                    self.send_response(403)
                    self.end_headers()
                    self.wfile.write(b'private-server-output-must-not-escape')
                    return
                if resource == 'metrics-api':
                    payload = json.dumps({'kind':'APIGroup','apiVersion':'v1','name':'metrics.k8s.io','versions':[{'version':'v1','groupVersion':'metrics.k8s.io/v1'}]}).encode()
                elif resource == 'metrics':
                    payload = json.dumps({'kind':'PodMetrics','apiVersion':'metrics.k8s.io/v1','metadata':{'name':'retrying-replacement','namespace':'research'},'timestamp':time.strftime('%Y-%m-%dT%H:%M:%SZ',time.gmtime(time.time()-10)),'window':'30s','containers':[{'name':'compute','usage':{'cpu':'215m','memory':'146Mi'}}]}).encode()
                elif resource == 'logs':
                    which = 'previous' if query['previous'] == ['true'] else 'current'
                    payload = f'{which} log\x1b[2J\ncheckpoint\n'.encode()
                elif resource == 'pod':
                    pods = json.loads((ROOT/'testdata/nautilus/pods.json').read_text())['items']
                    pod = next(p for p in pods if p['metadata']['name'] == 'retrying-replacement')
                    pod.update(kind='Pod', apiVersion='v1')
                    pod['metadata']['resourceVersion'] = 'pod-current'
                    payload = json.dumps(pod).encode()
                else:
                    payload = (ROOT/'testdata/nautilus'/f'{resource}.json').read_bytes()
                self.send_response(200)
                self.send_header('Content-Type', 'text/plain' if resource == 'logs' else 'application/json')
                self.send_header('Content-Length', str(len(payload)))
                self.end_headers()
                self.wfile.write(payload)

        api = ThreadingHTTPServer(('127.0.0.1', 0), API)
        tls = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        tls.load_cert_chain(cert, key)
        api.socket = tls.wrap_socket(api.socket, server_side=True)
        thread = threading.Thread(target=api.serve_forever, kwargs={'poll_interval': .05}, daemon=True)
        thread.start()
        try:
            cfg = {'kind': 'Config', 'apiVersion': 'v1', 'current-context': 'unselected',
                   'contexts': [{'name': name, 'context': {'cluster': 'local-test', 'user': 'test',
                                                         'namespace': 'wrong-default'}}
                                for name in ['selected', 'unselected']],
                   'clusters': [{'name': 'local-test', 'cluster': {
                       'server': f'https://127.0.0.1:{api.server_port}',
                       'certificate-authority-data': base64.b64encode(cert.read_bytes()).decode()}}],
                   'users': [{'name': 'test', 'user': {'token': token}}]}
            config = folder/'config with spaces.json'
            config.write_text(json.dumps(cfg))
            fake = folder/'kubectl'
            fake.write_text('#!'+sys.executable+'\n'
                            'import json,os,sys\n'
                            'from pathlib import Path\n'
                            'args=sys.argv[1:]\n'
                            'expected=["config","view","--context=selected","--minify","--flatten",'
                            '"--raw","--output=json","--kubeconfig="+os.environ["NRP_TEST_CONFIG"]]\n'
                            'assert args==expected\n'
                            'p=Path(os.environ["NRP_TEST_ARGS"]);p.write_text(json.dumps(args))\n'
                            'c=json.loads(Path(os.environ["NRP_TEST_CONFIG"]).read_text())\n'
                            'c["current-context"]="selected"\n'
                            'c["contexts"]=[x for x in c["contexts"] if x["name"]=="selected"]\n'
                            'print(json.dumps(c))\n')
            fake.chmod(0o700)
            env = {**os.environ, 'HOME': tmp, 'XDG_CONFIG_HOME': tmp, 'XDG_CACHE_HOME': tmp,
                   'TERM': 'xterm-256color', 'COLORTERM': 'truecolor',
                   'PATH': tmp, 'NRP_TEST_CONFIG': str(config), 'NRP_TEST_ARGS': str(folder/'args')}
            connection = ['--context=selected', '--namespace=research', '--kubeconfig', str(config)]

            def invoke(args, ok=True):
                result = subprocess.run([str(BINARY), 'nrp', *args], env=env,
                                        capture_output=True, text=True, timeout=15)
                assert (result.returncode == 0) == ok, result.stderr
                assert token not in result.stdout+result.stderr
                assert 'private-server-output' not in result.stdout+result.stderr
                return result

            # Real CLI config loading + TLS, but never a real Kubernetes server.
            prepared = invoke(['prepare','--name','analysis','--namespace','research',
                              '--image','python:3.13-slim','--cpu','500m','--memory','512Mi',
                              '--ephemeral-storage','1Gi','--time','5m','--manifest','--',
                              'python','-c','print(sum(range(1000)))']).stdout
            job_file=folder/'job.json'; job_file.write_text(prepared)
            checked=json.loads(invoke(['validate',str(job_file),*connection,'--json']).stdout)
            assert checked['data']['job_dry_run_admitted'] is True
            assert checked['data']['status']=='accepted' and checked['data']['next_step']
            assert checked['data']['manifest_sha256']==hashlib.sha256(prepared.encode()).hexdigest()
            assert checked['data']['quota_snapshot']['collection']['items']
            assert validation['bodies']==[prepared.encode()]
            text=invoke(['validate',str(job_file),*connection]).stdout
            assert 'not submitted' in text and 'Pods are admitted separately' in text
            denied.add('resourcequotas')
            checked=json.loads(invoke(['validate',str(job_file),*connection,'--json']).stdout)
            assert checked['data']['job_dry_run_admitted'] and checked['data']['quota_snapshot']['error']['kind']=='forbidden'
            denied.clear()
            for code, status, hint in [(401,'unverified','--login'),
                                       (403,'rejected','can create Jobs'),
                                       (404,'unverified','batch/v1'),
                                       (409,'rejected','metadata.name'),
                                       (422,'rejected','correct the manifest'),
                                       (500,'unverified','retry this dry-run')]:
                validation['code']=code
                before=len(validation['bodies'])
                checked=json.loads(invoke(['validate',str(job_file),*connection,'--json'],ok=False).stdout)
                assert not checked['data']['job_dry_run_admitted'] and checked['data']['error']
                assert checked['data']['status']==status and hint in checked['data']['next_step']
                assert len(validation['bodies'])==before+1, 'validation retried a POST'
                before=len(validation['bodies'])
                report=invoke(['validate',str(job_file),*connection],ok=False)
                assert f'Job dry-run {status} · not submitted' in report.stdout
                assert hint in report.stdout and f'Job dry-run {status};' in report.stderr
                assert len(validation['bodies'])==before+1, 'text validation retried a POST'
            validation['code']=201

            for name, count in [('jobs', 8), ('pods', 5), ('quotas', 3)]:
                r = json.loads(invoke([name, *connection, '--json']).stdout)
                assert r['source'] == 'kubernetes-api' and r['context'] == 'selected'
                assert r['scope']['namespace'] == 'research' and r['scope']['cluster'].startswith('k8s:')
                assert len(r['data']) == count
            p = json.loads(invoke(['pod', 'retrying-replacement', *connection, '--json']).stdout)['data']
            compute = next(c for c in p['containers'] if c['name'] == 'compute')
            assert compute['current']['phase'] == 'Running'
            assert compute['previous']['reason'] == 'OOMKilled'
            denied.add('pods')
            failure = invoke(['pods', *connection, '--json'], ok=False)
            assert failure.stdout == '' and 'denied' in failure.stderr
            denied.clear()
            sample=json.loads(invoke(['metrics','retrying-replacement',*connection,'--json']).stdout)['data']
            assert sample['api_version']=='metrics.k8s.io/v1' and sample['containers'][0]['cpu_cores']=='0.215'
            assert sample['containers'][0]['memory_working_set_bytes']=='153092096'
            denied.add('metrics')
            failure=invoke(['metrics','retrying-replacement',*connection,'--json'],ok=False)
            assert not failure.stdout and 'denied' in failure.stderr
            denied.clear()
            for previous in [False, True]:
                args = ['logs', 'retrying-replacement', '--container', 'compute', *connection]
                if previous:
                    args.append('--previous')
                log = json.loads(invoke([*args, '--json']).stdout)['data']
                assert log['pod']['uid'] == 'pod-replacement' and log['container'] == 'compute'
                assert log['previous'] == previous and log['byte_limit'] == 262144 and log['tail_lines'] == 2000
                assert log['text'] == ('previous' if previous else 'current')+' log\ncheckpoint\n'
                assert invoke(args).stdout == log['text']
            denied.add('logs')
            failed_log = invoke(['logs', 'retrying-replacement', '--container', 'compute', *connection], ok=False)
            assert failed_log.stdout == '' and 'denied' in failed_log.stderr
            denied.clear()
            if '--ui' in sys.argv[1:]:
                # Import the optional terminal dependencies only for UI acceptance.
                from test_terminal import Session
                s = Session(['nrp', 'ui', *connection], 100, 32, env=env, binary=BINARY)
                try:
                    s.wait('Jobs current · Pods current')
                    s.wait('indexed')
                    s.key('3')
                    s.wait('NVIDIA GPU requests')
                    s.wait('at limit')
                    denied.add('resourcequotas')
                    s.pump(2.1)
                    s.key('r')
                    s.wait('Stale quota snapshot')
                    assert 'compute' in s.text, 'quota permission failure erased last observation'
                    s.screenshot('nrp-quota-stale')
                    denied.clear()
                    s.pump(2.1)
                    s.key('r')
                    s.wait('Jobs current · Pods current')
                    s.wait('NVIDIA GPU requests')
                    assert 'Stale quota snapshot' not in s.text
                    s.key('1')
                    denied.add('jobs')
                    s.pump(2.1)
                    s.key('r')
                    s.wait('Jobs stale · Pods current')
                    assert 'indexed' in s.text, 'failed read erased cached Jobs'
                    s.screenshot('nrp-partial-failure')
                    s.key('4')
                    s.wait('denied')
                    assert 'private-server-output' not in s.text
                    denied.clear()
                    s.pump(2.1)
                    s.key('r')
                    s.wait('Jobs current · Pods current')
                    unauthenticated.update(['jobs', 'pods'])
                    s.pump(2.1)
                    s.key('r')
                    s.wait('Sign-in required')
                    s.key('1')
                    s.wait('indexed')
                    s.screenshot('nrp-auth-expired')
                    seen_auth = len(requests)
                    s.pump(2.1)
                    assert len(requests) == seen_auth, 'background request after 401'
                    unauthenticated.clear()
                    s.key('c')
                    s.wait('Jobs current · Pods current')
                    assert len(requests) == seen_auth+3
                    s.wait('indexed')
                    assert token not in s.raw.decode(errors='replace')
                    s.close()
                except BaseException:
                    print(s.text)
                    s.screenshot('nrp-live-ui-failure')
                    s.abort()
                    raise
                # Exercise the production log adapter from the TUI, including
                # permissions and expired credentials on the log subresource.
                s = Session(['nrp', 'ui', *connection], 80, 24, env=env, binary=BINARY)
                try:
                    s.wait('Jobs current · Pods current')
                    s.key('2')
                    s.key('/find retrying-replacement\r')
                    s.wait('1–1 of 1')
                    s.key('u')
                    s.wait('0.215 cores')
                    s.wait('146Mi working set')
                    denied.add('metrics')
                    s.pump(2.1)
                    s.key('r')
                    s.wait('Retained sample · read failed')
                    s.wait('Read failed · sample')
                    assert '0.215 cores' in s.text and '146Mi working set' in s.text
                    assert 'private-server-output' not in s.text
                    s.screenshot('nrp-metrics-stale-https')
                    denied.clear()
                    s.pump(2.1)
                    s.key('r')
                    s.wait('0.215 cores')
                    s.pump(.5)
                    assert 'Read failed' not in s.text and 'read failed' not in s.text
                    s.key('\x1b')
                    s.wait('1–1 of 1')
                    s.key('l')
                    s.wait('Containers')
                    s.key('\r')
                    s.wait('current · snapshot')
                    s.wait('current log')
                    denied.add('logs')
                    s.pump(2.1)
                    s.key('r')
                    s.wait('current · stale')
                    s.wait('denied')
                    assert 'current log' in s.text and 'checkpoint' in s.text
                    assert 'private-server-output' not in s.text
                    s.screenshot('nrp-log-stale-https')
                    # A denied previous read must never relabel retained current
                    # text as output from the previous container instance.
                    s.key('p')
                    s.wait('previous · read failed')
                    assert 'current log' not in s.text
                    denied.clear()
                    s.pump(2.1)
                    s.key('r')
                    s.wait('previous · snapshot')
                    s.wait('previous log')
                    unauthenticated.add('logs')
                    s.pump(2.1)
                    s.key('r')
                    s.wait('Sign-in required')
                    s.wait('previous · stale')
                    assert 'previous log' in s.text
                    seen_auth = len(requests)
                    s.pump(2.1)
                    s.key('r')
                    s.wait('Sign in with c')
                    assert len(requests) == seen_auth, 'log retry bypassed explicit login'
                    unauthenticated.clear()
                    s.key('c')
                    s.wait('Jobs current · Pods current')
                    s.wait('previous · snapshot')
                    s.wait('previous log')
                    assert len(requests) == seen_auth+6, 'reconnect did not refresh lists and selected log'
                    s.screenshot('nrp-log-reconnected-https')
                    s.key('\x1b')
                    s.wait('Containers')
                    s.key('\x1b')
                    s.wait('Filter: retrying-replacement')
                    assert token not in s.raw.decode(errors='replace')
                    s.close()
                except BaseException:
                    print(s.text)
                    s.screenshot('nrp-log-https-failure')
                    s.abort()
                    raise
            marker = (folder/'args').stat().st_mtime_ns
            seen = len(requests)
            for name in ['jobs', 'pods']:
                r = json.loads(invoke([name, '--demo', '--json']).stdout)
                assert r['source'] == 'synthetic-demo'
            demo_log = json.loads(invoke(['logs', 'retrying-replacement', '--container', 'compute', '--previous', '--demo', '--json']).stdout)
            assert demo_log['source'] == 'synthetic-demo' and '[demo]' in demo_log['data']['text']
            assert (folder/'args').stat().st_mtime_ns == marker and len(requests) == seen
            missing = invoke(['job', 'absent', '--demo', '--json'], ok=False)
            assert missing.stdout == '' and 'absence is not a completion' in missing.stderr

            # Explicit login gets a terminal on stdin but keeps JSON stdout
            # isolated from the foreground credential-plugin messages.
            login_plugin = folder/'synthetic-login.py'
            login_record = folder/'login-record.json'
            login_plugin.write_text(
                'import json,os,sys,termios,time\nfrom pathlib import Path\n'
                'info=json.loads(os.environ["KUBERNETES_EXEC_INFO"])\n'
                'interactive=info["spec"]["interactive"]\n'
                'if interactive:print("Synthetic login prompt",file=sys.stderr,flush=True)\n'
                'record={"interactive":interactive,"tty":sys.stdin.isatty(),'
                '"input":sys.stdin.readline() if interactive else "",'
                '"cluster":info["spec"].get("cluster")}\n'
                'if interactive:\n'
                ' modes=termios.tcgetattr(sys.stdin.fileno())\n'
                ' modes[3] &= ~(termios.ECHO | termios.ICANON)\n'
                ' termios.tcsetattr(sys.stdin.fileno(),termios.TCSANOW,modes)\n'
                ' record["changed_modes"]=True\n'
                'Path(os.environ["NRP_LOGIN_RECORD"]).write_text(json.dumps(record))\n'
                'print("Synthetic login instructions",file=sys.stderr,flush=True)\n'
                'if os.environ.get("NRP_LOGIN_FAIL"):sys.exit(7)\n'
                'if os.environ.get("NRP_LOGIN_WAIT"):time.sleep(10)\n'
                'status={"token":os.environ["NRP_LOGIN_TOKEN"]}\n'
                'if os.environ.get("NRP_LOGIN_EXPIRED"):status["expirationTimestamp"]="2000-01-01T00:00:00Z"\n'
                'print(json.dumps({"kind":"ExecCredential","apiVersion":info["apiVersion"],"status":status}))\n')
            cfg['users'][0]['user'] = {'exec': {
                'apiVersion': 'client.authentication.k8s.io/v1',
                'command': sys.executable, 'args': [str(login_plugin)],
                'interactiveMode': 'Always', 'provideClusterInfo': True,
                'env': [{'name': 'NRP_LOGIN_RECORD', 'value': str(login_record)},
                        {'name': 'NRP_LOGIN_TOKEN', 'value': token}]}}
            config.write_text(json.dumps(cfg))
            seen = len(requests)
            fail = invoke(['jobs', *connection, '--login', '--json'], ok=False)
            assert 'interactive terminal' in fail.stderr and not login_record.exists()
            assert len(requests) == seen

            if '--ui' in sys.argv[1:]:
                s = Session(['nrp', 'ui', *connection, '--login'], 100, 32, env={**env, 'TERM_PROGRAM':'ghostty', 'TMUX':'', 'STY':''}, binary=BINARY)
                try:
                    s.wait('Synthetic login prompt')
                    s.key('synthetic answer\n')
                    s.wait('Jobs current · Pods current')
                    marker = login_record.stat().st_mtime_ns
                    unauthenticated.update(['jobs', 'pods'])
                    s.pump(2.1)
                    s.key('r')
                    s.wait('Sign-in required')
                    assert login_record.stat().st_mtime_ns == marker, '401 silently ran plugin'
                    count_before_login = len(requests)
                    cfg['users'][0]['user']['exec']['env'].append({'name': 'NRP_LOGIN_FAIL', 'value': '1'})
                    config.write_text(json.dumps(cfg))
                    from test_hover import move, shape
                    move(s,12,2);shape(s,'pointer')
                    start = len(s.raw)
                    s.key('c')
                    s.wait('Synthetic login prompt')
                    output = bytes(s.raw[start:])
                    assert output.index(b'\x1b]22;default\x07') < output.index(b'Synthetic login prompt'), 'foreground login inherited hand cursor'
                    s.key('synthetic answer\n')
                    s.wait('Sign-in did not complete')
                    assert len(requests) == count_before_login
                    assert login_record.stat().st_mtime_ns != marker
                    assert json.loads(login_record.read_text())['changed_modes']
                    cfg['users'][0]['user']['exec']['env'].pop()
                    config.write_text(json.dumps(cfg))
                    unauthenticated.clear()
                    # Successful recovery uses the visible connection action,
                    # while the failed attempt above exercises the keyboard.
                    from test_footer import footer, activate
                    s.key('4')
                    move(s,*footer(s,'c connect'));shape(s,'pointer')
                    start = len(s.raw)
                    activate(s,'c connect')
                    s.wait('Synthetic login prompt')
                    s.key('synthetic answer\n')
                    s.wait('Jobs current · Pods current')
                    output = bytes(s.raw[start:])
                    assert output.index(b'\x1b]22;default\x07') < output.index(b'Synthetic login prompt')
                    assert len(requests) == count_before_login+3
                    s.key('2')
                    s.wait('Namespace Pods')
                    assert token not in s.raw.decode(errors='replace')
                    s.close()
                except BaseException:
                    print(s.text)
                    s.screenshot('nrp-login-ui-failure')
                    s.abort()
                    raise

            def terminal_login(ok=True, extra=()):
                master, slave = pty.openpty()
                before = termios.tcgetattr(slave)
                try:
                    process = subprocess.Popen([str(BINARY), 'nrp', 'jobs', *connection,
                                                '--login', '--json', *extra], stdin=slave,
                                               stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                               text=True, env=env)
                    os.write(master, b'synthetic answer\n')
                    stdout, stderr = process.communicate(timeout=15)
                    assert (process.returncode == 0) == ok, stderr
                    assert token not in stdout+stderr
                    assert 'Synthetic login instructions' in stderr
                    after = termios.tcgetattr(slave)
                    # Darwin sets this transient input-retype flag when canonical
                    # input is restored. All actual modes/control chars must match.
                    before[3] &= ~getattr(termios, 'PENDIN', 0)
                    after[3] &= ~getattr(termios, 'PENDIN', 0)
                    assert after == before, f'sign-in changed terminal modes: {before!r} -> {after!r}'
                    return stdout, stderr
                finally:
                    if 'process' in locals() and process.poll() is None:
                        process.kill(); process.communicate()
                    os.close(master); os.close(slave)

            for mode in ['Always', 'Never']:
                cfg['users'][0]['user']['exec']['interactiveMode'] = mode
                config.write_text(json.dumps(cfg))
                before = hashlib.sha256(config.read_bytes()).digest()
                stdout, stderr = terminal_login()
                assert len(json.loads(stdout)['data']) == 8
                record = json.loads(login_record.read_text())
                assert record['interactive'] == (mode == 'Always')
                assert record['tty'] == (mode == 'Always')
                assert record['input'] == ('synthetic answer\n' if mode == 'Always' else '')
                assert bool(record.get('changed_modes')) == (mode == 'Always')
                assert record['cluster']['server'] == cfg['clusters'][0]['cluster']['server']
                assert hashlib.sha256(config.read_bytes()).digest() == before
            marker = login_record.stat().st_mtime_ns
            seen = len(requests)
            failure = invoke(['jobs', *connection], ok=False)
            assert 'no credential command was run' in failure.stderr
            assert login_record.stat().st_mtime_ns == marker and len(requests) == seen
            cfg['users'][0]['user']['exec']['interactiveMode'] = 'Always'
            config.write_text(json.dumps(cfg))
            env['NRP_LOGIN_EXPIRED'] = '1'
            stdout, stderr = terminal_login(ok=False)
            assert stdout == '' and 'expired session' in stderr and len(requests) == seen
            del env['NRP_LOGIN_EXPIRED']
            for flag in ['NRP_LOGIN_FAIL', 'NRP_LOGIN_WAIT']:
                env[flag] = '1'
                stdout, stderr = terminal_login(ok=False, extra=['--timeout=2s'])
                assert stdout == '' and len(requests) == seen
                assert json.loads(login_record.read_text())['changed_modes']
                del env[flag]
            # Restore static credentials before checking installed kubectl.
            cfg['users'][0]['user'] = {'token': token}
            config.write_text(json.dumps(cfg))

            real_config_check = 'not installed; skipped'
            if kubectl:
                fake.unlink()
                fake.symlink_to(kubectl)
                before = hashlib.sha256(config.read_bytes()).digest()
                live = json.loads(invoke(['jobs', *connection, '--json']).stdout)
                assert len(live['data']) == 8
                assert hashlib.sha256(config.read_bytes()).digest() == before
                sentinel = folder/'credential-plugin-ran'
                cfg['users'][0]['user'] = {'exec': {
                    'apiVersion': 'client.authentication.k8s.io/v1', 'interactiveMode': 'Never',
                    'command': sys.executable,
                    'args': ['-c', 'from pathlib import Path; Path('+repr(str(sentinel))+').touch()']}}
                config.write_text(json.dumps(cfg))
                before = hashlib.sha256(config.read_bytes()).digest()
                seen = len(requests)
                failure = invoke(['jobs', *connection], ok=False)
                assert failure.stdout == '' and 'no credential command was run' in failure.stderr
                assert not sentinel.exists() and len(requests) == seen
                assert hashlib.sha256(config.read_bytes()).digest() == before
                real_config_check = 'static TLS query and exec-plugin refusal passed; config unchanged'
            print(json.dumps({'binary_sha256': hashlib.sha256(BINARY.read_bytes()).hexdigest(), 'nrp_cli': 'passed', 'api': 'local synthetic HTTPS only',
                              'nrp_ui': ('PTY partial failure, retained snapshots, explicit static/plugin reconnect, '
                                         'plugin failure recovery and terminal restoration passed') if '--ui' in sys.argv[1:] else 'not requested',
                              'checks': ['explicit scope', 'built-binary Job/Pod queries',
                                         'strict Job dry-run only, exact manifest bytes, quota partial failure, denied/conflict/admission failures',
                                         'bounded current/previous logs with UID checks and terminal sanitization',
                                         'log permission failure produces no stdout',
                                         'container current/previous states', 'permission-error redaction',
                                         'offline demo isolation', 'missing-object failure',
                                         'explicit PTY login and JSON isolation',
                                         'stdin Always/Never', 'expired credential refusal',
                                         'noninteractive login refusal',
                                         'terminal restoration after plugin changes, failure and timeout'],
                              'installed_kubectl': real_config_check}, indent=2))
        finally:
            api.shutdown()
            api.server_close()
            thread.join(timeout=3)


if __name__ == '__main__':
    main()
