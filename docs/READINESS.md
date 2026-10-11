# Perch readiness

Current audit: October 10, 2026. **The build goal remains active.** Perch has a
working Hummingbird workspace, CLI, read-only MCP server and explicit workload
actions. The current local preview is **`preview-20261010-reconciled`**. Its automated
checks passed within the scopes below. Native experience, authentication/alerts,
and GPU-shell acceptance remain open. The user approved the supplied Ioskeley
terminal profile on October 10; NRP was explicitly removed from active scope,
with its existing implementation and documentation preserved.

This candidate adds accounting identity/outcome guards, exact overlapping-array
progress and failed-task copying, partial range notifications with checkpoint
deduplication, and honest unresolved states between queue and accounting polls.
Missing submission evidence cannot establish an older attempt. Error explanations
wrap inside the overview pane. Known retries and disjoint stepped memberships
remain usable. The detailed implementation and targeted validation are recorded
in [validation history](VALIDATION-HISTORY.md).

## Current preview checks

`output/validation/reconciled-release-summary.json` binds archive checks, platform
runtime results and test logs to exact executable hashes.

| Check | Current evidence |
| --- | --- |
| Source checks | Full Go race suite, vet and formatting passed after the reconciliation and explanation-wrapping changes |
| Terminal regression | All 48 scripts passed, including navigation, mouse, workload actions, 5,000-task arrays, copy/activity/cache and attempt regressions |
| Chip playback | All six states, relative cadence, pause, aligned stepping and the faster running sample passed; emulator render reviewed |
| Four archives | Checksums, target architectures, linked docs, original artwork and dependency notices passed; final documentation bundled with the tested executables |
| Mac ARM | Packaged executable passed natively, including CLI/MCP and Hummingbird/NRP demo PTYs; search/sorting screens checked at 140, 80 and 48 columns |
| Mac Intel | Same runtime and screen checks passed through Rosetta on Mac ARM; no claim of native Intel hardware testing |
| Linux x86 | Packaged executable passed on Hummingbird in job 803524, COMPLETED 0:0 in 35 seconds; temporary stage removed |
| Linux ARM | Cross-built and archive-verified; current executable not runtime-tested |
| Experimental NRP | Packaged Mac ARM executable passed local synthetic HTTPS, plugin authentication/recovery and PTY checks; no real NRP namespace tested |

Source publication to the private repository is authorized; the preview artifacts
below describe the earlier local build, not a public release. The earlier
`preview-20261010-scheduler` remains unchanged; its 47-script/platform evidence is
in `output/validation/scheduler-release-summary.json`. The current preview
supersedes its source behavior but does not retroactively change those results.

## Latest access checks

- `output/validation/reconciled-gpu-capacity.json`: node-24 still reported all four
  P100 GPUs allocated. No repeat GPU probe was submitted. This is a timestamped
  allocation observation, not a prediction of future availability.
- `output/validation/scheduler-nrp-access-check.json`: kubectl returned no context
  names, and the OIDC login plugin was not discoverable. The user has been asked
  whether they already have NRP access and a namespace; credentials were not requested.
- `output/validation/acceptance-access-checkpoint.json` records these boundaries
  and the first-run documentation pass. No account or credential settings changed.

## Evidence rules

- **Verified implementation** means the cited source and tests cover the stated
  behavior. It does not imply that every related product requirement is done.
- **Live observation** applies to its recorded cluster version, job and command
  scope. An absent queue entry alone is not a successful outcome.
- **PTY render** means output from a real process interpreted by our terminal
  emulator. Native font seams, pointer appearance and notification banners need
  separate evidence. User-provided native screenshots establish only what they show.
- **Open** means unimplemented, unavailable, or not sufficiently verified. A
  candidate is not silently promoted to a required shipped feature, nor removed
  from the scope discussion merely because it is difficult.

The complete previous checklist and dated investigations are preserved in
[validation history](VALIDATION-HISTORY.md). Older statements such as “not yet
implemented,” “remaining,” and “final” describe that checkpoint. Use this audit
for current status and the history for provenance, failures and superseded work.

## Original objective and requirement coverage

