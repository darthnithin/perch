#!/usr/bin/env python3
"""Scoped GPU requests through production CLI, MCP and actual TUI processes."""
import json
import os
import pathlib
import queue
import shutil
import subprocess
import sys
import tempfile
import threading

from test_failure_terminal import FAKE_SSH
from test_terminal import ROOT, Session
from test_mouse import target, click

EXPECTED=[{'scope':'node','type':'p100','count':1},{'scope':'task','type':'','count':1}]


def mcp_check(env):
    p=subprocess.Popen([str(ROOT/'bin/perch'),'mcp','--host','perch-test.invalid','--no-config'],
                       stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE,
                       text=True,env={**os.environ,**env})
    messages=queue.Queue()
    def receive():
        try:
            for line in p.stdout: messages.put(json.loads(line))
        except Exception as e: messages.put(e)
        messages.put(EOFError('MCP ended before response'))
    threading.Thread(target=receive,daemon=True).start()
    def send(i,method,params):
        body={'jsonrpc':'2.0','method':method,'params':params}
        if i is not None:body['id']=i
        p.stdin.write(json.dumps(body)+'\n');p.stdin.flush()
        if i is None:return
        while True:
            message=messages.get(timeout=15)
            if isinstance(message,Exception):raise message
            if message.get('id')==i:return message
    try:
        assert 'result' in send(1,'initialize',{'protocolVersion':'2025-11-25','capabilities':{},'clientInfo':{'name':'gpu-check','version':'1'}})
        send(None,'notifications/initialized',{})
        result=send(2,'tools/call',{'name':'perch_job','arguments':{'job_id':'9300'}})['result']
        assert not result.get('isError'),result
        structured=result['structuredContent']
        assert structured['data']['gpu_requests']==EXPECTED,structured
        assert json.loads(result['content'][0]['text'])==structured
        p.stdin.close();assert p.wait(timeout=10)==0
        assert not p.stderr.read()
    finally:
        if p.poll() is None:p.kill();p.wait(timeout=5)
        p.stdout.close();p.stderr.close()


def main():
    with tempfile.TemporaryDirectory(prefix='perch-job-gpus-') as directory:
        folder=pathlib.Path(directory)
        state=folder/'state';state.write_text('ok')
        for name in ['queue','accounting','partitions']:
            shutil.copyfile(ROOT/'testdata/hummingbird'/f'{name}.json',folder/f'{name}.json')
        data=json.loads((folder/'queue.json').read_text());job=data['jobs'][-1]
        job.update(job_id=9300,name='gpu-study',partition='96x24gpu4',job_state=['PENDING'],state_reason='Resources',nodes='',array_task_string='',
                   tres_per_node='gres/gpu:p100:1',tres_per_task='gres/gpu:1',tres_req_str='cpu=1,mem=512M',tres_alloc_str='')
        for field in ['array_job_id','array_task_id','start_time','end_time']:
            job[field]={'set':False,'infinite':False,'number':0}
        data['jobs']=[job]
        def save(): (folder/'queue.json').write_text(json.dumps(data))
        save()
        ssh=folder/'ssh';ssh.write_text('#!'+sys.executable+'\n'+FAKE_SSH);ssh.chmod(0o700)
        env={'PATH':str(folder)+':/usr/bin:/bin','PERCH_TEST_STATE':str(state),'PERCH_TEST_FIXTURES':str(folder)}
        def cli(job_id,as_json=True):
            args=[str(ROOT/'bin/perch'),'job',job_id,'--host','perch-test.invalid','--no-config']
            if as_json:args.append('--json')
            return subprocess.run(args,capture_output=True,text=True,env={**os.environ,**env},check=True,timeout=15).stdout
        assert json.loads(cli('9300'))['data']['gpu_requests']==EXPECTED
        assert json.loads(cli('9000_0'))['data']['gpu_requests'] is None
        text=cli('9300',False)
        assert '1 p100 GPU per node' in text and '1 GPU per task' in text
        mcp_check(env)
        # A narrow side summary must keep scope, or explicitly defer the whole
        # request to the inspector when the remaining height cannot fit it.
        job['tres_per_node']='gres/gpu:nvidia_a100_80gb_pcie:2';save()
        s=Session(['--host','perch-test.invalid','--no-config'],96,28,env=env)
        try:
            s.wait('gpu-study');s.wait('2 nvidia_a100_80gb_pcie GPUs per');s.wait('node requested');s.wait('1 GPU per task requested')
            s.screenshot('job-gpu-wrapped-summary')
            s.resize(96,25);s.wait('2 GPU requests · enter')
            assert 'nvidia_a100' not in s.text,'partial GPU constraint in summary'
            s.screenshot('job-gpu-summary-overflow')
            click(s,*target(s,'full details'));s.wait('↑↓ scroll');s.key('\x1b[F')
            s.wait('2 nvidia_a100_80gb_pcie GPUs per node');s.wait('1 GPU per task')
            s.key('\x1b');s.wait('› 9300');s.wait('2 GPU requests · enter');s.close()
        except BaseException:
            print(s.text);s.screenshot('job-gpu-summary-failure');s.abort();raise
        for width,height in [(140,40),(80,24),(48,18)]:
            job['tres_per_node']='gres/gpu:p100:1';save()
            s=Session(['--host','perch-test.invalid','--no-config'],width,height,env=env)
            try:
                s.wait('gpu-study');s.key('/find gpu\r');s.wait('› 9300')
                if width==140:
                    s.wait('1 p100 GPU per node requested');s.screenshot('job-gpu-glance')
                s.key('\r');s.wait('gpu-study');s.key('\x1b[F');s.wait('1 p100 GPU per node');s.wait('1 GPU per task')
                s.screenshot(f'job-gpu-details-{width}')
                job['tres_per_node']='gres/gpu:bad';save();s.pump(2.2);s.key('r');s.key('\x1b[F')
                s.wait('unsupported scheduler format.')
                assert '1 p100 GPU per node' not in s.text and '0 GPU' not in s.text
                s.screenshot(f'job-gpu-unavailable-{width}')
                job['tres_per_node']='gres/gpu:p100:1';save();s.pump(2.2);s.key('r');s.key('\x1b[F');s.wait('1 GPU per task')
                s.key('\x1b');s.wait('› 9300');s.wait('Filter: gpu');s.close()
            except BaseException:
                print(s.text);s.screenshot('job-gpu-failure');s.abort();raise
    print('Job GPU requests passed: production CLI/MCP scope and null semantics; three-size PTYs, independent request constraints, summary/details, malformed metadata recovery, retained filter/selection, animation and terminal cleanup. Synthetic SSH only.')


if __name__=='__main__':main()
