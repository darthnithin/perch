# Feature design and implementation order

Saved October 9, 2026; status refreshed October 10. This document records proposed product behavior from the architecture discussion. Implementation is in progress. Monitoring, slash commands, charts, failure summaries, time warnings, failed-task copying, opt-in notifications, saved preferences, submission recovery and an optional private restart snapshot now exist. Native notification delivery and the remaining acceptance gates in [READINESS.md](READINESS.md) are still open. The user prioritizes building the product properly; sequencing should follow dependencies while preserving the intended quality.

## Shared foundation

Normalized snapshots → job/attempt reconciliation → domain events and evidence-backed insights → UI, notifications, and Chip.

Use one event interpretation for all consumers. Preserve stable cluster/job/array-task/attempt identity, requeues, and timestamps. Do not count `.batch` and `.extern` steps as additional jobs. A lost connection or absent job must never become a fabricated success/failure event.

## Monitoring and investigation

| Feature | Intended behavior and constraints |
| --- | --- |
| Finish/fail notifications | User-configured delivery; authoritative reconciled outcomes; grouping for large arrays; persistent deduplication within a bounded retention window; baseline existing history without flooding the user. |
| Failure summary | Combine scheduler state, exit/signal information, and an optional bounded log-tail scan. Distinguish confirmed scheduler outcomes from probable log-based explanations; retain links to evidence. A traceback or a CUDA message alone does not establish the cause of the final job failure. |
| Time-limit warnings | Compare known scheduler elapsed/remaining time with a finite limit. Deduplicate configurable thresholds, account for changed limits/requeues, and label stale estimates. Never represent missing or unlimited time as imminent failure. |
| Array progress | Count tasks/ranges without double-counting parent summaries or steps. Current queue membership wins over overlapping accounting ranges; failed-task copies exclude those queued/running members. Contradictory terminal ranges with the same attempt evidence make the summary unavailable instead of choosing an outcome by row order. The finished-work bar includes succeeded, failed and cancelled tasks with distinct segments and text counts. Running, pending and unresolved tasks remain unfilled. Show the observed basis of the total when the full submission set is unknown. CLI/MCP completed counts remain success-only. |
| Copy failed-task specification | Generate a validated, sorted, deduplicated `--array=...` argument from selected failed tasks. Copying is separate from resubmitting; no automatic retry or script execution. Clipboard transport support must be detected. |
| Submitted script | Read-only, on-demand view of the script captured at submission, not the potentially changed file at the original path. Use active-controller retrieval or accounting when available. Sanitize terminal output, avoid default disk caching, and never execute the viewed content. |
| Efficiency/history charts | Display scope-correct recorded CPU and memory data. Preserve missing samples and accounting lag; avoid treating a per-task high-water mark as aggregate memory. |

Desktop notifications should have a backend interface. For Mac-hosted Perch, native macOS delivery is the preferred initial path; validate notification permissions, identity, and delivery behavior. The initial opt-in backend uses an AppleScript bridge with fixed source and message data passed as arguments. Native banner visibility, app identity and permission behavior still need acceptance testing. Terminal notification escape sequences are an optional capability-dependent backend, with terminal/multiplexer settings considered; support must not be assumed. On-cluster mode cannot invoke the Mac's local notification API directly.

Notifications require Perch to be running and connected sufficiently to reconcile events. The current architecture does not include a background monitoring agent or promise alerts while the app is closed. Reconnection catch-up should be grouped and bounded. No external notifications have been sent while designing this feature.

## Additional capabilities

**Queue wait history:** offer retrospective statistics from the user's own accounting history, with sample count and date window. Separate submit-to-start time from eligible-to-start queue delay, since holds and dependencies can inflate the former. Partition alone is insufficient context: requested resources, QoS, constraints, and requeues matter. Label medians/percentiles as historical observations, not start-time guarantees; pending/cancelled jobs create selection bias.

**Pending-job priority:** `/priority`, CLI `priority`, and MCP `perch_priority`
show personal allocation/partition scores and weighted factors on demand. The
report separates score from queue position and never turns an absent held job's
factors into zeros. Preserve Slurm's reported total rather than reconstructing
it from rounded factors. Nice is a separate adjustment; zero contribution is not
proof of a disabled factor. Escape returns to the calling list/filter/selection.
Reads add no recurring controller polling. The report now reads controller
configuration in the same SSH batch and explains its fair-share priority weight.
Missing or failed configuration leaves the weight unknown without discarding
valid job scores. A fresh October 10 query confirmed observable personal shares
but weight zero; those shares currently contribute no job priority. Deeper
association analysis remains a separate candidate, not an implied queue rank.

**Node allocations:** `/nodes`, CLI `nodes`, and MCP `perch_nodes` read current
per-node CPU/memory/GRES allocations and preserve all state flags. Partitions has
a clickable `n nodes` shortcut. Missing memory stays unknown; system reservations
are subtracted before showing unallocated memory, which may be negative. These
values are not OS utilization or a promise that a pending job can start. No
recurring query is added; report observation time remains independent of queue
refresh. Keyboard/mouse scrolling works from the first content row.

