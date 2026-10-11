# Cancellation: scope and implementation contract

Status: October 10, 2026. The production `perch cancel` CLI, shared cancellation
adapter and TUI review are implemented. Initial scope experiments, CLI and TUI
acceptance are distinguished below. Native terminal visual acceptance remains
open; PTY emulator renders do not establish it.

## Workspace review

`/cancel` reviews one selected current job or concrete array task. `/cancel-array`
explicitly reviews all remaining tasks of the selected array. Both are searchable
in the command palette, with availability tied to the visible Jobs selection.
History, cached rows, unresolved summaries and unavailable connections cannot
authorize these actions. Grouped pending rows offer whole-array review; one
unmaterialized task is currently selected through the CLI, not through the TUI.

The read-only review checks the selected row's attempt before displaying its
fresh scope. Ctrl+X or the labelled Cancel button confirms. Enter does not
confirm. Reviews pause periodic queries, retain caller context, and require
scrolling through overflow before enabling confirmation. Keyboard and pointer
controls share hit geometry. Repeated input and navigation are blocked while
sending; Ctrl+C remains an explicit exit and preserves an unknown-outcome
message outside the alternate screen.

A receipt triggers an ordinary refresh, never a fabricated job-state change.
`/last-cancel` reopens the latest receipt during the session; Perch also prints
the latest real receipt after terminal restoration on exit. This is not a
durable cancellation journal. Demo performs no remote operation and does not
change scheduler states. Escape restores the prior view/filter/scroll position;
if the selected job disappears or its attempt changes during refresh, returning
clears its old logs/resources and shows the job list with the refreshed selection.

`tools/test_cancel_ui.py` covers keyboard, pointer, stale results, duplicate
input, uncertain outcomes, interrupted exit and real PTY rendering at 140×40,
80×24 and 48×18. `tools/test_cancel_live.py --tui --submit --host HOST --report
FRESH_PATH` uses fresh bounded probes and the actual TUI buttons. Its default
cases are a held ordinary job, single-task then remaining-array cancellation,
and a mixed running/pending whole array.

## Implemented command

`perch cancel JOB_ID` previews an ordinary allocation. `perch cancel ARRAY_TASK`
previews one task. `perch cancel ARRAY_ID --array` explicitly previews every
remaining array task. Add `--confirm` to send one request. `--json` emits either
the preview or a receipt; it never changes confirmation requirements.

The adapter resolves an owned target from a fresh, local-cluster, user-scoped
queue and keeps an opaque two-minute review. Execution consumes it once,
requeries, and compares destination, owner, cluster, submission, restarts,
start/state, raw identity, name and observed scope. Any change requires another
review. Inherited `SQUEUE_*`, `SCANCEL_*` and `SLURM_CLUSTERS` options are cleared;
the final `scancel --ctld` addresses the verified `--clusters` and `--user`.
One-job/task requests also restrict state. Whole arrays can include mixed
pending, running and suspended tasks. Overlapping or incomplete records are
refused, and counts use compact range arithmetic.

`requested` means the command returned success. `unknown` means transport did
not establish the outcome. `unconfirmed` means a nonzero command result; some
array tasks may still have been affected. There is no automatic retry. An empty
queue or absent accounting record does not become a fabricated CANCELLED job.

Heterogeneous/federated scope and states with additional transition flags are
refused. Grouped-task cancellation is enabled only on the tested Slurm 25.11.4
release. The general parser remains v0.0.44. A query and mutation cannot be
atomic: a scheduler transition in their interval remains possible.

Offline checks: `make cancel-test` runs the real CLI and TUI through synthetic SSH;
`make check` includes race-tested selection, lifecycle, scope and environment
tests. Live acceptance uses `tools/test_cancel_live.py --submit --host HOST
--report FRESH_PATH`; it accepts no existing job ID. Its default five probes
cover held/running ordinary jobs and arrays, plus a mixed running/pending array,
at most two CPUs and 128 MiB at a time, with two-minute limits. Held/running
arrays check that task 1 survives task-0 cancellation before explicitly
cancelling the whole remaining array. The mixed array uses a concurrency limit
of one and requests cancellation of both states together.

### Production CLI evidence

- `output/validation/cancellation-cli-production-live-20261010.json`: held job
  802778, running job 802779, held array 802780, and running array 802781. Every
  preview and confirmation used `bin/perch cancel`. Both array siblings remained
  after task-0 cancellation, then the explicit whole-array action removed them.
- `output/validation/cancellation-cli-live-followup-20261010.json`: independent
  follow-up confirmed all four IDs absent and recorded accounting. The cancelled
  unmaterialized task 802780_0 again lacked a separate accounting row.
