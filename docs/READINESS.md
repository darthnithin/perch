# Perch readiness

Current audit: October 11, 2026. **The build goal remains active.** Perch has a
working Hummingbird workspace, CLI, read-only MCP server and explicit workload
actions. The private draft is **`v0.1.0-preview.2`**. The application validation baseline is
**`efb9d02`** (the tested preview source), with checks scoped below.
All four packaged platforms have runtime evidence. The user confirmed ordinary
connect/reconnect, clean exit and completion banners with the local development
binary. On October 11 they also accepted the slower running motion and
confirmed tab/job clicks, output scrolling, return navigation and compact resizing.
The user accepted the hands-on experience; do not repeat the native checklist.
Actual password/MFA recovery and GPU-shell behavior remain unverified limitations. See the [October 11 acceptance update](ACCEPTANCE-2026-10-11.md)
for exact artifact identities and observations. The user approved the supplied Ioskeley
terminal profile on October 10; NRP was explicitly removed from active scope,
with its existing implementation and documentation preserved.

This candidate adds accounting identity/outcome guards, exact overlapping-array
progress and failed-task copying, partial range notifications with checkpoint
deduplication, and honest unresolved states between queue and accounting polls.
Missing submission evidence cannot establish an older attempt. Error explanations
wrap inside the overview pane. Known retries and disjoint stepped memberships
remain usable. The detailed implementation and targeted validation are recorded
in [validation history](VALIDATION-HISTORY.md).

## Private repository and presentation

