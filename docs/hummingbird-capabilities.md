# Hummingbird capability observations

Checked directly over an existing SSH connection on October 9, 2026 (America/Los_Angeles). These are session observations, not guarantees of future configuration or complete feature acceptance. Checks were read-only and used existing jobs; no diagnostic job was submitted.

| Capability | Observed result | Implication |
| --- | --- | --- |
| Slurm version | 25.11.4 | Use a version-aware adapter |
| JSON parsers | v0.0.41 through v0.0.44 listed; `--json=v0.0.44` returned valid JSON without reported errors | Select an explicitly supported schema |
| Accounting storage | accounting_storage/slurmdbd | Historical job queries available |
| Job accounting gatherer | jobacct_gather/cgroup | Test actual metrics; plugin presence alone is insufficient |
| Scheduler | sched/backfill | Pending jobs returned start estimates |
| Priority | priority/multifactor | sprio returned component scores |
| Fairshare | sshare returned values; PriorityWeightFairShare=0 | Fairshare is observable but currently contributes no priority weight |
| Controller retention | MinJobAge=120 sec | Persist log metadata promptly; reconcile outcomes through sacct |
| GPU resources | GresTypes=gpu; node-24 reported gpu:p100:4 | Allocation overview available; utilization not verified |
| Visible partitions | 128x24 (default), 96x24gpu4, 256x44 | Query dynamically rather than hard-code |
| Completed-job resource data | TotalCPU and `.batch` MaxRSS populated | Efficiency view feasible with correct scope/units |
| Live resource data | Numeric job ID + `.batch` returned AveCPU, MaxRSS, AveRSS | Live accounting works for at least the tested batch step |
| Historical paths | sacct returned StdOut filename pattern and WorkDir for a completed array task | Accounting can recover log metadata after controller eviction |
| seff | /usr/bin/seff available; completed-job report worked | Useful reference tool, not a required dependency |
| Stored submitted scripts | `sacct --batch-script` returned an existing completed array task's script; earlier config output included job_script in AccountingStoreFlags | Historical script viewing is feasible; script content was not saved |
| Storage mounts | Home, /groups, and /scratch resolved to beegfs at /mnt/beegfs | Do not count them as independent filesystem capacity pools |
| Quota tooling | beegfs and quota executables present | Actual user/group quota retrieval remains unverified |

## Concrete examples

A completed one-core job reported 8m14s elapsed, 8m10s CPU utilized, 99.19% CPU efficiency, and 47.11 MB memory utilized against 2.00 GB allocated (2.30%). This was a single tested job; it does not validate general multi-node memory aggregation.

A running `.batch` step returned AveCPU=00:12:48, MaxRSS=125916K, and AveRSS=125660K. These are accounting measurements, not a directly measured instantaneous CPU utilization percentage.

Historical stdout contained `%A_%a` substitutions. Paths must be interpreted with the correct array identity; the file's continued existence and accessibility are separate checks.

## Pitfalls encountered

- An array-form identifier passed to sstat did not resolve the running batch step. Using the numeric job ID from squeue JSON succeeded.
- `squeue --only-job-state --user=…` was rejected. The state-only option accepts a more restricted set of companion arguments. Ordinary `squeue --json=v0.0.44 --user=…` worked.
- JSON responses can be large because they include detailed allocations. Scope queries, bound responses, normalize locally, and avoid logging whole responses.
- `rg` was absent on the login node. Perch should not assume remote developer utilities are installed.
- Public login-node policy does not explicitly authorize a persistent third-party monitoring TUI. Technical ability to run a command is not approval to deploy a login-node service.

## Remaining verification

October 10 fair-share follow-up: an own-user `sshare` query returned an
association with nonzero share data, while a fresh controller read reported
`PriorityWeightFairShare=0`, `PriorityDecayHalfLife=7-00:00:00` and
`PriorityUsageResetPeriod=NONE`. Share data is observable but does not currently
contribute to weighted job priority. Perch's on-demand priority report now reads
the weight alongside scores; it does not hard-code this observation. See
`output/validation/fairshare-discovery.json` and the later production validation
in `output/validation/priority-policy-live-final.json`. No workload was submitted.

