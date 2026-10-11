# Shells in running allocations

`perch shell` opens an additional interactive step in an allocation you already
own. Use it to inspect the compute-node environment or run a short diagnostic.
It shares resources with the workload; it does not attach to the original process
or recreate variables and modules set inside its batch script.

In the workspace, select a running job and press `o` or use `/shell`. The job
inspector also has a clickable **o shell** shortcut. Review the node/resources,
then click **Open shell** or press Enter. Esc leaves the preview without opening
anything. `n` cycles allocated nodes when the job has more than one. The TUI uses
the same one-CPU, 64-MiB, ten-minute defaults; custom resources are available in
the CLI below. A live check runs again before terminal handoff. The existing
workspace SSH connection is reused, so its interactive sign-in applies here.

After `exit`, the prior inspector/output/list view, selection and filter are
retained while monitoring resumes. Ordinary Ctrl-C belongs to the foreground
shell. Terminating Perch ends its foreground child and restores terminal input
modes before quitting. Demo mode shows the preview and returns without opening
a real shell. Cached, disconnected and non-running selections cannot authorize
entry.

```sh
# Read-only preview using the live queue
perch shell 12345 --host your-cruzid@hb.ucsc.edu

# Enter the shell explicitly
perch shell 12345 --host your-cruzid@hb.ucsc.edu --open

# Pick one node from the allocation and adjust the shared resources
perch shell 12345 --node node-14 --cpus 1 --mem 256M --time 5m --open

# Preview a specific array task for an agent or script
perch shell 12300_7 --json
```

The preview lists the node, working directory, CPU/memory request and step time
limit. Defaults are one task on one node, one CPU, 64 MiB and ten minutes. No GPUs
are requested. The parent allocation's lifetime is still the upper bound.
`--node` must name one of its currently allocated nodes; omission picks the first.
The selected node must have enough resources for the requested step; Slurm makes
the final decision. Step resource contention can affect the workload.

Use `exit` or Ctrl-D to return. Ctrl-C interrupts the foreground command inside
the shell. Perch never cancels the parent allocation as part of shell exit.
Terminating Perch stops its SSH child. A lost network connection can take time to
be detected; this is not a persistent terminal session and Perch does not promise
to preserve it for reconnection.

SSH credentials must work without a prompt, as for other CLI queries. A TUI
sign-in uses that TUI process's private master and does not authenticate a separate
CLI process. Configure system SSH keys/agent access for repeated CLI use. `--local`
runs against local Slurm where cluster policy permits; it does not install Perch
or start a persistent login-node service.

## Execution boundaries

- Preview is read-only and requires a live queue response. `--open` requires a
  terminal on stdin and stdout, and cannot be combined with `--json`.
- Both preview and entry check ownership, exact RUNNING state, concrete array
  task, submission/restart/start identity, allocated nodes and working directory.
  Changes detected before entry require another preview. Slurm still arbitrates
  changes racing with the final request.
- `--overlap` shares resources; `--exact`, one task/node, explicit CPU/memory and
  `--gres=none` constrain the request. A five-second immediate-resource wait
  avoids sitting indefinitely behind another step. GNU `timeout` on the login
  host bounds the foreground `srun`, with a ten-second termination grace. Perch
  also bounds its local child if the connection does not return. Scheduler
  cleanup and a broken network can outlast the requested duration. `--timeout`
  bounds each read-only preflight, independently of the shell's `--time` limit.
- The new shell uses the login-side environment. Inherited `SLURM_*`/`SRUN_*`
  options are removed before `srun` so they cannot silently redirect output,
  request extra resources or change the step mode. Slurm supplies fresh step
  variables. Bash startup files are not sourced again; load the modules you need
  inside the shell. No submitted script is evaluated.
- The interactive session is an ordinary terminal, not a sanitized log viewer.
  Its input/output are not written into Perch diagnostics or cached snapshots.
- MCP tools remain read-only. They can explain this human-operated command and
  workspace action, but do not open shells.