| Requirement | Current implementation and evidence | Remaining boundary |
| --- | --- | --- |
| Beautiful, calm interface inspired by Claude Code and superfile | Warm charcoal/cream/peach, top tabs, persistent command input, rounded boxes, responsive lists and inspectors; Ioskeley previews. `internal/ui`; latest visual/process evidence below. User accepted the border fix and described the 12-pt tight Chip view as “quite good.” | Reference-profile appearance approved by the user; wider native behavior remains scoped below. The wordmark remains deliberately deferred. Quality comparable to the inspirations is not proved by passing tests. |
| Preserve and refine Chip; animations and size | Approved D/stronger-wing octant rendition, fixed 15×7 footprint, six states, 64 frames each, running at 40 ms and other states at 80 ms, one-shot completion, reduced motion. Detailed original assets are checksum-protected in `assets/bird`. Fresh preview test passed. | Native review across all states at the reference font/spacing; other terminals/fonts are not certified. Harmonica remains an optional transition tool, not a replacement for approved frame animation. |
| Easy keyboard and mouse navigation | Shared draw/hit geometry, tabs, job selection, help, slash input, global palette, job-specific action menu, log scrolling, compact controls and exit restoration. `tools/test_terminal.py`, `test_mouse.py`, `test_hover.py`, `test_footer.py`, `test_job_actions.py`. | Native pointer/interaction acceptance. Terminal.app uses hover highlights; OSC 22 shapes are enabled only for detected supporting terminals. |
| Connect → inspect → logs → disconnect → reconnect | First-connection setup, system SSH, private multiplexing, batch background reads, foreground authentication, retained snapshots/output, backoff and stale-response rejection. `internal/transport`, `internal/ui`; live backend and synthetic-SSH PTY checks. | Actual interactive password/MFA expiry and recovery in the user's terminal. Fake SSH reconnect tests do not close that gate. |
| Correct Slurm state, arrays, accounting and attempts | Explicit v0.0.44 parser, set/unset/infinite values, logical versus allocation IDs, range/step separation, attempt identity, accounting lag and missing-array summaries. Domain/app/adapter tests and scrubbed Hummingbird fixtures. | Future Slurm schemas remain explicitly unsupported until tested; heterogeneous/federated mutations are refused. |
| Output and submitted scripts | Bounded incremental stdout/stderr, path substitutions, rotation/truncation, control sanitization, failed-read retention, active/historical submitted scripts. Live lifecycle tests recorded in history; fresh retained-output/script read passed. | Files can be missing or inaccessible; no durable remote log retention is promised. |
| Resource charts and useful failure explanations | ntcharts is presentation-only; per-step live samples and gaps, recorded CPU efficiency, peak-RSS scope and missing values remain explicit. Failure hints label scheduler facts separately from log evidence. `test_live_steps.py`, `test_resource_steps.py`, `test_diagnosis.py`; live compute-step job 802458 in history. | Native chart review, wider GPU/multi-node measurement acceptance. GPU allocation is not utilization; task peaks are not summed into total memory. |
| Finish/fail notifications and time-limit warnings | Opt-in macOS bridge, grouped/deduplicated events, bounded private checkpoint, restart and reconnect catch-up. Configurable `/warnings`, `--time-warnings` and saved defaults now share measured remaining time with the UI. Tests cover multiple thresholds, off, changed limits, requeues, failed refreshes, edited preferences, legacy keys and 5,000-job checkpoint compaction. | Native banner delivery/permission/identity acceptance. No native notification was sent for the preference change. |
| Consistent event presentation | `/activity` consumes the reconciled notification/Chip events: latest 100 per session, explicit observation timestamps, stable selection, exact-attempt inspection, keyboard/mouse and compact geometry. App/UI tests cover startup silence, all outcomes, retained events and restart refusal; real PTY exercises navigation and resize. | Native visual acceptance. Session observations are not a persisted scheduler audit. |
| CLI and agent guidance through MCP | Structured read-only CLI, explicit units/nulls, 15 bounded MCP tools, guides and investigation prompt. Shared query service, SDK and independent stdio protocol checks. Configuration example in `examples/mcp-config.json`. | Individual desktop-client configuration is not claimed. MCP remains read-only; workload actions use separately authorized CLI/TUI paths. |
| Explore Hummingbird and expose useful capabilities | `doctor`, selectable installed-software catalog/prerequisites, partition allocations, pending explanations and estimates, personal wait history, on-demand priority factors, per-node CPU/memory allocations, recorded/live resources. Three partitions and Slurm 25.11.4 were confirmed in the fresh query. | Personal quota currently permission-denied; priority scores are not queue positions or start predictions. Native report review remains open. |
| Move beyond monitoring: workload preparation | Offline resource/script preview, non-submitting validation, explicit one-shot submission, single/multi-node tasks, Open MPI, arrays/concurrency, successful-job dependencies, private submission journal and uncertain-receipt recovery. Production live job evidence is retained in history. | Arbitrary MPI ABI and multi-GPU placement are not certified. No automatic upload, retry or execution of viewed scripts. |
| Allocation shells and cancellation | CLI and TUI review, fresh owned-attempt/node/scope validation, foreground shell return; separate job/task/whole-array cancellation, one-shot execution and honest receipts. Production live CLI/TUI evidence includes two-node CPU selection, Ctrl-C, return and parent survival on job 803078. See [shells](ALLOCATION-SHELL.md) and [cancellation](CANCELLATION.md). | Native shell interaction; GPU shell acceptance and layouts beyond the tested cases. One unmaterialized pending array task is selected through the CLI, not the grouped TUI row. Cancellation receipts are session-local plus exit text, not a durable journal. |
| Preferences, cache and troubleshooting | Explicit saved destination/appearance, preview/apply, optional private bounded snapshots, metadata/raw diagnostic modes, corruption/ownership/type handling, atomic replacement. `internal/preferences`, `snapshotcache`, `diagnostics`, `launch/journal`; process tests. | Cache is display-only, not mutation authority. Logs/scripts/credentials are not cached. Raw diagnostic captures need deliberate review before sharing. |
| Mac/Linux executable and eventual distribution | Four current candidate archives, checksums, doc/artwork comparison, binary target/dependency validation and runtime hash matching. Current Mac ARM and Linux x86 tested natively; Mac Intel tested under Rosetta. Older Linux ARM/OrbStack evidence applies only to that older binary. | Current Linux ARM runtime acceptance; repackage after further source changes; select licensing before publication. Source is tracked in the private darthnithin/perch repository. No public release or Homebrew tap is published. |
| NRP (deferred; outside current scope) | Explicit namespace/context, typed Job/Pod/container identity, bounded HTTPS reads and log snapshots, explicit credential handling, CLI and namespace UI. Synthetic TLS/plugin/PTY coverage. See [NRP](NRP.md). | No configured context was found in the fresh local check. Quota reads, exact quantities and partial failures now have synthetic CLI/HTTPS/PTY coverage. On-demand Pod CPU/memory metrics also have synthetic parser, CLI, HTTPS and PTY coverage. Existing experimental code, tests and documentation are retained. No further NRP onboarding or implementation is required for the Hummingbird release. |