- End-to-end reconnect/authentication behavior inside the actual TUI.
- Perch-managed SSH socket lifecycle, deadlines, error classification, and batch response framing.
- Log access, incremental reads, truncation/rotation, and per-task filename resolution across representative jobs.
- GPU utilization metrics, if desired; only scheduler allocations were checked.
- Administrator guidance for persistent login-node mode and polling cadence.
- Behavior after Slurm upgrades, permission changes, and delayed accounting.
- Generalized resource-efficiency aggregation for multi-step and multi-node jobs.
- User/group quota query support, permission scope, and inode limits.
- Desktop/terminal notification delivery. The later explicit allocation-shell CLI and its remaining acceptance gates are documented in [ALLOCATION-SHELL.md](ALLOCATION-SHELL.md).

Scrubbed JSON fixtures are now saved under `testdata/hummingbird` and exercised by parser tests; see the readiness audit for later implementation checks. This document intentionally omits personal job identifiers, usernames, and home paths from the observations.

## October 10 software and storage exploration

- Lmod 9.2 provides `module --redirect -t spider` and `module --redirect -t avail`. The live spider response contained 280 exact entries across compiler families; avail has a narrower login-environment scope. Perch now exposes both distinctions through software search. No module was loaded.
- `module --redirect spider singularity-ce/singularity-ce.4.1.4` returned a container-runtime description and `ohpc` prerequisite. The runtime was not executed, and container workload policy/support has not been established by this metadata query.
- Live names included CUDA 12.8.1 and 13.1.1; GNU 13/14/15 families; OpenMPI5; Singularity CE; Nextflow; and Snakemake. Names/versions can change; always query. CLI, TUI and MCP software lookup were verified against the live cluster.
- BeeGFS CLI is 8.4.1. Its `quota list-usage --uids current --gids current --output json --raw` operation failed because the current user cannot read `/etc/beegfs/conn.auth`. No permissions were changed or bypassed. Personal quota remains unavailable. A standard `quota -s` produced no data; silence is not evidence of unlimited quota.
- `df -PT /home /groups /scratch` again resolved all three paths to one BeeGFS mount. This describes shared filesystem capacity, not personal allowances. No recursive usage scan was run.

Software setup and verified batch-template generation are the next useful expansion path. Quota UI must preserve the permission-denied state until an authorized cluster mechanism becomes available.


## Array launch policy check · October 10

A current `scontrol show config` query returned `MaxArraySize=10001`, `MaxJobCount=40000`, and `MinJobAge=120 sec`. The configured array-index range is therefore 0–10000; scheduler/account limits still determine what can run. Perch's doctor reports the live array capability, and launch validation passes the complete array/dependency request to `sbatch --test-only`. Array 801856 ran two tasks with concurrency one, followed by afterok job 801858. Accounting confirmed serial task execution and dependent start after both task end times; per-task output was retrieved through the same log adapter used by Perch.


The failed-dependency pair 801884 → 801886 also verified `--kill-on-invalid-dep=yes`: the prerequisite failed and the dependent was cancelled. A previous pair exposed default `sacct` eligibility filtering; cancelled jobs with no eligible/start time require explicit state selection to appear in ordinary history. Perch now selects all documented base states, preserving its seven-day bound. A raw no-start sentinel is normalized to missing time.

## Multi-task and Open MPI acceptance · October 10

