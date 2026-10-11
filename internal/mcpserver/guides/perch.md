# Using Perch

Perch is a local Go application using system SSH; it installs no remote agent.

## Interface

`perch --demo` opens synthetic workloads offline. `perch --host YOUR_CRUZID@hb.ucsc.edu` opens the live workspace. Keys 1–4 select Jobs, History, Partitions and Connection. Arrow keys or j/k navigate. Enter inspects; l/s/u open output, script and resources. `/` focuses commands, Ctrl+k opens the palette, `?` shows help, and q exits. `c` performs interactive SSH reconnect in the TUI. `--reduced-motion` freezes Chip's animation; `--no-color` omits colors/artwork. Font choice belongs to the terminal.

## CLI and agents

`perch setup` is an interactive destination chooser for a person at a terminal.
It also appears on an interactive first launch without a saved or explicit host.
No SSH connection starts before acceptance. Remembering the destination is
optional; `--no-config` makes the choice session-only. Escape cancels. A saved
destination is not proof of successful authentication. Agents should use an
explicit `--host` for automated queries rather than attempting the setup screen;
MCP startup never opens it.

`perch config show` inspects saved defaults offline. Supported keys are host,
theme, color-mode, reduced-motion and mouse; `config set KEY VALUE`, `config unset KEY`, and
`config reset` change only local preferences. Explicit invocation flags override
them, and `--no-config` bypasses them. The saved host also applies to MCP startup
and workload launches; use an explicit `--host` when a specific destination is
required. Preferences never enable notifications, local execution or submission.
MCP tools do not mutate these preferences.

The TUI `/settings` command previews palette, color mode and Chip motion with keyboard or
mouse controls. Space changes the selected value, `s` applies/saves, and Escape
cancels the preview. Mouse capture changes on apply. Demo and `--no-config` use
session-only appearance changes; the normal Save defaults button persists them.
Terminal font and line spacing are never changed by Perch.

`perch jobs`, `history`, `job ID`, `logs ID`, `script ID`, `resources ID`, `arrays`, `waits`, `priority`, `nodes`, and `partitions` support `--demo`, `--host`, `--local` and `--json`. General query JSON is versioned with explicit units, observation time and source. `doctor --json` has a separate capability-report format. Historical lookup covers seven days.

`perch mcp --host YOUR_CRUZID@hb.ucsc.edu` starts a stdio MCP server. Standard output is reserved for protocol messages. Guides and tool discovery work without connecting to Hummingbird; live queries authenticate on demand. Configure working noninteractive SSH access before live MCP use. Password entry never occurs through MCP. Failed authentication is a tool error, not an empty job list.

MCP lists return up to 50 records by default (maximum 200), with a next offset. Logs return a bounded tail; scripts return a bounded prefix. `text_truncated` identifies additional MCP text trimming. The log backend itself reads at most 256 KiB. No arbitrary shell, job submission, cancellation or remote software installation is exposed by this server.

Use live tools on demand. Avoid tight polling; the TUI normally refreshes jobs every 30 seconds. For connection/schema errors, use `perch_doctor` and report the actual error. Never weaken SSH host-key checks or claim that a missing measurement equals zero.

## Explicit CLI launch

`perch run --workdir /absolute/cluster/path --cpus 1 --mem 1G --time 10m -- executable args...` previews a job offline, defaulting to one node and one task. `--check` validates without submitting; `--submit` validates then makes one submission. Inputs must already exist on the cluster. Modules can be specified repeatedly in prerequisite order. The command accepts optional `--host`, `--partition`, `--gpus`, and `--json`. This CLI action is not exposed through MCP tools. Obtain user authorization for the concrete workload before using a mutating command. A failed or timed-out submission may already have reached Slurm; inspect the queue/accounting using its reported scheduler comment before retrying.

The TUI also exposes `/run`: Tab moves through the form, Enter reviews, `v` validates, and Ctrl+s explicitly submits. Its command field is interpreted by Bash, as labelled in the form. Changes require revalidation; demo mode simulates this flow without submission. Enter on the receipt focuses the new job after a refresh. These mutating UI/CLI paths are separate from MCP's read-only tool set.