- `output/validation/cancellation-cli-mixed-live-20261010.json`: array 802788 had
  one running task and one pending task (`1%1`). The final binary's whole-array
  preview counted both states, confirmation removed both, and accounting
  reported both CANCELLED. Two queue checks confirmed probe absence.
- The first production held-job test (802776) successfully cancelled its target,
  but the harness rejected Slurm's empty-queue `Zero jobs to dump` warning during
  cleanup verification. This was a test-harness failure, not a passing run.
  `cancellation-cli-held-followup-20261010.json` independently confirms absence
  and CANCELLED accounting. The warning handling was corrected before the four
  passing cases; the original failure report is preserved.
- `testdata/hummingbird/cancellation-scopes.json` retains seven scope snapshots
  from those production probes, with the username scrubbed. The unit suite
  replays them through the final adapter validation as well as synthetic cases.
- `output/terminal/cancellation-preview-48.png` and `-80.png` are reviewed
  Ioskeley emulator renders from real CLI PTYs. They prove text wrapping/layout,
  not native Terminal.app appearance or TUI modal behavior.

## Why scope needs its own model

Job launching and allocation shells now exist, but users still need an explicit
way to stop a mistaken or unwanted job. Cancellation is irreversible for the
running processes, so the review must identify both the target and its extent.
It must never be an incidental effect of a key used for navigation.

Three different targets belong in the model:

| Target | Review must say | Scheduler selector |
| --- | --- | --- |
| Ordinary job | This allocation and all of its steps | `JOB_ID` |
| One array task | This task only; siblings continue | `ARRAY_ID_TASK_ID` |
| Whole array | Every remaining task in this array | `ARRAY_ID` |

A task's numeric allocation ID is not a substitute for the array-task selector.
In the running experiment, task 1's allocation ID was **the same number as the
parent array ID**. Passing that number to scancel would select the whole array.
The shell feature's use of a numeric allocation ID for `srun --jobid` must not
be copied into the cancellation adapter.

