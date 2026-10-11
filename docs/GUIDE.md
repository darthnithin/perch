# Perch user guide

[Back to Perch](../README.md) · Detailed commands, navigation and behavior.

A terminal workspace for UCSC Hummingbird. Prepare workloads, follow jobs and
arrays, read output, and open a shell inside a running allocation. Review
resources and scope before submitting or cancelling work.

**Development build:** the monitoring interface and SSH adapter work, with unit, race, live-backend, and pseudo-terminal checks. The release audit is incomplete. See [readiness](../docs/READINESS.md) for outstanding work; this is not a published release.

## Start here

Extract the preview archive for your operating system and CPU. From the extracted
directory, try the workspace with sample jobs:

```sh
./perch --demo --no-config
```

This tour is offline and leaves saved preferences alone. Go and Python are not
needed to run the prebuilt executable. There is no public release yet; local
preview packages are for review.

Try these controls before connecting:

| Do this | What you will see |
| --- | --- |
| Select a row with arrows or a click, then press Enter | Full job details; Escape returns to the list |
| Press `a` | Actions for the selected job, including output, script and resources |
| Press `l`, then `f` | Output and local text search; Escape clears search, then returns |
| Press Ctrl+k | Search workspace commands such as software, priority or settings |
| Press `?` | All shortcuts; Escape closes help |

Resize the window to check the compact layout. Press Escape to leave an input or
panel, then `q` to quit. Demo submission and shell actions never run cluster work.

To connect to Hummingbird, run the executable **on your Mac or local Linux
machine** with your SSH destination:

```sh
./perch --host your-cruzid@hb.ucsc.edu
```

Perch uses your system SSH client. Nothing is installed on Hummingbird. If sign-in
is needed, press `c` in the workspace; SSH handles the password/MFA prompt and
returns you to Perch. Use `./perch setup` to choose and optionally remember a
destination for later launches.

If you are running from the source checkout instead, build with Go 1.27.2 or
newer, and use `./bin/perch` in place of `./perch`:

```sh
make build
./bin/perch --demo --no-config
```

## Connection and setup

On your first interactive launch without a saved destination or `--host`, Perch
opens a connection screen. Enter `your-cruzid@hb.ucsc.edu` or an SSH config alias,
then choose **Open workspace**. No connection is made before you accept. Check
**Remember this destination** to save it for future TUI, CLI and MCP launches;
otherwise the choice applies only to this invocation. Appearance preferences
are preserved. `--no-config` makes setup session-only.

Run `perch setup` to choose again, including when your current login is wrong.
Escape cancels without opening a connection. After entering the workspace,
press `c` if SSH sign-in is needed. Setup never asks for a password.

Keep the included documentation and [third-party notices](../THIRD_PARTY_NOTICES.md)
with the package. Run `./perch doctor --host your-cruzid@hb.ucsc.edu` for a
read-only connection and capability report.

To place a reviewed executable on your own PATH without administrator access:

```sh
mkdir -p "$HOME/.local/bin"
install -m 755 perch "$HOME/.local/bin/perch"
# Add $HOME/.local/bin to PATH in your shell configuration if needed.
```

An existing SSH config alias works with `--host`. Background queries use batch authentication. Press `c` or run `/connect` inside Perch to suspend the interface, authenticate through OpenSSH, and return. Passwords are handled by SSH and never collected by Perch.

Select a job and press **a**, click **a actions**, or use `/actions` to see its
output, script, resources, shell and cancellation options in one menu. Type to
filter; arrow keys and the mouse choose an action. Shell and cancellation
entries open a separate review. Unavailable actions explain why, and a queue
refresh cannot silently redirect the menu to a different job or attempt.
Escape returns to the same view and preserves your filter and scroll position.
**Ctrl+k** still searches all workspace commands.

SSH queries use a private control socket and bounded local output. Batched
queries retain at most 1 MiB of stderr per command plus one overflow-detection
byte in a private remote temporary directory, drain the remainder, and report
an error rather than treating truncated output as success. Normal completion
and handled signals remove the spool. Client cancellation bounds local waiting;
it does not guarantee immediate termination of every remote descendant.

`--local` runs Slurm commands directly where policy permits. Hummingbird login-node permission for a persistent third-party TUI remains unverified; the default Mac-hosted SSH mode avoids running the UI on the cluster.

When an observed array loses queue records before accounting catches up, Jobs
keeps an **awaiting accounting** summary. Its count is the gap between a previously
observed total and current records, not an inferred list of task IDs. Enter or
`/job ARRAY_ID` explains the gap; History holds reported outcomes. Output, scripts,
and resources require an individual task. Earlier totals are retained for up to
512 arrays and eight days after their last queue observation. Connection reports
when that limit is reached. Restart retention currently uses the opt-in
notification checkpoint. The optional snapshot cache below restores recorded
rows, but does not persist missing-array totals or notification observations.

## Saved preferences

Open `/settings` in the workspace to preview the palette, color mode and Chip motion. Arrow
keys select a control; Space or Enter changes it, and `s` applies the form.
You can also click the controls and Save button. Escape cancels the preview;
clicking a workspace tab also discards it. Mouse capture changes on apply, so
the form stays clickable while editing. The wide layout includes a live Chip
preview; all controls remain available at 48×18.

**Save defaults** persists appearance and applies it immediately. In demo mode
or with `--no-config`, the button applies only to the current session. Saving
appearance preserves the configured destination. Font and line spacing remain
under your terminal's control.

Color mode offers **Auto**, **True color**, and **256 colors**, with a live preview.
If colors change when running on a cluster through SSH, choose **True color**
and save. This overrides terminal color detection without changing shell or
terminal settings. `--no-color`/`NO_COLOR` still disable colors. You can also use
`--color-mode=truecolor` for one launch or `perch config set color-mode truecolor`
to save it from the command line. Auto restores the profile detected at startup.

Save defaults once instead of repeating flags:

```sh
./bin/perch config set host your-cruzid@hb.ucsc.edu
./bin/perch config set theme warm
./bin/perch config set reduced-motion true
./bin/perch config set mouse false
./bin/perch config show
```

These apply on the next invocation, including the saved destination for remote CLI queries,
MCP and workload previews/submissions. Explicit flags win, including boolean
overrides such as `--reduced-motion=false` and `--no-mouse=false`.
`--no-config` uses built-in defaults plus your explicit flags. `NO_COLOR` still
controls color independently. Launch flags never silently rewrite saved choices.

`config unset KEY` restores that key's built-in default; `config reset` clears
all saved preferences. `config path` prints the private JSON file location,
and `config show --json` prints saved overrides. Configuration commands are
offline and do not authenticate or connect. Preferences do not store passwords,
notification opt-ins, local execution mode, or submission actions.

The file is limited to 16 KiB and uses a private directory and atomic writes.
Unknown keys, malformed files and unsupported versions produce an error rather
than choosing a different destination silently. `--no-config`, `config path`,
help and version remain available for recovery.

## Last-known view on restart

Use `./bin/perch --host your-cruzid@hb.ucsc.edu --cache` to retain a private
monitoring view. This is opt-in on each invocation and applies only to the live
Hummingbird TUI; demo, CLI queries and MCP never return cached data.

