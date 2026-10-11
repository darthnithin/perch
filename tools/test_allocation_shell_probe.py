#!/usr/bin/env python3
"""Offline checks of the opt-in live probe; never submits a job."""
import subprocess
import sys
import unittest

from test_allocation_shell_live import ROOT, parse_gpu_observation, probe_resource_args


class GPUProbeTests(unittest.TestCase):
    def test_resource_scope(self):
        cpu = probe_resource_args(2, False)
        self.assertNotIn('--gpus', cpu)
        self.assertEqual(cpu[cpu.index('--mem')+1], '128M')
        gpu = probe_resource_args(1, True)
        for flag, value in [('--nodes','1'),('--cpus','1'),('--mem','512M'),
                            ('--gpus','1'),('--partition','96x24gpu4'),('--time','3m')]:
            self.assertEqual(gpu[gpu.index(flag)+1], value)
        with self.assertRaises(ValueError): probe_resource_args(2, True)

    def test_visibility_is_observation_not_gpu_authorization(self):
        for code, output in [(0,b'Tesla P100-PCIE-16GB'),(9,b'No devices were found'),
                             (124,b''),(127,b'nvidia-smi: command not found')]:
            wire = (b'echoed command PERCH_GPU_ENV ignored\r\n'
                    b'PERCH_GPU_ENV cuda=unset step=unset\r\n'+output+
                    b'\r\nPERCH_GPU_END '+str(code).encode()+b'\r\n')
            observation = parse_gpu_observation(wire)
            self.assertFalse(observation['step_requests_gpus'])
            self.assertEqual(observation['nvidia_smi_exit'], code)
            self.assertEqual(observation['nvidia_smi_output'], output.decode())
            self.assertIn('not proof', observation['scope'])

    def test_incomplete_or_unbounded_output_is_not_success(self):
        for wire in [b'PERCH_GPU_END 0',b'\r\nPERCH_GPU_ENV cuda=0 step=0\r\n',
                     b'\r\nPERCH_GPU_ENV cuda=0 step=0\r\n'+b'x'*4097+b'\r\nPERCH_GPU_END 0\r\n']:
            with self.assertRaises(AssertionError): parse_gpu_observation(wire)

    def test_invalid_invocations_stop_before_ssh(self):
        base=[sys.executable,str(ROOT/'tools/test_allocation_shell_live.py'),
              '--host','perch-test.invalid','--report','/unused/probe.json']
        for args, explanation in [(['--gpu'],'explicit --submit'),
                                   (['--gpu','--submit'],'--gpu requires'),
                                   (['--gpu','--submit','--perch-tui','--nodes','2'],'--gpu requires')]:
            result=subprocess.run(base+args,env={'PATH':''},capture_output=True,text=True,timeout=5)
            self.assertEqual(result.returncode,2,result.stderr)
            self.assertIn(explanation,result.stderr)


if __name__ == '__main__': unittest.main()