Three small authorized jobs verified the shared launch service. Job 801973 ran
two independent `srun --mpi=none` tasks on one node; stdout contained task IDs 0
and 1 exactly once. Job 801976 compiled a small C MPI program on a compute node
using `gnu15/15.2.0` followed by `openmpi5/5.0.10`. Job 801981 then ran it through
`mpirun` with one rank on each of two nodes. Both ranks reported size 2, a
successful `MPI_Allreduce` sum of 1, and different processor names. Accounting
confirmed two nodes and COMPLETED. Each job requested 256 MiB per node; time
limits were one minute for execution and two minutes for compilation.

The live module lookup explicitly identified GNU 15.2.0 as the prerequisite
for this Open MPI version. These versions are observations, not permanent
defaults. `perch module` can inspect current requirements. Perch uses explicit
Open MPI mapping (`ppr:T:node:PE=C`) and core binding; its independent-task srun
mode deliberately disables MPI. This test does not establish arbitrary MPI
ABI compatibility, multi-GPU assignment, or aggregate memory monitoring.

The opt-in acceptance test is `TestHummingbirdTopologyIntegration` in
`internal/launch/topology_live_test.go`. It submits once per phase and fails
without automatically resubmitting if an outcome cannot be observed. Synthetic
source and logs remain in the private scratch directory printed by the test.


## Later shell breadth observation · October 10

A two-node synthetic shell probe remained pending for Priority and was cleaned
up after its bounded wait. No compute shell ran, so this does not extend the
single-node shell evidence. See [allocation-shell verification](ALLOCATION-SHELL.md).
A concurrent partition query reported all four P100 GPUs allocated; no GPU probe
was submitted. These are point-in-time availability observations.

`sprio --long` returned a pending probe's component scores, including age, job
size, partition and QOS. The reported fairshare contribution for that job was
zero; that observation alone is not proof that the plugin is disabled. The
separate configuration observation above reports its zero priority weight.
That earlier observation established factor retrieval; the implementation and later production query are documented below.


## Priority report acceptance · October 10

The production `perch priority` path returned an empty personal queue successfully.
An owned held probe returned no factors; an eligible probe returned allocation and
partition identity, total 10010022, and weighted job-size 23, partition 10000 and
QOS 10000000. This demonstrates why Perch preserves the total rather than summing
rounded components. Both probes were cancelled and verified absent from the queue;
this check does not claim compute execution. The TUI and MCP use the same read-only
query service. No priority polling was added, and the view does not predict rank
or execution time. See [sprio field definitions](https://slurm.schedmd.com/sprio.html).


## Capacity context and live report · October 10

A later two-node shell probe stayed pending for Priority and was cancelled after
180 seconds without running. Accounting and cleanup are recorded in the shell
acceptance document. The 128x24 partition then reported 155 idle CPUs, while
16 of 18 nodes had all 122880 MiB of configured memory allocated. The other two
nodes each had one idle CPU and nonzero unallocated memory. This is not a
confirmed explanation of the scheduler's Priority reason, but demonstrates why
CPU-only availability should not be presented as immediate schedulability.

The existing `/priority` TUI and `perch_priority` MCP tool both returned the
actual pending probe's factors through production SSH. The TUI returned to Jobs
and restored its terminal correctly; the SDK subprocess session closed cleanly.
Native Terminal appearance is still a separate acceptance gate.


## Per-node allocation report · October 10

Production `perch nodes --json` read 20 visible node records via local
`scontrol --json=v0.0.44 show nodes`. Real-memory, allocated-memory and
specialized-memory fields are MiB; normalized output uses bytes and preserves
missing values. Unallocated memory is their configured-minus-allocated-minus-
specialized difference, not the OS `free_mem` field. State flags such as
MIXED/PLANNED and partition memberships are retained. Values can change between
observations, and this does not calculate a pending job's schedulability.

The TUI's `/nodes` and MCP `perch_nodes` both passed against Hummingbird. Reads
are on demand. A scrubbed projection of this response is embedded as the demo
fixture; see `testdata/hummingbird/README.md`. No node settings, partitions or
workloads were changed. Definitions follow [scontrol node fields](https://slurm.schedmd.com/scontrol.html).