**Job GPU requests:** owned live probe 803377 requested one GPU
per node. Its queue response reported `tres_per_node=gres/gpu:1`, while
`tres_req_str` omitted the GPU and `tres_alloc_str` was empty because it had not
started. The inspector, summary and shared CLI/MCP job model now retain scoped
queue GPU requests without deriving allocation from a request, assuming missing
means zero, or summing overlapping per-job/per-node/per-socket/per-task constraints.
Typed GPU requests remain distinct; unsupported details produce an explicit note.
The side summary wraps each constraint as a whole item. When vertical space is
insufficient, a counted request hint leads to the full inspector rather than
clipping away the scope or silently presenting a partial list as complete.
The probe's accounting record also omitted the GPU request, so historical values
remain unknown rather than being reconstructed. Selected live fields are recorded in
`output/validation/gpu-shell-request-fields.json`.

**Pending-job guidance:** the inspector distinguishes user/coordinator and
administrator holds, invalid account/QoS settings, partition state/time limits,
unsatisfiable constraints and required-node unavailability. Original scheduler
reasons remain visible, including unknown site-specific values in the summary.
These explanations do not promise that clearing one blocker makes a job runnable.
At 24 rows or fewer, the inactive command box yields four rows to the inspector;
`/` restores input and Escape returns the space without changing selection/filter.

**Storage:** distinguish filesystem free space, user quota, group quota, inode limits, and directory usage. `/home`, `/groups`, and `/scratch` mapped to the same BeeGFS mount during a read-only check; three directories are not necessarily three independent capacity pools. Prefer bounded filesystem-native quota queries when authorized and supported. Do not substitute `df` for personal quota or periodically recurse over home directories with `du`. The installed BeeGFS CLI denied quota retrieval because its authentication file is not accessible to this account; a supported user-facing route remains unresolved.

**Software discovery:** `/software` results are selectable by arrows/page keys,
wheel and pointer. Enter or clicking a result reads its exact module setup;
Escape returns through the catalog to the caller's list/filter or cluster panel.
Catalog search (`f`) is separate from job filtering. Refresh keeps the selected
module's name when possible; unavailable or empty results cannot activate hidden
job actions. Setup reads do not load modules or execute their displayed commands.
Prerequisites and login-environment availability remain distinct from workload
compatibility. At compact widths each module uses separate name and status rows.

**Interactive shell in an allocation:** the explicit `perch shell` CLI and TUI `/shell` preview now open an additional shared step with fresh ownership/attempt/node checks. The TUI preserves the calling view/filter and restores its terminal after exit; the existing SSH connection is reused. It does not attach to the batch process or copy its runtime environment. See [allocation shells](ALLOCATION-SHELL.md) for resource scope, tested behavior and remaining gates. Two-node CPU selection passed live on job 803078; native terminal acceptance and GPU scope remain open. Never trigger a shell through a capability probe or automatic diagnostic; the underlying `srun` command is not a universal recipe.

**Explicit cancellation:** stopping an allocation, one
array task, and a whole array are separate scopes. Live held/running array
experiments verified the controller-directed task selector on Hummingbird and
exposed missing accounting for a cancelled unmaterialized task. The production
CLI now previews by default and requires `--confirm`, with `--array` for whole
arrays. The TUI now has separate `/cancel` and `/cancel-array` reviews with
Ctrl+X/pointer confirmation, restored caller context, and a retained session
receipt. Its identity, receipt and acceptance requirements are in
[cancellation](CANCELLATION.md); native visual acceptance remains open.

**Discovering job actions:** `a`, the clickable **a actions** footer, and
`/actions` open a compact menu for the selected job. Output, scripts, resources,
shells, failed-task copying and cancellation are grouped together, with typed
filtering and explanations for unavailable actions. Shell/cancellation choices
lead to their reviews. The menu remains pinned to the opening job attempt; a
replacement selection cannot silently become the target. At minimum terminal
size, the inactive command bar is hidden to leave room for five action rows.
Escape preserves the previous view, selection/filter and paused-log position.
Ctrl+k remains the general workspace palette.

## Diagnostics and support

**Correcting a job draft:** validation identifies the field through structured
errors shared with the CLI, rather than matching error-message text. The TUI
focuses that input and displays a wrapped explanation beside it. At compact
sizes the inactive preparation progress bar yields space to the correction;
keyboard and pointer controls use the same geometry. Other draft values remain
intact. Correcting a field returns to the existing review/validate/submit flow;
validation errors never perform remote operations.

- `perch doctor` uses the same capability model as startup and explains unsupported versus temporarily unavailable features.
- Unsupported optional panels can be omitted from ordinary navigation while remaining explained in doctor. Permission/connection failures must not silently masquerade as an unsupported feature.
- Implemented: `--debug` captures bounded operation metadata with identifiers/paths/argv/output omitted; requested parser and application versions are retained.
- Implemented: `--debug --debug-raw` captures selected scheduler JSON responses in a separate private directory. Each mode retains at most 4 MiB; individual raw responses cap at 256 KiB. Script/log reads, stdin and interactive login are excluded. Scheduler metadata can still contain secrets or unpublished work. Publishing/uploading is never automatic.
- Fixture creation is a deliberate scrub-and-review workflow. Debug captures and shareable fixtures are different artifacts.