On restart, Jobs, History and Partitions can show the saved view while SSH is
unavailable. The status explicitly says **cached**. Fresh reads replace each
section; a different verified user or cluster discards the old view. Cached
rows never generate notifications. Reading output/scripts/live resources needs
matching fresh job evidence, and failed-array copying waits for a fresh view.
No log text, scripts, file paths, raw responses or credentials are cached.
Job names and scheduler metadata remain private user data.

Only the most recent destination is kept, for up to 24 hours: at most 10,000 job
rows plus 10,000 accounting records, 128 partitions and 8 MiB. Oversized views
are skipped as a whole, preserving the previous file. An atomic save may use one
additional bounded pending file. The owned cache directory is 0700; files are
0600, with no-follow checks and bounded locking. Connection reports save/load
problems without stopping live monitoring. It also shows the file path.

The file is `~/Library/Caches/perch/snapshot/view.json` on macOS, or
`${XDG_CACHE_HOME:-$HOME/.cache}/perch/snapshot/view.json` on Linux. To forget it,
quit cached workspaces and remove that file. Omitting `--cache` neither reads
nor updates it. A malformed/unsafe file is not overwritten implicitly. Font,
appearance and notification defaults remain separate from this view cache.

## GPU requests

Job details show reported GPU requests with their scope, such as **1 GPU per
node** or **2 p100 GPUs per task**. CLI/MCP JSON exposes `gpu_requests` as separate
`scope`, `type` and `count` values. These can overlap; do not add them or infer
allocated GPUs or utilization. Null means unavailable, not zero. Unsupported
metadata produces `gpu_request_note`. Hummingbird accounting may omit a GPU
request that was visible in the queue, so historical requests can be unknown.

## Queued-job timing

The job inspector explains common scheduler reasons while retaining the raw
reason. When Slurm reports a start prediction, Perch labels it tentative. A past
prediction is shown as awaiting an update; the job remains queued until the
scheduler reports otherwise. Missing estimates remain unavailable. `BeginTime`
means the requested earliest start has not arrived; resource or priority limits
may still delay execution after that time.

