#!/usr/bin/env python3
"""Independent stdio/JSON-RPC acceptance of the built Perch demo server."""
import argparse
import json
from pathlib import Path
import queue
import subprocess
import threading

root = Path(__file__).resolve().parents[1]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--binary', type=Path, default=root / 'bin/perch')
args = parser.parse_args()
process = subprocess.Popen([str(args.binary.resolve()), 'mcp', '--demo', '--no-config'],
                           stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                           stderr=subprocess.PIPE, text=True)
messages = queue.Queue()
def read_messages():
    for line in process.stdout:
        try:
            messages.put(json.loads(line))
        except json.JSONDecodeError:
            messages.put(AssertionError('non-protocol stdout: ' + line[:120]))
    messages.put(EOFError('server exited before reply'))
threading.Thread(target=read_messages, daemon=True).start()
sequence = 0
def send(method, params, request=True):
    global sequence
    body = {'jsonrpc': '2.0', 'method': method, 'params': params}
    if request:
        sequence += 1
        body['id'] = sequence
    process.stdin.write(json.dumps(body) + '\n')
    process.stdin.flush()
    if not request:
        return
    while True:
        message = messages.get(timeout=10)
        if isinstance(message, Exception):
            raise message
        if message.get('id') == sequence:
            return message
        assert 'id' not in message, ('unexpected response', message)

def result(method, params):
    response = send(method, params)
    assert 'error' not in response, response
    return response['result']

try:
    init = result('initialize', {'protocolVersion': '2025-11-25',
        'capabilities': {}, 'clientInfo': {'name': 'perch-wire-check', 'version': '1'}})
    assert init['serverInfo']['name'] == 'perch'
    send('notifications/initialized', {}, request=False)
    tools = result('tools/list', {})['tools']
    assert len(tools) == 15
    assert all(tool['annotations']['readOnlyHint'] for tool in tools)
    jobs = result('tools/call', {'name': 'perch_jobs', 'arguments': {'limit': 2}})
    assert not jobs.get('isError')
    structured = jobs['structuredContent']
    assert structured['source'] == 'demo'
    assert structured['page']['returned'] == 2
    assert structured['page']['next_offset'] == 2
    assert len(structured['data']) == 2
    assert json.loads(jobs['content'][0]['text']) == structured
    pending = result('tools/call', {'name': 'perch_job', 'arguments': {'job_id': '9231'}})
    assert not pending.get('isError')
    snapshot = pending['structuredContent']
    assert snapshot['data']['state'] == 'PENDING'
    assert snapshot['data']['started_at'] is None
    assert snapshot['data']['estimated_start_at'] is not None
    assert json.loads(pending['content'][0]['text']) == snapshot
    nodes = result('tools/call', {'name':'perch_nodes','arguments':{'limit':1}})['structuredContent']
    assert nodes['page']['next_offset']==1 and nodes['data'][0]['unallocated_memory_bytes']==0
    assert 'PLANNED' in nodes['data'][0]['states']
    gpu_nodes = result('tools/call', {'name':'perch_nodes','arguments':{'limit':100}})['structuredContent']
    gpu = next(n for n in gpu_nodes['data'] if n['name']=='node-24')
    assert gpu['gpus']==[{'type':'p100','configured':4,'allocated':4,'unallocated':0}]
    assert gpu['gres_used']=='gpu:p100:4(IDX:0-3)'
    priority = result('tools/call', {'name':'perch_priority','arguments':{'limit':1}})['structuredContent']
    assert len(priority['data']) == 1 and priority['page']['next_offset'] == 1
    assert priority['data'][0]['score'] == 10010148
    assert priority['priority_policy']['fairshare_weight'] == 0
    assert 'does not contribute' in priority['priority_policy']['note']
    waits = result('tools/call', {'name':'perch_waits','arguments':{}})['structuredContent']
    assert waits['history_days'] == 7 and waits['data']['partitions']
    assert waits['data']['partitions'][0]['submit_to_start']['samples'] > 0
    bad = send('tools/call', {'name': 'perch_job', 'arguments': {'job_id': ';whoami'}})
    assert 'error' in bad or bad['result'].get('isError')
    for resource in result('resources/list', {})['resources']:
        data = result('resources/read', {'uri': resource['uri']})
        assert len(data['contents'][0]['text']) > 300
    guide = result('tools/call', {'name': 'perch_guide', 'arguments': {'topic': 'perch'}})
    assert not guide.get('isError')
    modules = result('tools/call', {'name':'perch_software','arguments':{'search':'CUDA','limit':1}})
    assert modules['structuredContent']['data'][0]['name'] == 'cuda/12.8.1'
    module = result('tools/call', {'name':'perch_module','arguments':{'name':'cuda/12.8.1'}})
    assert module['structuredContent']['data']['load_command'] == 'module load cuda/12.8.1'
    prompt = result('prompts/get', {'name': 'investigate_job', 'arguments': {'job_id': '9100_7'}})
    assert '9100_7' in prompt['messages'][0]['content']['text']
    process.stdin.close()
    assert process.wait(timeout=10) == 0
    assert not process.stderr.read(), 'unexpected stderr'
    print('MCP wire checks passed: handshake, discovery, schemas, structured/text results, pagination, errors, resources, prompt, EOF cleanup.')
finally:
    if process.poll() is None:
        process.kill()
        process.wait()
    process.stdout.close()
    process.stderr.close()