## Verification scope

The initial live experiment on Hummingbird used an owned one-CPU, 128-MiB,
three-minute synthetic allocation. A one-CPU, 64-MiB shell step reached a compute
node with TTY stdin/stdout, one-CPU affinity and the login-side environment.
Ctrl-C returned to the shell, normal exit completed the step, and terminating SSH
ended that step while leaving the parent allocation running. The harness checked
identity before cancelling its own allocation and removed its own temporary
directory. See the latest `allocation-shell-*-20261010*.json` reports under
`output/validation` for production CLI results and cleanup evidence.

The production CLI passed the same entry, Ctrl-C, exit and process-termination
checks, plus a one-minute expiry with the parent still running. Accounting
recorded the expired step as CANCELLED after one minute; Perch's timeout wrapper
ended the session rather than waiting for Slurm's full termination grace. An
earlier experiment using only `srun --time=1` exceeded the 90-second test window;
Hummingbird reports `KillWait=60`, and that step was eventually recorded TIMEOUT.
Both the failed experiment and successful retest retained cleanup evidence.

Offline tests cover failed/foreign/stale allocations, requeues between preview
and entry, exact array-task targeting, invalid node selection, command quoting,
inherited scheduler environment, explicit PTY entry and child termination.
GPU inspection and native Terminal.app acceptance are not established by these
CPU-only probes. The two-node selection result is recorded below. The TUI's synthetic SSH/PTY tests also
exercise node choice, pointer reset, returning to the same filtered inspector,
requeue refusal, cancellation during preflight and abrupt child termination.

The opt-in `--perch-tui` harness also passed against a small owned Hummingbird
allocation: mouse entry reached a real shell, Ctrl-C worked, and exit restored
the same inspector/filter and terminal input modes. The parent job remained
running until the harness's separate cleanup. The report is
`output/validation/shell-ui-hummingbird-live-20261010.json`. This verifies the
production TUI/SSH round trip through a PTY; native window appearance and GPU
behavior remain separate acceptance gates.

