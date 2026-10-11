#!/usr/bin/env python3
"""Exercise the built offline NRP draft CLI; optional official Swagger validation.

No cluster access or workload submission. --schema accepts a downloaded
Kubernetes api/openapi-spec/swagger.json, never an inferred cluster schema.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--schema', type=Path)
    args = p.parse_args()
    binary = ROOT/'bin/perch'
    with tempfile.TemporaryDirectory(prefix='perch-prepare-') as tmp:
        tmp = Path(tmp)
        marker = tmp/'unexpected-exec'
        for name in ('kubectl', 'ssh', 'python', 'bash'):
            stub = tmp/name
            stub.write_text(f'#!/bin/sh\necho called >> "{marker}"\nexit 99\n')
            stub.chmod(0o700)
        env = dict(os.environ, PATH=str(tmp), KUBECONFIG=str(tmp/'absent'), HOME=str(tmp))
        flags = ['nrp','prepare','--name','analysis','--namespace','research',
                 '--image','python:3.13-slim','--cpu','500m','--memory','512Mi',
                 '--ephemeral-storage','1Gi','--time','5m']
        command = ['python','-c',"print('$(HOME) $$')",'','--namespace','other','\x1b[2J']

        def run(extra, mode='--json'):
            return subprocess.run([str(binary),*flags,*extra,mode,'--',*command],
                                  env=env, capture_output=True, text=True, timeout=5)

        drafts = []
        for extra in ([], ['--pvc','research-data','--mount-path','/data','--workdir','/data/project'],
                      ['--gpus','1','--gpu-resource','nvidia.com/a40','--opportunistic']):
            result = run(extra)
            assert result.returncode == 0, result.stderr
            data = json.loads(result.stdout)
            assert data['source'] == 'local-draft' and data['command'] == command
            manifest = json.loads(run(extra,'--manifest').stdout)
            assert manifest == data['manifest']
            assert '\x1b' not in result.stdout
            assert manifest['metadata']['namespace'] == 'research'
            container = manifest['spec']['template']['spec']['containers'][0]
            assert container['resources']['limits'] == container['resources']['requests']
            assert container['args'][1] == "print('$$(HOME) $$$$')"
            assert manifest['spec']['activeDeadlineSeconds'] == 300
            drafts.append(manifest)
        review = subprocess.run([str(binary),*flags,'--',*command], env=env,
                                capture_output=True,text=True,timeout=5)
        assert review.returncode == 0 and 'Local review · not submitted' in review.stdout
        assert '\x1b' not in review.stdout
        for extra in (['--cpu','0'], ['--memory','512'], ['--time','0'], ['--pvc','data'],
                      ['--gpus','1'], ['--json'], ['--login']):
            result = run(extra, '--manifest')
            assert result.returncode != 0 and not result.stdout.lstrip().startswith('{'), result
        assert not marker.exists(), 'offline preparation invoked an external command'
        assert not (tmp/'absent').exists(), 'preparation wrote kubeconfig'

        evidence = {'binary_sha256':hashlib.sha256(binary.read_bytes()).hexdigest(),
                    'cpu_pvc_gpu_exports':'passed', 'argv_and_flag_boundaries':'passed',
                    'external_commands':'none', 'submission':'not attempted'}
        if args.schema:
            import jsonschema
            schema_bytes = args.schema.read_bytes()
            swagger = json.loads(schema_bytes)
            schema = {'$ref':'#/definitions/io.k8s.api.batch.v1.Job',
                      'definitions':swagger['definitions']}
            validator = jsonschema.Draft4Validator(schema)
            for manifest in drafts:
                validator.validate(manifest)
            # Make sure the independent schema catches incompatible field types.
            broken = json.loads(json.dumps(drafts[0]))
            broken['spec']['activeDeadlineSeconds'] = '5m'
            assert list(validator.iter_errors(broken)), 'schema failed its negative control'
            evidence['official_schema'] = {'version':swagger['info']['version'],
                                          'sha256':hashlib.sha256(schema_bytes).hexdigest(),
                                          'three_manifests_and_negative_control':'passed'}
        print(json.dumps(evidence,indent=2))


if __name__ == '__main__': main()