`perch receipts [--host EXACT_DESTINATION] [--json]` and TUI `/receipts` read
private local submission records offline, including after restart. A token is
saved before the submission call; a returned ID marks it accepted. Accepted is
not a running/completed state. Unconfirmed may indicate no call or a lost reply:
look for its scheduler comment before retrying. The journal does not reconcile
automatically. Recorded destinations are SSH arguments, not verified permanent
cluster/user identities. Records exclude commands, paths, scripts and logs.
MCP tools do not expose this local journal or mutate it.


## Arrays and dependent work

The CLI and `/run` form accept array indices (`--array '0-99%4'`), with an entire allocation per array element and an aggregate concurrent ceiling in the review. Each element receives `SLURM_ARRAY_TASK_ID`; shell expansion requires an explicit `bash -c` command in the CLI. Output is separated using `%A_%a`. Use `--afterok ID[,ID]` to wait for successful jobs or array tasks; a parent array ID requires every task to succeed. Perch adds `--kill-on-invalid-dep=yes`, so an impossible dependency cancels the dependent job. Submit dependencies while prerequisites remain known to the scheduler. Both features retain offline preview, non-submitting check and explicit one-shot submission. Inspect capabilities/current validation rather than assuming a fixed array-size limit. No mutating MCP tool is exposed.

## Multiple tasks and MPI

`--nodes N --tasks-per-node T` requests N × T processes. `--cpus` is per process;
`--mem` and `--gpus` are per node. Array elements each receive this full allocation.
`--launcher auto` resolves to direct execution for one task and `srun` for multiple
independent tasks. Perch's `srun` launcher uses `--mpi=none`; it is not an MPI launcher.
Choose `--launcher mpirun` for Open MPI. It maps T ranks per node with C cores per
rank and requires compatible modules and an already-built executable. On October
10, a two-node collective passed using `gnu15/15.2.0` then `openmpi5/5.0.10`; inspect
current module prerequisites before using this example. Scheduler validation does
not establish executable/MPI compatibility. GPU rank assignment is not automated.
The TUI exposes the same topology fields and shows totals before submission.

Queue-wait planning: `/waits`, `perch waits [--json]`, and `perch_waits` summarize
available accounting starts in the last seven days by partition. Median/P90
submit-to-start includes holds/dependencies; eligible-to-start uses eligibility
timestamps and its own sample count. Null eligibility means unavailable. P90 uses
nearest rank. Array tasks are separate samples and can dominate. Restarted,
duplicate, aggregate, unstarted, invalid and out-of-window records are excluded.
Do not describe this as a pending-job prediction or a fair comparison of different
resource requests. TUI data follows the normal accounting refresh.

For troubleshooting, the user can start Perch/MCP with `--debug` for private,
bounded operation metadata; announcements go to stderr, never protocol stdout.
`--debug-raw` additionally captures selected raw scheduler JSON and requires
`--debug`. Metadata and raw files live in separate cache directories with 4 MiB
retention each. Raw metadata can include private names, paths and job commands;
do not recommend uploading it unreviewed. Script/log reads, submission stdin and
interactive login are not captured. Capture failure does not change execution
results. No MCP tool enables capture or reads local capture files.

Job JSON includes `restart_count`. Point queries prefer current scheduler rows;
otherwise they choose the newest comparable accounting attempt by submit time,
then restart count. History lists retain original accounting records. Distinct
attempts can share a displayed job ID: do not join their outcomes or resources
using only that ID. Retained TUI observations stay UNKNOWN when accounting only
confirms an older attempt. A requeue resets in-flight detail and chart data.

Allocation shells are a separate human-operated CLI action. `perch shell JOB_ID
--host HOST --json` previews a live, owned RUNNING allocation or one array task;
it does not start a step. In a real terminal, the user can add `--open` without
`--json` to enter a new shared step. Defaults are one node/task/CPU, 64 MiB, no GPUs
requested and ten minutes. The shell uses the login environment, not variables
or modules established by the submitted script. `exit` leaves the parent job
alone. Ownership/attempt/node/directory are rechecked before entry. This is not
an MCP execution tool and not a persistent reconnectable session; never silently
open it as a diagnostic. GPU and multi-node live shell acceptance remain open.
In the workspace, select a running job and use `o` or `/shell` for the same explicit
preview. Enter or the Open shell button starts it; Esc cancels the preview. The
TUI reuses its existing SSH connection and returns to the previous view/filter
after exit. Demo mode never starts a real shell.