Sources: [Hummingbird getting started](https://hummingbird.ucsc.edu/getting-started/),
[January 2026 behavior changes](https://hummingbird.ucsc.edu/2026/01/),
and [Slurm step semantics](https://slurm.schedmd.com/srun.html). Public Hummingbird
guidance permits interactive work in allocated resources and notes changes to
`salloc`/`srun`; the live probe establishes the tested behavior, not blanket
administrator approval for every use or partition.


### Two-node acceptance attempt · October 10

The opt-in harness accepts `--nodes 2 --perch-tui`. Once its owned allocation
starts, it checks both CLI previews, rejects an outside node, opens a CLI shell
on the second node, and visits each node through the production TUI's node
selector. Shell output must report the chosen hostname; the parent allocation
must remain running after each shell exits. Queue waits are bounded by
`--wait-seconds` (70 by default, 30–900 allowed) and never cause resubmission.

Job 802978 never started: its queue reason was Priority throughout the original
70-second window. Accounting recorded CANCELLED, zero runtime and no assigned
nodes. The report verifies queue absence and removal of the private probe
directory: `output/validation/shell-two-node-hummingbird-20261010.json`.
Its original failure result remains intact. The new harness classification
records future queue expirations as `not-exercised`; live two-node selection
remains unverified. No GPU allocation was requested.


### Second bounded two-node attempt · October 10

Job 803043 remained PENDING/Priority for the new 180-second bound. No shell
opened. Accounting recorded CANCELLED, zero runtime, zero allocated CPUs and no
assigned nodes; the harness verified queue absence and removal of its private
remote directory. The report is
`output/validation/shell-two-node-hummingbird-20261010-b.json`, classified
`not-exercised`, not a shell implementation failure. No automatic resubmission
was performed.

The point-in-time scheduler prediction was 14:15 local cluster time. It was not
waited out or treated as a guarantee. Although the partition reported 155 idle
CPUs, a per-node query showed all configured memory allocated on 16 of its 18
nodes. The two other nodes each had one idle CPU and some unallocated memory.
These are contextual observations, not a reclassification of Slurm's Priority
reason. CPU counts alone do not establish immediately schedulable capacity.

### Two-node acceptance passed · October 10

A fresh capacity observation showed additional idle CPUs and unallocated memory
on node-19 and node-22. New bounded probe **803078** obtained both nodes. The
production CLI previewed each, refused an outside node, and opened a shell on
node-22. The production TUI then selected and entered node-19 and node-22 through
its mouse controls. Each shell reported the requested hostname, job/step identity
and terminal stdin/stdout; Ctrl-C returned to Bash and normal exit returned to
the same inspector and filter. Pointer reset, terminal restoration and parent
allocation survival were checked.

All three shell steps completed with exit 0. The harness then separately
cancelled its parent probe, verified queue absence and removed its private
directory. The parent/batch cancellation is intentional cleanup, not shell
failure. Evidence: `output/validation/shell-two-node-hummingbird-20261010-c.json`,
including the development binary SHA-256. Reviewed the second-node preview and
returned-inspector PTY renders under `output/terminal/shell-ui-hummingbird-*node-2.png`.
This closes the two-node CPU shell-selection check; it does not certify GPU
inspection, arbitrary allocation layouts or native terminal appearance.

### GPU-allocation probe

The opt-in harness now accepts `--gpu --perch-tui --nodes 1`. It requests one
GPU on Hummingbird's `96x24gpu4` partition, one CPU and 512 MiB for at most three
minutes. It requires `--submit` and a fresh report path, retains the one-shot
submission receipt, bounds its queue wait and cleans up only its identified job.

Before shell entry it verifies that the owned allocation actually has one GPU.
The production shell still explicitly requests **no GPUs**. Inside that shell,
the probe records `CUDA_VISIBLE_DEVICES`, `SLURM_STEP_GPUS`, and the exit/output
of a five-second GPU-name query. It does not inspect other users' processes or
run CUDA work. It then checks Ctrl-C, return to the inspector and parent-job
survival. A successful round trip does not certify GPU access: device visibility,
allocation ownership, isolation and CUDA usability are different observations.

Slurm documents [per-step GPU environment](https://slurm.schedmd.com/gres.html)
and [`--gres=none`](https://slurm.schedmd.com/srun.html). Perch does not infer GPU
access from the parent allocation or silently change the shell's resource policy.

The harness's resource scope, bounded result parsing, missing-tool/timeout cases
and rejection of invalid invocations passed offline checks in
`tools/test_allocation_shell_probe.py`. The production CLI's offline preview
also generated the intended one-GPU request. At the harness preparation check all
four P100 GPUs were allocated, so this probe was **not submitted or exercised**.
GPU acceptance remains open. Evidence is in
`output/validation/gpu-shell-probe-summary.json`.

### First bounded GPU attempt · October 10

Owned probe **803377** was subsequently submitted once with a 180-second queue
bound. It remained PENDING/Resources throughout that bound. No shell opened.
The harness separately cancelled the probe, confirmed queue absence and removed
its private directory. Accounting reported CANCELLED, zero elapsed runtime,
zero allocated CPUs and no assigned nodes. This is **not exercised**, not a
successful GPU-shell check or an implementation failure. Evidence:
`output/validation/gpu-shell-hummingbird-20261010-a.json`.

The actual pending job was inspected through the production TUI at 140×40 and
48×18. Reviewed emulator renders retained the original Resources reason and
showed the October 13 start prediction as tentative; it was not treated as a
guarantee. The compact inspector displayed eight content rows and restored the
terminal on exit. Evidence: `output/validation/gpu-shell-pending-live-view.json`.
The inspection exposed missing job-level GPU-request presentation, now tracked
in [feature design](feature-design.md). Do not repeat this queue attempt without
a changed capacity observation or another concrete scheduling reason.