Holds, invalid account/QoS settings, unavailable partitions or required nodes,
excessive partition time requests and unsatisfiable constraints have explicit
explanations. A user/coordinator hold is distinct from an administrator hold.
Unknown site-specific reasons stay visible in At a glance and in full details;
Perch does not infer a remedy or start time from them. These explanations describe
the reported [Slurm reason](https://slurm.schedmd.com/job_reason_codes.html), which
may not be the job's only blocker. Viewing guidance never changes a job.

CLI/MCP JSON uses separate nullable `started_at` and `estimated_start_at` fields.
A pending job's prediction is never reported as its actual start. Estimates
depend on the scheduler's observations and can change; see
[Slurm's queue reference](https://slurm.schedmd.com/squeue.html).

## Time-limit warnings

Open `/warnings` to edit thresholds. Enter applies them to this session;
Ctrl+S saves defaults as well, and Escape cancels the draft. Demo and `--no-config`
sessions only apply in memory. The command palette also lists **Time warning
preferences**. Keyboard and pointer controls work down to 48×18.

```sh
./bin/perch --host your-cruzid@hb.ucsc.edu --time-warnings 15m,5m,1m
./bin/perch config set time-warnings 15m,5m,1m
./bin/perch config set time-warnings off
./bin/perch config unset time-warnings
```

The default is `5m`. Choose one to five distinct, whole-minute durations from
`1m` through `24h`, or `off`. Flags override saved values; `--no-config` ignores
them. Each threshold applies only to job limits at least twice its length:
a five-minute warning applies to limits of ten minutes or longer.

Warnings use the scheduler's recorded elapsed time, never a local countdown.
Missing/unlimited limits and failed queue refreshes cannot create warnings. Job
highlights and activity use the same policy; desktop delivery separately requires
`--notify desktop`. Disabling warnings leaves existing activity intact.

When a refresh crosses several thresholds, only the most urgent new one is
reported, with the measured remaining time. Deduplication distinguishes job
attempts and changed limits. Applying preferences baselines thresholds already
passed in the current snapshot. On notification-checkpoint restoration under a
changed policy, the first successful queue refresh baselines warnings while still
allowing confirmed finish/failure catch-up. Checkpoints compact warning keys to
the most urgent threshold per attempt/limit to avoid multiplying array storage.

## Desktop notifications

Opt in on your Mac with `./bin/perch --host your-cruzid@hb.ucsc.edu --notify desktop`.
Notifications are off by default and unavailable in demo mode. Finish, fail,
cancel and configured time-limit events use the same reconciliation as the UI.
Array tasks are grouped per refresh; a large catch-up becomes one summary.

Perch must be running to send alerts. On restart, recently observed jobs can
catch up when accounting confirms their outcomes. It does not announce unrelated
old history or assume a disappeared job succeeded. Tracking retains at most
5,000 active observations and 10,000 deduplication keys for eight days in private
files under `~/Library/Caches/perch/notifications`. Scripts and log contents are
never stored there. A second window using the same destination/user/cluster
keeps monitoring but leaves notification delivery to the first window.

Collapsed pending array ranges remain notification evidence across empty queue
polls and accounting delays, including a restart. Perch stores compact range
membership for the same user, cluster, submission and restart attempt; a
confirmed task outcome is still required before notifying. Range evidence
expires eight days after that array attempt was last observed in the queue.
Tracking is bounded to 512 array attempts and 8,192 range segments (1,024 per
attempt), without expanding every task index. If an observation exceeds those
limits, becomes too fragmented, or lacks an identity needed for matching, Connection reports limited
array alert coverage. Existing job/history queries remain available.
Confirmed task indices leave the retained ranges, so old outcomes cannot replay
merely because a large array exceeds the saved notification-key limit.

The initial macOS backend uses Apple's script notification bridge. Banner
visibility depends on macOS notification settings and Focus; successful command
execution does not prove a banner appeared. Native delivery acceptance is still
pending. Tracking/delivery errors appear in Connection. An unreadable or corrupt
state file disables alerts rather than silently discarding the history; to reset,
close notifying Perch windows and remove the affected notification state file.
Delivery is attempted once after recording the event: a crash or OS delivery
failure can lose a banner, and Perch does not retry it automatically. Job History
remains available for checking outcomes. There is no background notification agent.

## Command line

The same views work without opening the TUI:

```sh
./bin/perch jobs --host your-cruzid@hb.ucsc.edu
./bin/perch history --host your-cruzid@hb.ucsc.edu --json
./bin/perch job 801460 --host your-cruzid@hb.ucsc.edu
./bin/perch logs 801460 --stderr --host your-cruzid@hb.ucsc.edu
./bin/perch script 801460 --host your-cruzid@hb.ucsc.edu
./bin/perch resources 801460 --host your-cruzid@hb.ucsc.edu
./bin/perch arrays --demo
./bin/perch waits --host your-cruzid@hb.ucsc.edu
./bin/perch partitions --demo --json
```

Flags work before or after the command and job ID. `perch --help` lists them.
Every query supports `--demo`, `--local`, `--host`, `--timeout 45s`, and
`--json`. CLI queries use noninteractive SSH authentication and exit with an
error if credentials are unavailable; they do not open password prompts in a
pipeline. The TUI supports interactive sign-in, or configure SSH key access.

JSON query results have `schema_version: 1`, a `source` (`demo` or
`hummingbird`), an observation timestamp, and `data`. Durations are seconds,
memory values are bytes, unavailable measurements are `null`, and empty lists
are `[]`. Commands that consult accounting include `history_days: 7`.
Failures go to stderr with a nonzero exit status; unavailable data is not
reported as an empty successful result. `doctor --json` retains its separate
capability-report format.

`logs` prints a sanitized tail of at most 256 KiB. `script` prints the submitted
script as text. Neither executes output. Arrays report observed task totals;
`failed_array_spec` is a convenience for a future resubmission, not an action.
Collapsed ranges count distinct task indices, including overlapping stepped
ranges; a concurrency throttle does not change the total. Malformed or
excessively complex ranges fail the refresh instead of producing a guessed count.
Single-job commands accept one allocation or array task, such as `9100_11`.
Historical lookup is limited to the same seven-day window as the workspace.
Explicit state selection includes jobs cancelled before becoming eligible;
missing start times stay unset. Queued-job details explain common scheduler
reasons and retain the original reason code.

## Open a shell in a running allocation

Preview the node and shared resources, then explicitly enter the shell:

```sh
./bin/perch shell 12345 --host your-cruzid@hb.ucsc.edu
./bin/perch shell 12345 --host your-cruzid@hb.ucsc.edu --open
```

This starts an additional step in your running job. Defaults are one CPU,
64 MiB, no GPUs requested and a ten-minute limit. It shares resources with the
workload and uses the login environment; it does not copy batch-script variables
or modules. `exit` returns without cancelling the parent allocation. Specific
array tasks and `--node`, `--cpus`, `--mem`, `--time` are supported. Preview accepts
`--json`; `--open` requires a real terminal and working SSH authentication.
See [allocation shells](../docs/ALLOCATION-SHELL.md) for semantics and tested scope.
In the workspace, select a running job and press `o`, use `/shell`, or click
**o shell** in its inspector. The preview shows shared resources before anything
opens. Click **Open shell** or press Enter to proceed; Esc returns without entry.
For multiple allocated nodes, `n` cycles the node choice. Exiting the foreground
shell returns to the same view and filter. Demo mode previews without execution.

## Cancel a job or array task

Preview the exact scope first. Add `--confirm` to send one cancellation request:

```sh
./bin/perch cancel 12345 --host your-cruzid@hb.ucsc.edu
./bin/perch cancel 12345 --host your-cruzid@hb.ucsc.edu --confirm

# One task; siblings continue
./bin/perch cancel 12345_4 --host your-cruzid@hb.ucsc.edu

# Explicit whole-array scope
./bin/perch cancel 12345 --array --host your-cruzid@hb.ucsc.edu
```

A bare array ID is refused without `--array`. Steps and task ranges are not
accepted. The review shows destination, owner, cluster, scope and observed
states. Confirmation rereads the live queue and refuses a changed attempt or
scope; history and cached rows cannot authorize cancellation. All steps and
shells in scope stop, and dependencies requiring success may remain blocked.
Perch does not remove output files.

`--json` returns the preview or request receipt. **Cancellation requested** means
Slurm accepted the request, not that every process has stopped. After an unknown
or unconfirmed outcome, inspect `jobs` and `history` before considering another
request. Perch never retries automatically. MCP tools remain read-only.

In the workspace, select a current job and use `/cancel`, or find **Review
cancellation** in Ctrl+K. `/cancel-array` explicitly selects every remaining
task in the selected array. The review shows the target, owner, cluster and
consequences. **Ctrl+X** or the labelled Cancel button confirms; Enter does not.
Escape returns to your prior view and filter. Long reviews must be scrolled
through before confirmation becomes available. Grouped pending rows support
whole-array review; to cancel one unmaterialized task, use the CLI's explicit
`ARRAY_TASK` selector.

`/last-cancel` reopens the latest request receipt during the session. A real
receipt is also printed when Perch exits, including an unknown outcome if you
interrupt an in-flight request with Ctrl+C. This is session retention, not a
permanent cancellation journal. Demo confirmation sends nothing and changes no
job state. See [cancellation scope and acceptance](../docs/CANCELLATION.md).

## Prepare and run a workload

`perch run` turns an executable and its arguments into a Slurm job or task array.
Allocations default to one node and one task, with optional multi-node and multi-task layouts.
The default action is an offline preview of the exact resource arguments and
script. `--check` validates the working directory and asks Slurm to check the
request without submitting. `--submit` performs that validation and then submits
once, returning the accepted job ID.

```sh
./bin/perch run --host your-cruzid@hb.ucsc.edu \
  --workdir /scratch/your-cruzid/project --cpus 2 --mem 4G --time 30m \
  -- python3 train.py --epochs 5
# Add --check or --submit before the -- separator when ready.
```

All inputs must already exist on the cluster; only the generated script is
transferred. The working directory must be an absolute, existing, writable
cluster path. Arguments after `--` are literal; use an explicit `bash -lc '...'`
command if shell expansion or a pipeline is intended. Perch options go before
`--`. Run `perch run --help` for the full flag list.

Defaults are one CPU per task, 1 GiB per node and ten minutes. Use `--partition`, `--gpus`, and
repeated `--module` flags as needed. Modules load in the given prerequisite
order inside a login Bash on the compute node; use software discovery first.
`OMP_NUM_THREADS` is set to the requested CPU count per task. GPU requests are
per node; Perch does not assign individual GPUs to ranks automatically.

Output and errors go to separate `perch-%j.out` and `.err` files in the working
directory, with private permissions and append mode for requeues. `--json`
returns the plan, validation and receipt as structured data. No receipt means
success has not been confirmed: a lost connection may occur after acceptance.
Perch never retries a submission automatically. Use the reported scheduler
comment token to identify an uncertain request before trying again. A scheduler
validation estimate is tentative, not a reservation or a promised start time.

### Recover a submission after closing Perch

`perch receipts` lists saved submission records **offline**. Add `--host` with
the exact destination used when submitting, `--local`, or `--json`. In the TUI,
`/receipts` shows records for the current destination; it works while disconnected.
Accepted IDs can be inspected with `/job ID` once loaded in Jobs or History.

Perch saves a recovery token before contacting Slurm, then records the returned
job ID. **Accepted** means Slurm returned an ID, not that a job ran or completed.
**Unconfirmed** means the request may not have reached Slurm, or its reply may
have been lost. Inspect the token in Slurm's queue/accounting comments before
submitting again. This view does not query the scheduler or reconcile outcomes
automatically. Destination labels are recorded SSH arguments; aliases may be
reconfigured, so confirm the cluster/user before interpreting old job IDs.

The private journal stores only destination, job name, timestamps, token and
accepted ID/cluster. It excludes commands, paths, scripts, logs and credentials.
On macOS it lives under `~/Library/Application Support/perch/submissions`; on
Linux under `$XDG_STATE_HOME/perch/submissions` or `~/.local/state/perch/submissions`.
Files use mode 0600 and the directory 0700. It keeps at most 1,000 records, removing
the oldest accepted records first. Unconfirmed records are never automatically
evicted; if all 1,000 are unconfirmed, further submissions stop until the journal
has been reviewed. A corrupt or unwritable journal also stops submission before
contacting Slurm. If saving an accepted reply fails, Perch still reports its ID
and keeps the earlier recovery token. Existing jobs predating this version are
not added retroactively. Demo mode reads and writes no real receipts.

In the TUI, `/run` opens the job form (`n` from Jobs or History does the same).
An observed empty queue offers clickable preparation, history and partition
shortcuts. Opening preparation retains your draft and does not submit it.
If a value is invalid, Perch focuses that field and shows the explanation beside
it; the rest of your draft is kept. Correct it and press Enter to review again.
If a refresh fails, the empty view says it reflects the last refresh instead
of claiming the current queue is clear. Tab moves between fields; Enter opens
review. Press `v` to validate, then Ctrl+s to submit explicitly. The command
field is a Bash shell command, so shell syntax is intentional here; the CLI
continues to accept literal argument arrays. Drafts survive closing the form.
After acceptance, Enter finds the job in your queue or history. Demo mode walks
through the same screens without contacting the cluster or submitting anything.
MCP tools remain read-only.

## Multiple tasks and MPI

Set `--nodes N` and `--tasks-per-node T` to request `N × T` processes. `--cpus`
applies to each process; `--mem` and `--gpus` apply to each node. For example,
two nodes with three tasks per node and two CPUs per task request twelve CPUs.
The review displays these totals before anything runs.

`--launcher auto` runs a single task directly and uses `srun` for multiple
independent tasks. The `srun` launcher explicitly disables MPI and stops the
step if a task exits unsuccessfully. Choose `--launcher mpirun` for an Open MPI
program, with compatible modules loaded in prerequisite order:

```sh
./bin/perch run --host your-cruzid@hb.ucsc.edu \
  --workdir /scratch/your-cruzid/project \
  --nodes 2 --tasks-per-node 1 --cpus 1 --mem 256M --time 1m \
  --module gnu15/15.2.0 --module openmpi5/5.0.10 \
  --launcher mpirun -- ./hello-mpi
```

This remains an offline preview. The executable must already be built for the
cluster and its MPI environment. `mpirun` maps the requested ranks per node and
binds each rank to the requested number of cores. A scheduler check validates
the allocation request, not the executable or its MPI compatibility. Use
`perch module openmpi5/5.0.10` to inspect current prerequisites.

The `/run` form provides the same Nodes, Tasks per node and Launcher fields.
A small live acceptance test verified two independent tasks on one node and
an Open MPI collective across two nodes; this is not validation of arbitrary
MPI applications or multi-GPU placement.

## Parameter sweeps and dependent jobs

Use `--array '0-99%4'` to prepare 100 tasks with at most four running at once.
Each array element gets the **entire requested allocation**, including its node
and task layout. CPU counts remain per process, memory/GPU counts per node.
The review shows the per-element allocation and maximum concurrent resource total. Without `%N`,
Perch adds no array concurrency cap; scheduler/account limits still apply.
Each task receives its index in `SLURM_ARRAY_TASK_ID` and writes separate
`perch-%A_%a.out` / `.err` logs (`%A` is the parent array ID, `%a` the task index).

```sh
./bin/perch run --host your-cruzid@hb.ucsc.edu \
  --workdir /scratch/your-cruzid/project --array '0-99%4' \
  --cpus 2 --mem 4G --time 30m \
  -- bash -c 'python3 train.py --seed "$SLURM_ARRAY_TASK_ID"'

# After explicitly submitting the array, use its returned ID here:
./bin/perch run --host your-cruzid@hb.ucsc.edu \
  --workdir /scratch/your-cruzid/project --afterok 12345 \
  -- python3 summarize.py
```

Both commands above are previews. Add `--check` or `--submit` before `--` when
ready. `--afterok` accepts comma-separated job IDs or individual array task IDs.
All prerequisites must succeed; an array parent means every task in that array.
Perch explicitly requests cancellation if a dependency can never succeed.
Dependencies must still be known to Slurm: submit while the prerequisite is
active or soon after it ends. Hummingbird reported a 120-second retention window
and array indices 0–10000 on October 10; validation checks current cluster policy.

The TUI exposes the same options as **Array tasks** and **After success** at the
end of `/run`. Enter on an accepted array receipt opens an observed task from
that array. Lists, ranges and stepped ranges are supported (`1,3,5-15:2%3`);
overlapping indices are rejected so review counts stay exact. Perch bounds
preparation to 100,000 tasks; the cluster may impose lower limits.

See Slurm's [array documentation](https://slurm.schedmd.com/job_array.html) and
[dependency options](https://slurm.schedmd.com/sbatch.html#OPT_dependency).

## Find software

Use `/software` in the workspace, or search with `/software cuda`. Select a
result with the arrows and Enter, or click it, for prerequisite instructions and
an exact load command to use in your cluster shell or batch script. **f** searches
the catalog; Escape returns to the same result, then to your previous workspace.
Refresh preserves the selected module by name when it is still present.
`/module cuda/12.8.1` still opens a known module directly. The command palette
also opens the catalog. Searches query Lmod on demand; nothing is loaded or installed.
In module details, press or click **y copy load** to copy the exact load command
to your local clipboard. Load any listed prerequisites first; they are not
included in that single command. Copying does not run the command. If your
terminal host has no clipboard helper, Perch keeps the command visible for manual
copying. Failed or still-loading details cannot supply a command.

```sh
./bin/perch software cuda --host your-cruzid@hb.ucsc.edu
./bin/perch module singularity-ce/singularity-ce.4.1.4 --host your-cruzid@hb.ucsc.edu
./bin/perch software --demo --json
```

The catalog includes modules across compiler families. `available_in_login_environment`
means that the module appears in `module avail` for the query's login shell;
it does not guarantee compatibility with a particular workload. Other modules
can require loading a compiler or another prerequisite first. Module names and
versions are queried from the cluster rather than copied from an old package list.

## Agent interface

`perch mcp --host your-cruzid@hb.ucsc.edu` starts a local stdio MCP server.
It exposes fifteen tools, four practical guides, and a job-investigation prompt.
Use `--demo` for an offline walkthrough. Guides remain available when SSH is
unavailable; live tools authenticate on demand. See [MCP setup and behavior](../docs/MCP.md)
for client configuration, tool inputs, pagination, output limits and verification.

## Hover feedback

The Hummingbird and NRP workspaces highlight controls and rows under the mouse
without changing selection. Ghostty, Kitty, Foot and iTerm2 also get a hand over
click targets and a text pointer over input fields. Known CSS/X11 pointer names
are selected from the terminal environment; unknown terminals (including macOS
Terminal) and sessions inside tmux/screen receive visual feedback without OSC 22.
This is an enhancement, not a terminal installation requirement. `--no-mouse`
disables capture and hover. Keyboard navigation, resizing and focus loss clear
the hover; quitting or suspending for foreground sign-in resets the pointer.
NRP also highlights Pod and container choices without opening or reading their
logs. Native pointer appearance remains to be checked in each supported terminal.

The bottom keyboard shortcuts are also clickable in both workspaces.
For example, in Hummingbird select a job, click **enter inspect**, then **l output** or
**esc back**. This works in compact windows with no side inspector. The up/down
arrows and resource-step brackets are separate targets; their descriptive text
is not a button. Input, filter, settings, palette and launch actions keep their
normal keyboard meaning. Launch submission still requires review, successful
validation and the explicit **ctrl+s submit** action; merely hovering never acts.
In NRP, click **l logs**, choose a Pod and container, then **enter open**.
The log footer switches current/previous output and returns through those
choices without losing the workload filter. Connection has a **c connect**
action; it follows the existing explicit sign-in flow and `--login` requirement.

## Reading resources

Resource values distinguish requested memory from allocated memory. A recorded
peak carries its task/node scope; it is not a sum of simultaneous memory across
the whole job. Live sampling reads all running steps in one query. Use `[` / `]`
(or click Previous / Next) to switch independent charts; missing steps leave
gaps. Numbered compute steps are selected first when initially available.
Interrupted CPU accounting
has a visible caveat, and requeued jobs omit ambiguous efficiency percentages.
JSON exposes `allocated_memory_bytes` and `cpu_efficiency_note` alongside the
existing measurements; unavailable values remain null. For finished jobs, the
resource view also shows each accounting step's CPU time, task count and RSS
peak. CLI and MCP resource results expose these in `steps`; absent measurements
stay null, and an empty list means no step records were observed. Step peaks
are kept separate, since adding peaks from different times would misrepresent
whole-job memory usage. Live CLI/MCP results also include `live_steps`, with
step IDs, task peak RSS and average task CPU time. The existing top-level live
measurements remain batch-only for compatibility; they are not job totals.

## Navigation

Click a tab or job to select it; press Enter to inspect the selected job, or
click the full-details link at the bottom of At a glance. This summary keeps
status and array progress visible; the full inspector wraps and scrolls longer
values, including in compact windows. Click
the command input to type, or click a command in the palette to run it. The
mouse wheel moves through jobs and scrolls open output, scripts, help, and
detail panels under the pointer. Keyboard controls remain available.

At 24 rows or fewer, the inspector gives the inactive command box's four rows
to job details. `/` restores command entry; Escape cancels it and restores the
reading space. The selected job and filter remain intact.

Use **`/sort`** to choose a job-list order, or enter a choice directly:
`/sort name`, `/sort elapsed`, `/sort cpus`, `/sort memory`, `/sort state`, or
`/sort submitted`. Name/state default to A–Z; numeric values and submission times
default to largest/newest first. Add `asc` or `desc` to choose direction.
Click Name, State or the rightmost metric heading to sort; click it again to
reverse. `/sort default` restores the normal running-first order.

Jobs and History keep independent choices for the current session. Sorting keeps
the selected job and active filter, including through refresh; missing values
stay last in either direction. CPU and memory sorts show their values directly
in the list, including compact windows. Memory is the reported requested memory
per job/task record, not measured usage or the sum of an array. Sorting changes
only Perch's display, never scheduler priority or queue position.

Output opens at the latest retained lines with **following** shown in its header.
Scroll upward or press Home to pause automatic scrolling while you read; data
continues to refresh. End, or scrolling back to the bottom when no search is
active, resumes following.
Resizing preserves that choice, and switching stdout/stderr starts a fresh view.
If a refresh fails, previously read output remains visible with a **stale**
heading and the read error below. Recovery resumes from the retained cursor;
your paused/following choice survives. An initial read failure still shows the
error instead of inventing output, and a confirmed truncation replaces old text.
Output and script borders show the visible wrapped-row range when scrolling is
needed. These positions describe retained content, not the complete remote file;
the 256 KiB output buffer can discard older text as new output arrives.
In windows at most 24 rows tall, output and script readers hide the inactive
command box to show four more lines. Press `/` to bring it back; Escape returns
to reading. Long script lines and software-report paths wrap to the available
width without being clipped.
Historical script reads request the script without Slurm's report header.
An absent or unretained script is reported as unavailable, including Slurm's
`NONE` response; command failures show a bounded, sanitized diagnostic. Failed
reads never expose partial script text as a successful result. This behavior is
shared by the TUI, CLI and MCP.

Press **f** inside output to search its retained text. Matches are literal and
case-insensitive; **[** and **]** move between them and wrap at either end.
The selected match is highlighted, with its position and match count below.
Enter keeps the search, Escape cancels an edit, and Escape after committing
clears the search while keeping the log open. **End** clears search and resumes
following. Search never changes your job filter, fetches an entire remote file,
or treats “no matches” as evidence about output outside the retained tail.
The query is limited to 100 characters; at most 4,096 matches are indexed, with
`4096+` shown when more exist. Changing streams or job attempts clears search.

At 24 rows or fewer, output borrows four rows from the inactive command box.
The input returns when you press **f**, **/** or **:**; a committed search keeps
its query in the output header. This leaves six readable log rows at 48×18.
The footer's search and previous/next controls remain clickable.

Submitted scripts use the same **f**, **[**, **]** and Escape controls, with
matches scoped to the loaded script. **End** moves to the bottom without clearing
script search. Re-reading an unchanged script preserves the selected match;
changed content resets that match selection, and a failed read clears the script
and its search. Search never changes or executes the script, and returning to
the job keeps your job filter and selection.

After reading a failed job's output, its inspector can retain a small **log
hint** with the stream and read time. It is explicitly unconfirmed and never
overrides a specific scheduler outcome. The hint follows only the selected job
attempt, is replaced by the next successful log read, and is never saved to disk.
Changing jobs or retries clears it; a read failure does not make old evidence
look fresh. Scheduler state, exit code and the output remain separately visible.

Jobs and History each remember their selected job and filter when you switch
tabs. Escape from a job's details, output, script or resources returns to the
filtered list; Escape again from the list clears the filter. Press Ctrl+k and type to search commands (for example, `logs`, `run`, or
`software`); use arrows and Enter, or click a result. Job commands show their
selected job ID and name below the input. Unavailable commands stay visible
with a dash and an explanation; choosing one keeps the menu open. Inspect and
Filter are searchable too. A search with no matches does not execute anything.
`/job 9100` can also jump to an observed member of
array 9100 when its parent has no separate row.

Connection health stays visible while you filter. On a failed refresh, Perch
keeps the last observed jobs and shows recovery keys near the command input;
Connection opens with the error and sign-in guidance. Partial refreshes are
labelled separately from a lost connection. Clipboard and completion notices
expire after eight seconds so they do not occupy the command hints indefinitely.

Use `--no-mouse` to leave text selection and scrolling to your terminal.
Perch releases mouse capture when it exits or suspends for SSH sign-in.


| Key | Action |
| --- | --- |
| ↑ / ↓ or j / k | Select a job; scroll an open detail view |
| 1 / 2 / 3 / 4 | Jobs, history, partitions, connection |
| Tab / Shift+Tab | Next / previous workspace tab (outside text entry and forms) |
| Enter | Inspect selected job |
| n | Prepare a job from the Jobs or History list; does not submit |
| / | Focus the command input |
| f | Filter the current job list |
| l / s / u | Output, submitted script, resources |
| f in output / [ / ] | Search retained output / previous or next match |
| f in script / [ / ] | Search submitted script / previous or next match |
| /sort | Choose Jobs/History order; optional `asc` / `desc` |
| e | Switch stdout/stderr while reading output |
| y | Copy confirmed failed-task IDs for the selected array |
| r / c | Refresh / reconnect |
| Ctrl+k / ? | Command palette / keyboard help |
| Esc | Return or clear a filter |
| q / Ctrl+c | Exit and restore the terminal |

Help groups navigation, selected-job tools, cluster commands and preferences.
It uses two columns in wide terminals and a scrollable single column in compact
windows. Home/End jump through an open panel; Esc returns to your workspace
without losing its selected job or filter.

Commands include `/run`, `/shell`, `/software cuda`, `/module cuda/12.8.1`, `/jobs`, `/history`, `/partitions`, `/logs`, `/script`, `/resources`, `/find text`, `/job 812046`, `/failed`, and `/connect`. Tab completes a command. A bare job ID in the input jumps to it. The input dispatches Perch actions; it is not a shell.

Clipboard copying uses `pbcopy` on macOS or an available `wl-copy`/`xclip` on Linux. Perch shows progress while copying, allows one copy at a time, and stops the helper after three seconds or when Perch exits. If copying fails, the notice includes the array specification. It never resubmits tasks. `NO_COLOR` or `--no-color` disables colors. Font family and size come from your terminal.

Failed-task selections include collapsed ranges and exclude tasks with newer attempts. Progress and the title-bar counts use the same distinct task membership. If overlapping ranges exceed Perch's computation or fragmentation bounds, the summary is marked unavailable and copying is disabled instead of returning a partial selection. In CLI/MCP JSON, `failed_array_spec` is the full selection; `failed_task_ids` contains only individually observed failed IDs. Array queries return an error if a summary cannot be computed.

The array bar measures **finished work**, including successes, failures and cancellations. Its green, red and muted segments distinguish those outcomes, with exact text counts alongside them. Running, queued and unconfirmed tasks leave the bar unfilled. The total remains the number of observed tasks, which may be smaller than the original submission. CLI/MCP `completed` still means successful tasks only.

## What the numbers mean

- Queue and accounting responses are reconciled. A job disappearing from the queue remains unresolved until accounting confirms its outcome.
- History covers the last seven days. Array totals describe observed tasks; they may not describe the entire original submission.
- Partitions show allocation, not CPU/GPU utilization.
- CPU efficiency uses recorded CPU time divided by allocated CPU wall time. Task peak RSS is not aggregate job memory.
- Live charts collect selected batch-step samples while the resource view is open. Missing readings and long sample gaps are not interpolated.
- Output is read incrementally into a bounded 256 KiB tail. Submitted scripts are viewed on demand and never executed.

## Development checks

```sh
make check
python3 -m venv .venv
.venv/bin/pip install -r tools/requirements.txt
make terminal-test
make mcp-test
PERCH_LIVE_HOST=your-cruzid@hb.ucsc.edu go test -count=1 ./internal/hummingbird -run TestHummingbirdLive -v
VERSION=dev make release
```

The terminal harness uses a real PTY and a VT emulator, exercises keyboard flows and resizing, and checks terminal restoration. Its PNGs are emulator renderings, not native Terminal.app screenshots. The screenshot renderer uses installed Ioskeley Mono when available, with Menlo fallback. Live integration is opt-in and only reads existing jobs; it submits or cancels nothing.

For short live transport checks, opt in with
`PERCH_TRANSPORT_LIVE_HOST=your-cruzid@hb.ucsc.edu go test -count=1 ./internal/transport -run TestHummingbirdTransportIntegration -v`.
This checks quoting, stdin, private multiplexing, partial errors, bounded stderr,
cancellation/recovery and cleanup using synthetic shell commands and temporary
spools. It does not submit a job or exercise password/MFA entry.

For log-file lifecycle checks, opt in with `PERCH_LOG_TEST_HOST=your-cruzid@hb.ucsc.edu go test -count=1 ./internal/hummingbird -run TestLogLifecycleIntegration -v`. This creates and removes a private temporary directory containing synthetic files; it does not touch existing output. `PERCH_LOG_TEST_HOST=local` runs the same check on Linux.

`testdata/monitor-smoke.sh` is an optional four-minute development workload (one CPU, 128 MiB request). Submit it explicitly with private, separate stdout/stderr paths, then run `.venv/bin/python tools/test_live_terminal.py your-cruzid@hb.ucsc.edu JOB_ID` while it is running. The harness checks live output, scripts, samples, reconnect, completion and terminal restoration. It never submits a job itself. Live tests use `-count=1` so Go cannot reuse a cached result from an earlier cluster state.

Release tooling builds local macOS/Linux candidates for arm64 and amd64. Cross-compilation is not runtime verification. Publication and license selection are pending.

## Design and scope

Ioskeley Mono is the approved reference font and recommended terminal setup. The terminal controls its actual font. Wordmark design is deferred for later review.

The user-selected default is warm charcoal with cream and peach accents (`--theme warm`). Cool graphite remains available with `--theme graphite`. Both use the same layout; `--no-color` overrides either theme.

The visual references are Superfile, Claude Code, and the original Hummingbird mockup. The title bar and persistent command input anchor the workspace. Top tabs keep navigation above the jobs and detail panels. Chip sits on the My jobs border in roomy terminals, using the selected original-derived octant treatment. His six animations reflect workload state; `--reduced-motion` uses still poses. The detailed image artwork is preserved for web and other uses. The small terminal artwork needs Unicode 16 block mosaics, available in Ioskeley Mono; native line-spacing seams remain under refinement.

- [Architecture](../ARCHITECTURE.md)
- [Feature design](../docs/feature-design.md)
- [Hummingbird capabilities](../docs/hummingbird-capabilities.md)
- [Readiness and remaining work](../docs/READINESS.md)
- [Approved Chip artwork](../assets/bird/README.md)

Perch is independent of any course or workload. Native notification acceptance, final persistence/diagnostics review, disconnect/reconnect acceptance, and final visual acceptance remain part of the active build goal. Quota views remain open; explicit allocation shells are available in the CLI and workspace, with acceptance scope documented above.

### Activity this session

Open `/activity` or choose **Activity this session** in the command palette.
The feed shows the latest 100 finishes, failures, cancellations and time warnings
that Perch observed, newest first. Existing accounting history is not replayed as
new activity. Times say when Perch observed the change, in your local timezone.

Use arrows, Page Up/Down, Home/End or the mouse wheel to choose an entry; Enter
or a click opens its exact job attempt in Jobs or History. Escape returns to the
feed, then your previous list and filter. If that attempt is no longer loaded,
the entry stays visible and explains why it cannot open; it never follows a reused
ID or restart. New events preserve the selected entry until it leaves retention.

Activity shares the normal refresh and notification event stream. It adds no
polling, survives a disconnect in the current process, and is not saved between
runs or reconstructed from the snapshot cache. It requires no desktop notification
permission. This is a workspace view; CLI/MCP history remains scheduler history.

### Your queue-wait history

Open `/waits` or choose “Queue wait history by partition” in the command palette.
`perch waits --json` provides the same analysis for scripts; MCP exposes
`perch_waits`. The TUI reuses its normal accounting refresh and supports keyboard
and mouse scrolling, including compact windows.

For starts within seven days of the accounting snapshot, each partition shows
submit-to-start and eligible-to-start median/P90, with separate sample counts.
P90 uses nearest rank; an even sample's median averages the middle two values.
Missing eligibility remains unavailable. Every array task is a separate sample;
large arrays can dominate, and resource requests may differ. These observations
are not forecasts or a recommendation of the fastest partition.

Restarted jobs, duplicate attempts, aggregate array rows, unstarted jobs, invalid
chronology and starts outside the window are excluded. Requeues can reset Submit;
Eligible records when a job became eligible to run. See the
[Slurm accounting field definitions](https://slurm.schedmd.com/sacct.html).
The view labels its accounting snapshot time; an older snapshot remains visible
when a refresh fails, with the normal connection error status.

### Node allocations

On **Partitions**, press or click **n nodes**, or use `/nodes` from any tab.
The report shows each visible node's partitions, state flags, CPU allocation,
configured/allocated memory, unallocated memory and GRES. Escape restores your
calling list/filter/selection; `r` reads again. CLI `perch nodes --json` and MCP
`perch_nodes` expose the same fields, with missing quantities as null.
Press or click **g GPUs** to show only nodes with GPU resource descriptions;
**g all nodes** restores the full report. This local filter survives refresh and
does not issue a cluster query. GPU rows show the resource type, allocated /
configured count and unallocated count while retaining raw GRES for inspection.
Counts refer to scheduler GPU resources, which may be MIG instances. Unknown,
ambiguous or inconsistent allocation counts remain unavailable, not zero.
Reports also remember your reading position in Partitions and Connection.
After resizing or updated data, Escape uses nearby unchanged text to restore
that context; if it has disappeared, the previous scroll offset is clamped to
the available content.

These are scheduler allocations, not utilization. Unallocated memory subtracts
job allocations and system-reserved memory from configured memory; it is separate
from OS free memory. Idle CPUs do not guarantee a job can start, and flags such
as PLANNED, RESERVED and DRAIN matter. Negative unallocated memory is preserved
when reported allocations exceed configured capacity. No background node polling
is added. The report's age is independent of the normal queue refresh.

### Pending-job priority

Open `/priority` or choose **Queue priority factors** in the command palette.
`perch priority --host your-cruzid@hb.ucsc.edu [--json]` and MCP `perch_priority`
provide the same personal report. Reads happen on request; `r` refreshes the
report, and Escape restores your previous list, filter and selection.

Each allocation/partition shows Slurm's total score and its weighted factors.
These are **scores, not queue positions or promised start times**. Resources,
holds, dependencies and reservations can still prevent a high-scoring job from
starting. Zero contribution does not prove a factor is disabled. Displayed
factors may be rounded, so Perch preserves the reported total instead of summing
them. Array allocations may represent more than one pending task. Held or newly
submitted jobs can have no reported factors; query errors remain errors.

The report also reads the configured fair-share priority weight in the same
SSH batch. A confirmed weight of zero means fair-share does not contribute to
job priority. CLI/MCP expose it as `priority_policy.fairshare_weight`; missing or
failed configuration stays null while valid job scores remain available. This
is an on-demand observation, not a permanently assumed cluster setting. On short
screens, the inactive command box yields space to the report; `/` restores input.

### Troubleshooting captures

Start Perch with `--debug` to record operation names, durations, exit codes,
classified failures, byte counts, requested parser version and application
version. Metadata omits arguments, host/user identifiers, paths, input,
stdout/stderr content and arbitrary error messages. The path appears on stderr
and in **Connection**. It works with the workspace, queries, `doctor`, `mcp`, and
`run`; offline demo/launch preview only record session startup.

```sh
./bin/perch --host your-cruzid@hb.ucsc.edu --debug
./bin/perch jobs --host your-cruzid@hb.ucsc.edu --debug --debug-raw --json
```

`--debug-raw` requires `--debug` and additionally captures raw responses from
JSON `squeue`, `sacct`, `sinfo` and `sstat` queries. It can include private names,
paths, job commands and other unpublished metadata; review before sharing.
Script/log reads, submission stdin/output and interactive login are excluded
from raw capture. No passwords are collected. Metadata-only and raw sessions
use separate directories, so a later metadata session does not inherit raw
responses from an earlier one. Neither flag is saved in preferences.

Captures live under the platform cache directory in
`perch/diagnostics/metadata` or `perch/diagnostics/raw`. Each keeps
`events.jsonl` and `events.previous.jsonl`, each at most 2 MiB (4 MiB per mode,
8 MiB total), plus an empty lock file. Directories/files are created with
0700/0600 permissions. Rotation discards the oldest file. Close Perch before
removing captures you no longer need. Nothing is uploaded automatically.

Each JSONL event has a session ID and schema version. Raw streams are base64
bytes, preserving invalid UTF-8 without emitting terminal control sequences.
At most 256 KiB combined stdout/stderr is retained per response;
`raw_truncated` and original byte counts identify incomplete captures. Raw
capture is troubleshooting evidence, not an automatically shareable fixture.
A crash can leave a partial final JSONL record. An SSH framing failure can only
be recorded as classified metadata because the runner discards the invalid
framed response.

If the capture cannot start, Perch reports an error before contacting the
cluster. If a later write fails, capture stops and the error appears in
Connection and on stderr at exit. The actual command/submission result is
preserved: a diagnostic disk failure must not prompt a duplicate submission.

Job identity includes cluster, user, submit time and restart count. Current
scheduler records take precedence; duplicate accounting IDs select the newer
comparable attempt instead of depending on response order. A disappeared retry
stays **awaiting accounting** when only an older attempt's outcome is available.
The inspector and single-job CLI output show nonzero restart counts, and JSON
exposes `restart_count` for agent/script consumers. Changing attempts clears
pending log/script replies and live chart samples even if the displayed ID stays
the same. Historical accounting returned by `perch history` remains uncollapsed.


### NRP experimental archive

NRP was removed from the active product scope on October 10, 2026. The existing
implementation, tests and instructions below are preserved for future work;
real-cluster acceptance is not claimed and is no longer a Hummingbird release gate.

You can now prepare a batch Job without cluster access:

```sh
perch nrp prepare --name analysis --namespace research --image python:3.13-slim \
  --cpu 1 --memory 512Mi --ephemeral-storage 1Gi --time 5m \
  -- python -c 'print(sum(i*i for i in range(1000000)))'
```

This prints a local review. Add `--manifest` before `--` and redirect stdout to
`job.json` to export a Kubernetes Job, or `--json` for the review and manifest
in one versioned response. CPU, memory, local storage and a finite deadline are
explicit; requests equal limits. `--pvc NAME --mount-path /data` mounts an
existing namespace volume; `--workdir` names a directory inside the container.
GPU drafts require both `--gpus N` and `--gpu-resource KEY`. This does not submit,
read credentials or prove access, quota, image compatibility or admission.
See [workload preparation](../docs/NRP.md#offline-workload-preparation).

With an explicitly configured context, check the exported Job against the server:

```sh
perch nrp validate job.json --context MY_CONTEXT --namespace MY_NAMESPACE
```

This sends one Job create request with `dryRun=All` and requests strict field
validation. It creates no Job or Pod. The result includes a manifest hash and an
independent quota snapshot. Job admission does **not** establish Pod admission,
image availability, PVC access or scheduling capacity. Add `--json` for a
versioned report; a rejected or unverified dry-run exits nonzero while still
printing the report. A successful Job dry-run can have an unavailable quota
snapshot, which stays explicit. Reports distinguish an actual rejection from an
unverified result and include a next step for login, scope, manifest or connection
problems. `--login` retains the existing foreground-only
credential flow. See [server validation](../docs/NRP.md#server-job-validation).

The experimental read-only workspace and monitoring commands have an offline demo:

```sh
perch nrp ui --demo
perch nrp jobs --demo
perch nrp pods --demo
perch nrp quotas --demo
perch nrp pod retrying-replacement --demo
perch nrp job indexed --demo --json
perch nrp logs retrying-replacement --container compute --previous --demo
```

Jobs retain Kubernetes controller states; Pods expose current and previous
container states, restarts, init/sidecar/ephemeral roles and owner UIDs. Unknown
counts stay unknown. The demo is synthetic and never reads kubeconfig or uses
the network.

Contexts already using a static token or client certificate can be queried with
`perch nrp jobs --context NAME --namespace NAME` (optionally `--kubeconfig FILE`).
The selected configuration is read through kubectl without changing the current
context. Requests then use verified, bounded HTTPS; no credential plugin runs
unless sign-in is explicitly requested.

For a configured exec/OIDC context, explicitly sign in from a terminal:

```sh
perch nrp jobs --context NAME --namespace NAME --login
```

This runs the selected kubeconfig's credential plugin once in the foreground;
it may open a browser. Credentials stay in memory for the command, and API
errors never silently restart sign-in. The plugin may manage its own normal
credential cache. Without `--login`, exec configurations are refused. Legacy
`auth-provider` configurations remain unsupported.

`perch nrp ui --context NAME --namespace NAME --login` opens the namespace
workspace with explicit sign-in. Use Jobs and Pods tabs, Enter to inspect,
`f` to filter, and `c` to reconnect in the foreground. Tab/Shift+Tab and mouse
navigation work down to 48×18; `--no-mouse` leaves the mouse to your terminal.
Job, Pod and quota snapshots refresh independently, retain stale data on errors, and
stop automatic reads when authentication expires. Kubernetes UIDs preserve
selection identity; a recreated workload does not inherit the old inspector.

`perch nrp logs POD --container NAME` reads a bounded current-container tail;
add `--previous` for the previous terminated instance. The same destination and
explicit `--login` flags apply. Plain output is sanitized text; `--json` also
identifies the Pod UID, namespace, container, observation time and limits.
Perch checks Pod identity before and after reading and discards the result if
the Pod was replaced or an observed restart occurred during the read. This is
a bounded snapshot, not durable history or a live stream.

In the workspace, `l` opens logs from a Job or Pod. Choose a controller-owned
Pod, then an explicit container; `p` switches current/previous output and `r`
reads another snapshot. Esc walks back through your choices without losing
the workload filter. Failed reads retain the same selection's output with a
stale label; changing modes clears it before reading. `c` reconnects explicitly
and rereads an open log. Logs do not poll or follow automatically.

**Namespace quota is now available** with `perch nrp quotas --demo`, or
`perch nrp quotas --context NAME --namespace NAME`. Use `--json` for typed
resource limits, usage, remaining amounts and scope selectors. In the namespace
workspace, tab **3** or `/quotas` shows the same quota observations. Empty,
denied, unknown and stale reads remain distinct. Limits are not free hardware;
scoped quotas can overlap. For a Pod's CPU/memory snapshot, use **u resources**
or `/metrics`; from a Job, first choose one of its Pods. The CLI equivalent is
`perch nrp metrics POD --context CONTEXT --namespace NAMESPACE [--json]`.
Use `--demo` with Pod `retrying-replacement` for an offline example.
Sample age/window, stale readings and missing values remain explicit; CPU is
average core usage and memory is an estimated working set. Metrics are read on
demand, require a served Metrics API and permission, and have synthetic-only
acceptance so far. Workload actions and GPU metrics remain unfinished.
The workspace, query and login paths have passed synthetic plugin/local TLS acceptance,
not a real kubelogin/CILogon session or live NRP acceptance. See the [implementation notes and remaining gates](../docs/NRP.md).
Hummingbird's existing commands and workspace remain separate.

`make nrp-test` runs the built CLI against a local temporary TLS server and
synthetic kubeconfigs. It requires Python and OpenSSL; installed kubectl is
also checked when available. No real credentials or NRP cluster are used.
`make nrp-ui-test` additionally exercises the real-PTY workspace, including
failed refreshes and explicit plugin reconnect, using the development Python
environment from `tools/requirements.txt`. Its screenshots are emulator renders,
not native Terminal.app acceptance.


### Verify local release candidates

`VERSION=dev make release` produces four archives and a checksum manifest in
`dist/`. Each archive contains the executable, linked documentation, MCP example
and Chip's original artwork, plus copied upstream dependency notices. Release
packaging requires Python 3.9+ and Go. Builds use temporary staging and omit macOS
extended metadata so extraction is quiet on Linux. Checksums use archive
basenames and remain usable when `dist/` is copied elsewhere.

`make notices-check` verifies the saved dependency texts against the current
four-target Go dependency graphs and toolchain. After dependency/toolchain
changes, run `python3 tools/notices.py` and review the generated inventory;
obsolete files require review rather than silent deletion. Release builds fail
if the notice bundle is out of date. The pinned Unicode supplement records its
upstream source. This inventory does not choose Perch's own license or replace
the final licensing review.

```sh
VERSION=dev make release-check
python3 tools/test_release_archives.py dev --extract-to output/release-check
python3 tools/test_release_runtime.py output/release-check/darwin-arm64/perch --version dev
```

Choose the extracted target that runs in your test environment. The archive
checker validates all four checksums, file types, executable mode, binary
OS/architecture, documentation
links, documentation/example bytes against the workspace, unchanged artwork and
notice hashes. It reads each binary's Go build
metadata and requires an exact match with the notice inventory's module versions
for that target. The runtime checker uses Python's standard library and needs
Python 3.8+, the executable, and `tools/test_mcp.py` beside the runtime script.
It exercises offline CLI queries (including nodes and priority) and array
preparation, the MCP wire protocol, Job actions, Activity, time-warning settings,
node/priority reports, selectable software/setup and return, job-draft correction,
NRP namespace quotas and Pod CPU/memory metrics, allocation-shell review,
compact cancellation review and session receipts,
real PTY navigation and resize, CSS/X11 hover-pointer requests, mouse-mode cleanup and terminal
restoration in both workspaces, in a temporary home. It makes no cluster calls
and does not verify native fonts or the mouse pointer's actual appearance.
Cross-compilation alone does not establish runtime acceptance. Architecture and
binary hash are included in each runtime report; emulated runs must be labelled.

To check first-run setup against an extracted candidate, use
`.venv/bin/python tools/test_setup.py --binary output/release-check/darwin-arm64/perch`.
This uses isolated preferences and synthetic SSH to exercise destination entry,
explicit saving, cancellation, error recovery and compact layouts without
changing your normal configuration.

For an explicitly authorized native Linux x86 check on Hummingbird, use the
opt-in harness below. It stages the executable and Python scripts in a private
directory, submits one three-minute job with one CPU and 256 MiB, and removes
that staging directory after successful completion. It installs nothing. The
job exercises synthetic demo data only. The report must be a fresh path; an
ambiguous submission or observation timeout is recorded without resubmission.
Inspect that report's job ID/comment before taking further action.

```sh
python3 tools/test_release_live.py --submit --host your-cruzid@hb.ucsc.edu \
  --binary output/release-check/linux-amd64/perch --version dev \
  --report output/validation/release-native-NEW.json
```