## Native review to complete

The reference is the user's Terminal.app with Ioskeley Mono 12 pt and 0.90 line
spacing. Perch does not change the terminal's font/profile. The user accepted the supplied
[profile](terminal/Perch-Ioskeley.terminal) and its appearance on October 10.
This does not establish password/MFA recovery, desktop banner delivery or every
interaction/state; those remain separate checks.

1. Run `./bin/perch --demo --no-config` from the source checkout, or
   `./perch --demo --no-config` from the extracted preview. Check Chip's movement,
   border continuity, tab clicks, output scrolling and a smaller window. Use
   Escape to leave input/panels and `q` to quit. Terminal.app uses hover highlights;
   a hand pointer is only expected in a detected supporting terminal.
2. Connect with `--host your-cruzid@hb.ucsc.edu`. Use `c` when sign-in is required.
   Confirm the real password/MFA flow returns to the workspace and that quitting
   restores the terminal. Credentials belong in SSH's terminal prompt, not chat.
3. For a deliberate notification review, launch with `--notify desktop` and
   observe an owned finite job ending. Confirm the visible banner, app identity
   and permission behavior. Synthetic sender tests do not prove native delivery.

These checks supplement the automated navigation tests. No native app-control
restriction has been bypassed, and a known-good older screenshot does not certify
all current screens or states.

## Open acceptance and implementation work

1. **Current native experience.** Review the complete interface and all six Chip
   states at the user's reference Ioskeley Mono 12 pt / 0.90 line spacing. Check
   actual pointer behavior in a supporting terminal and the Terminal.app
   highlight fallback. The prior native-control rejection has not been bypassed.
2. **Authentication and alerts.** Exercise real interactive sign-in/MFA expiry,
   return to the same workspace and terminal restoration. Verify opt-in native
   completion/failure banners, app identity and permissions. Do not equate a
   compiled AppleScript bridge or fake notification sender with visible delivery.
3. **Resource/action breadth.** Two-node CPU shell selection passed on 803078.
   Verify remaining GPU shell and wider resource-measurement cases, or explicitly
   constrain the supported behavior based on evidence.
   Preserve the scope distinction between live steps and whole-job accounting.
   GPU probe 803377 remained pending for 180 seconds and was cleaned up without
   starting; GPU-shell acceptance remains open. Scoped GPU requests discovered by
   the live inspection are now represented without implying GPU allocation.
4. **NRP deferred by the user.** Preserve all existing source, fixtures, tests
   and documentation. No NRP access request or live acceptance gate remains in
   the active Hummingbird scope.
5. **Delivery.** The reconciled preview is verified as described above; repeat packaging
   and artifact/runtime identity checks after further accepted source changes.
   Complete current Linux ARM runtime acceptance. Resolve project/artwork licensing before
   publication. The user authorized a private GitHub repository and release preparation.
   Public publication and contacting administrators remain separate actions.
6. **Final completion audit.** Revisit every row above and the authoritative
   architecture/feature documents. Unverified native behavior
   and unimplemented in-scope requirements cannot be closed by a clean build.

## Candidate and policy boundaries

These remain tracked rather than silently advertised as implemented:

- Personal BeeGFS quota: the installed CLI denied access to its authentication
  file. Do not bypass permissions or present shared `df` capacity as user quota.
- Persistent login-node TUI: technical execution is possible, but explicit site
  approval is unverified. Mac-hosted SSH remains the default; compute-allocation
  tests do not authorize a login-node service.
- Deeper fairshare analysis, GPU utilization and wider distributed workflows
  need specific capability/metric evidence. Catalog entries are not available
  allocations, and a zero fairshare priority weight is not an active factor.
- Harmonica transitions, new branding/wordmark studies and arbitrary multi-cluster
  plugins are not prerequisites for the current approved frame animation or
  Hummingbird-focused design. Revisit them only for a concrete product need.

See [architecture](../ARCHITECTURE.md), [feature design](feature-design.md),
[Hummingbird observations](hummingbird-capabilities.md), [MCP](MCP.md),
[NRP](NRP.md), and [validation history](VALIDATION-HISTORY.md) for the full scope.