Slurm's reference describes differing array behavior for records that have and
have not split out of their parent. It also distinguishes controller-directed
requests from other invocation paths. Product support needs evidence for its
exact invocation, not an assumption based on a similar command. [scancel reference](https://slurm.schedmd.com/scancel.html)

## Live observations

Both probes used Slurm 25.11.4 and `scancel --ctld --user=researcher
--state=STATE -- ARRAY_TASK`. Each submitted its own two-task array, saved the
receipt, and verified username, job name, unique comment and parent array ID
before cancellation. No existing workload was accepted by the harness.

### Held, grouped tasks — array 802749

- The queue initially exposed one pending record with `array_task_string=0-1`,
  unset `array_task_id`, and reason `JobHeldUser`.
- Cancelling `802749_0` returned successfully. The same pending record then had
  `array_task_string=1`. The sibling remained held; no compute allocation ran.
- Cleanup cancelled the remaining owned array. A subsequent queue query
  confirmed absence.
- Accounting initially showed `802749_[1]`, then `sacct --array` showed
  `802749_1` as CANCELLED with zero elapsed time. **No separate task-0 record was
  observed**, including in the follow-up query. Do not infer that every accepted
  task-cancellation request will yield a separate accounting row.

Evidence: `output/validation/cancellation-scope-hummingbird-20261010.json`.

### Running, separate tasks — array 802755

- Two tasks ran, each requesting one CPU and 64 MiB with a two-minute limit.
  Their command was `exec sleep 90`.
- Task 0 had allocation ID 802758; task 1 had allocation ID 802755, which was
  also the parent array ID.
- Cancelling `802755_0` removed only task 0. Task 1 remained RUNNING.
- Accounting recorded task 0 CANCELLED after two seconds. Cleanup then cancelled
  task 1, recorded after six seconds. Both batch steps recorded signal 15.
  The `.extern` steps were COMPLETED; those are not successful job outcomes.
- The final queue check confirmed no remaining probe tasks.

Evidence: `output/validation/cancellation-scope-running-hummingbird-20261010.json`.

These observations support the tested controller-directed array-task invocation
on this Hummingbird version. They do not establish equivalent behavior on every
Slurm release, with federated/heterogeneous jobs, or with different flags.

### Production TUI evidence

- `output/validation/cancellation-ui-production-live-20261010.json`: actual
  TUI review and mouse confirmation cancelled held job 802802, task 0 of running
  array 802803 while task 1 stayed RUNNING, then the remaining array. Mixed array
  802813 contained one running and one pending task; whole-array confirmation
  removed both. All three cases passed their final queue-absence checks and
  recorded accounting. Only the harness's newly submitted probes were affected.
- `output/validation/cancellation-ui-terminal-all-20261010.txt`: the complete
  terminal regression suite passed, including cancellation, launch, shells,
  navigation, compact layouts, hover, log retention, cache and reconnect cases.
- `output/validation/cancellation-ui-final-unit-r3-20261010.txt`: focused race
  tests passed after tightening the return path. Unchanged selections preserve
  scroll; different jobs, new attempts of the same ID and disappearance clear
  stale detail. A preceding test failed because it changed the selection ID
  without updating the model's selected row; that failure remains recorded.
- Review images from the live task-0 test and the compact uncertain-outcome
  view were inspected. These are Ioskeley emulator renders of real PTY output,
  not native Terminal.app captures. Native visual acceptance remains open.

## Product contract

1. **Read-only preview first.** Resolve a target from a fresh user-scoped queue.
   Show the name, exact selector, owner, cluster, state, and whether the scope is
   one allocation, one array task, or the whole array. For arrays show distinct
   observed task counts and any incomplete data. Do not expand huge ranges into
   individual objects merely to count them.
2. **Explicit confirmation.** The TUI review has a separate Cancel job/task/array
   control. Escape returns with the same filter, selection and scroll context.
   Selecting an array task never silently promotes it to whole-array scope.
   CLI invocation previews by default; actual cancellation requires an explicit
   confirmation option. Explain that all steps in scope, including shells, stop
   and successful-job dependencies can become unsatisfiable. Output files are
   not removed by Perch.
3. **Fresh identity before execution.** Verify the SSH destination, resolved user,
   cluster, submit time and restart/attempt identity again immediately before
   the request. Refuse changed, foreign, missing or unsupported targets. Cached
   rows and accounting history cannot authorize cancellation. Confirming a
   stale form must not act on a new attempt. Document the unavoidable interval
   between a Slurm query and mutation; this is not an atomic compare-and-cancel
   API.
4. **Preserve selector semantics.** Construct the command from validated identity,
   not display text or an arbitrary list of flags. Array-task selectors retain
   both parent and task IDs. Reject steps, heterogeneous components, mixed
   clusters and ambiguous ranges until their exact behavior is separately
   supported. A whole-array action is distinct, not an error fallback.
5. **One mutation per confirmation.** Disable repeated input while sending.
   Clear inherited `SCANCEL_*` options and account for `SLURM_CLUSTERS`; ensure
   the final command addresses the inspected cluster. Keep normal scheduler
   termination behavior—no added force signal or hurried cleanup. Use a bounded
   deadline and do not automatically retry a timed-out or interrupted request.
6. **An honest receipt.** A zero command exit means the cancellation request was
   accepted, not that all processes stopped or accounting is complete. Surface
   “Cancellation requested” separately from job state; retain unknown outcomes
   after transport failure. Refresh queue/accounting through the existing
   coordinator. Report a final CANCELLED state only from scheduler evidence.
   The missing held-task accounting record above must remain a supported case.
7. **Keep MCP read-only.** Guides can explain the CLI/TUI action and how to inspect
   outcomes, but no mutating MCP tool is part of this change. Demo confirmation
   may illustrate the transition and must never invoke a real command.

## Required acceptance before enabling the control

- Parser/selection cases: ordinary pending and running jobs; grouped and split
  array tasks; a task whose numeric allocation ID equals the array parent;
  whole arrays with mixed states; overlapping ranges; missing/invalid identity;
  heterogeneous/federated refusal; requeues and job-ID reuse.
- Mutation cases: foreign owner, connection loss, changed attempt or scope,
  expired preview, duplicate input, nonzero scancel exit, uncertain timeout,
  a job completing during review, and an accepted request without an accounting
  record. No mutation may occur on preview, hover, cache load or reconnection.
- Real PTY keyboard/mouse reviews at wide and 48×18 sizes, including Escape,
  request-in-flight state, receipt, return context, and terminal cleanup.
- Production CLI/TUI live tests using newly created bounded synthetic workloads.
  Test a pending job, a running job, single tasks and explicit whole-array scope.
  Keep a sibling alive long enough to verify scope. Clean only owned probes.

## Reproducing the scope experiments

`tools/test_cancellation_scope_live.py` requires `--submit`, an SSH destination,
and a **fresh** report path. It will not resume or duplicate a previous report.
The default mode holds both tasks; `--running` allows the bounded two-task run.
It accepts no existing job ID and installs nothing remotely. These are opt-in
tests and are not part of `make terminal-test`.

```sh
python3 tools/test_cancellation_scope_live.py \
  --host YOUR_CRUZID@hb.ucsc.edu --submit \
  --report output/validation/cancel-held-NEW.json

python3 tools/test_cancellation_scope_live.py \
  --host YOUR_CRUZID@hb.ucsc.edu --submit --running \
  --report output/validation/cancel-running-NEW.json
```

Inspect an existing report and reconcile an uncertain receipt before starting
another experiment. A report or an observation timeout alone is not proof that
a remote workload has stopped.