Source is published to the [private repository](https://github.com/darthnithin/perch).
The README includes a looping workspace GIF and a high-resolution PNG fallback,
rendered from real PTY output with Ioskeley Mono 12 pt and 0.90 line spacing.
The GIF retains 64 samples over 1.28 seconds; the user accepted the slower
curved flight in the reference Terminal profile on October 11. Original detailed Chip assets remain
unchanged. See [media provenance](media/README.md) for reproduction and limits.

The software hover regression now compares panel contents and deliberately crosses
a report-age clock tick. This avoids mistaking a changing age label for navigation
while still checking selection and module opening. The corrected check passed at
48, 80 and 140 columns locally. GitHub verification of source commit `6d67a5c` is
recorded in [run 38170883928](https://github.com/darthnithin/perch/actions/runs/38170883928);
that run completed successfully, including all 48 terminal scripts and six-state
Chip playback.
The draft release notes identify the packaged source, exact archive checks and
runtime coverage. A private draft does not close the native acceptance gates.

## Application validation baseline

`output/validation/preview2-source.json` and the preview-2 reports listed in
[the acceptance update](ACCEPTANCE-2026-10-11.md) bind the current private archives
and runtime results to executable hashes. The earlier
`output/validation/oct11-private-publication-summary.json` applies to preview 1. The earlier
`reconciled-release-summary.json` remains historical evidence, not the identity
of the current draft artifacts.

| Check | Current evidence |
| --- | --- |
| Source checks | Full Go race suite, vet and formatting passed after the reconciliation and explanation-wrapping changes |
| Terminal regression | All 48 scripts passed, including navigation, mouse, workload actions, 5,000-task arrays, copy/activity/cache and attempt regressions |
| Chip playback | All six states, relative cadence, pause, aligned stepping and the slower curved running sample passed locally; emulator render reviewed and native running motion accepted |
| Four archives | Checksums, target architectures, linked docs, original artwork and dependency notices passed; final documentation bundled with the tested executables |
| Mac ARM | Packaged executable passed natively, including CLI/MCP and Hummingbird/NRP demo PTYs; search/sorting screens checked at 140, 80 and 48 columns |
| Mac Intel | Same runtime and screen checks passed through Rosetta on Mac ARM; no claim of native Intel hardware testing |
| Linux x86 | Current packaged executable passed on Hummingbird in job 810653, COMPLETED 0:0 in 36 seconds; temporary stage removed |
| Linux ARM | Exact packaged executable passed CLI, MCP and PTY checks on a native aarch64 GitHub runner, [run 38177035642](https://github.com/darthnithin/perch/actions/runs/38177035642); executable hash matched |
| Experimental NRP | Packaged Mac ARM executable passed local synthetic HTTPS, plugin authentication/recovery and PTY checks; no real NRP namespace tested |

The flight-change source CI [run 38176491104](https://github.com/darthnithin/perch/actions/runs/38176491104)
passed Go/MCP, all 48 terminal scripts and six-state Chip playback. The packaged
source differs from that tested code only in documentation/media.
`output/validation/preview2-publication-summary.json` records the final hashes,
CI results, exact runtimes and private-draft verification.

Source publication to the private repository is authorized; the preview artifacts
below describe the private draft, not a public release. The earlier
`preview-20261010-scheduler` remains unchanged; its 47-script/platform evidence is
in `output/validation/scheduler-release-summary.json`. The current preview
supersedes its source behavior but does not retroactively change those results.

## Latest access checks

- `output/validation/oct11-gpu-capacity-latest.json`: the October 11 13:57 PDT
  read of node-24 still reported all four
  P100 GPUs allocated. No repeat GPU probe was submitted. This is a timestamped
  allocation observation, not a prediction of future availability.
- NRP access is deferred by the user. Existing access-check evidence is historical;
  no context setup or live NRP acceptance is required for this Hummingbird release.

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
| Preserve and refine Chip; animations and size | Approved D/stronger-wing octant rendition, fixed 15×7 footprint, six states, 64 frames each, running samples at 20 ms through a 640 ms curved stroke and other states at 80 ms, one-shot completion, reduced motion. Detailed original assets are checksum-protected in `assets/bird`. Fresh preview test passed. | User accepted the integrated slower curved wing motion with the original hover on October 11. Native review across all states at the reference font/spacing; other terminals/fonts are not certified. Harmonica remains an optional transition tool, not a replacement for approved frame animation. |
| Easy keyboard and mouse navigation | Shared draw/hit geometry, tabs, job selection, help, slash input, global palette, job-specific action menu, log scrolling, compact controls and exit restoration. `tools/test_terminal.py`, `test_mouse.py`, `test_hover.py`, `test_footer.py`, `test_job_actions.py`. | Native pointer/interaction acceptance. Terminal.app uses hover highlights; OSC 22 shapes are enabled only for detected supporting terminals. |
| Connect → inspect → logs → disconnect → reconnect | First-connection setup, system SSH, private multiplexing, batch background reads, foreground authentication, retained snapshots/output, backoff and stale-response rejection. `internal/transport`, `internal/ui`; live backend and synthetic-SSH PTY checks. | Actual interactive password/MFA expiry and recovery in the user's terminal. Fake SSH reconnect tests do not close that gate. |
| Correct Slurm state, arrays, accounting and attempts | Explicit v0.0.44 parser, set/unset/infinite values, logical versus allocation IDs, range/step separation, attempt identity, accounting lag and missing-array summaries. Domain/app/adapter tests and scrubbed Hummingbird fixtures. | Future Slurm schemas remain explicitly unsupported until tested; heterogeneous/federated mutations are refused. |
| Output and submitted scripts | Bounded incremental stdout/stderr, path substitutions, rotation/truncation, control sanitization, failed-read retention, active/historical submitted scripts. Live lifecycle tests recorded in history; fresh retained-output/script read passed. | Files can be missing or inaccessible; no durable remote log retention is promised. |
| Resource charts and useful failure explanations | ntcharts is presentation-only; per-step live samples and gaps, recorded CPU efficiency, peak-RSS scope and missing values remain explicit. Failure hints label scheduler facts separately from log evidence. `test_live_steps.py`, `test_resource_steps.py`, `test_diagnosis.py`; live compute-step job 802458 in history. | Native chart review, wider GPU/multi-node measurement acceptance. GPU allocation is not utilization; task peaks are not summed into total memory. |
| Finish/fail notifications and time-limit warnings | Opt-in macOS bridge, grouped/deduplicated events, bounded private checkpoint, restart and reconnect catch-up. Configurable `/warnings`, `--time-warnings` and saved defaults now share measured remaining time with the UI. Tests cover multiple thresholds, off, changed limits, requeues, failed refreshes, edited preferences, legacy keys and 5,000-job checkpoint compaction. | User confirmed Notification Center delivery and a completion banner under Script Editor after enabling Banners, using the development binary. Failure-banner behavior and other Focus/permission settings are not certified; see the acceptance update. |
| Consistent event presentation | `/activity` consumes the reconciled notification/Chip events: latest 100 per session, explicit observation timestamps, stable selection, exact-attempt inspection, keyboard/mouse and compact geometry. App/UI tests cover startup silence, all outcomes, retained events and restart refusal; real PTY exercises navigation and resize. | Native visual acceptance. Session observations are not a persisted scheduler audit. |
| CLI and agent guidance through MCP | Structured read-only CLI, explicit units/nulls, 15 bounded MCP tools, guides and investigation prompt. Shared query service, SDK and independent stdio protocol checks. Configuration example in `examples/mcp-config.json`. | Individual desktop-client configuration is not claimed. MCP remains read-only; workload actions use separately authorized CLI/TUI paths. |
| Explore Hummingbird and expose useful capabilities | `doctor`, selectable installed-software catalog/prerequisites, partition allocations, pending explanations and estimates, personal wait history, on-demand priority factors, per-node CPU/memory allocations, recorded/live resources. Three partitions and Slurm 25.11.4 were confirmed in the fresh query. | Personal quota currently permission-denied; priority scores are not queue positions or start predictions. Native report review remains open. |
| Move beyond monitoring: workload preparation | Offline resource/script preview, non-submitting validation, explicit one-shot submission, single/multi-node tasks, Open MPI, arrays/concurrency, successful-job dependencies, private submission journal and uncertain-receipt recovery. Production live job evidence is retained in history. | Arbitrary MPI ABI and multi-GPU placement are not certified. No automatic upload, retry or execution of viewed scripts. |
| Allocation shells and cancellation | CLI and TUI review, fresh owned-attempt/node/scope validation, foreground shell return; separate job/task/whole-array cancellation, one-shot execution and honest receipts. Production live CLI/TUI evidence includes two-node CPU selection, Ctrl-C, return and parent survival on job 803078. See [shells](ALLOCATION-SHELL.md) and [cancellation](CANCELLATION.md). | Native shell interaction; GPU shell acceptance and layouts beyond the tested cases. One unmaterialized pending array task is selected through the CLI, not the grouped TUI row. Cancellation receipts are session-local plus exit text, not a durable journal. |
| Preferences, cache and troubleshooting | Explicit saved destination/appearance, preview/apply, optional private bounded snapshots, metadata/raw diagnostic modes, corruption/ownership/type handling, atomic replacement. `internal/preferences`, `snapshotcache`, `diagnostics`, `launch/journal`; process tests. | Cache is display-only, not mutation authority. Logs/scripts/credentials are not cached. Raw diagnostic captures need deliberate review before sharing. |
| Mac/Linux executable and eventual distribution | Four current candidate archives, checksums, doc/artwork comparison, binary target/dependency validation and runtime hash matching. Current Mac ARM, Linux x86 and Linux ARM tested natively; Mac Intel tested under Rosetta. | Repackage current examples with the MIT code license and reserved artwork terms; preserve existing Git history as requested. Source is tracked in the private darthnithin/perch repository. No public release or Homebrew tap is published. |
| NRP (deferred; outside current scope) | Explicit namespace/context, typed Job/Pod/container identity, bounded HTTPS reads and log snapshots, explicit credential handling, CLI and namespace UI. Synthetic TLS/plugin/PTY coverage. See [NRP](NRP.md). | No configured context was found in the fresh local check. Quota reads, exact quantities and partial failures now have synthetic CLI/HTTPS/PTY coverage. On-demand Pod CPU/memory metrics also have synthetic parser, CLI, HTTPS and PTY coverage. Existing experimental code, tests and documentation are retained. No further NRP onboarding or implementation is required for the Hummingbird release. |

## Native review and observed limits

The reference is the user's Terminal.app with Ioskeley Mono 12 pt and 0.90 line
spacing. Perch does not change the terminal's font/profile. The user accepted the supplied
[profile](terminal/Perch-Ioskeley.terminal) and its appearance on October 10.
On October 11 the user confirmed a loaded live workspace, normal quit and
reconnect, and a visible completion banner under Script Editor. Those observations
apply to the development binary recorded in the [acceptance update](ACCEPTANCE-2026-10-11.md).
Actual password/MFA prompts, failure banners and the remaining visual interactions
remain separate checks. The user accepted the integrated slower curved wing motion on October 11.
Other native checks remain scoped below.

1. Run `./bin/perch --demo --no-config` from the source checkout, or
   `./perch --demo --no-config` from the extracted preview. Check Chip's movement,
   border continuity, tab clicks, output scrolling and a smaller window. Use
   Escape to leave input/panels and `q` to quit. Terminal.app uses hover highlights;
   a hand pointer is only expected in a detected supporting terminal.
2. Connect with `--host your-cruzid@hb.ucsc.edu`. Use `c` when sign-in is required.
   Confirm the real password/MFA flow returns to the workspace and that quitting
   restores the terminal. Credentials belong in SSH's terminal prompt, not chat.
3. Completion-banner review passed with `--notify desktop` after Banners was
   enabled for Script Editor. A future failure-banner check should use one
   explicitly owned finite probe and a user-observed banner. Do not repeat
   the completed success check without a relevant delivery change.

The user completed the normal motion/navigation review and accepted the native
experience. The scenarios above describe the review procedure and the limits of
what was observed, not a request to repeat that checklist. These observations
supplement the automated navigation tests. No native app-control
restriction has been bypassed, and a known-good older screenshot does not certify
all current screens or states.

## Open acceptance and implementation work

1. **Native experience accepted.** The user accepted the slower running motion
   and normal keyboard/mouse navigation, output scrolling and compact resizing
   in the reference Ioskeley Terminal profile on October 11. Do not repeat the
   hands-on checklist. Other terminal/font configurations remain uncertified.
   The prior native-control rejection has not been bypassed.
2. **Authentication and alert limits.** Actual interactive password/MFA expiry
   was not exercised; retain that boundary in the verification record.
   Ordinary reconnect and quit passed; completion banners and Script Editor
   identity were observed with the development binary. Failure-banner behavior
   and wider permission/Focus configurations remain unverified.
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
   All four current archive runtimes passed. The user selected MIT code and reserved
   Chip artwork; include those notices in new archives before publication. The user chose to retain the existing repository and history. The user authorized a private GitHub repository and release preparation.
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
