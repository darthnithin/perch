#!/bin/bash
# Explicitly submitted development workload; The monitoring harness never submits this script automatically.
#SBATCH --job-name=perch-smoke
#SBATCH --partition=128x24
#SBATCH --cpus-per-task=1
#SBATCH --mem=128M
#SBATCH --time=00:08:00

set -eu
python3 -u - <<'PY'
import sys
import os
import time

print('PERCH_SMOKE_BEGIN', flush=True)
print('PERCH_SMOKE_STDERR', file=sys.stderr, flush=True)
blocks = []
started = time.monotonic()
for tick in range(min(48, max(1, int(os.environ.get('PERCH_SMOKE_TICKS', '48'))))):
    if tick % 8 == 0:
        blocks.append(bytearray(4 * 1024 * 1024))
    burst = time.monotonic() + 0.1
    while time.monotonic() < burst:
        sum(i * i for i in range(1000))
    print(f'PERCH_SMOKE_TICK {tick:02d} allocated={len(blocks)*4}MiB', flush=True)
    time.sleep(max(0, started + (tick + 1) * 5 - time.monotonic()))
print('PERCH_SMOKE_COMPLETE', flush=True)
PY