The separate `perch cancel JOB_ID` CLI previews a cancellation scope. One array
task must use `ARRAY_TASK`; every remaining task requires `ARRAY_ID --array`.
The preview is read-only. `--confirm` explicitly sends one cancellation after
fresh ownership, cluster, attempt and scope checks; obtain user authorization
before running it. All steps and allocation shells in scope stop. Successful-job
dependencies may remain blocked. Output files are not removed by Perch.
`--json` returns a preview or request receipt. A `requested` receipt is not a
terminal job state. After an `unknown` or `unconfirmed` result, inspect jobs and
history; never automatically repeat the action. Task ranges, steps,
heterogeneous and federated jobs are refused. The TUI's `/cancel` and
`/cancel-array` offer the same explicit review from a current selected job.
Ctrl+X or the labelled button confirms; Enter does not. Escape preserves the
calling view and filter. A grouped pending row supports whole-array review;
use the CLI's explicit task selector for one unmaterialized task.
`/last-cancel` retains the latest receipt during this session, and a real receipt
is printed after terminal restoration on exit (including an interrupted unknown
outcome). This is not a permanent cancellation journal. Demo never mutates.
No mutating cancellation MCP tool is exposed.

In the TUI, `a` or `/actions` opens a menu for the selected Jobs/History row.
It groups output, script, resources, shell review, failed-task copying and
cancellation review. Unavailable actions explain their requirements. The menu
is pinned to the selected job attempt; if refresh replaces that selection,
close it and choose again. Merely selecting a shell/cancellation menu entry
does not authorize execution; the separate review still requires confirmation.
Ctrl+k remains the full workspace command palette.

TUI `/activity` shows the latest 100 job changes observed by this running workspace,
using the same reconciled events as notifications. These are session observations,
not a complete scheduler audit or persisted history. Enter/click opens the exact
loaded job attempt; Escape returns to the feed and then the previous filtered list.
Missing or restarted attempts cannot redirect an old entry to a replacement job.
For agent queries use `perch_history`; no separate activity MCP tool is exposed.

TUI time-warning preferences: `/warnings` opens an editor (Enter applies this
session, Ctrl+S explicitly saves defaults, Escape cancels). CLI startup flag
`--time-warnings 15m,5m,1m` overrides saved `perch config set time-warnings VALUE`.
Default 5m; accept 1–5 distinct whole-minute durations, 1m–24h, or off. Thresholds
apply to limits at least twice as long and use recorded scheduler elapsed time,
not a local countdown. Notification opt-in is separate (`--notify desktop`).
No warning-configuration MCP mutation tool is exposed.


`/priority`, `perch priority [--json]` and `perch_priority` read your pending
allocation/partition priority factors on demand. Preserve the reported total:
displayed factors can round. Scores are not queue positions or start guarantees;
resources, holds, dependencies and reservations still matter. Zero contribution
does not establish disabled configuration. Held/new jobs may be absent. Numeric
allocation IDs can represent array records; do not count them as individual tasks.
Avoid tight polling. TUI `r` refreshes; Escape restores the calling list/filter.


`perch nodes [--json]`, MCP `perch_nodes`, and TUI `/nodes` read visible per-node
scheduler allocations on demand. On Partitions, `n` is a clickable shortcut.
Configured, allocated, system-reserved and unallocated memory are separate byte
fields; missing values are null. Unallocated is configured minus job allocation
minus system reservation, not OS free memory or a guarantee of schedulability.
Preserve state flags such as PLANNED/DRAIN/RESERVED and negative unallocated
values. GRES is allocation, not GPU utilization. The report age is separate from
background queue refresh. Escape restores the calling list/filter/selection.