## Implementation order

1. Model identity, capability results, transport, reconciliation, events, demo fixtures, diagnostics, and reconnect behavior alongside the initial monitoring UI.
2. Complete and verify the connect/inspect/log/disconnect/reconnect workflow with the approved Chip artwork.
3. Build notifications, failure explanations, time warnings, array progress/copy, script viewing, and resource charts on that foundation. Test delivery, deduplication, evidence, and unit semantics end to end.
4. Expand into historical queue analysis and verified quota views; evaluate explicit interactive allocation actions separately.

This is implementation sequencing, not a finalized promise that every candidate ships in the first release. Chart-library compatibility and terminal animation quality should be investigated early enough to avoid later rewrites. Preserve the Hummingbird focus; capability detection alone does not establish support for arbitrary Slurm clusters.

## Evidence and references

- Live check: `sacct --batch-script` successfully returned a stored script for an existing completed array task. Content was examined only to confirm availability; it was not saved into the project or displayed in chat.
- Live check: home, groups, and scratch resolved to BeeGFS at `/mnt/beegfs`; `beegfs` and `quota` executables were present. A later BeeGFS 8.4.1 quota query failed with permission denied reading `/etc/beegfs/conn.auth`; no permissions were bypassed. Silent `quota -s` output does not establish an unlimited quota.
- [Slurm accounting and submitted scripts](https://slurm.schedmd.com/sacct.html)
- [Slurm srun step/resource semantics](https://slurm.schedmd.com/srun.html)
- [Slurm scontrol](https://slurm.schedmd.com/scontrol.html)
- [macOS script notifications](https://developer.apple.com/library/archive/documentation/LanguagesUtilities/Conceptual/MacAutomationScriptingGuide/DisplayNotifications.html)
- [Hummingbird storage areas](https://hummingbird.ucsc.edu/getting-started/)
- [ntcharts dependencies](https://github.com/NimbleMarkets/ntcharts/blob/main/go.mod)
- [Harmonica](https://github.com/charmbracelet/harmonica)

### Observed activity

`/activity` and the global command palette expose the latest 100 reconciled job
changes for this session. Arrow/page/home/end selection, wheel navigation and
click/Enter inspection share drawn geometry down to 48×18. New arrivals retain
the selected event. Opening an entry matches its exact attempt, not just a job ID;
Escape returns through the feed to the previous list/filter. Job action shortcuts
are unavailable while the feed is visible, so they cannot operate on a hidden
list selection. Existing history and cached snapshots never become fresh activity.
Observation times are explicitly local; notifications remain independently opt-in.

### Time warning preferences

Implemented through `/warnings`, `--time-warnings`, and `perch config set
time-warnings`. The editor separates session Apply from explicit Save defaults
and cancels drafts without changing warning behavior. Its compact centered card
uses the same draw/hit geometry for keyboard and pointer input. Policies accept
1–5 distinct whole-minute thresholds, 1m–24h each, or off; default remains 5m.
Highlights, activity and desktop messages share recorded scheduler evidence.
Missing/unlimited time, failed refreshes and local clock ticks do not create
crossings. Each threshold applies only to limits at least twice its duration.
Restart deduplication, changed limits/requeues and preference changes are tested;
configuration does not implicitly enable desktop notifications.


### Searching retained output

The output view uses `f` for a local literal, case-insensitive search. Its input
reuses the visual style of the bottom command bar. Enter keeps the query; `[` / `]`
select previous/next matches with wraparound. Escape cancels an edit back to the
prior query, match and reading position; Escape on a committed search clears it
before leaving output. End clears search and resumes following. The global
**Filter jobs** palette command and `/find` retain their job-filter meaning.

Search operates on sanitized, bounded output only. It performs no extra remote
read. Matching and highlighting run on query/content changes, not animation
frames. Appends preserve the selected match and paused reading position; when
rotation or buffer retention replaces text, the previous match index is cleared.
Failed reads retain the existing searchable tail and stale indication. The
selected attempt, stdout/stderr scope and caller's activity/list/filter remain
separate from the query. Tab characters render as four spaces for deterministic
cell positioning. The input is bounded at 100 characters and indexing at 4,096
matches, with an explicit `+` for additional matches.

For windows at most 24 rows tall, an inactive log command bar yields four rows
to output. Editing search or entering a command restores the bar. Committed
queries remain visible in the output header. Drawn and clickable geometry share
this mode so a hidden input cannot intercept clicks on log text.

Output is prewrapped at grapheme boundaries when its content, highlight or
viewport width changes. This prevents repeated scans of a newline-free 256-KiB
line during animation. Each physical highlight run has a complete style/reset;
joined emoji and combining marks remain intact across selection and wrapping.
