# Hummingbird fixtures

Captured by read-only `squeue`, `sacct`, and `sinfo` queries on October 9, 2026, using Slurm 25.11.4 and `--json=v0.0.44`.

These fixtures retain actual field shapes, set/unset wrappers, resource units and timestamps. They are deliberately projected onto fields consumed by Perch. Usernames, job IDs, array IDs, job names and storage paths are replaced; environment, commands, scripts, comments and unrelated identifying metadata are excluded. Node and partition names describe public cluster hardware.

- `queue.json`: three running tasks from the same real array, with scrubbed identity.
- `accounting.json`: one completed array task, its batch/extern steps, and real CPU/MaxRSS measurements.
- `partitions.json`: multiple node-state records per partition, useful for detecting double-counted totals.

Synthetic edge cases are constructed in parser tests from these shapes, including a million-task collapsed range, unset memory, unlimited time and unsupported schemas. They are not represented as live observations.

October 10 Lmod fixtures: `module-spider.txt` and `module-avail.txt` retain the live 9.2 machine-readable catalog and login-environment listing. They contain installed package names/versions and public system modulepath headings, not user environments, secrets or scripts. The spider capture contains 280 exact entries (family markers are excluded by the parser). Hierarchical MPI packages in spider but absent from avail exercise prerequisite handling. Availability is a point-in-time account/environment observation.


`cancelled-before-start.json` is a minimal scrubbed October 10 Slurm 25.11.4/v0.0.44 record from a failed-dependency test. It retains the actual zero eligibility time, NO_VAL32 start sentinel (4294967294), zero elapsed time and CANCELLED outcome. Job ID/name/user are synthetic; paths, command, comments and unrelated metadata are omitted. The live record was missing from an unfiltered `sacct` query, but appeared when CANCELLED was explicitly selected. Perch now requests all documented base states within its bounded accounting window.

October 10 mixed-requeue evidence:

- `accounting-bitmap.json` projects the cancelled pending portion of synthetic array 802120. Slurm 25.11.4/v0.0.44 returned `array.task: "0x2"`, meaning task 1. Only array identity, state, restart count and time shapes remain, with synthetic IDs/name/user; no paths, commands, comments, scripts or account metadata.
- `mixed-requeue.json` projects final records from synthetic array 802126. Task 0 was requeued once, changing submission time from 11:19:04Z to 11:19:39Z. Task 1 kept the original submission time. `--duplicates` retained the earlier task-0 PENDING record alongside its later COMPLETED attempt. Both members completed. IDs/names/user are scrubbed; observed time/state/restart fields are preserved.
- Intermediate queue phases used in tests are reconstructed from these identities and the live test's metadata log; they are not claimed as captured raw queue JSON.

Bitmap decoding follows [Slurm 25.11 `bit_unfmt_hexmask`](https://github.com/SchedMD/slurm/blob/slurm-25.11/src/common/bitstring.c): low bits of the rightmost hexadecimal digit identify the lowest task indices. The adapter converts contiguous bits into ranges, bounds bitmap input and resulting range text to 1 MiB, and reports unsupported oversized/fragmented responses rather than allocating a task record per bit.


`accounting-multistep.json` projects the October 10 read-only accounting query
for Perch's completed two-node MPI smoke job. Synthetic job/step IDs and user
replace identity; paths, commands, scripts and unused fields are omitted. It
retains three observed batch/extern/MPI step records, their task counts, missing
extern RSS, recorded zero extern CPU, and the actual job CPU shape of
`seconds: 0, microseconds: 1296842`. Slurm accumulates step seconds and
microseconds separately, so a microsecond count above one million is valid.
The independent text query reported TotalCPU `00:01.296`. This is regression
evidence for conversion and separate step scope, not a memory aggregation test.

`live-multistep.txt` is the actual October 10 `sstat --allsteps` response from
bounded single-task compute job 802458, with its job ID replaced by 9400.
It contains only step IDs, task peak RSS and average task CPU time. The compute
step reports 36,428 KiB RSS and 14 CPU seconds; the batch step reports 4,776 KiB
and zero CPU seconds. The extern step's `213503982334-14:25:51` is the formatted
INFINITE64/1000 sentinel, which must parse as unavailable, not CPU usage.
The matching Slurm 25.11.4 [sstat formatter](https://github.com/SchedMD/slurm/blob/slurm-25-11-4-1/src/sstat/print.c)
divides the CPU TRES value before formatting the duration.


`nodes.json` projects a read-only October 10, 2026 `scontrol` node response with
Slurm 25.11.4/v0.0.44. Only consumed name/partition/state, CPU counters, memory
allocation/system-reservation values and GRES fields remain. Client identity,
addresses, OS versions, comments, reasons, timestamps, paths and unrelated fields
are omitted. Public node/partition names and actual quantities are retained.
Demo mode labels this frozen capture as demo data; it is not current capacity.
Synthetic tests cover missing quantities, over-allocation, reserved memory,
overflow, malformed schemas, duplicate nodes and the distinction from OS free
memory. Field units follow [scontrol node definitions](https://slurm.schedmd.com/scontrol.html).
