# Validation history

This archive preserves the prior readiness checklist and dated investigations.
Older status statements describe their checkpoint, not the current build.
Use [current readiness](READINESS.md) for the active audit. The original document
is retained verbatim below, including superseded plans and recorded failures.

---

# Readiness checklist

This is the completion audit for the active build goal. A checked item must cite concrete evidence; build success alone is not application acceptance.

## Foundation

- [x] Go build, pinned dependencies, formatting, vet, race-tested unit suite. Evidence: `make check && make build`, October 9, 2026.
- [x] Domain types distinguish jobs, array ranges/tasks, steps, unset values, attempts, and stale snapshots. Evidence: source-and-test audit below, October 10; live workload and presentation gates remain separate.
- [x] SSH and local runners: quoting, private multiplexing, timeouts, bounded output, classified failures, cleanup. Evidence: source/real-process audit and live transport checks below, October 10; remote cancellation limits remain explicit.
- [x] Explicit parser selection and doctor capability report, tested against Hummingbird. Evidence: live integration on October 9 returned Slurm 25.11.4 / v0.0.44, 72 accounting records and 3 partitions.
- [x] Scrubbed real fixtures plus deterministic demo scenarios. Evidence: `testdata/hummingbird`, parser tests, `internal/demo/demo.go`, and the PTY demo workflow. Additional failure/disconnect scenarios remain part of the broader acceptance work.

## Full monitoring workflow

- [ ] Connect and authenticate, inspect jobs/arrays, show details, partitions, history.
- [x] Bounded log reading, rotation/truncation behavior, sanitization, missing/error/stderr handling. Evidence: opt-in `TestLogLifecycleIntegration` passed on Hummingbird; real PTY stdout/stderr checks and domain control-sequence tests.
- [x] Submitted-script viewing from active or historical jobs. Evidence: running smoke job in PTY; active and retained accounting script retrieval in `TestHummingbirdLive`.
- [ ] Live resources and post-job efficiency with correct units/scope/missing values.
- [ ] Refresh ordering, partial errors, disconnection, preserved stale data, retry/backoff, in-TUI reconnect and terminal restoration.
- [x] Live end-to-end checks with Hummingbird jobs; small synthetic test jobs explicitly authorized by the user on October 9. Evidence: `output/terminal/live-results.json`, job 801460, successful real PTY flow through completion.

## Experience

- [ ] Polished wide/narrow layouts, keyboard help, command palette/search, focus and scrolling, clean exit.
- [ ] Approved round Chip sprite, meaningful states, static/reduced-motion mode and terminal color handling.
- [ ] Resource charts backed by actual samples and visible gaps; chart library integration evaluated.
- [ ] Array progress and copying failed-task specification without mutation.
  - Mixed-array progress and failed-task copying, including collapsed ranges, now have CLI/PTY acceptance, clipboard failure/timeout/repeated-input/exit cleanup checks and a read-only live query. Native clipboard acceptance remains open; see the October 10 clipboard and range audits below.
- [ ] Evidence-labelled failure summaries and time-limit warnings.
- [ ] Notification backend, explicit preference, grouping/deduplication, startup baseline and reconnection catch-up.
- [ ] Real terminal screenshots and scripted keyboard/resize/restore checks.

## CLI and platform expansion

- [x] Scriptable read-only CLI with JSON, explicit units, bounded output and failure handling. Evidence: October 10 query/CLI unit tests, full race/vet suite, live queue/history/partition checks, and unchanged PTY navigation checks.
- [x] MCP server and Hummingbird/Perch guidance, built on the shared query service. Evidence: SDK stdio integration, independent JSON-RPC wire check and live Hummingbird MCP queries on October 10; client-specific desktop acceptance remains separate.
- [x] Personal partition wait history in TUI, CLI and MCP, with sample counts and explicit evidence limits. See queue-wait evidence below.
- [x] Installed-software discovery and prerequisite guidance across TUI, CLI and MCP. Evidence: live Lmod fixtures, parser/input/error tests, real TUI SSH lookup and MCP round trips, October 10.
- [x] Single-node CLI launch primitive: offline script/resource preview, non-submitting scheduler check and explicit one-shot submission. Evidence: live job 801781 completed with exit 0, expected stdout/stderr, module loading and 0600 output permissions; race/vet and literal-argument/error tests.
- [x] TUI single-node launch form, review, validation and explicit submission, with draft retention and return-to-job navigation. Evidence: live TUI job 801806 completed with exit 0 and expected output; demo PTY, state/duplicate/stale-result and compact-layout tests.
- [x] Array preparation, concurrency/resource review and successful-job dependencies in CLI/TUI. Evidence: live serial array 801856 and dependent 801858; failed prerequisite 801884 and cancelled dependent 801886; shared parser, preview and compact TUI tests.
- [ ] Distributed workload preparation and additional platform actions.
  - Multi-node/task preparation is implemented and live-tested for independent tasks and a two-node Open MPI collective (801973/801976/801981). Additional actions and wider distributed/GPU behavior remain open; see the topology evidence below.
- [x] Mouse navigation for tabs, job selection, command input, palette and panel scrolling. Evidence: shared render/hit geometry, unit cases across six sizes, real SGR mouse PTY checks including 48×18 and mode cleanup, October 10. Native Terminal.app interaction acceptance remains separate.
- [ ] Native animation refinement across states and terminal/font combinations.
- [x] NRP support feasibility (stretch goal). Evidence: official NRP/UCSC/Kubernetes sources and offline local-tool checks in [NRP assessment](NRP.md), October 10.
- [ ] NRP adapter, namespace-scoped UI/CLI and live acceptance (stretch goal; feasibility is not implementation).

## Delivery

- [x] Bounded private cache/debug diagnostics and explicit raw capture behavior.
  - Submission recovery journal, saved preferences, bounded opt-in diagnostics and optional restart snapshots are implemented. Evidence: the October 10 diagnostics and snapshot audits below; `make check` and the full terminal suite passed with cache restart/isolation cases.
- [ ] Build/release scripts for macOS and Linux, install instructions, doctor/demo usage, license decision, known limits.
  - The `workspace-20261010` candidates include setup, allocation shells and cancellation. All four targets pass the portable runtime checks; Linux x86 also ran natively in Hummingbird job 802838. Mac x86 uses Rosetta and Linux ARM uses the existing OrbStack compatibility environment. License and final native visual acceptance remain open. See the release evidence below.
- [ ] Docs updated to implemented behavior; remaining candidates honestly scoped.
- [ ] Final requirement-by-requirement audit with commands, fixtures, screenshots, and live observations recorded.

Quota views remain a candidate follow-on. Explicit allocation shells are now implemented in the CLI and TUI; tested scope and remaining acceptance gates are in [ALLOCATION-SHELL.md](ALLOCATION-SHELL.md). Capability reporting must explain what is available; no unverified or unsafe feature should be silently advertised. Publishing a release, creating a remote repository, and installing anything on the cluster are not implied by the local build goal.

## Current evidence and next gates

October 9 implementation checkpoint:

- The workspace now uses warm charcoal, a cream/peach hierarchy, aligned job columns, persistent slash-command input, and navigation rather than a decorative bird sidebar. The screenshot harness respects bold text. These changes respond to the user's explicit design corrections; visual acceptance is still pending.
- `make check` runs race tests, vet, and formatting. Parser tests use scrubbed actual Hummingbird shapes; app tests cover accounting lag, initial baseline, requeues, partial refreshes and array-parent/range overlap.
- `tools/test_terminal.py` runs the executable in a real PTY, covers output, scripts, filters, sections, command input/palette, resize and demo reconnect, and checks cooked-mode flags, alternate-screen exit and cursor restoration. PNGs in `output/terminal` are VT-emulator renderings. Native Terminal.app control is blocked by the computer-use tool, so native-window verification is not claimed.
- ntcharts v0.5.2 is isolated behind a rendering-only adapter. Missing samples and long intervals break the plotted segments. A combined live PTY pass on a synthetic Hummingbird job verified measured RSS, CPU time, three samples, chart headings and gaps text. The screenshot emulator now implements CSI S/T/Z scrolling and renders Braille dots explicitly; it no longer leaves stale lines or missing-glyph boxes.
- Failed array IDs can be copied through the local clipboard helper. Specs are sorted, deduplicated and compressed. No task submission/cancellation is implemented.
- `tools/release.sh` creates local macOS/Linux candidates and checksums. Cross-compiled targets still require runtime verification. No release is published and no license has been selected.

Remaining work includes native notification delivery acceptance, bounded private preferences/cache/debug capture, compact Chip placement, and the remaining failure/reconnect/log-rotation acceptance cases. The goal remains active; the current build is not declared release-ready.

Earlier live checkpoint: a completed job returned 768 bytes of retained output and 1,137 bytes of accounting-backed submitted script. The backend test did not print or persist their content. The four local release archives passed SHA-256 checks; both macOS candidates ran `version` successfully. Linux runtime verification remains outstanding; candidates need rebuilding after subsequent source changes.

## Authorized synthetic workload acceptance

The user explicitly authorized small Hummingbird test submissions on October 9. `testdata/monitor-smoke.sh` requests one CPU and 128 MiB with an eight-minute hard limit, writes separate predictable stdout/stderr, and exits after at most four minutes. It installs nothing. The Perch application remains monitoring-only.

- Job 801446 completed with exit `0:0` after 4m01s. Accounting reports 28,012 KiB batch task peak RSS and 4.838 CPU-seconds. Perch observed the running job, growing output, separate stderr, captured script, live charts, reconnect and accounting-confirmed completion.
- The first acceptance harness reached completion but chose an unrelated history entry for its final log check. The harness now selects the exact job ID; the shorter repeat on job 801460 passed every step, including final output and terminal restoration. It completed with exit `0:0` after 2m12s, with 19,804 KiB peak RSS and 2.634 CPU-seconds. This was a harness selection error, not evidence that the smoke job failed.
- A separate check on the latest build verified real interactive SSH suspension, alternate-screen re-entry, restored connection and cooked terminal flags on exit. This uses existing SSH authentication; password/MFA entry and genuine network-loss recovery are still separate acceptance cases.
- `PERCH_LOG_TEST_HOST=ndanday@hb.ucsc.edu go test -count=1 ./internal/hummingbird -run TestLogLifecycleIntegration -v` passed initial reads, incremental appends, truncation, rotation, the 256 KiB limit, missing files, FIFO rejection, and paths containing spaces, quotes, shell syntax and literal `%j`. Only synthetic files in a private temporary directory were changed, then removed.
- Reconnect now invalidates cancelled detail replies while retaining visible data. Regression tests verify that a late reply cannot replace the retained log. Numeric resource parsing rejects NaN, infinity and overflow.
- `make check` and `make terminal-test` passed. Use `-count=1` on live Go tests to prevent cached cluster results from being mistaken for fresh observations.

The temporary stdout/stderr files for these two synthetic jobs were removed after verification. Captured synthetic screens and the check list remain under `output/terminal`; accounting still retains the completed jobs. Clearing a filter also now preserves the selected job, covered by a UI regression test.

Interactive visual review: the user requested both palettes on the same refined layout. `output/terminal/palette-warm.png` and `palette-graphite.png` render real PTY sessions. Wide job rows have more breathing room and an in-panel heading; repeated connection/array summaries were removed from the sidebar. Both themes are available through `--theme`; the user selected A, warm charcoal, as the default. UI tests verify content equivalence and model-local palette behavior.

Palette decision, October 9: the user selected **A — warm charcoal** after comparing both real TUI renders. This approves the palette direction, not the complete interface. Continue interactive review of hierarchy, layout and compact Chip placement.

Title/hierarchy review: after selecting warm charcoal, the user chose title bar and text hierarchy as the next focus. The current proposal uses a spaced `P E R C H` wordmark, aligned in-panel headings, bold active navigation, brighter selected job values, and quieter summary labels. Screens: `output/terminal/typography-wide.png` and `typography-narrow.png`. UI tests and actual PTY wide/narrow rendering/exit passed. The wordmark is provisional pending user reaction; do not treat this as final visual acceptance.

Typography correction: the user rejected the spaced-capitals/header pass as weird and bad. That pass was reverted, preserving the selected warm palette. The user asked to choose fonts before further layout changes. `output/design/font-study.png` compares installed Ioskeley Mono, JetBrains Mono and IBM Plex Mono at equal size/weight. No fonts were installed and no terminal settings changed. Font choice remains pending.

Font exploration: at the user’s request, Xanh Mono Regular was downloaded from Yellow Type Foundry’s repository into ignored `output/design/fonts/xanh-mono`, with its OFL license and italic face. `output/terminal/xanh-mono-wide.png` renders actual PTY output at 18 px with Xanh text and Menlo fallback for terminal symbols. The family has no bold face in the downloaded source; the preview does not fabricate one. No terminal font setting was changed. Selection remains pending.

Design steering: Ioskeley Mono is provisionally selected. The user explicitly deferred the wordmark and requested moving to the next design topic. Continue interactive layout/information-density review using Ioskeley and the approved warm palette.

Top-tabs trial: removed the wide Workspace navigation sidebar, retained all four tabs above content, and kept workload counts at the right when space permits. Jobs and inspector now share the recovered width. Ioskeley/warm preview: `output/terminal/layout-top-tabs-ioskeley-3x.png` (4896×3000, rendered directly at 3× from real PTY output). `go test ./internal/ui` and `make terminal-test` passed, including navigation and 80×24 resize. User visual review remains pending; wordmark remains deferred.

Border rendering investigation: user reports separated vertical strokes in Terminal.app, while their Claude Code screenshot shows continuous rounded borders in the same terminal. Do not attribute this to font/spacing without evidence. Superfile uses Lip Gloss border definitions; btop explicitly defines U+2500/U+2502 and U+256D–U+2570, matching Perch. `tools/borders/main.go` compares plain Unicode, native Lip Gloss, separately styled edges and explicit background, with optional `--tui` for the Bubble Tea renderer. It compiles and emits the comparison; native Terminal pixel comparison remains pending. No border fix is yet verified. The screenshot renderer also uses glyphs with arbitrary line height and cannot establish native-terminal continuity.

Border follow-up: both user-provided native screenshots show seams in the comparison, including direct stdout without Bubble Tea. Thus Bubble Tea is not established as the sole cause. The user noticed a relationship to background painting. Removed redundant per-row canvas SGR backgrounds from Perch; `View.BackgroundColor` still owns the warm/graphite screen background and selection retains its explicit fill. `make check` and `make terminal-test` passed after rebuilding. Native visual verification of this candidate is pending; do not claim the seams are fixed yet.

Native border acceptance: the user confirmed "looks fixed" and supplied a Terminal.app screenshot after the background change. The screenshot shows continuous vertical strokes and rounded corners in the actual Perch demo. Saved as `output/terminal/borders-fixed-terminal-app.png`. This verifies this visual fix in Terminal.app; it does not constitute acceptance of the complete design.

Interactive Chip placement trial: a still compact rendition sampled from the approved idle sprite now sits on the My jobs border, near the left corner, on roomy screens (96×34 or larger). Source PNGs remain unchanged; the branch is cropped and the foot half-cells replace border cells. He is hidden on other views, compact terminals and no-color mode. Animation/state integration and visual acceptance remain pending. Preview: `output/terminal/chip-perched-trial-3x.png`. The preview renderer now draws box strokes to cell boundaries so font fallback line-height artifacts do not misrepresent the confirmed native border fix.

Chip silhouette correction: user rejected the thicker compact conversion. Removed all 2×2 aggregation. The perched renderer now preserves the original idle sprite body crop (x=9..27, y=9..25), one source pixel per terminal half-cell, with the separate branch removed. Verified all 148 opaque pixels and all transparent pixels against actual PTY cells. UI tests passed. This restores pixel shape but increases the header to nine sprite rows including feet; this larger size is a review trial, not accepted design. Preview: `output/terminal/chip-original-pixels-3x.png`.

Smaller Chip review: replaced the oversized source-pixel trial with a deliberately drawn 12×11-pixel idle rendition using the original palette. This is a small variant, not a claim of pixel-identical scaling. It keeps a separate eye, folded wing, beak, feet and tail gaps, and takes six terminal rows including the border overlay. Canonical sprites remain unchanged. `go test ./internal/ui`, build, and PTY render/clean exit passed. Preview: `output/terminal/chip-small-refined-3x.png`; acceptance pending.

Chip scaling correction: user explicitly requested smaller size without altered proportions. Removed the hand-redrawn grid. Current trial uses the canonical idle sprite with the same 1/3 nearest-neighbor scale on both axes and a fixed source sampling phase; only the separate branch is omitted. PNG assets remain unchanged. At this block-cell resolution fine details are visibly lost, so visual fidelity is not accepted or solved. Preview: `output/terminal/chip-uniform-scale-3x.png`. UI tests passed before the sampling refinement; rebuilt binary and real PTY rendering/cleanup passed afterward.

Fractional-block Chip trial, October 10: replaced nearest-neighbor pixel dropping with fitting standard Unicode quadrant and eighth-block glyphs to the original sprite. A 10-column, 5-row footprint uses 2x4 source pixels per terminal cell, retaining uniform scale under the usual 1:2 cell aspect assumption. Original PNGs and geometry are unchanged, but the two-color-per-cell fit approximates detail; increased contrast weighting keeps the eye visible. The separate branch remains omitted. Preview `output/terminal/chip-fractional-blocks-3x.png` is an emulator render of actual PTY output, with explicit Unicode block geometry, not a native Terminal.app capture. Visual acceptance and native glyph rendering remain pending. `make check` and `make terminal-test` passed, including navigation, resizing and terminal restoration; renderer tests cover transparency, small-eye retention, overlay-visible solid cells and cache isolation.

Native fractional-block rejection: the user's Terminal.app screenshot shows broken shapes and visible cell artifacts. The idealized emulator block rendering concealed these problems; passing tests did not establish native visual fidelity. Treat this renderer as unresolved, not accepted. At the user's request, `output/design/chip-placement/index.html` and `all-poses.gif` show all six original sprite animations anchored to the My jobs border for placement review. These are explicitly image-based mockups, not evidence of terminal support. Original assets remain unchanged. The review page includes pose selection, pause/play and reduced-motion preference; JavaScript syntax and exported animated board were checked, but browser interactions were not independently exercised.

Terminal redraw rejected, October 10: the foreground-only half-block experiment lost the approved mascot's character. The user explicitly rejected it. Removed mascot drawing from the main workspace pending an acceptable terminal rendition; do not ship this draft as Chip. Detailed artwork is preserved: all 21 matching PNG/GIF files were verified byte-for-byte against the approved status-set-v2 source directory. The terminal experiment and its review tool remain separate, labeled rejected. Mascot integration remains incomplete.

Chip direction selected, October 10: after native Terminal comparison in a separate Ioskeley profile, the user chose **D — stronger wing shading**, an original-derived Unicode 16 octant conversion. This supersedes the rejected hand redraw. `assets/bird/octant.go` now renders the detailed sprites into a fixed 15-column, 7-row frame with foreground-only colors, a negative-space eye and deterministic weighted palette selection. Original detailed art is unchanged and protected by checksum tests. Seven initial conversion methods plus a stronger-shading variant were studied using actual macOS CoreText glyph rendering; the study exposed font/spacing artifacts hidden by earlier geometric preview rendering.

All six original animations are now integrated on the Jobs border in roomy views. Animation advances independently of SSH refresh; compact, no-color and other tabs omit Chip. Reduced-motion uses static posters. Completion clamps at its final frame rather than looping while waiting for the next clock tick. The standalone `tools/chip-preview` now uses the selected octant conversion; the rejected drawing is archived outside the application package. `make check` and `make terminal-test` passed, and an additional live PTY check verified changing animation frames, removal/return across tab navigation, retained controls and terminal cleanup. All 384 source-state frames have fixed dimensions and one-cell glyph-width checks. An eight-frame wingbeat sheet and both complete six-state animation sheets were rendered and reviewed; the complete workspace was also captured from a running PTY and rendered with Ioskeley/CoreText (`output/design/chip-lab/workspace-d-motion.gif`). These are font renders, not native Terminal captures. Native static shape/shading was selected, but thin line-spacing seams and native animated appearance still need refinement/acceptance; do not claim they are solved.

CLI milestone, October 10: added `jobs`, `history`, `job`, `logs`, `script`, `resources`, `arrays`, and `partitions`, with human-readable output and a versioned JSON envelope. Flags may precede or follow job IDs. These commands use a shared `internal/query` service over the existing backend, preserving user scoping, scheduler precedence, unresolved accounting states, array reconciliation, bounded/sanitized log reads, and explicit missing measurements. Remote identity lookup avoids repeating the full capability probe for every CLI query. This shared service is a foundation for MCP; an MCP server has not been implemented yet.

Live CLI checks returned an empty personal queue, 74 accounting records, and all three Hummingbird partitions. These are point-in-time observations, not stored capacity promises. Offline checks cover every command and JSON serialization, invalid requests before backend access, partial backend failures, stale accounting, control-sequence sanitization, empty arrays, null measurements and output write failures. Terminal regression checks passed navigation, resizing, inspection and restoration after the entrypoint changed. No workload was submitted for these CLI checks. Native Chip animation acceptance and the broader release audit remain outstanding.

Additional CLI acceptance: live `job`, `resources`, and historical `script` succeeded for completed smoke job 801460; `arrays` returned six observed arrays. The deliberately removed smoke-job output produced a nonzero error with empty stdout, as expected. Script content was checked in memory and not logged or persisted. All eight CLI commands also passed subprocess JSON parsing in demo mode, and invalid commands/IDs/options returned nonzero status without stdout. `make check` passed after the accounting-state correction. These checks do not establish interactive credential entry or a live resource sample from a currently running job.


MCP milestone, October 10: `perch mcp` now serves ten typed read-only tools, four embedded Markdown guides/resources, and an `investigate_job` prompt over stdio using the official Go SDK v1.8.0. Tools share the CLI query service. Authentication is deferred until a live tool call so discovery and guides work offline. Cluster operations are serialized with cancellable deadlines; list responses have bounded pagination, logs/scripts have character caps and explicit truncation metadata, and remote output is treated as untrusted data. General query results retain schema/source/time/unit semantics. No arbitrary shell execution or workload mutation is exposed.

Verification: all tools and guides passed an official SDK client running the actual command entrypoint in a subprocess. Tests cover invalid schemas/IDs, partial/error paths, pagination, text truncation, offline guidance, cancellation, queued deadlines and recovery, plus clean EOF exit. `make mcp-test` independently exercises the built binary via newline-delimited JSON-RPC using Python's standard library. Live MCP tools `perch_jobs`, `perch_history`, `perch_partitions`, `perch_arrays`, and `perch_doctor` passed against Hummingbird; no job was submitted. `make check` passed race tests, vet and formatting. Setup is documented in `docs/MCP.md` and an example config; no desktop client's settings were changed or its UI integration claimed. Public guide sources were checked, with dated local capability observations distinguished from guarantees.

Native visual feedback, October 10: the user described the full application at Ioskeley Mono 12 pt with 0.90 line spacing as “quite good.” This is the current Mac Terminal reference profile (`output/design/chip-seams/Perch-12pt-tight.terminal`); no default profile was changed. The screenshot demonstrates a much more continuous running pose but does not establish every animation frame or other font/terminal combinations. At the user’s request, running playback is now 40 ms per frame (twice as fast), with other states retaining 80 ms and reduced-motion unchanged.

Mouse navigation, October 10: added cell-motion mouse capture with `--no-mouse` for native terminal selection. Click targets share the rendered tab, Chip, pane and job-row geometry. Jobs select on click; Enter opens inspection. Wheel input is scoped to the panel under the pointer, and ignores borders, modifiers and unsupported buttons. Command-input drafts survive changing tabs. Palette clicks only execute within its command-column region. Compact lists now reclaim their blank header line at the minimum supported height so context and the footer fit. `make check`, keyboard/resize PTY regression checks, and `tools/test_mouse.py` passed. The mouse test sends actual SGR clicks/releases/wheel events and checks capture enable/disable, focus, commands, selection, scrolling and resize. Reviewed `output/terminal/mouse-minimum.png`; renders remain emulator output, not native Terminal.app captures. No terminal preferences or original Chip assets changed.

Software exploration, October 10: implemented `/software [search]` and `/module name/version`, matching CLI commands, and `perch_software`/`perch_module` MCP tools (12 tools total). Queries discover Lmod modules across compiler families, distinguish login-environment availability, and display prerequisites plus a load command. Metadata lookup never loads a module or executes a returned instruction. Exact names are validated and passed as positional shell arguments; catalog text is parsed strictly and terminal controls are removed. Search is a local substring filter. MCP results paginate and bound module descriptions. Software panes remain independent of job selection/refresh and discard stale responses when navigating away; module prose reflows on resize. Live CLI CUDA search, Singularity prerequisite lookup, seven live MCP tools, and a real SSH-backed TUI flow (including resize, scroll, return and cleanup) passed. `make check`, `make terminal-test`, and `make mcp-test` cover regressions. BeeGFS quota is still permission-denied; no fake quota feature was added. Workload preparation/submission and other broader goal requirements remain incomplete.

Launch primitive, October 10: `perch run` prepares single-node commands with explicit resources, exact shell-quoted arguments, ordered modules and separate log patterns. Default preview does not connect. `--check` checks the existing remote directory and streams the generated script to `sbatch --test-only`. `--submit` first checks, then calls `sbatch --parsable` once with a random comment token. Failed or malformed responses are labelled unconfirmed and never retried automatically; output-write failures after acceptance report the job ID. Hidden SBATCH environment options are cleared. A 1-CPU/128-MiB/1-minute authorized smoke job (801781) loaded GNU 14, printed expected stdout and stderr, completed with exit 0 in one second, and produced owner-only 0600 logs. The scheduler's test-only estimate was much later than the actual backfill start, confirming that it must remain a tentative estimate. Live submitted-script retrieval matched the generated body. Local tests cover literal quote/metacharacter arguments, invalid resource values, path filename-pattern escaping, input bounds, transport errors, no automatic retry, exact receipt parsing and clearing inherited submission options. CLI integration covers offline previews and validation. This does not complete the planned TUI launch form, array/MPI recipes, notifications, persistence or release audit.

TUI launch, October 10: `/run` and the command palette open an editable nine-field form. Enter previews the exact generated script and resources; `v` performs non-submitting validation; Ctrl+s submits only from a fresh validated state. Keyboard repeat cannot submit twice. Editing invalidates validation; cancelled check results are discarded, and ambiguous submission outcomes lock the retry path. The form preserves drafts across closing/tab navigation and keeps focused fields visible down to 48×18. Enter on an accepted receipt requests a fresh snapshot and focuses the allocation in Jobs or History. Demo mode never contacts the cluster. A live real-PTY form submission produced job 801806; Perch opened its inspector, then accounting confirmed COMPLETED/0:0 and the expected stdout. Both output files had 0600 permissions. Reviewed real-PTY emulator screenshots for the form, review and job inspector; these do not establish native Terminal.app acceptance. `tools/test_launch.py` now runs with `make terminal-test`; race/vet and all terminal/MCP regression checks remain required. Native design review, notifications/persistence and wider workload recipes remain outstanding.


Notification tracking, October 10: `--notify desktop` enables opt-in macOS delivery, backed by a fixed AppleScript program with argument-only message data. Baseline history is silent; observed outcomes, cancellation and finite-limit warnings use existing reconciliation. Collapsed array observations can establish short tasks that finish between polls. Array events are grouped and bursts bounded. Private state retains at most 5,000 active identities and 10,000 deduplication keys for eight days, with 4-MiB reads, 0700 directories, 0600 files, atomic save/sync and an exclusive connection-identity lock. Tests cover restart catch-up, duplicate suppression, identity mismatch, expiration, private metadata, corrupt/symlinked state, save failure, delivery failure, burst grouping, stale writes and disabled/demo behavior. Live test job **801833** ran for 15 seconds with 1 CPU and 128 MiB; the test restored its observation from disk and reconciled COMPLETED into exactly one fake notification, with no repeat on another refresh. No actual desktop banners were sent. Native macOS delivery/permission/identity acceptance remains unverified, so the full notification readiness item remains unchecked. File cache caps can omit older/large-queue observations; this is a bounded best-effort alert feature, not a durable external messaging service.

Notification verification follow-up: `make check`, `make terminal-test` and `make mcp-test` passed after wiring the feature. The final notification packages and UI also passed race tests after the metadata whitelist and copy refinements. Apple's compiler accepted the fixed script; an echo-only AppleScript test verified literal argument transport without executing `display notification`. Reviewed `output/terminal/notifications-connection.png` from the real PTY emulator, with demo-specific notification/reconnect copy. This remains distinct from native Terminal.app or native notification-banner acceptance.


Array/dependency launch, October 10: the CLI supports `--array` and `--afterok`; `/run` adds Array tasks and After success. Review shows exact task count, per-task resources and aggregate concurrency ceiling. Expressions are bounded, normalized and checked for duplicate indices; each task gets separate `%A_%a` logs. Dependencies use afterok plus explicit cancellation of impossible dependencies. Array receipts locate an observed array member; manual tab/row changes stop deferred launch focus. Live array **801856** ran tasks 0 and 1 serially under `%1`, and dependent **801858** started after both ended. All three completed and returned expected per-task output through the log adapter.

The first failed-dependency test (801867 → 801869) exposed an accounting-query gap: Slurm cancelled the dependent correctly, but default `sacct` omitted it because it never became eligible. History/probe queries now explicitly select all twelve documented base states within the existing date window; `--state=all` was tested and rejected by this Slurm, so it is not used. A scrubbed real fixture also captured raw NO_VAL32 (4294967294) for the start time; it now becomes unset instead of a date in 2106. The corrected test **801884 → 801886** passed: failed prerequisite, cancelled dependent, outcomes visible through the normal adapter. CLI inspection of 801869 confirmed CANCELLED with null start and zero elapsed. This improves UI, CLI, MCP and restart-notification reconciliation together.

Pending inspectors now explain dependency, impossible dependency, array throttle, priority and resource reasons while preserving the original scheduler code. Unknown site-specific reasons remain uninterpreted. The explanations wrap within the detail panel. Preview captures prefer installed Ioskeley Mono (emulator only; no terminal preferences changed). Reviewed the array/dependency preview and queued inspector; native Terminal.app remains a separate acceptance gate.

Final array/dependency checks: race tests, vet and formatting (`make check`) passed. The terminal suite initially stopped at its old literal `WAITING FOR` expectation after the intentional pending-copy change; the assertion now checks both the new explanation and original scheduler reason. All demo navigation, mouse, launch-form and cleanup checks subsequently passed. Independent MCP wire checks passed. A live real-PTY `/job 801886` opened the cancelled dependent from History and restored terminal state on exit; reviewed `output/terminal/live-cancelled-dependency.png`. The temporary raw accounting captures used for diagnosis were removed after saving the deliberately scrubbed fixture. No changes were made to terminal preferences or notification delivery settings.

Navigation polish, October 10: Ctrl+k now searches command names, slash aliases
and descriptions as you type. Empty matches cannot dispatch an action; filtered
mouse targets, Unicode backspace and bounded/sanitized paste have regression
coverage. User-facing shortcuts show `/run` and `/software`, rather than the
internal action names. Jobs and History preserve independent filters and selected
IDs across tabs. Explicit jumps clear the destination filter while preserving the
previous tab's context; array parent IDs can select an observed member. Manual
selection cancels delayed receipt focus. Typed spaces and pasted words work in
job filters.

Reconnect regression repaired: signing in previously called final shutdown,
which cancelled notifications and released their state lock. Reconnect now only
cancels cluster requests. A fake-delivery lifecycle test verifies that a tracked
job can complete and notify after reconnect, and that final shutdown cancels the
notification context. No real desktop banner was sent by this test.

Navigation verification: `make check` passed race tests, vet and formatting.
Real-PTY keyboard checks passed searchable commands, no-match Enter, independent
tab filters, resize and terminal restoration. Mouse checks passed remembered
selection, filtered palette targets (unit tests), SGR navigation and 48×18
layout. Reviewed `output/terminal/palette-search-narrow.png` and
`palette-search-wide.png` using installed Ioskeley in the emulator. Native
Terminal.app acceptance remains separate. The filter feedback issue found in
this pass is addressed by the status/recovery pass below.

Status and recovery polish, October 10: connection health now has a permanent
status row, with the filter alongside it. At compact widths, age yields space
before state/filter. Recovery keys and error context replace idle command hints;
Connection starts with the error and sign-in guidance before the capability
inventory, and wraps prose to the available width. Partial refreshes retain data
and are visibly distinct from disconnection. Transient clipboard/completion
notices expire after eight seconds. Empty history is only described as empty
after accounting has actually succeeded; background refresh no longer replaces
an empty filter result with a misleading connecting message.

`internal/ui/status_test.go` covers width constraints, retained snapshots,
backoff/cadence recovery, error precedence, partial refresh, notice expiry,
wrapped connection errors and unavailable history. `tools/test_failure_terminal.py`
runs the built TUI in a real PTY with a local fake SSH executable and scrubbed
Slurm fixtures: network failure with retained filtered jobs, automatic retry,
failed and successful interactive login, partial queue failure, initial auth
failure, and terminal mode restoration. It makes no network connection and
does not establish real password/MFA or actual network-loss acceptance.
Reviewed the narrow/wide disconnected and partial-refresh Ioskeley emulator
captures in `output/terminal`. The fake-SSH harness is included in
`make terminal-test`; overall readiness and native acceptance remain open.

Final status/recovery verification: `make check` and `make terminal-test` passed on the rebuilt binary, including existing keyboard, mouse and launch-form regressions plus the new failure harness. No cluster workloads or desktop notifications were created during this pass.

Resource semantics audit, October 10: fixed accounting normalization that had
labelled allocated memory as requested memory. The two are now independent in
the domain, CLI and additive JSON fields. Requested TRES also retains memory
requests for jobs that never started. Recorded RSS peaks retain task/node/unknown
scope instead of uniformly claiming a task peak. Interrupted jobs show a CPU
accounting caveat; restarted jobs omit ratios with ambiguous attempt boundaries.
Negative/missing CPU readings are rejected. The later multi-step audit below
corrects an overly strict check on accumulated microseconds.
Live samples explicitly cover the batch step, display missing CPU readings, and
both failed and successful queries obey the same 90-sample chart-history cap.
No whole-job memory efficiency is inferred from task peaks.

Live read-only checks: `perch resources 801460 --host ndanday@hb.ucsc.edu --json`
reported 134,217,728 requested/allocated bytes, 20,279,296 peak-RSS bytes,
2.634412 CPU-seconds, 132 elapsed seconds and 1.9957667% CPU efficiency.
Independent `sacct --units=K` returned ReqMem=131072K, MaxRSS=19804K for
801460.batch, TotalCPU=00:02.634 and ElapsedRaw=132, agreeing at its displayed
precision. Cancelled-before-start job 801886 now retains its 128-MiB request,
null allocated memory, null peak RSS and null efficiency. Both historical
resource views passed real-PTY navigation and terminal cleanup; reviewed
`accounting-resources-live.png` and `unallocated-resources-live.png`.

Sources: [Slurm sacct](https://slurm.schedmd.com/sacct.html) documents interrupted
CPU accounting and task/node resource scope; [sstat](https://slurm.schedmd.com/sstat.html)
defines the live fields. Parser/domain/query tests exercise distinct allocations,
never-allocated requests, node/unknown peaks, invalid ratios, restart/interruption
caveats and exact batch-step matching. UI tests exercise repeated failed samples
and recovery. `make check` and `make mcp-test` passed. Full resource readiness is
still open for broader step-level/multi-node coverage and final chart acceptance.

## Multi-task and distributed launch evidence · October 10

The CLI and TUI now share node count, tasks per node and launcher selection.
CPU requests are per process; memory and GPU requests are per node. Every array
element gets the whole allocation. The review shows total resources per
allocation and across the maximum concurrent array elements. Zero/invalid
topology, incompatible direct launch and aggregate memory overflow are rejected
before connecting. `auto` resolves to direct or independent `srun` execution;
Open MPI is explicitly selected with `mpirun`. No implicit application upload,
compilation or GPU rank assignment occurs.

`TestHummingbirdTopologyIntegration` passed uncached in 77.58 seconds. It created
the private synthetic workspace `/scratch/ndanday/perch-topology.SCdQKCOl` and
submitted three one-shot jobs, all confirmed COMPLETED through accounting:

- **801973:** two independent tasks, IDs 0 and 1, on one node.
- **801976:** compute-node compilation using GNU 15.2.0 and Open MPI 5.0.10.
- **801981:** two ranks on distinct nodes, both confirming an MPI_Allreduce sum
  of 1 and communicator size 2; accounting confirmed two allocated nodes.

Each phase requested 256 MiB per node and one CPU per task, with one-minute
execution limits and a two-minute compilation limit. Sources and logs remain
in the private scratch directory for inspection. The test did not install
software or compile on the login node.

Reviewed `output/terminal/launch-multitask-review.png` and
`launch-topology-minimum.png`: topology/resource totals are visible in review,
and the focused launcher field remains accessible at 48×18. These are real-PTY
emulator renders using Ioskeley, not native Terminal.app acceptance. Tests cover
offline CLI previews, literal launcher arguments, invalid CLI/form inputs,
array resource multiplication and compact navigation. The form now has fourteen
fields; the earlier nine-field observation describes the original launch pass.

This verifies the small supported CPU/MPI launch flow. It does not close the
broader resource-monitoring, native design, GPU placement, additional platform
actions, durable preferences/submission receipts, or final release gates.

Launch form polish: the active field's guidance now remains visible below the
command bar even when its input has a default value. All fourteen field hints
fit at 48 columns, and validation errors use that same visible feedback area;
they previously could fall below the clipped form panel. A compact field count
in the panel header shows progress. A regression initially caught an overlong
dependency hint; it was shortened and the full size matrix then passed.

Final topology/form checks: `make check` passed (race tests, vet, formatting).
The complete `make terminal-test` suite passed, including keyboard, mouse,
launch and simulated transport recovery. After the final dependency-hint text
change, the binary was rebuilt and the launch PTY check and `make mcp-test`
passed again. Reviewed the compact form and minimum-size launcher render after
the guidance change. No native terminal settings or notification preferences
were changed.

## Durable submission recovery · October 10

Production CLI/TUI submission now writes a recovery token before contacting
Slurm and records the accepted ID afterward. `perch receipts [--host HOST]
[--json]` and TUI `/receipts` read these records offline, including while the
cluster is unavailable. No saved record is treated as a live job or an execution
outcome. Destinations are the exact SSH arguments used, not verified permanent
identities; old aliases must be interpreted with that limitation. No automatic
reconciliation or resubmission occurs.

The journal stores destination, name, timestamps, token and accepted ID/cluster;
it excludes scripts, commands, paths, logs and credentials. It uses owned private
files/directories, no-follow directory-relative operations, bounded locking,
atomic replacement and file/directory sync. The byte limit is 1 MiB and the
record limit 1,000. Accepted records are evicted oldest first at capacity;
unconfirmed records are never automatically discarded. An all-unconfirmed full
journal blocks further submissions. One reserved staging file under the lock
bounds interruption residue and is recovered on the next valid write.

Evidence: journal tests cover restart, concurrent writers, immutable accepted
IDs, capacity, corruption/oversize, symlinks/FIFO/public permissions, lock
cancellation, partial staging recovery, and save failures before/after transport.
A final reply is persisted using a bounded context independent of the expired
network context. UI tests ensure an accepted ID remains accepted even when its
final save fails; failure before submission remains editable and is not called
an uncertain remote attempt.

`tools/test_receipts.py` uses the real executable with fake SSH and an isolated
HOME/state directory. The fake scheduler verifies intent is already durable
before receiving the request. Tests cover accepted replies, lost replies, an
actual SIGKILL while waiting for a reply, and independent-process receipt reads.
Exactly three synthetic transport attempts occurred, with no retry. The TUI
then reopened saved receipts while disconnected, scrolled at 48×18, and restored
the terminal. Reviewed the warm-color real-PTY emulator screenshots
`output/terminal/receipts-recovered-offline.png` and `receipts-minimum.png`.
No cluster job was submitted for these tests; native Terminal acceptance is not
claimed. Existing historical jobs are not retroactively journaled, and broader
preferences/cache/debug/release gates remain open.

Verification for this pass: `make check`, the full `make terminal-test` suite,
and `make mcp-test` passed. After the final viewport/receipt guidance edits,
the receipt and launch PTY workflows passed again; the final receipt mouse-wheel
and viewport-restoration regressions passed under the UI race suite. The current
binary was rebuilt and Linux amd64 compilation passed. Linux runtime acceptance,
native terminal review, and automatic token reconciliation remain unverified or
unimplemented as stated above.

## Array notification catch-up across accounting gaps · October 10

The previous reconciler only recognized a collapsed array range from the
immediately preceding visible snapshot. Once an empty successful queue refresh
removed it, later individual accounting outcomes no longer qualified as observed
tasks. The store now retains unresolved membership separately as arithmetic
progressions, keyed by array ID, cluster, user, submission time and restart count.
Checkpoint persistence retains that evidence across restart; old checkpoints
can seed it from their active ranges. Empty queue data is never terminal evidence.

Confirmed task IDs are subtracted in sorted batches. This also fixes a newly
identified replay risk: retaining resolved membership could otherwise re-notify
old outcomes after more than 10,000 deduplication keys were checkpointed. A
10,005-task regression now confirms no replay after restart despite key eviction.
Range membership never rounds over gaps, and no task is allocated in memory
merely because its index lies in a large range.

Bounds: eight days after the last queue observation of an array attempt;
512 attempts, 8,192 total progression segments and 1,024 per attempt. Invalid or
insufficient identity, capacity and excessive fragmentation produce a visible
limited-coverage notice and Connection explanation. Confirmed outcomes in the
current batch remain available even if retaining future membership hits a bound.
This is a bounded best-effort notification mechanism, not guaranteed delivery.

Evidence: deterministic store tests cover multiple empty polls, partial history,
JSON checkpoint/restart, mixed success/failure/cancellation, stepped-range gaps,
cluster/user/submit/restart identity, requeued attempts with retained old history,
expiry, checkpoint isolation/validation, billion-index range compaction and
fragmentation bounds. A seeded randomized test compares progression unions with
enumerated small-set membership. The private notification session test saves a
disappeared range to disk, reopens it, groups three delayed completions into one
fake message, and verifies no repeat after another restart. UI tests check the
coverage warning. No actual notification banner or cluster job was sent.

`make check` and `make mcp-test` pass on the updated reconciler. The terminal
navigation/mouse/launch/recovery/receipt suite passed during this pass; final
terminal verification after resolved-membership subtraction is recorded below.
Native banner acceptance and the broader domain/array-progress audit remain open.
At this point a completely disappeared collapsed array could still be temporarily
absent from Jobs. The subsequent display pass below addresses that gap without
presenting retained notification evidence as a synthetic job outcome.

Final verification: the rebuilt application passed the complete terminal suite
after range subtraction and fragmentation limits were in place, including
keyboard navigation, mouse handling, launch review, connection recovery,
submission-receipt restart recovery and terminal cleanup.
The full terminal regression suite also passed after these changes. No new jobs
were submitted; the live checks reused completed synthetic test workloads.

## Visible accounting gaps · October 10

Jobs now retains a navigable array summary when a previously observed total
exceeds current queue/accounting records. The view explicitly reports a minimum
missing count and awaiting-accounting status. This uses the observed high-water
total, not inferred task membership or outcomes. The real scheduler snapshot and
notification event processing never receive the presentation row; it counts as
zero additional scheduler tasks. Logs, scripts and resource actions lead to an
explanation instead of issuing commands with a synthetic ID. Enter, `/job`, tabs,
filtering and scrolling work with the summary. Individual accounting parent
records no longer count as task outcomes.

Metadata is scoped to array ID, user, cluster and submission time; task restart
counts can differ within an array. A new observed submission resets the baseline.
Fully confirmed arrays retire it, so expiration of history does not recreate an
awaiting row. Display retention is bounded to 512 arrays and eight days after the
last queue observation. Reaching capacity produces a notice and Connection
explanation; current scheduler records remain visible. Display totals and the
coverage warning survive the existing private notification checkpoint. This is
not yet a general cache for sessions with notifications disabled.

Tests cover repeated empty polls, partial accounting and queue failures, JSON
checkpoint/restart, completed-array retirement, ID reuse, capacity/expiry,
parent records, and a four-billion-index range represented in under 2 KiB of
checkpoint data. UI tests cover `/job`, task-action guards, History separation
and 140/80/48-column layouts. `tools/test_array_gap_terminal.py` drives the real
binary with synthetic SSH, checks row retention/recovery, inspects and scrolls
at 48×18, and verifies terminal cleanup. No live workload or desktop notification
is used. Reviewed emulator renders from real PTY output at
`output/terminal/array-awaiting-accounting-{wide,narrow,narrow-scrolled}.png`;
removed repeated headings so the missing count is visible at minimum size.
These are not native Terminal.app acceptance screenshots.

The broader array/attempt domain audit and general cache remain open. In
particular, the display count is only an observed lower bound; Perch does not
claim to know unseen task IDs or a submitted total without scheduler evidence.

Verification: `make check` passed, followed by the affected UI/app/domain race
suites after the final layout edits. The complete `make terminal-test` suite,
`make mcp-test`, final `go vet ./...`, and formatting checks passed. The final
48×18 render shows both awaiting status and the missing count without scrolling;
End reaches the remaining guidance. No native terminal settings were changed.

## Explicit saved preferences · October 10

`perch config show [--json]`, `path`, `set KEY VALUE`, `unset KEY`, and `reset`
now provide offline preference management. Supported defaults are SSH host,
warm/graphite palette, reduced motion and mouse capture. Remote CLI queries,
MCP startup and workload plans use the saved host. Explicit flags take precedence,
including false boolean values; `--no-config` bypasses saved settings. Help,
version and configuration-path inspection work even with a malformed file.
CLI receipts retain their explicit all-destinations/default behavior. Flags do
not silently persist; notifications, local execution and submission switches
are deliberately absent from the saved schema.

Storage is schema-versioned, bounded to 16 KiB, and created only by explicit
edits. The directory/files must be owned and private, with directory-relative
no-follow operations, a two-second advisory-lock bound, one reserved staging
file, atomic replacement and file/directory sync. Duplicate and unknown keys,
future versions, malformed/oversized/empty files fail visibly. An explicit reset
can replace malformed data but cannot bypass ownership, permissions or file-type
checks. No scripts, credentials, workload data or terminal font settings are
stored here.

Unit/race tests cover default read without state creation, four saved values,
unset/reset, explicit false and bypass precedence, concurrent edits without
lost values, stale staging recovery, cancellation, symlink/FIFO/directory/public
file rejection, bad schemas and offline recovery. `tools/test_preferences.py`
uses the real binary in isolated HOME/config directories with fake SSH. It
verifies saved destinations for queries and offline launch previews, TUI palette
and mouse mode after restart, flag overrides without persistence, broken-file
bypass and reset, and terminal cleanup. Tests do not change the user's settings
or connect to Hummingbird.

Reviewed `output/terminal/preferences-saved-graphite.png` and
`preferences-warm-override.png`, rendered from real PTY output. This exposed and
fixed a screenshot-emulator omission: canvas colors set with OSC 10/11 now
render correctly instead of always falling back to warm charcoal. The preferences
workflow checks the graphite canvas pixel as well as the application's ANSI
palette and mouse-mode output. These remain emulator renders, not native
Terminal.app acceptance. Warm remains Perch's built-in palette.

General monitoring cache, debug capture and interactive TUI preference editing
remain open; this pass implements explicit persistent defaults only.

Verification: `make check`, the full `make terminal-test` suite and `make
mcp-test` passed. The final CLI output-error handling change passed the CLI and
preferences race suites and vet; the rebuilt binary passed preferences process/
PTY checks again. Linux amd64 compilation succeeded; Linux runtime acceptance
remains separate. Final preference screenshots use clock-adjusted synthetic
queue fixtures so their elapsed times stay meaningful during visual review.

## In-workspace appearance controls · October 10

`/settings` and the command palette now open a keyboard/mouse appearance form.
Palette and Chip-motion changes preview immediately; Escape or clicking a
workspace tab restores the original choices. Mouse capture remains usable while
editing and changes only on apply. A single save transaction changes the three
appearance fields while preserving the latest configured host, including a
concurrent CLI edit. Demo and `--no-config` apply only for the session. Save
failures remain visible in the form; duplicate saves and stale replies are
guarded. Terminal fonts and line spacing are never changed.

The wide layout pairs the controls with a live Chip preview. The 48×18 layout
keeps all four controls visible, with complete keyboard instructions. Rendered
control positions drive mouse hit testing. Motion scheduling now tracks one
pending frame timer, preventing rapid pause/resume from creating parallel
animation loops. No-color sessions retain text controls and explicitly explain
why color/artwork preview is unavailable.

Evidence: UI tests cover preview/cancel, single-timer scheduling, atomic save
with destination preservation, failed save recovery, duplicate/stale saves,
demo/session-only behavior, rendered mouse targets across 140/100/80/48-column
sizes, tab cancellation and no-color output. The real-process preferences PTY
workflow covers command/palette entry, live preview, cancellation without disk
writes, saving via mouse at 48×18, restart persistence, motion enable/freeze,
demo-only application and quitting directly from Settings. Terminal cleanup
includes OSC foreground/background resets after a palette preview.

Reviewed warm, graphite/animated and minimum-size emulator renders:
`output/terminal/settings-warm-preview.png`, `settings-graphite-animated.png`,
and `settings-minimum.png`. The compact footer was shortened after review to
keep Escape visible. This is real PTY/emulator evidence, not native Terminal.app
acceptance. All preferences tests use temporary isolated files; the user's
actual settings remain unchanged.

`make check`, the complete terminal suite and MCP wire checks passed during
this pass. Follow-up UI race tests cover tab/no-color behavior; final settings
PTY verification checks the additional motion and demo exit paths. General
monitoring cache, debug capture and the larger readiness audit remain open.

## Personal queue-wait history · October 10

Implemented `/waits`, `perch waits [--json]`, and read-only `perch_waits` MCP.
Statistics cover usable starts within seven days of the accounting snapshot,
grouped by partition: median and nearest-rank P90 for submit-to-start and
eligible-to-start, independent eligibility sample counts, array-task counts and
excluded-record count. Missing eligibility remains null. Restarted jobs,
duplicate attempts, array aggregates, unstarted/invalid/out-of-window records
are omitted. These are observed waits, not predictions or equivalent-resource
comparisons; large arrays can dominate the sample.

The TUI caches analysis by successful accounting snapshot time and reuses the
normal refresh coordinator. No additional polling was added. It labels snapshot
time and older data, supports scroll/resize/mouse/palette/Escape, and keeps
unavailable accounting distinct from a valid empty cohort. CLI/MCP require
successful accounting but can work when the queue portion fails.

Live read-only verification at 03:54 PDT returned 67 usable starts in 128x24
(48 array tasks) and 18 in 256x44 (16 array tasks), with two exclusions. Observed
submit-to-start medians were 654 and 4047 seconds; eligible-to-start medians were
611 and 4004 seconds. This account's recent synthetic tests materially influence
these values. No new job was submitted for this feature.

Race/vet checks cover statistics, zero waits, missing/invalid eligibility,
restarts, future/old timestamps, aggregates, duplicate identities, unavailable
accounting, control sanitization, snapshot aging and compact navigation. The
independent MCP wire test checks the new tool's structured result. Real PTY
checks exercise CLI JSON, keyboard and pointer navigation, 48×18 scrolling,
resize and terminal cleanup. Renders live under `output/terminal/waits-*.png`;
these are emulator renders from application output, not native Terminal.app
acceptance. Broader release gates remain open.

Final visual review tightened the 48×18 view so its first screen shows a
partition and both wait measurements, rather than only introductory copy. The
updated PTY test asserts those values are visible, then verifies wheel/End
scrolling through the evidence notes. Final UI race tests, vet and Linux amd64
compilation passed. Linux runtime and native Terminal acceptance are not implied.

## Opt-in private diagnostics · October 10

Implemented `--debug` across workspace/query/doctor/MCP/run entrypoints and
separate `--debug-raw` for scheduler JSON responses. Default metadata records
fixed operation names, durations, exit/error classification, byte counts,
requested parser/application version and random session identity; it omits
argv, host/user/path fields, arbitrary error text and response content. Raw mode
captures only JSON squeue/sacct/sinfo/sstat responses, never explicit script/log
reads, submission stdin/output or interactive login. Scheduler JSON can itself
contain private job commands or other metadata, so raw captures require review
before sharing. Captures are not automatically uploaded or fixture-ready.

Metadata and raw modes have separate cache directories, each retaining two
2-MiB JSONL files plus an empty lock file (8 MiB combined maximum created by
Perch). Directories/files are 0700/0600; no-follow directory-relative access,
regular/owner/private/single-link checks and a 100-ms lock deadline guard writes.
Raw bytes use base64 and a combined 256-KiB cap per result, with original lengths
and truncation explicit. Malformed batch framing produces metadata only because
the underlying transport discards the invalid frame. Crash-interrupted JSONL can
have a partial record. A write failure stops capture and appears in Connection
and stderr, while preserving the actual command/submission outcome. Startup
capture failures stop before cluster access. Flags are not saved preferences.

Verification: race-tested default redaction, raw byte fidelity/selection,
truncation, bounded rotation, concurrent writers, lock deadline, closed-store
behavior, unsafe files/directories/hardlinks/FIFOs/symlinks and oversized files.
Actual CLI/fake-SSH processes verified opt-in-only creation, directory separation,
private permissions, malformed parser evidence and JSON stdout. Actual TUI/PTY
verified debug status plus interactive SSH suspension/re-entry; actual MCP
handshake/query verified diagnostic stderr does not contaminate protocol stdout.
The existing receipt process test now runs with raw diagnostics enabled, including
lost responses and SIGKILL, and checks that submitted command content never enters
capture files and no automatic retry occurs. No real cluster job or raw user data
was captured for these tests.

Reviewed `output/terminal/diagnostics-connection.png`, an emulator render of real
PTY output. Full race/vet and terminal suites, independent MCP wire checks and
Linux amd64 compilation passed. Native acceptance and the broader release goal
remain open; general snapshot caching is still separate work.

## Job-attempt audit fixes · October 10

Three failing regression cases reproduced real defects: accounting slice order
could select an older same-ID record; disappearance of an observed restarted or
reused ID could expose an old completion as its current outcome; and an in-flight
log/script/resource reply could survive a restart with the same displayed ID.
The store and point query now compare scoped submit/restart identities, retain
UNKNOWN for unconfirmed newer attempts, preserve prior confirmed terminal state
against regressing accounting, and allow records to age out normally. TUI refresh
invalidates detail content/cursors/samples and request sequences on attempt
changes. JSON adds `restart_count`; human inspectors show nonzero counts.

A fourth regression showed an earlier failed array attempt double-counted beside
a newer pending range. Summary inputs now exclude superseded task attempts while
keeping historical records visible. Retained newer observation membership also
prevents the old failure from resurfacing during empty polls. The same filtering
feeds lower-bound gap counts; CLI/MCP array queries use Store summaries too.

Tests cover reversed history order, new submit times, equal-submit restart
counts, three empty polls with old accounting, delayed completion of the new
attempt, no duplicate/old outcome event, terminal accounting regression and
normal expiry, scope/missing-time comparisons, stale detail replies in all three
views, compact-range accounting gaps and shared array query results. Actual PTY
and CLI tests use the real Slurm parser and SSH adapter with synthetic fixtures;
they verify UNKNOWN survives older history and CLI selection is order-independent.
Reviewed `output/terminal/requeued-awaiting-accounting.png`, an emulator render.
No live job was requeued or otherwise mutated for this test.

These cases improve the foundation but do not close its full audit: mixed array
requeue submit times, conflicting/ambiguous accounting identities, exact range
union semantics and broader unset-value coverage still need verification. Native
visual/interaction acceptance and the other release gates remain open.

Final verification for the attempt fixes: `make check`, `make mcp-test`, full
`make terminal-test` (including the new attempt scenario), and Linux amd64
compilation passed. Current local binary rebuilt. This is regression evidence,
not native Terminal acceptance or completion of the remaining foundation audit.

## Live mixed-array requeue investigation · October 10

The opt-in `TestHummingbirdMixedArrayRequeue` uses only its newly submitted,
serial two-task array (one CPU, 128 MiB, two-minute task limit, `/dev/null` output).
Its failure/timeout cleanup cancels that exact synthetic array, never other jobs.

- Array 802120 exposed a real parser failure: accounting returned collapsed task
  membership as `0x2`. The test stopped and its cleanup cancelled the array;
  a read-only accounting query confirmed CANCELLED records.
- After bitmap decoding was added, array 802126 ran successfully. Task 0 was
  requeued exactly once; task 1 completed normally. The restarted task's submit
  time changed from 11:19:04Z to 11:19:39Z while its sibling retained 11:19:04Z.
  Both completed with one recorded restart on task 0. No test jobs remain active.
- The original harness passed its final-outcome assertion but its intermediate
  logs exposed two wrong totals: stale pending accounting ranges inflated 2 to
  4, and mixed submit times then reduced 2 to 1. A passing final assertion alone
  did not validate this workflow. The harness now checks intermediate totals too.
- Reconciliation now treats pending accounting ranges as coverage, connects
  observed retry identities, preserves siblings/minimums through checkpoint
  restore, and requests duplicate accounting records for cold-start evidence.
  Tests cover both queue/history orders, same raw-member/scope requirements,
  bitmap-only gaps, and scrubbed final records from the actual synthetic job.
- The TUI omits post-job CPU-efficiency caveats while a job is pending/running;
  completed requeued jobs still explain why their efficiency is unavailable.

Captured metadata logs: `output/validation/requeue-live.txt` (initial parser
failure) and `requeue-live-bitmap-fixed.txt` (successful workload with the
intermediate defects). Scrubbed source fixtures and provenance are documented in
`testdata/hummingbird/README.md`. Tests reconstruct intermediate queue phases;
they do not claim to be recordings of those raw queue responses. Native visual
acceptance and the remaining readiness gates remain open.

Verification of the mixed-requeue changes: full `make check` and MCP wire tests
passed, followed by race tests for the final app/parser edits and targeted vet.
The full terminal run passed emulator, demo, mouse, launch, failure recovery,
receipts, array gaps, preferences, wait history and diagnostics. Its new attempt
scenario initially expected the wrong empty-state wording and an immediate
accounting refresh; the harness was corrected to respect the 60-second accounting
cadence. The final attempt PTY/CLI run passed, including intermediate UNKNOWN,
cold-start requeue progress, and final 2/2 outcomes. The reviewed emulator render
is `output/terminal/mixed-array-requeue.png`; it is not native Terminal acceptance.
A read-only query with the rebuilt production CLI independently reported array
802126 as observed=2, completed=2, pending=0, unresolved=0. Linux amd64 compilation
passed. The entire foundation and release checklist is not yet complete.

## Bounded job summary and reachable full inspector · October 10

The prior At a glance panel reused and cropped the complete inspector. At common
terminal heights, pending explanations and resource caveats pushed array progress
and navigation below the visible box. It now has a height-aware summary: identity,
lifecycle status, array progress/failures/unconfirmed counts, then resource
context as space allows. A persistent Enter/click link opens the complete
scrollable inspector; `--no-mouse` advertises only the keyboard action. Even a
96×18 split layout retains its link. Returning preserves the selected job.

The full inspector wraps long names and field values at grapheme boundaries,
and removes decorative blank rows/rules at narrow widths so small windows show
useful content sooner. Complete evidence and metrics remain in this inspector;
the summary does not discard them from the model.

`internal/ui/glance_test.go` verifies bounded rows/cell widths, retained progress,
render-located mouse targets at five sizes, selection-preserving return, disabled
mouse/border behavior and lossless wrapping of long values. `tools/test_glance.py`
uses the real PTY to verify pointer entry, 48×18 scroll-to-end, narrow/wide resize,
96×18 entry, queued/history views and cooked-mode restoration. Reviewed emulator
renders: `glance-running-wide.png`, `glance-queued-wide.png`, `glance-short.png`,
`glance-history-wide.png`, and `inspector-minimum-scrolled.png` under
`output/terminal`. These do not replace native Terminal.app visual acceptance.

Verification: `make check` and the complete `make terminal-test` passed. Final
compact-summary and mouse-hint refinements also passed UI race tests, the targeted
PTY workflow and vet; the binary was rebuilt. Linux amd64 cross-compilation
passed. Near-limit remaining time and failed/unconfirmed array counts retain
their warning colors. No cluster queries or job mutations were needed for this
UI pass, and the Chip source artwork and proportions were unchanged.


## NRP feasibility · October 10

[The assessment](NRP.md) records source-backed access requirements, local
kubectl/plugin/context evidence, a provider-specific product/adapter design,
and implementation/acceptance gates. No authentication, settings change, package
installation or cluster operation was performed. Local kubectl exists but the
effective configuration has no contexts and the documented OIDC plugin is not
on PATH. Live NRP acceptance therefore needs onboarding later; offline adapter
work and Hummingbird work remain possible. The broader goal remains active.


## NRP parser foundation · October 10

Added `internal/nautilus` with namespace/cluster/UID identity, separate Job/Pod/
container models, terminal-condition-based Job outcomes, controller-UID grouping,
compact indexed progress and current/previous container state. Synthetic fixtures
cover retries, replacement Pods, OOM, image pulls, unschedulable Pods, init and
sidecar containers, completion targets, deletion and reused names. Validation
rejects malformed/ambiguous envelopes, cross-namespace objects, duplicates,
contradictory states and invalid counts/ranges; unknown fields are tolerated.
Pages carry continuation metadata and are byte/item/depth bounded. Full
`make check` passed; bounded parser fuzzing passed. Evidence is in
`output/validation/nautilus-parser-{check,fuzz}.txt`. These are offline parser
tests only: no adapter runner, CLI/UI, auth, metrics/log retrieval or NRP live
acceptance is claimed. The NRP implementation checklist remains unchecked.


## NRP list adapter · October 10

`internal/nautilus/client.go` consumes the parsers through an injected page
reader. Lists require explicit cluster/namespace scope and retain one consistent
resource version across all pages, bounded by deadline, bytes, objects, pages
and token length. Duplicate identities/names, loops, expiry, malformed pages or
read failures return no partial collection. Jobs/Pods have independent outcomes
and timestamps. Tests cover empty and multi-page lists, encoded opaque tokens,
namespace validation, partial permission failure, cancellation/late responses,
all limits and fresh-list recovery after expiry. `make check` and targeted fresh
behavior tests passed; see `output/validation/nautilus-list-{check,behavior}.txt`.
This is fake-reader acceptance, not fake-command or live-cluster acceptance.
Actual credential-safe transport, CLI/UI integration and stale-data presentation
remain open, so the NRP implementation readiness item remains unchecked.


## Collapsed array count audit · October 10

Found and fixed additive counting of overlapping components in a collapsed
range. For example, `0-9,5-14,0-12:2` now represents 15 distinct tasks rather than
27. The exact compact counter handles different strides, repeated components,
large integer intersections and input reordering without expanding every task.
It validates balanced brackets, positive concurrency/stride syntax and reserved
Slurm indices. Queue validation happens before stripping `%N`, so malformed
suffixes cannot be hidden by normalization. Bounds on bytes, work and terms
return errors rather than guessed counts. The app's task-index bound now shares
the domain constant.

Evidence: 3,000 seeded comparisons to a separate enumerated-set oracle (also
shuffled and duplicated), billion-index cases, overflow-boundary intersections,
malformed input and work-budget tests; queue parser tests; checkpoint/empty-poll/
completion reconciliation proving no phantom unresolved tasks. A 15-second fuzz
run passed over 1.5 million executions. One two-progression, four-billion-index
benchmark measured about 1.34 microseconds and 464 allocated bytes per count on
this Mac; this is a specific synthetic benchmark, not a general performance
promise.

`make check`, rebuild, real-PTY/CLI array-gap and attempt/requeue tests, and MCP
wire checks passed. Reviewed `output/terminal/array-overlap-distinct-tasks.png`:
15 queued, row count 15, and 0/15 progress. The fixture uses synthetic SSH and no
cluster workload was submitted. Screens are PTY emulator renders, not native
Terminal.app acceptance. Logs are `output/validation/array-union-*.txt`.

This audit closes the within-expression duplicate-count bug. The broad domain,
array experience and final readiness checkboxes remain open for their other
identity, state, visual and acceptance requirements.


## Release packaging and runtime audit · October 10

Fixed missing documentation in release archives: candidates now include all
README-linked project docs, MCP configuration example, and original Chip assets.
Temporary staging avoids retaining old files from a previous candidate. USTAR
format plus disabled macOS copy metadata eliminates AppleDouble files and
extended-attribute extraction warnings. Checksum manifests contain portable
basenames. Nothing was published or installed.

Added `tools/test_release_archives.py` for exact target/checksum, member type,
executable mode, internal documentation-link and original-artwork verification.
Added the standard-library-only `tools/test_release_runtime.py`, which accepts
an extracted binary, isolates HOME/config/cache, checks its version/architecture/
hash, exercises five offline CLI queries and the full MCP wire suite, then runs
real PTY navigation, output, software lookup, 48×18 resize/help, exit and terminal
mode restoration with mouse capture enabled and disabled. The MCP harness now
accepts `--binary` and ignores saved configuration.

| Candidate | Runtime evidence |
| --- | --- |
| macOS ARM64 | Passed on this ARM Mac |
| macOS x86-64 | Passed through the Mac's existing x86 execution support |
| Linux ARM64 | Passed in existing OrbStack Linux, using the course image's x86 userland test driver |
| Linux x86-64 | Passed under OrbStack emulation and on a native Hummingbird compute node |

Both Linux container tests used the already-installed course image, `--pull
never`, no network, read-only project/root mounts and temporary filesystems. No
VM, image, package or plugin was installed. Test containers exited and were
removed. Python's reported machine type in those containers reflects the x86
userland driver; each tested executable's ELF architecture and SHA-256 were
independently checked. Emulation is not claimed as native hardware acceptance.

For native Linux acceptance, Perch's actual `run --submit` workflow submitted
**802233**, one CPU / 256 MiB / two-minute limit, to execute the same portable
harness from a private temporary home directory. Accounting confirmed COMPLETED,
exit `0:0`, elapsed five seconds. CLI, MCP and both PTY modes passed with empty
stderr. The tested binary hash matches the Linux x86 candidate. The test binary,
harnesses and logs were removed from Hummingbird after capturing the bounded
synthetic result; directory removal was verified. No unrelated job was touched.
This does not grant or verify persistent login-node TUI policy.

Evidence: `output/validation/release-archive-audit.json`, the four
`release-runtime-{darwin,linux}-{arm64,amd64}.json` reports,
`release-runtime-linux-amd64-native.json`, and the private native submission/state
records. Native Terminal.app font/animation acceptance, live authentication-loss
cases, licensing/third-party notices and the remaining product audit are separate
open gates. The local `audit-20261010` artifacts are candidates, not a published
or fully approved release.

## Compact navigation and scroll discovery — October 10

At 48 columns, the four full tab names now fit without an ellipsis. Rendering
and pointer hit testing use the same responsive labels and spacing. The compact
Connection footer retains reconnect, commands and quit. Long informational
panels wrap at word boundaries, falling back to hard wrapping for unbroken IDs
and error values so every character remains reachable.

Scrollable help, partition, connection, inspector, resource, wait-history and
saved-submission panels show their visible line range and remaining directions
in the bottom border, consuming no content rows. The indicator disappears when
all content fits. Enlarging a viewport now clamps the prior scroll offset even
when the underlying text has not changed; previously the top could remain
hidden in an otherwise large enough window.

Evidence: `internal/ui/compact_test.go` covers all four tab targets after repeated
resizes, long error preservation, word wrapping, End and height-only resize
recovery. `tools/test_compact.py` repeats navigation with actual SGR pointer,
keyboard and wheel input through a PTY, and is included in `make terminal-test`.
Ioskeley emulator renders are saved as `output/terminal/compact-*-after.png`,
`compact-partitions-bottom.png`, `compact-connection-scrolled.png` and
`connection-medium-after.png`. These are reviewed emulator renders, not native
Terminal.app font or pointer acceptance. The earlier `audit-20261010` release
candidates predate this UI change and must be rebuilt before release.

Validation passed: full race/vet/format checks (`compact-check.txt`) and the full
terminal workflow suite (`compact-terminal.txt`). After the final compact-footer
refinement, race/vet/format checks plus compact navigation, glance/inspector and
baseline keyboard/resize/terminal-restoration PTY checks passed again
(`compact-footer-terminal.txt` for the baseline run). Compact footers now preserve
back/quit controls for the inspector, logs, resources, software, list, command
input and palette; their width and recovery hints have regression coverage.

## NRP HTTPS transport · October 10

Added the production `HTTPSReader` connection boundary described in
[the NRP implementation notes](NRP.md#credential-snapshot-https-transport--october-10).
It performs scoped, bounded, cancellable reads with verified TLS and in-memory
credential snapshots. Background reads have no plugin/browser/prompt path.
Local TLS-server tests cover pagination, both auth mechanisms, scope enforcement,
certificate failures, redirects, permission splits, response bounds, cancellation,
known expiry and redacted errors. Full race/vet/format checks and build passed;
evidence is `output/validation/nautilus-http-{behavior,check}.txt`.

This advances actual transport beyond injected readers. It does not complete
NRP onboarding, credential acquisition, CLI/UI integration, resource/log views or
live API acceptance. No real configuration, credentials, API or workload was
accessed. The NRP readiness checkbox remains open.

## Experimental Nautilus CLI · October 10

Added `perch nrp jobs|job|pods|pod`, with explicit context/namespace for live
queries and isolated synthetic `--demo` output. JSON has schema version, source,
scope, resource version and observation time. Plain summaries distinguish Job
outcomes, Pod counts, indexed completion ranges and current/previous container
state; current scheduling/image-pull problems are visible in the Pod list.

The config-only kubectl path supports existing static tokens and client
certificates without changing current context. Exec/OIDC, auth-provider,
impersonation, token-file and unsupported TLS/proxy settings fail explicitly.
The selected endpoint/trust determines identity instead of mutable aliases.
No real user config or credentials were accessed during implementation.

`make check`, build, `make nrp-test` and MCP wire checks passed. The new process
harness ran the executable against a temporary local TLS API through a fake
config command and installed kubectl; it verified namespace routing, output
semantics, error redaction, demo isolation, unchanged config bytes, and a
credential-plugin sentinel that never executes. Temporary test material was
removed. Logs: `output/validation/nrp-cli-{unit,process,check,mcp}.txt`.

NRP interactive OIDC connection, workspace UI, logs/quotas/metrics/actions and
live NRP acceptance remain incomplete. The broader readiness gate stays open.

## Explicit NRP sign-in and subprocess output bounds · October 10

The CLI now offers explicit foreground `--login` for selected exec credentials,
with terminal stdin required, matching ExecCredential v1/v1beta1 protocol,
private credential output, bounded instructions and no automatic renewal from
API reads. Configuration and trust are validated before plugin execution. The
terminal's original settings are restored before querying, even when a plugin
changes modes and then fails or times out. These paths were exercised with a
synthetic credential plugin and a local TLS API, not the user's account.

The first oversized-plugin process test found a substantive bounds bug:
embedding `bytes.Buffer` exposed `ReadFrom`, so process-pipe `io.Copy` could
bypass the wrapper's `Write` limit. Both the new Kubernetes config/plugin buffer
and the existing Hummingbird local/SSH output buffer used this pattern. They now
contain a named private buffer so every copied chunk goes through their bounded
writer. Oversized credential output cancels the plugin immediately and is never
returned. Hummingbird output still drains safely, retaining at most 32 MiB stdout
and 1 MiB stderr, and returns an output-limit error instead of query success.
Real helper subprocesses now verify both streams; direct writer tests alone
had not established this property.

Validation: targeted race tests, full `make check`, rebuild, built-binary
`tools/test_nrp.py`, and MCP wire checks passed. The PTY plugin deliberately
changes echo/canonical input; successful login, expiry, plugin failure and
timeout all restore modes. JSON isolation, nonterminal refusal, unchanged
kubeconfig, no API request after failed login, and no plugin renewal after 401
also have coverage. Evidence: `output/validation/nrp-login-{check-targeted,check,
process,mcp}.txt`, `internal/transport/runner_test.go`, and
`internal/nautilus/login_test.go`.

Real browser/MFA sign-in, NRP namespace access, workspace UI, resource/log views
and live acceptance remain open. Main Hummingbird/native-terminal acceptance,
licensing and the broader product audit also remain open; this is not a release
completion claim. Existing release candidates predate these changes.

## Help and workspace navigation polish · October 10

Reviewed fresh Ioskeley emulator renders of Jobs, Help, the command palette and
Partitions. Help's manually spaced shortcut column was inconsistent, and newer
submission-recovery and wait-history commands were missing. Help now uses
structured sections with consistently padded accent-colored shortcuts, wrapped
descriptions and hanging continuation lines. At 112 terminal columns and above,
navigation/job tools and cluster/preferences tools form two columns. Smaller
windows use the same content in one scrollable column. Long command syntax is
retained, and demo-only controls are omitted from live workspaces.

Shift+Tab now moves backward through workspace tabs, preserving their filters
and selected jobs like forward navigation. Forms and text entry retain their
own key handling. Keyboard tab navigation dismisses Help just as pointer tab
selection does. Job-view shortcuts and filtering also leave Help immediately,
instead of opening an invisible view behind it. The command hint now correctly
says Esc preserves a filter when returning from Help or the palette.

Evidence: `internal/ui/help_test.go` checks width changes, command preservation,
column reading order, reverse-tab context and Help dismissal. The new
`tools/test_help.py` is included in `make terminal-test` and exercises actual
PTY keyboard and wheel input, 120×38 / 80×24 / 48×18 layouts, Home/End, resize,
selection/filter retention, reverse tabs and terminal cleanup. Reviewed renders
are `output/terminal/help-polished-{wide,medium,compact-top,compact-end}.png`.
These are emulator renders, not native Terminal.app acceptance.

Full race/vet/format checks and the entire terminal suite passed. After the last
Help dismissal/hint refinements, the full checks, rebuild and focused Help/mouse
PTY workflows passed again. Logs: `output/validation/help-polish-{check,terminal,
final}.txt`. The broader experience/readiness gates remain open.

## Output follow mode and reading position · October 10

Output now explicitly shows whether it is following the latest retained text or
paused for reading earlier rows. Home and upward scrolling pause automatic
scrolling; End or returning to the bottom resumes it. Refreshes still read data
while paused. Follow intent is separate from the viewport's current bottom
position, so shrinking a terminal cannot accidentally disable following and
enlarging one cannot silently resume it. Switching stdout/stderr or jobs resets
the log view and follow state. Failed reads show retry status instead of claiming
to be following successfully.

Output and submitted-script panels now carry a visible wrapped-row range and
scroll directions in their existing bottom border. No content row is consumed.
The log hint states its 256 KiB retention bound; the indicator describes the
retained buffer, not a complete remote file. Older content may age out of this
buffer. Existing byte limits, file rotation/truncation handling and sanitization
remain in force.

Evidence: `internal/ui/logs_test.go` exercises appended output while paused,
initial follow, Home/End, repeated width/height changes, wheel pause/resume,
read errors and stdout/stderr isolation. `tools/test_logs.py`, now in the
terminal suite, checks actual PTY input at 80×24 and 48×18, script scrolling and
terminal cleanup. Reviewed emulator renders: `output/terminal/logs-follow-
medium.png`, `logs-paused-medium.png`, `logs-paused-compact.png`,
`logs-follow-compact.png`, and `script-scroll-medium.png`. No native Terminal.app
acceptance or new live-cluster test is claimed for this change.

Full race/vet/format checks, rebuild and the entire terminal workflow suite
passed. Logs: `output/validation/logs-polish-{check,terminal}.txt`. Broader product
and release readiness gates remain open.

## Reproducible dependency notices and refreshed candidates · October 10

Release archives now include `THIRD_PARTY_NOTICES.md`, a machine-readable
inventory and unchanged upstream notice files. `tools/notices.py` enumerates
the command's Go package dependencies separately for macOS/Linux on ARM64/x86-64,
collects each module's shipped license/notice/patent files, and records versions,
target membership and SHA-256 hashes without local paths. Go runtime and linked
standard-library/vendor notices are included, plus a pinned supplemental Unicode
license fetched from its official source. The current bundle has 37 components
(35 Go modules, Go itself and Unicode data) and 51 notice texts. `go mod verify`
passed against the local module cache. Fonts used for preview rendering are not
part of these release packages. Perch/Chip license selection and final licensing
review are still open; collection of upstream notices does not resolve them.

`make notices-check` regenerates expected bytes in memory and fails on drift.
Release staging now runs that check before building. The archive checker verifies
notice hashes, source-copy equality, documentation links and an exact comparison
between each binary's embedded Go dependency versions and its target's inventory.
The verifier reads foreign-architecture build metadata without executing it.
Negative tests proved that changed notice bytes and a stale dependency version
fail even when the test's source mirror matches the altered archive.

Local `audit-notices-20261010` candidates include the subsequent NRP login,
Help/navigation and log-follow improvements. All four binary runtime harnesses
passed CLI queries, MCP wire behavior, PTY mouse/no-mouse navigation, compact Help
and terminal restoration. macOS ARM ran locally; macOS x86 used the existing
compatibility environment. Both Linux binaries ran in the existing OrbStack
course image with network disabled, read-only mounts and temporary storage.
The image is x86-64, including when running the ARM binary; this is compatibility
acceptance, not a new native Linux hardware/cluster test. No VM/image was installed
or downloaded and no cluster job was submitted during this pass.

Evidence: `output/validation/release-notices-archives.json`,
`release-notices-negative.txt`, `release-notices-build.txt`, and the four
`release-notices-runtime-{darwin,linux}-{arm64,amd64}.json` reports. These local
candidates are not published, and the broader readiness checklist remains open.

## Experimental NRP namespace workspace · October 10

`perch nrp ui` now provides namespace Jobs/Pods, controller-UID ownership,
current/previous container inspection, filters, keyboard/mouse navigation,
responsive scrolling and explicit foreground reconnect. Independent snapshots
retain stale data after permission/transport failures; successful empty reads
clear rows; authentication failures stop scheduled refresh. Object UID and
request generation guard selection and stale replies. See [NRP notes](NRP.md).

Evidence: race/vet/format `output/validation/nrp-ui-check.txt`, real-PTY demo
navigation `nrp-ui-pty.txt`, and local synthetic TLS/plugin acceptance
`nrp-ui-https.txt`. Reviewed wide Jobs, Pod details, minimum-size list/details,
and connection renders. The plugin test changes terminal modes, fails one
reconnect, then succeeds; the workspace resumes keyboard navigation and restores
terminal modes on exit. Demo and `--no-mouse` cleanup are also checked.

This is progress toward the NRP stretch goal, not completion: real onboarding,
OIDC/browser/MFA and cluster access are unavailable; logs, quota/metrics,
workload actions, Chip placement and native terminal acceptance remain open.
The existing `audit-notices-20261010` release archives predate this workspace and
must be rebuilt and verified before delivery. No release-ready claim is made.

## Retained Hummingbird output on read failure · October 10

A failed log refresh now keeps the last successfully read tail, remote cursor,
scroll position and paused/following intent. The header marks retained output
stale and offers retry (or reconnect when disconnected); the command hint shows
the read error. A first-read failure still displays an error with no fabricated
output. Recovery appends incrementally, while a confirmed rotation/truncation
replaces the old content, including a confirmed empty file. Switching stream,
job or attempt still invalidates the detail state. This is the existing bounded
in-memory tail, not a new disk cache.

Unit evidence in `internal/ui/logs_test.go` covers paused and following recovery,
retained cursor/offset/content, sanitized errors, initial failure, disconnected
copy, stdout/stderr separation and empty reset. `tools/test_log_retention.py`
uses the production adapter through a local fake SSH in a real PTY to verify
incremental cursor reuse, failed reads, recovery and confirmed truncation, with
reviewed 80×24 and 48×18 renders. No real cluster file or workload is touched.
Logs: `output/validation/log-retention-unit.txt`, `log-retention-pty.txt`,
`log-retention-check.txt` and the full terminal run `log-retention-terminal.txt`.

## Domain foundation source-and-test audit · October 10

This closes the domain-type foundation item only. It does not close live
resource, notification, connection or native UI acceptance:

- `internal/domain/model.go` separates allocation `Job` from nested `Step`, raw
  allocation IDs from array parent IDs, nullable task IDs from collapsed ranges,
  and nullable measurements/exit values from observed zero. Presentation-only
  unresolved array rows are explicitly excluded from scheduler task counts.
- `internal/hummingbird/parse.go` maps the pinned Slurm wrappers and sentinel
  values into those types. Inspected `TestRealAccountingFixture`,
  `TestRealQueueAndPartitions`, `TestUnsetInfiniteAndCollapsedArray` and
  `TestSchemaFailureIsNotEmptyQueue`: they check raw/task identity, nested steps,
  unset versus measured memory/exit data, non-finite limits, million-task ranges,
  and invalid schemas failing instead of becoming empty queues.
- `AttemptKey` and `AttemptAfter` distinguish scoped job/submit/restart identity.
  Inspected domain and app attempt tests cover ID reuse, restart changes,
  missing times, different scopes, unordered accounting, regressing history,
  terminal confirmation and normal expiration.
- `Snapshot` has independent queue/history/partition observation times.
  `Store.Apply` advances only successful resources; a queue failure preserves
  jobs and its observation time. Inspected tests confirm that queue absence
  becomes UNKNOWN until accounting confirms an outcome, failed reads retain
  rows, and collapsed array parents/tasks are not double counted.

These cases pass in `make check` (`output/validation/log-retention-check.txt`).
The audit inspected the assertions and implementation, not just test names.

## NRP container-log adapter and CLI · October 10

`perch nrp logs POD --container NAME [--previous]` now reads bounded current or
previous container logs with explicit namespace/context and the existing login
path. The adapter validates Pod UID before/after the log read, rejects observed
replacements/restarts and enforces byte/deadline limits. Output is sanitized;
failed or oversized reads produce no partial success payload. Demo output is
synthetic. [Implementation and limits](NRP.md#bounded-container-logs-in-the-cli--october-10).

Evidence: `internal/nautilus/logs_test.go`, CLI validation/demo tests, and the
built CLI against a local TLS server in `tools/test_nrp.py`; see
`output/validation/nrp-logs-unit.txt`, `nrp-logs-final.txt`, `nrp-logs-check.txt`
and `nrp-logs-cli.txt`. Existing transport and CLI race tests continue to pass.
This adapter checkpoint did not close the NRP gate. TUI log navigation is
implemented in the subsequent checkpoint below; quota/metrics, actions and
real account/cluster acceptance remain outstanding.

The shared-transport follow-up `make nrp-ui-test` also passed after the log
addition (`output/validation/nrp-logs-ui-regression.txt`): namespace navigation,
partial failures, explicit reconnect, synthetic plugin failure/recovery, and
terminal cleanup continue to work. This run remains entirely local/synthetic.


## NRP workspace container logs · October 10

The namespace TUI now supports Job → controller-owned Pod → explicit container
→ current/previous log snapshots, or direct Pod → container navigation. The
chooser labels container roles and state. Keyboard and mouse selection, nested
Esc navigation, filter/selection retention, independent log scrolling, help and
48×18 resize are covered. `l` and `/logs` enter; `p` changes mode; `r` reads;
`c` explicitly reconnects and rereads the selected log.

Failed same-selection reads preserve text and position with a stale header and
error; changing modes clears old text immediately. The header shows the last
successful read time and highlights failures. A log 401 latches sign-in even
if a namespace read succeeds. Closing or changing views cancels reads and
rejects late results. Reads retain the existing UID/restart verification,
256 KiB/2000-line bounds and sanitization. No automatic streaming or disk log
cache is implied.

Evidence: race-tested `internal/nautilusui/logs_test.go`, demo traversal in
`tools/test_nrp_logs_ui.py`, and production-adapter TUI checks against the local
TLS server in `tools/test_nrp.py --ui`. The latter exercises denied log reads,
retained text, mode isolation, expired credentials, explicit reconnect and
rereading the selected log, without accessing an NRP server. Reviewed screenshots
include container selection, current output, compact previous output and a
failed/recovered HTTPS read. They are emulator renders, not native acceptance.
Logs: `output/validation/nrp-log-ui-check.txt`, `nrp-log-ui-unit.txt`,
`nrp-log-ui-pty.txt`, `nrp-log-ui-https.txt`, `nrp-log-ui-regression.txt`.

NRP quota/metrics, workload actions and real credentials/cluster acceptance are
still open. Existing release archives predate this feature and must be rebuilt
and verified before delivery.


## Hummingbird step resources and accumulated CPU correction · October 10

A read-only recheck of completed two-node MPI job 801981 found a normalization
bug: Slurm returned job CPU `seconds=0, microseconds=1296842`, while Perch rejected
microseconds above one million and showed an unavailable total. Slurm's
[sacct aggregation implementation](https://github.com/SchedMD/slurm/blob/slurm-25.11/src/sacct/options.c)
adds step seconds and microseconds independently. The conversion now accepts
nonnegative accumulated microseconds; missing seconds and negative fields remain
unavailable. The rebuilt CLI reports 1.296842 CPU-seconds and 8.1052625% of the
2-CPU × 8-second allocation. Independent `sacct` text reported `00:01.296`,
agreeing at its displayed precision.

The resource view for finished jobs now includes separate step cards with state,
task count, CPU time, peak RSS and its scope. CLI/MCP resource results add a
`steps` array with explicit seconds/bytes and nullable measurements. The live
batch/extern/MPI records show 0.837101/0/0.459741 CPU-seconds, respectively;
extern RSS is unavailable rather than zero. The job peak remains 31,154,176
bytes, not the sum of 31,154,176 and 27,254,784 step peaks. No whole-job memory
efficiency is inferred. Live sampling still covers only the batch step.

The new scrubbed `accounting-multistep.json` fixture preserves those observations.
Parser, shared-query, CLI and UI assertions cover aggregate conversion, separate
step values, missing versus zero and narrow rendering. The fake-SSH PTY harness
`tools/test_resource_steps.py` exercises the production parser, resource view,
scroll/resize/back navigation and terminal cleanup. Wide and compact renders
were reviewed; they are emulator renders, not native Terminal acceptance.

Evidence lives under `output/validation/multistep-*`: independent live JSON/text,
before/after CLI JSON, unit/check/build results, terminal and MCP regression
runs. This advances post-job multi-step acceptance; live non-batch/GPU metrics
and broader native chart/animation acceptance remain open. No workload was
submitted or changed for this audit.


The live historical resource view also passed through Perch's real SSH adapter
in a PTY (`multistep-live-pty.txt`, reviewed `resource-steps-live.png`). A saved
comparison (`multistep-live-comparison.json`) checks every returned step's CPU,
task count and memory against the live JSON, then checks CPU/RSS against the
independent text command at its displayed precision. The aggregate CPU ratio
is verified against the allocation's two CPUs and eight elapsed seconds.

Final verification for this checkpoint: `make check`, the full
`make terminal-test`, and `tools/test_mcp.py` passed. The local binary was rebuilt
after the embedded MCP guidance update. No release archive was refreshed yet.


## SSH stderr spool bounds and lifecycle audit · October 10

The local process buffers were bounded, but batched SSH queries previously
redirected stderr into an uncapped remote file until each command finished.
The batch shell now reads stderr through a FIFO, retains at most 1 MiB plus one
overflow byte, then drains remaining bytes without writing them to disk. The
frame parser classifies overflow as an output-limit failure, with no partial
successful batch returned. Collector failures fail the whole operation.
The directory is private under umask 077; EXIT removes it, and HUP/INT/TERM now
exit explicitly so a handled signal cannot continue into following queries.

Source-and-assertion audit covers system SSH argv separation, POSIX argument
quoting (including quotes/newlines/substitution syntax), private short control
paths, explicit BatchMode for background queries, separate interactive login,
caller deadlines, bounded stdout/stderr buffers, strict nonce/index framing,
per-query exit codes, classified/redacted connection errors and cleanup.
Current UI queries have 15/30/40-second operation deadlines; CLI/MCP use their
configured bounded contexts. Interactive password/MFA remains a separate gate.

`internal/transport/batch_test.go` executes actual shell batches to verify
4 MiB stderr is drained but only the bounded prefix reaches the frame, exact
limit/empty streams remain valid, later queries survive an ordinary command
failure, and a termination signal prevents later queries and removes the spool.
Helper subprocess tests separately cover the local output-copy limits.
Classification tests distinguish SSH exit-255 host-key/auth/network failures
from a remote command's ordinary nonzero exit without exposing raw error text.

The opt-in `TestHummingbirdTransportIntegration` passed against Hummingbird:
literal argv/stdin, private multiplexing, framed partial failures, oversized
stderr rejection, temporary-directory permissions/removal, client cancellation,
a subsequent successful query, socket-directory removal and disappearance of
the specific private SSH master PID. It also observes the cancelled batch's own
spool and verifies its later removal after the short command ends. This does
not promise immediate cancellation of arbitrary remote descendants or cleanup
after SIGKILL, host failure or loss of the remote shell. No scheduler job,
existing file, cluster configuration or credential was changed.

Evidence: `output/validation/transport-spool-{unit,live,check,terminal,mcp}.txt`.
The full terminal regression suite passed with the changed batch format. This
closes the runner foundation within its documented cancellation/cleanup limits;
real interactive reconnect, native UI acceptance and overall release readiness
remain open.

## Queued-job start estimates · October 10

Queue parsing now keeps a pending job's predicted start in `EstimatedStart`,
separate from its actual `Start`. CLI/MCP JSON exposes this as nullable
`estimated_start_at`; a queued prediction no longer appears in `started_at`.
The inspector and glance label future estimates tentative, past estimates as
awaiting an update, and missing estimates as unavailable. CLI text identifies
predictions that were already past at the response's observation time. No clock
comparison changes scheduler state or guarantees execution. BeginTime now has
an explanation alongside the original scheduler reason.

The opt-in `tools/test_pending_live.py` submitted one delayed-start test job,
802423, requesting one CPU and 64 MiB with a one-minute hard limit and both
output streams directed to `/dev/null`. It verified PENDING/BeginTime through
Slurm, actual-start null and a tentative estimate through the built CLI, and
the real SSH-backed TUI at 120×38 and 48×24. It then cancelled that exact job and
verified its absence from the queue. Its receipt records the unique comment
and cleanup result. The reviewed emulator screenshots preserve the explanation,
prediction qualification, raw reason and usable scroll/back controls at both
sizes; they do not establish native Terminal font acceptance.

Parser assertions distinguish pending predictions from running start timestamps;
query JSON, CLI text and UI tests cover missing/future/past estimates without
inventing execution. The independent MCP wire check also verifies that its
structured and text representations agree on the separate timestamp fields.
README, MCP documentation and the embedded jobs guide describe these semantics.
Evidence is under `output/validation/pending-estimate-*` and
`output/terminal/pending-estimate-live*`. Existing release archives remain stale.

Final checks passed for this checkpoint: `make check`, the complete
`make terminal-test`, and `python3 tools/test_mcp.py` against the rebuilt binary.
The live delayed-start check and cancellation ran once; no workload was repeated
for the local regression suite. Overall native UI and release acceptance remain
open.

## Chip playback review alignment · October 10

The application already played running wings at 40 ms per frame, while the
developer preview still played every state at 80 ms. Terminal timing now lives
in `bird.FrameInterval`, shared by the app and review tool. The all-state review
advances running at 40 ms and the other five states at 80 ms; the single-state
sample follows the same policy. State captions show source frame and interval.
Pause freezes both clocks, and manual arrow stepping aligns all states to one
source index so frame comparisons remain possible. Completion still loops only
in the review tool; the app's once-per-observed-completion behavior is unchanged.

This changes playback only. All 21 approved detailed artwork checksums match;
no sprite geometry, palette, conversion, web preview or GIF was rewritten.
Model tests cover every frame through two running loops, the slower state
sequence, pause/aligned stepping and each single-state loop boundary.
`make chip-test` runs the built review in a real PTY, verifies the relative
cadence and controls, and checks terminal restoration. The six-state sheet and
running sample were visually inspected as emulator images under
`output/terminal/chip-cadence-*`. Native Terminal.app rendering and remaining
state approval are still separate gates.

`make check`, `make chip-test`, the terminal-emulator tests and the main app's
PTY navigation/resize/reconnect/cleanup test passed. Both local binaries were
rebuilt. Evidence is under `output/validation/chip-cadence-*`; release archives
have not yet been refreshed for these changes.

## Empty workspace discovery · October 10

A successfully observed empty Jobs view now uses one panel with preparation,
history and partition shortcuts rather than an empty list beside “Select a
job.” Keyboard `n` from the Jobs/History list opens the existing preparation
form; the empty-state labels are also clickable. Opening the form does not
submit, and escaping/reopening preserves the draft. The command hints expose
software discovery and saved submission receipts as well.

The same view fits 48×18 with three compact actions. Larger layouts retain Chip
on the border and omit the border title under his feet. Failed refreshes label
the retained empty snapshot “No jobs at last refresh”; connection health and
reconnect guidance remain visible. Initial unobserved queues and filtered lists
do not enter this empty-workspace state.

`internal/ui/empty_test.go` exercises mouse targets found in rendered text at
five sizes, draft retention, shortcut scope, and stale/unobserved/filter
distinctions. `tools/test_empty_workspace.py` drives the built app through a
fixture-backed SSH adapter with real keyboard/mouse PTY input, including compact
resize, retained draft, history/partitions navigation, filter clearing, a
disconnected empty snapshot and terminal restoration. No cluster command or
job submission is performed by this fixture test. Screens under
`output/terminal/empty-workspace-*` are emulator views, not native acceptance.
The new PTY check is included in `make terminal-test`.

Final verification: `make check` passed. The terminal suite passed through its
new empty-workspace test and all preceding checks, then its final attempt test
stopped on the replaced empty-queue wording. That expectation was updated to
“No active jobs.” and the entire attempt test passed on rerun, including its
accounting-lag, requeue ordering and 2/2 array completion assertions. Logs are
`output/validation/empty-workspace-{check,terminal,attempts}.txt`. The final wide,
compact and stale-state renders were reviewed; the local app is rebuilt.

## Packaged review candidate · October 10

`review-20261010` refreshes all four local archives from the current application,
including NRP container logs, separate recorded resource steps, bounded SSH
stderr spooling, queued-start semantics, Chip timing and empty-workspace
navigation. Checksums, archive structure, documentation links, original artwork,
dependency inventories and unchanged upstream notices passed. Deliberately
changed notice bytes and stale dependency versions still fail verification.

The portable release-runtime test now exercises the packaged NRP functionality
as well as Hummingbird demo CLI/MCP/TUI: Job/Pod lists and inspection, current
and previous container logs, namespace navigation, log selection, compact
resizing, Help, and mouse/no-mouse terminal restoration. Its raw-output checks
account for incremental redraws that omit unchanged text prefixes. Full visible
screen assertions remain in the emulator suites. Both existing NRP UI/log
screen suites also passed with their Session factory directed at the extracted
macOS ARM64 candidate, and the compact previous-log render was reviewed.

All four packaged binaries passed. macOS ARM64 ran natively; macOS x86 used the
existing compatibility environment. Both Linux targets ran using the existing
x86 course image, with image pulls disabled, network disabled, read-only mounts
and temporary writable storage. These Linux runs do not establish native
cluster/hardware or live NRP acceptance. No VM, image or software was installed,
no workload was submitted, and no release was published.

`output/validation/release-review-summary.json` verifies each runtime report's
binary hash against both the extracted binary and archive report, and rechecks
all archive checksums. Other evidence is in `release-review-{build,archives,
negative,runtime-*,nrp-screens}` under the same directory. Packaged docs reflect
the source checkpoint before this evidence paragraph was appended. Project and
artwork licensing, native UI/notification acceptance, live authentication/NRP
acceptance and the other unchecked requirements remain open.

## Failure-hint navigation fix · October 10

The source audit found that log-based diagnoses were effectively unreachable:
the inspector used `m.logRaw`, but navigating from logs to the inspector cleared
that buffer. Perch now retains only one derived log-pattern hint with its scoped
job/submit/restart identity, stream and successful-read timestamp. Navigation
within the same attempt can display it without keeping a second raw log buffer.
Selecting another job, ID reuse or a retry clears it. It never persists to disk
and adds no log reads or background cluster commands.

The inspector explicitly labels a log hint as an unconfirmed cause and preserves
its stdout/stderr provenance and read time. A failed read cannot refresh that
timestamp; a successful empty/truncated or different-stream read supersedes the
hint. Late cancelled reads cannot restore discarded evidence. Log pattern
matching uses visible sanitized text, so hidden terminal escape payloads are
not treated as output evidence. A nonfailed job is not diagnosed from text alone.
Specific scheduler outcomes remain authoritative; BOOT_FAIL, PREEMPTED and
DEADLINE now have explicit explanations following
[Slurm's state definitions](https://slurm.schedmd.com/job_state_codes.html).
DEADLINE describes the latest permitted start, not a runtime TIMEOUT.

Domain/UI tests cover navigation, raw-buffer clearing, attempt isolation,
failed-read timestamps, truncation, late replies, nonfailed states and scheduler
precedence. `tools/test_diagnosis.py` verifies the formerly broken log → list →
inspector path through the production adapter in a real PTY using synthetic
SSH, then checks provenance/caveats at 100×36 and 48×24 and confirms that a later
empty read removes the hint. Reviewed renders are under
`output/terminal/diagnosis-inspector-*`; no workload was submitted. The previous
`review-20261010` archives remain a tested snapshot and do not include this
subsequent fix yet. Broader array/insight and native acceptance gates stay open.

Final verification: `make check` (race tests, vet and formatting) and the full
`make terminal-test` suite both completed successfully. Logs are saved as
`output/validation/diagnosis-navigation-check.txt` and
`output/validation/diagnosis-navigation-terminal.txt`.

## Live compute-step resources · October 10

The previous live resource pane sampled only `.batch`, omitting separately
launched `srun` work. A single `sstat --allsteps` query now collects separate
running-step records. `[` / `]` and clickable Previous / Next controls select an
independent chart. The first numbered compute step is preferred when present
in the initial sample. Choice stays stable when a step disappears; its later
samples remain gaps. Switching charts uses retained samples and sends no new
scheduler query. History still retains at most 90 samples, with at most 1,024
steps and 256 bytes per input row; duplicate, foreign or malformed step records
fail the query instead of becoming partial measurements.

Live CLI/MCP resource results add `live_steps` with step IDs, task peak RSS and
average task CPU seconds. Existing top-level live measurements preserve their
batch-only meaning. Missing values stay null; step peaks are never summed.
Recorded post-job step resources remain separate. The adapter follows
[Slurm's allsteps and field semantics](https://slurm.schedmd.com/sstat.html).

Authorized synthetic job **802458** requested one CPU, 128 MiB, and a three-minute
limit. Its `srun` task hashed a 32-MiB buffer with a 110-second internal bound;
stdout/stderr went to `/dev/null`, and nothing was installed. CLI and real-PTY
checks observed compute and batch measurements, two samples, independent charts,
keyboard switching, compact pointer controls and terminal restoration. The job
was cancelled after acceptance and verified absent from the queue. Reviewed
`output/terminal/live-steps-hummingbird-{compute,batch,compact}.png`; these are
emulator renders of SSH-backed PTY output, not native Terminal.app acceptance.

This live query exposed an extern-step CPU sentinel rendered as
`213503982334-14:25:51`. The adapter now maps the formatted INFINITE64/1000 bound
to unavailable. `testdata/hummingbird/live-multistep.txt` preserves the scrubbed
real response, and a regression test verifies measured zero, missing RSS,
compute/batch separation and the unavailable CPU sentinel. The original live
JSON capture predates this correction; the final parser and fake-SSH/real-PTY
checks exercise it without submitting another job.

`tools/test_live_steps.py` additionally verifies disappearance gaps, CLI JSON
scope/nulls, keyboard and mouse navigation, local-only switching, resize/back
and terminal cleanup, and runs in `make terminal-test`. The opt-in real-cluster
harness is `tools/test_live_steps_hummingbird.py HOST`; it has a one-shot
submission receipt and exact-job cleanup. GPU metrics, native acceptance and
the broader resource audit remain open. Packaged `review-20261010` archives
predate these changes.

Final verification for this checkpoint: `make check` passed race tests, vet and
formatting; the full `make terminal-test` suite passed. After the sentinel,
input-bound and two-step demo refinements, `make check`, the focused live-step
PTY/CLI test and the general demo/navigation PTY test passed again. Independent
`make mcp-test` discovery/schema/wire checks passed with the new resource tool
description. Evidence is in `output/validation/live-steps-{check,terminal,pty,demo,mcp}.txt`.
The real job's accounting confirms CANCELLED, with its batch/compute steps
terminated; the receipt records verified queue absence. Demo mode now exposes
both synthetic compute and batch charts and was visually reviewed as well.

## Mouse hover and terminal pointers · October 10

Hummingbird hover and clicks now share one pure hit test for tabs, job rows,
command input, command-palette entries, empty-workspace actions, inspector links,
resource-step controls, launch fields and settings. Hover never changes selection
or executes an action. Controls underline, nonselected job/palette rows receive
a subdued background, and text fields preserve their input appearance.
Keyboard use, resize and window-focus loss dismiss stale hover coordinates.

The workspace requests all-motion mouse reports. Recognized Ghostty/Kitty/Foot
terminal environments use CSS pointer names; iTerm2 uses its X11 names. Unknown
terminals, macOS Terminal, and tmux/screen sessions do not receive OSC 22. Mouse
disabled at startup produces neither tracking nor pointer-shape changes. Turning
capture off or shrinking below the minimum releases tracking and resets an
already-used pointer. NRP's separate workspace has not yet adopted hover.
Vocabularies follow [Ghostty's OSC 22 documentation](https://ghostty.org/docs/vt/osc/22)
and [iTerm2's supported pointer names](https://iterm2.com/downloads.html).

Pointer commands are emitted only when the desired shape changes. Final cleanup
restores the terminal's default shape after Bubble Tea exits, including handled
signals. A reconnect test caught that `tea.Raw` output can remain buffered when
`ExecProcess` suspends the renderer. The SSH handoff now writes the reset through
Bubble Tea's assigned terminal output after suspension and before starting the
SSH child; the real-process test verifies this order with a fake login marker.
No terminal settings or installed applications were changed.

Evidence: `make check` passed race tests, vet and formatting; the full terminal
suite passed click/navigation regressions after the shared hit-test change.
The final `tools/test_hover.py` additionally passed actual SGR motion reports,
CSS and X11 OSC output, shape deduplication, hover without activation, keyboard
and resize clearing, unsupported/no-mouse fallback, normal and SIGTERM cleanup,
and the corrected fake-SSH reconnect handoff. Native pointer appearance is not
claimed by these byte-level checks and still needs supported-terminal acceptance.

Reviewed wide/compact hover renders under `output/terminal/hover-*`. The emulator
now draws underline attributes and ignores xterm modifyOtherKeys controls that
stock pyte misinterprets as SGR underline; fragmented-input regression tests keep
that screenshot artifact separate from application styling. Evidence logs:
`output/validation/hover-{check,terminal,pty,emulator}.txt`. The installed local
binary was rebuilt; older release archives do not contain this work.

## Array clipboard workflow audit · October 10

Added `tools/test_array_copy.py` to the terminal suite. A private fake SSH
serves synthetic records using the real Slurm JSON fixture shapes. CLI and TUI
agree on 57 observed tasks: one completed, four failed, one cancelled, one
running retry and 50 pending. Duplicate records and an older failed attempt do
not add failures. Copying produces exactly `--array=3-5,42`, via both `y` and
`/failed`; selecting a standalone job never invokes the helper. Recorded SSH
arguments contain no submission, cancellation or allocation commands.

Improved copy feedback and lifecycle: an immediate “Copying failed tasks…”
notice replaces the previous result, repeated input cannot start concurrent
copies, and a pending helper is cancelled when Perch closes. The helper has a
three-second deadline. Process-level tests verify success bytes, helper failure,
bounded timeout, helper termination, recovery on a later copy and terminal
restoration. Tests use temporary clipboard executables and never touch the
user's actual clipboard. Assertions wait for new helper invocations and output,
so an old success notice cannot satisfy a later request.

`make check`, rebuild and the focused CLI/PTY test passed. Reviewed the wide
success and 48-column fallback captures at
`output/terminal/array-copy-{success,fallback}.png`; these are emulator renders,
not native Terminal.app acceptance. Logs: `output/validation/array-copy-*.txt`.

Remaining scope: the current specification is built from individually confirmed
failed task IDs. A failed collapsed range contributes to the failed count but is
not expanded into the clipboard selection. This audit therefore does not close
the full array-copy gate or claim every collapsed failure is included. Native
clipboard delivery and additional terminal appearance checks remain separate.

## Collapsed failure selection and distinct progress · October 10

Closed the collapsed-range omission identified in the preceding clipboard
audit. Array summaries now assign each task once, preferring newer attempts and
then individual observations over ranges from the same attempt. Failed
selections preserve compact ranges and strides, subtract running/retried,
completed and cancelled members, and avoid duplicate IDs. The title-bar totals
and missing-accounting gap calculations now use these reconciled counts too.
Older records remain available in History; copying does not submit or cancel.

The range set uses disjoint arithmetic progressions with bounded subtraction.
For example, removing a newer even-indexed pending range from failures across
0–4,000,000,000 leaves `--array=1-3999999999:2` without allocating billions of
IDs. Limits are 8,192 fragments and 200,000 overlapping subtraction operations.
An over-complex summary displays an explicit unavailable message, disables
copying and makes the shared CLI/MCP query return an error rather than zeros or
an incomplete selection. Individually reported tasks have a separate direct
membership path; a 20,000-record array still produces `--array=0-19999`.

Evidence: 5,000 seeded subtraction comparisons, 500 multi-range union comparisons
and 300 mixed-outcome array comparisons against independently enumerated task
sets; duplicate/input-order checks; retry and state exclusions; large-index and
fragment/work-bound cases. UI and shared-query tests cover the unavailable path.
A benchmark on this M4 measured about 1.9 microseconds and 2,648 allocated bytes
for the specific two-range, four-billion-index example; this is not a general
latency guarantee.

The production CLI and PTY copy workflow also passed an overlapping failed
0–20 range and a running retry of task 17 with a changed submission time. The
result is 21 observed tasks, 20 failures and exactly `--array=0-16,18-20` on the
private test clipboard. Wide and 48-column captures are at
`output/terminal/array-copy-collapsed-{wide,narrow}.png` (emulator renders).
The test still covers helper failure, timeout, duplicate suppression, recovery
and exit cleanup without touching the user's clipboard.

Read-only live `perch arrays` succeeded against Hummingbird and returned nine
arrays whose state totals matched their observed counts; receipt:
`output/validation/array-range-live.json`. No cluster jobs were submitted.
Native clipboard and terminal appearance acceptance remain separate. The
preceding checkpoint's collapsed-copy limitation is superseded by this work.

Final verification for this change: `make check` (race tests, vet, formatting),
`make terminal-test`, the additional seeded outcome oracle under the race
detector, and MCP wire checks passed. After tightening the title-bar assertion
and aligning synthetic running-job start times for readable captures, the
focused array CLI/PTY check passed again. Rebuilt the binary with the updated
embedded MCP guide and repeated its wire check. The final wide capture shows
one running task and 20 failed tasks in both header and inspector; the compact
capture keeps the complete copied specification visible. Validation logs use
`output/validation/array-range-*`.

## Array progress presentation · October 10

Changed the array bar from successful-task count to finished-work count:
succeeded + failed + cancelled. A sweep with 20 failures and one running task
now reads “20 / 21 observed tasks finished” and has a red completed portion,
rather than showing an empty bar. Green, red and muted segments distinguish
successful, failed and cancelled tasks. Exact textual counts remain visible in
no-color mode; active and unconfirmed tasks do not fill the bar. The observed
denominator remains explicit, and CLI/MCP `completed` retains its success-only
meaning.

The glance, full inspector and missing-accounting view share the same progress
component. Failure copying and unconfirmed outcomes precede less urgent details.
Short glance layouts drop decoration before these counts and retain the full
inspector link. Also corrected the shortest glance's unavailable-summary path
so a range-complexity error cannot appear as “0/0 finished.”

Verified mixed outcomes, fully failed/cancelled arrays, all-unconfirmed arrays,
segment colors, exact bar widths and small-height summaries in unit tests.
Actual PTY checks cover 8/40 finished with seven successes and one failure,
6/57 finished with four failures and one cancellation, and 20/21 finished with
a running retry excluded from the copy selection. Reviewed emulator captures:
`glance-running-wide.png`, `array-copy-success.png`,
`array-copy-collapsed-narrow.png`, `array-progress-no-color.png` and
`array-progress-short.png` under `output/terminal`. The last is 96×18; the
failure-heavy compact inspector is 48 columns. Native appearance remains a
separate acceptance check.

`make check` (race tests, vet, formatting) and the full terminal suite passed
with the rebuilt binary. Logs: `output/validation/array-progress-*.txt`.
README and the feature design now explain finished versus successful counts.

## Large-array animation and navigation · October 10

Found repeated projection work on every animation frame: selected-job lookup,
row rendering, hover geometry and the inspector repeatedly reconstructed the
same filtered rows, array gaps and summaries. With 5,000 observed tasks (four
running and 4,996 finished), a complete animation Update + View benchmark on
this M4 took 59–61 ms in Jobs and allocated about 276 MB per iteration. History
took 43–44 ms and allocated about 224 MB.

The UI now retains a read-only projection of one store revision. Apply and
successful Restore advance that revision; switching stores also invalidates the
projection. Tab/filter changes rebuild their filtered rows, while animation,
geometry and log updates reuse unchanged task membership. A replaced snapshot
slice or timestamp is also detected. The cache holds only the current
projection and current filtered view, not a growing history of searches.

The same benchmark after the change measured 0.60–0.62 ms and about 128 KB per
Jobs frame; History measured 0.61–0.62 ms and about 136 KB. These are allocations
per benchmark iteration, not resident RAM measurements. Fixture construction,
initial refresh, SSH and native-terminal drawing are outside this benchmark;
it is not a native FPS guarantee. Reproducible benchmark:
`go test ./internal/ui -run '^$' -bench BenchmarkLargeArrayAnimation -benchtime=200ms -benchmem -count=2`.
Before/after results are `output/validation/animation-performance-{before,after}.txt`.

Tests cover same-timestamp refreshes, changed names/outcomes and failed-task
specifications, case-insensitive filtering, tab changes, partial queue failures,
restored missing-array evidence with no source-job changes, rejected restore,
and connection-scope replacement. Repeated unchanged row/array lookups allocate
nothing in the unit check.

Added `tools/test_large_array.py` to the terminal suite. It feeds 5,000 task
records through the real parser and SSH adapter using a private fake SSH,
with animation enabled. It exercises History, filtering, the inspector, an
80-column resize, a refresh with a renamed/requeued member and retained tab
filters. It caught an existing Escape bug: leaving job details cleared the
filter despite README's promised behavior. Escape now returns to the filtered
list; a second Escape in the list clears the filter while keeping selection.
Hints and keyboard tests agree with this behavior for details, logs, scripts
and resources.

The focused large-array PTY check passed, including terminal cleanup. Its
History/filter timings were about 0.16 seconds, including the harness's fixed
150-ms input pump; these are not precise UI latency measurements. Reviewed
`output/terminal/large-array-{animated-jobs,filtered-compact,refreshed}.png`.
These are emulator renders, not native Terminal.app acceptance. No cluster job
or real clipboard operation was used by this workload test.

The recorded full terminal-suite output also reaches the final passing
attempt-identity check (`output/validation/animation-cache-terminal.txt`).
The corresponding race/vet/format check passed; the earlier session handle has
expired, so no running regression process remains to resume.

## NRP hover and mouse focus · October 10

The namespace workspace now shares one visible-target calculation between
clicks and hover. Tabs use an underline; unselected workload and Pod/container
picker rows use a muted warm fill. The selected row keeps its stronger fill.
Moving the pointer does not change object identity, selection, filter or log
choice, and does not initiate a log read. Output text, empty rows, borders,
headers and hidden lists behind help/inspection are not click targets.

The CLI uses the existing conservative CSS/X11 terminal detection for both
workspaces. NRP emits a hand over click targets and a text pointer over input;
unknown terminals and multiplexers receive only visual feedback. The current
shape is deduplicated. Keyboard/paste input, focus loss and resize clear hover.
Small-window and no-mouse states expose no targets. A refresh that removes the
row under the pointer resets its shape instead of leaving an invisible target.

Foreground login writes the cursor reset through Bubble Tea's assigned Exec
writer after suspending rendering and before the credential plugin starts.
Returning from either successful or failed login discards the old hover.
Normal and signal exit reset after terminal restoration. NRP's signal path may
print a cancellation message after that reset; this is tested without requiring
the reset to be the last bytes of process output.

Mouse selection now also blurs the command input. Previously, clicking a row
or picker while the command field had focus left the next key acting on that
field. Enter and log shortcuts now act on the clicked object. Input drafts are
not discarded by a row click.

Unit coverage includes responsive/shared geometry, nonmutating hover,
selection identity, supported/unsupported/disabled modes, modified clicks,
keyboard/focus/resize reset, empty refresh, picker/output distinction and
foreground reset ordering. `tools/test_nrp_hover.py` exercises real SGR mouse
reports, CSS/X11 output, highlights, compact resize, input-to-row/picker focus,
and normal/SIGTERM cleanup. It is part of `make nrp-ui-test`. The synthetic
HTTPS/plugin test also checks reset ordering before both a failed login and a
successful retry. No real NRP account or credential was used.

Reviewed `output/terminal/nrp-hover-{tab,pod,container,compact}.png` in the
Ioskeley emulator. The first row treatment looked like a divider across the
panel; the reviewed version uses the softer background. These are PTY/emulator
checks, not native pointer or live NRP acceptance. README and the NRP guide now
match the behavior, including the already-implemented TUI log picker.

Verification: `make check` passed race tests, vet and formatting. The full
`make nrp-ui-test` suite passed after the hover/render/login changes. The final
input-focus adjustment then passed `make check` and the focused real-PTY hover
suite with explicit input-to-row/container cases. The executable was rebuilt.
Logs: `output/validation/nrp-hover-{check,terminal,final}.txt`.

## Optional restart snapshot · October 10

Implemented `--cache` for the live Hummingbird TUI. It retains one most-recent
SSH/local destination's normalized jobs, history and partitions for at most
24 hours. No other invocation reads or updates it implicitly; CLI/MCP queries
remain live-only, and `--cache` with demo or query commands is rejected.

A separate display store holds the restored view. The status explicitly says
cached, section timestamps remain visible through age/Connection, and Chip stays
in its reconnecting state. Each successful jobs/history/partition read replaces
its saved section; all three fresh sections retire the fallback. The live store,
notification checkpoint, polling cadence and completion events do not consume
cached observations. A cached RUNNING job later reported completed does not
produce a new startup alert solely because it appeared in the saved view.
Missing-array totals/notification observations still use their separate checkpoint;
this view cache does not claim to retain them.

The host string must match before loading. Successfully identified differences
in cluster or remote username discard the saved view, even when a later
scheduler-capability check fails. Scope changes also clear retained live data
and invalidate pending details. Cached rows require a verified matching fresh
job attempt before output/script/live-resource reads; failed-task copying waits
until the fallback has been replaced. Cache read guards explain the saved view
inside the requested panel rather than leaving an empty error panel.

The whitelist excludes paths, logs, scripts, raw replies, steps, capabilities,
credentials and notification deduplication. Names, states, allocation metadata
and recorded allocation-level measurements are user data and are stored only
with explicit opt-in. Whole-view limits are 10,000 job rows, 10,000 accounting
records, 128 partitions and an 8-MiB JSON encoding; oversize views are skipped
without partial array counts. The last good file remains available.

Storage uses an owned 0700 directory and 0600 regular single-link files,
directory-relative no-follow access, nonblocking bounded locks, file/directory
sync and atomic rename. One reserved pending file bounds interrupted writes to
one additional 8-MiB file; normal saves remove it. Older snapshots cannot replace
newer saved data. Malformed/unsafe files produce a cache diagnostic instead of
being implicitly overwritten. Monitoring continues; Connection gives the path
for explicit removal. No real user cache was created by the tests.

Unit cases cover whitelist/copy ownership, scope, timestamps, whole-view size
limits, corruption, cancellation/contention, stale writers, pending-file recovery,
permissions and symlink/FIFO/hardlink rejection. UI cases cover separate event
tracking, missing/fresh component handoff, navigation, cached read/copy guards,
account changes including failed capability probes, and opt-in isolation.
`tools/test_cache.py` exercises production parsing/transport/UI in real PTYs with
private fake SSH and temporary HOME: save, restart during connection failure,
inspect/filter/resize, blocked log read, reconnect without losing the filter,
identity change, expiry and terminal cleanup. It is included in the terminal
suite. This is not native-terminal or real-network acceptance.


Final snapshot verification: `make check` passed race tests, vet and formatting.
The full `make terminal-test` suite passed, including the new cache workflow and
existing navigation, hover, logs, resources, launch, receipts, arrays,
preferences/color modes, diagnostics and attempt-identity checks. Logs:
`output/validation/cache-check.txt` and `cache-terminal-full.txt`. Reviewed
`output/terminal/cache-offline-inspector.png`, `cache-offline-compact.png` and
`cache-reconnected.png` with the final warm palette. The cache test pins terminal
color capabilities and relative fixture start times so its renders are legible
and do not imply a running job has already exceeded its synthetic limit. Native
acceptance and the broader release audit remain open; review archives are stale
relative to this source and executable.

## Refreshed packaged runtime acceptance · October 10

The local `polish-20261010` candidates supersede the older review archives. They
include restart snapshots, saved color preferences, large-array projection
caching and hover in both workspaces. They are review artifacts, not a published
release. `make check` passed; packaging and notice-negative checks passed.

All four extracted executables passed the portable runtime harness: macOS arm64
natively, macOS amd64 through existing Rosetta, and both Linux targets through
the existing OrbStack course-image environment. Linux tests used `--pull never`,
no network, a read-only root/project mount and a temporary filesystem. No image,
VM or package was installed. Containers were removed after testing. The image's
Python process reports x86-64; the harness independently identifies each tested
ELF architecture and hashes the actual executable. These compatibility runs do
not establish native Linux hardware or native terminal acceptance.

The harness exercises Slurm and NRP demo CLI queries, MCP wire behavior, six PTY
sessions per binary (CSS pointer names, X11 pointer names and mouse disabled in
each workspace), logs, navigation, help, 48×18 resizing and terminal restoration.
Actual mouse-motion reports must produce hand/text pointer requests; exit must
reset the pointer after leaving the alternate screen. Mouse-disabled sessions
must emit neither capture nor shape changes. The NRP assertion was corrected
from its old cell-motion mode to the implemented all-motion mode.

Archive checks now compare packaged documentation and examples with the current
workspace as well as validating links, checksums, original artwork and notices.
Documentation was refreshed after runtime checks, then repackaged; each final
archive's binary hash is required to match its runtime report.

Evidence: `output/validation/release-refresh-check.txt`,
`release-polish-build.txt`, `release-polish-archives.json`, the four
`release-polish-runtime-{darwin,linux}-{arm64,amd64}.json` reports, and
`release-polish-summary.json`. The native Linux result for job 802233 belongs to
an older executable and is not reused as acceptance of these binaries. Native
font/animation/pointer appearance, notification delivery, real interactive
authentication and NRP access, project/artwork licensing, and the remaining
requirement audit are still open.

## Clickable Hummingbird footer and compact launch recovery · October 10

The Hummingbird footer's visible keyboard shortcuts are now pointer targets.
This closes the compact-layout gap where a mouse could select a job but the
side inspector's details link was absent: click `enter inspect`, `l output`,
`e stdout/stderr`, and `esc back` to make the complete round trip. Returning
retains the selected job and filter. Hover underlines the action and requests
the supported terminal's hand pointer, without activating it.

Footer actions reuse the keyboard event path with its current modal focus.
Command input, filter entry, palette choices, settings and launch forms keep
their own Enter/Tab/Escape meanings. Up/down and resource-step brackets have
separate direction targets; explanatory prose, gaps and clipped hints do not.
Disabled mouse capture, modified clicks and right clicks cannot activate them.
Saving settings and waiting launch stages expose no footer action. Compact
launch footers now retain the back action and the explicit validated-submit
action at 48 columns. The submission validation/receipt logic is unchanged.

Evidence: `internal/ui/footer_test.go` checks rendered hit positions, hover,
selection/filter retention, modal ownership, non-targets and compact launch
states. `tools/test_footer.py`, included in `make terminal-test`, sends actual
SGR pointer reports through wide and 48×18 PTYs, opens/switches/returns from
output, submits command text, opens/closes help and completes a compact demo
launch review/validation/explicit-submit/close flow. No remote job was submitted.
`make check`, the focused PTY and the full `make terminal-test` suite passed;
logs are `output/validation/footer-check.txt`, `footer-terminal-focused.txt`
and `footer-terminal-full.txt`. The final focused pass additionally checks the
latest pointer request and captures the compact inspector's back-action hover.

Reviewed emulator renders: `output/terminal/footer-jobs-140.png`,
`footer-jobs-48.png`, `footer-inspector-48.png` and `footer-launch-compact.png`.
These are actual PTY output rendered with Ioskeley, not native terminal
acceptance. The `polish-20261010` archives precede this navigation change and
must be rebuilt before a final release; the current development binary includes
it. Broader release and native acceptance gates remain open.

## NRP footer parity and contextual navigation · October 10

The NRP workspace now uses the same visible-shortcut geometry as Hummingbird,
factored into `internal/shortcuts`. Its footer can inspect a Job/Pod, open the
owned-Pod and container choices, switch current/previous output, reread, return,
open help and quit. Connection exposes explicit `c connect`, with compact labels
that retain Back. These clicks use the existing keyboard handlers and sign-in
requirements; hover cannot read logs or start authentication.

Command input owns its footer: Enter runs that command and Escape leaves input,
rather than advertising an inspector action that would not occur. A filtered
inspector says Escape returns, matching the preserved filter. The log snapshot
limit and picker hints now fit at 48 columns without duplicated mode-switch
instructions. Current/previous labels and output remain isolated.

Tests locate controls in the rendered footer and cover 48×18 through 120×38,
modal input, selection/filter retention, the full Job → Pod → container → logs
round trip, reverse navigation, current/previous output, pointer requests,
disabled/modified/right-click guards, connection return and click-to-quit
terminal restoration. The shared helper also has ANSI/wide-character geometry
and clipped-target tests. A synthetic HTTPS/credential-plugin test recovers
through the Connect footer and proves the pointer resets before foreground
sign-in. Real NRP access and native pointer appearance remain unverified.

## Explicit allocation-shell CLI · October 10

Added `perch shell JOB_ID`: a live read-only preview by default, with explicit
`--open` for foreground entry. The preview names the exact array task/allocation,
node, working directory, CPU/memory request and duration; it explains resource
sharing and the login-side environment. `--node` selects only a node currently
belonging to the allocation. Entry rechecks ownership, RUNNING state, submit /
restart / start identity, node list and directory. Collapsed arrays, unknown
identity, foreign jobs and stale attempts cannot authorize a step. The command
is described in `docs/ALLOCATION-SHELL.md` and the MCP guide; MCP still has no
shell execution tool. No automatic probe or background poll opens a shell.

The SSH interactive path explicitly replaces query-only TTY options, uses the
private master, keeps background authentication noninteractive and hands the
terminal to the child. Local mode uses the same step plan. Inherited scheduler
environment options are removed before execution. CPU/memory, one node/task,
GPU exclusion, MPI mode, five-second immediate-resource wait and step duration
are explicit. `timeout` on the login host provides a separate foreground bound
with a ten-second termination grace; a local timer reaps the child if it does
not return. Shutdown never issues parent-job cancellation.

Evidence and scope:

- `allocation-shell-live-20261010-r2.json`: owned synthetic job 802649 established
  TTY stdin/stdout, one-CPU affinity, login environment rather than batch runtime,
  Ctrl-C, normal exit and SSH disconnect without ending the parent allocation.
- `allocation-shell-cli-live-20261010.json`: production CLI entry/exit/termination
  passed, but an idle `srun --time=1` shell exceeded the 90-second observation
  bound. Job 802663 ultimately reported its step as TIMEOUT. Live configuration
  showed KillWait 60 seconds; cleanup waited through COMPLETING and was verified
  on follow-up. The failed result is preserved, not relabeled as passing.
- `allocation-shell-cli-live-20261010-r2.json`: final execution behavior on job
  802668 passed normal exit, Ctrl-C, terminating Perch and one-minute expiry.
  Step 2 was CANCELLED after 00:01:00 with the parent still RUNNING. Every probe
  verified its identity before cancelling only its own allocation, confirmed
  queue absence and removed its private temporary directory.
- `make check` passed (`allocation-shell-check-20261010.txt`). Unit tests cover
  task identity, owner/state/resource/node guards, changed attempts, literal argv,
  inherited scheduler options and local child deadlines. `make shell-test`'s
  offline harness checks the production CLI via fake SSH and a real PTY,
  including read-only preview, requeue refusal and SIGTERM child cleanup.
- MCP wire checks passed after adding the guide text
  (`allocation-shell-mcp-20261010.txt`). No new tool or MCP mutation was added.
- Production preview renders at 100 and 48 columns are in
  `output/terminal/shell-preview-{100,48}.png`, generated by
  `.venv/bin/python tools/test_shell.py --render`. The narrow pass uses word
  wrapping and separate resource rows. These are emulator renders in Ioskeley,
  not native terminal captures.

Remaining: TUI preview/entry/return and mouse navigation, native Terminal.app
acceptance, live multi-node selection and GPU inspection. The CPU probe is not
evidence for those scopes. Existing release archives predate this addition and
must be rebuilt and reverified before distribution. The overall build goal
remains open.

`make check` and the full `make nrp-ui-test` suite passed; Hummingbird footer and
hover PTYs passed after sharing the geometry. The final shortened log hint was
rechecked with `make check` and a focused NRP footer PTY. Evidence:
`output/validation/nrp-footer-check.txt`, `nrp-footer-full.txt`,
`nrp-footer-hummingbird.txt`, `nrp-footer-login.txt` and `nrp-footer-focused.txt`.
Reviewed `output/terminal/nrp-footer-jobs-48.png`, `nrp-footer-logs-48.png` and
`nrp-footer-connection-120.png`; these are emulator renders, not native captures.

One focused rerun exposed a harness race: it clicked input after seeing an
earlier title in a partially drawn initial frame. The test now waits for the
actual input. Failure cleanup now drains terminal output while terminating,
then reaps the child and closes descriptors, with a bounded kill fallback.
A real child that writes 128 KiB during termination verifies graceful cleanup;
the four emulator/cleanup tests passed (`nrp-footer-harness.txt`). No test child
was left running. No cluster workload, real credential or configuration was
changed by these tests. The earlier review archives predate these source changes.

## Allocation shells in the workspace · October 10

The selected running job now has an `o` / `/shell` action, a command-palette
entry, and a clickable inspector shortcut. The preview names the concrete job,
node, CPU/memory share, duration and working directory, and explains that the
login environment is distinct from the batch process. Enter, the primary
**Open shell** control or the footer opens only after fresh revalidation. Esc
cancels the preview; a late result cannot reopen it. `n` cycles allocated nodes.
The TUI uses one CPU, 64 MiB and ten minutes; custom resource settings remain
available through the CLI. Demo mode never starts a foreground process.

Preview requests own their sequence/cancellation and pause monitoring work while
visible. They use a copied client for identity checks, so refreshing and
identification do not race over the client's username. Entry uses the workspace's
existing SSH transport, including its private authentication socket and the
non-capturing diagnostics delegate. Exit restores the caller's view, panel
position and filter, then refreshes monitoring. Disconnected, pending and cached
unverified selections cannot authorize entry. A terminal smaller than the
supported 48×18 preview cannot activate Enter to open it.

The foreground runner is shared with the CLI. It saves the released terminal's
input modes and restores them after every child outcome. The synthetic SIGTERM
check exposed two issues: Bubble Tea ignores its own quit handling during a
foreground handoff, and reacquiring a TTY left raw by an abruptly terminated child
can save the wrong final state. A typed interruption now quits after handoff
cleanup, and explicit state restoration runs before Bubble Tea reacquires the
terminal. Ctrl-C typed inside the normal SSH shell still belongs to that shell.

Verification:

- `make check` passed (`shell-ui-check-20261010.txt`); the focused final race
  tests are in `shell-ui-final-unit-20261010.txt`. Checks include explicit entry,
  stale results, duplicate activation, modal input ownership, node cycling,
  cancellation, disconnected/pending selections and pointer geometry.
- The full keyboard/mouse/resize/demo/failure/navigation suite passed in
  `shell-ui-terminal-full-20261010.txt`. The subsequent compact-preview,
  primary-button and abrupt-shutdown fixes passed the focused production PTY
  suite (`shell-ui-focused-20261010.txt`) and CLI regression checks
  (`shell-ui-cli-regression-20261010.txt`). The synthetic shell test uses the
  real adapter/parser and renderer, but never connects to a cluster. It checks
  node-15 versus the original node-14, exact allocation 9101 for task 9100_39,
  requeue refusal, preview cancellation, pointer reset before handoff, Ctrl-C,
  error/normal return, child reaping and terminal modes after SIGTERM. Interactive
  contents do not appear in diagnostics captures.
- `shell-ui-hummingbird-live-20261010.json` records an actual TUI round trip on
  owned synthetic job 802690. Mouse preview/open entered step 0 on the compute
  allocation with TTY stdin/stdout. Ctrl-C interrupted the command, `exit`
  returned to the same inspector, the job filter survived, and final terminal
  attributes were restored. The parent was still RUNNING after shell exit.
  Accounting recorded the step COMPLETED. The harness then cancelled only its
  own identified test allocation, confirmed queue absence and removed its private
  directory. No unrelated workload was submitted to, modified or cancelled.
- MCP wire checks passed after the guide update (`shell-ui-mcp-20261010.txt`).
  Shell execution remains outside MCP tools.
- Reviewed Ioskeley renders: `shell-ui-live-preview.png` (synthetic),
  `shell-ui-demo-48.png`, `shell-ui-stale-48.png`, and
  `shell-ui-hummingbird-returned.png`. At 48×18, the node/resources/duration now
  appear immediately; long explanations remain scrollable. The action is a
  clickable primary control instead of an inert navigation caption. These are
  real PTY emulator renders, not native Terminal.app captures.

Remaining: native terminal appearance/interaction acceptance, live multi-node
and GPU shell scope, and the broader product/release gates above. Existing
release archives still predate these changes. The overall goal remains active.

## Command discovery and visible targets · October 10

The command palette previously offered job actions on Partitions and Connection,
then dismissed itself even when the requested view could not appear. Keyboard,
slash-command and palette actions now share a visible-job guard. A selection
retained behind a cluster tab cannot become the target of a job action.

- Unavailable job actions remain discoverable, with a dash and a reason. Enter
  or a mouse click leaves the palette open. The footer only offers Enter when
  there is an available result; empty searches cannot dispatch a command.
- Available job actions identify their selected job ID and name. Eligibility is
  recomputed from current state, including changes while the menu is open.
  Pending/finished or collapsed jobs cannot open an allocation shell. Cached
  array outcomes and arrays without confirmed failed IDs cannot offer copying.
- Inspect and Filter are searchable commands. Long labels truncate before their
  shortcut, so the invocation remains visible even at 48×18. Selected rows use
  a continuous background, including disabled labels and their trailing dash.
- Final `make check` passed in `output/validation/palette-context-final-check-20261010.txt`;
  final focused UI race tests passed in `palette-context-final-unit-20261010.txt`.
  Cases cover hidden/empty/changed targets, direct and slash shortcuts, pointer
  dispatch, pending/disconnected shells, filter recovery and compact labels.
- `tools/test_palette.py` passed at 120×38, 80×24 and 48×18, exercising real
  keyboard/SGR mouse input, unavailable-command retention, selected-job output,
  return with the same filter/selection, Inspect/Filter, empty searches and
  terminal restoration. Report: `palette-context-pty-20261010.txt`.
- The terminal regression run passed through receipt recovery, then stopped
  because the accounting-gap test expected the established phrase “Choose a
  confirmed task”; the new text had inserted “array”. The guard itself still
  refused the read. Restored the established wording and reran that case
  successfully (`palette-context-array-gap-20261010.txt`). The remaining eight
  suites also passed: array copying, 5,000-task navigation, preferences, cache,
  color mode, wait history, diagnostics and attempt identity
  (`palette-context-remaining-20261010.txt`). The original failed run is retained
  as `palette-context-terminal-20261010.txt`; this evidence comes from the
  initial run plus focused follow-up, not an uninterrupted green make target.
- Reviewed `output/terminal/palette-context-menu-120.png`,
  `palette-context-job-48.png` and `palette-context-unavailable-48.png`.
  These use the Ioskeley PTY renderer; native terminal acceptance is separate.

README describes the implemented behavior. No cluster jobs or other remote
mutations were needed for this navigation change. Release archives still need
rebuilding after source changes; this is not a release-ready declaration.

## First connection and destination choice · October 10

Interactive first launch now opens a destination form when neither saved
preferences nor an explicit flag selects a host. `perch setup` opens the same
form on demand, prefilled with the configured destination. Demo, local mode,
CLI queries and MCP do not enter automatic setup. Saved hosts and explicit
`--host` invocations continue directly to their workspace.

The form accepts a complete SSH destination or alias without guessing a user or
rewriting aliases. Enter or the primary mouse target opens the workspace.
Remember is off by default; opting in atomically edits only the host preference.
`--no-config` has no persistence control. Escape closes without connecting. SSH,
diagnostic capture and the monitoring model are created only after acceptance,
so there is no old workspace identity or cache to carry into a new connection.
Passwords remain in the existing SSH sign-in path (`c` after workspace entry).
Connection now explains how to choose a different destination.

Evidence:

- `setup_test.go`: input validation, rejected multiline/control paste, rendered
  pointer geometry at 140×40/80×24/48×18, nonmutating hover, duplicate-save guard,
  persistence completion/error, unrelated preference retention, no-mouse,
  no-color, minimum size and cancel. `make check` passed
  (`output/validation/setup-check-final-20261010.txt`); focused final UI/CLI
  race tests passed after the footer refinement (`setup-final-unit-20261010.txt`).
- `tools/test_setup.py` passed with an isolated HOME and synthetic SSH, verifying
  zero SSH invocations before acceptance, exact destination forwarding, private
  saved files, persistence across process restart, explicit-host bypass,
  session-only choice, edited-destination cancellation, save-error recovery,
  no-mouse, resize, noninteractive rejection and terminal-mode restoration.
  Final report: `setup-pty-final-20261010.txt`. The initial run correctly reached
  the chosen destination but failed because the test SSH fixture only allowed
  its old bare hostname; its allowlist now explicitly names the synthetic
  destinations used in this test. That failure is retained in
  `setup-pty-20261010.txt`.
- Read-only live check: `tools/test_setup.py --live-host ndanday@hb.ucsc.edu`
  accepted the choice, connected as ndanday to Slurm 25.11.4, navigated Connection
  and Jobs, then restored terminal modes on exit. It used `--no-config`, did not
  change saved preferences and submitted no jobs. Evidence:
  `output/terminal/setup-hummingbird-results.json` and
  `output/validation/setup-hummingbird-20261010.txt`.
- Existing preference, cache/reconnect, failure recovery, help, compact layout
  and terminal-emulator tests passed (`setup-regressions-20261010.txt`). MCP wire
  checks passed; setup is not a protocol tool and never runs on MCP startup.
- Reviewed Ioskeley PTY renders: `setup-compact-48.png`, `setup-wide-120.png`,
  `setup-remember-80.png`, `setup-save-error-80.png`, plus live Hummingbird choice
  and connected screens. Fixed a clipped input edge and made the footer explain
  Space when Remember has focus. Native Terminal.app appearance and manual
  password/MFA entry were not verified by these tests.

README and the embedded agent guide describe setup, its persistence boundary,
and the separate SSH sign-in step. Release candidates still need rebuilding
after current changes; broader acceptance gates remain open.

## Cancellation scope investigation · October 10

This is platform evidence for the next explicit job-management action, not a
shipped cancellation command or UI. The design and remaining implementation
gates are in [CANCELLATION.md](CANCELLATION.md).

- Added `tools/test_cancellation_scope_live.py`, an opt-in harness that accepts
  no existing workload ID. It requires a fresh report, saves submission intent
  and receipt, verifies owner/name/unique comment/array identity before actions,
  and finally cancels only its own remaining tasks. No automatic resubmission
  follows an ambiguous receipt. Python compilation passed.
- Held array **802749**: cancelling `_0` through `scancel --ctld --user=ndanday
  --state=PENDING` changed its collapsed pending range from `0-1` to `1`; the
  sibling stayed held. Cleanup confirmed queue absence. No task-0 accounting
  record was observed, even in a later `sacct --array` query; task 1 was recorded
  CANCELLED with zero elapsed time.
- Running array **802755**: `_0` was allocation 802758, while `_1` used allocation
  802755—the parent array's numeric ID. Cancelling the logical `_0` selector
  removed just that task; `_1` stayed RUNNING. Accounting recorded task 0
  CANCELLED after two seconds. Cleanup cancelled task 1 after six seconds and
  verified queue absence. Each task requested one CPU/64 MiB, with a two-minute
  limit; no GPU or remote installation was used.
- Reports: `output/validation/cancellation-scope-hummingbird-20261010.json` and
  `cancellation-scope-running-hummingbird-20261010.json`. Both harness processes
  exited successfully; no probe remains queued. These are direct Slurm scope
  checks, not production cancellation-adapter acceptance.

The implementation must preserve logical array-task selectors, keep explicit
whole-array scope separate, and distinguish request acceptance from a recorded
terminal outcome. In particular, it must not reuse the numeric allocation ID
used by the shell adapter or synthesize CANCELLED accounting from disappearance.

## Production cancellation CLI · October 10

The shared cancellation adapter and `perch cancel` CLI are implemented. Preview
is read-only; `--confirm` sends one request after a fresh ownership, cluster,
attempt and scope comparison. Ordinary jobs, logical array tasks, and explicit
`--array` targets have separate semantics. Cached rows and history do not
participate in authorization. The opaque review expires after two minutes and
can be consumed once, including under duplicate concurrent calls. The final
command clears inherited scheduler filters, selects the verified cluster and
owner, and preserves array-task selectors even when a task allocation ID equals
the parent. Receipts distinguish requested, unconfirmed and unknown outcomes;
there is no automatic mutation retry or invented final scheduler state.

Validation:

- `cancellation-cli-check-final-20261010.txt`: full race suite, vet and formatting
  passed after the production implementation and identity refinements.
- `cancellation-cli-final-scopes-20261010.txt`: focused race tests after adding
  scrubbed real-scope replay. Includes changed/reused IDs, restarts, missing
  identity, scope changes, huge and overlapping ranges, unsupported federation
  and heterogeneous records, duplicate confirmation, environment injection,
  failed commands and uncertain transport outcomes.
- `cancellation-cli-render-20261010.txt`: production CLI through synthetic SSH,
  exact command arguments, preview/no mutation, JSON, nonzero exit and timeout
  receipts, plus 48/80-column PTY wrapping and unchanged terminal attributes.
  Reviewed Ioskeley emulator images: `output/terminal/cancellation-preview-48.png`
  and `cancellation-preview-80.png`; these are not native Terminal.app captures.
- `cancellation-cli-mcp-20261010.txt`: protocol checks passed after adding CLI
  guidance to the embedded resource. No mutating MCP tool was added.
- `cancellation-cli-production-live-20261010.json`: held/running ordinary jobs
  802778/802779 and held/running arrays 802780/802781 passed using the production
  command. Both array tests retained sibling 1 after task-0 cancellation, then
  removed the remainder with explicit whole-array scope. Independent final
  queue/accounting evidence is in `cancellation-cli-live-followup-20261010.json`.
- `cancellation-cli-mixed-live-20261010.json`: final-binary whole-array action
  removed both the running and queued task of array 802788. Accounting recorded
  both CANCELLED; cleanup independently confirmed absence.

The first held-job harness run 802776 accepted cancellation but failed to
recognize Slurm's `Zero jobs to dump` empty-queue warning. Its failure report is
preserved. The separate `cancellation-cli-held-followup-20261010.json` confirms
absence and CANCELLED accounting; subsequent probes used corrected warning
handling. No probe remains active. No pre-existing workload was cancelled.

Remaining: TUI preview/confirmation, keyboard and mouse navigation, return
context, request-in-flight/receipt views, demo behavior, and native acceptance.
The CLI does not establish those gates. Grouped-task cancellation is constrained
to the tested Slurm 25.11.4 release. The read/mutation interval cannot be made
atomic with this interface. See [CANCELLATION.md](CANCELLATION.md) for full scope
and unsupported cases. Release archives still need rebuilding for these changes.

## Production cancellation workspace · October 10

The TUI now exposes `/cancel` for a selected current job or concrete array task,
`/cancel-array` for explicit whole-array scope, and `/last-cancel` for this
session's latest receipt. The command palette shows the same actions and their
availability. Review checks the selected attempt against a fresh queue before
showing scope, owner, destination and consequences. Ctrl+X or the labelled
button confirms; Enter never confirms. Overflow must be reviewed first.

While a request is in flight, duplicate input and navigation cannot submit
again or discard the receipt. Explicit Ctrl+C exit restores the terminal and
prints an uncertain-outcome message. Requested, unconfirmed and unknown
receipts remain separate from scheduler state; refresh supplies the actual
queue/history state. Returning to an unchanged selection preserves filter and
scroll. When the selection disappears or its attempt changes, old logs and
resources are cleared before returning to the refreshed job list. Demo sends
nothing and does not pretend to change scheduler state.

Validation:

- `output/validation/cancellation-ui-production-live-20261010.json`: real TUI
  reviews and pointer confirmation on Hummingbird passed for held job 802802,
  running array 802803, and mixed running/pending array 802813. Cancelling task
  802803_0 retained its running sibling before an explicit whole-array request.
  All three cases verified final queue absence and recorded accounting. Only
  newly submitted, bounded synthetic probes were affected; none remained active
  at the final cleanup observations.
- `cancellation-ui-terminal-all-20261010.txt`: the full terminal suite passed,
  covering navigation, hover, compact layouts, logs, launch, shells, cancellation,
  reconnect, cache, settings, receipts and array cases. This preceded the last
  return-context fix and receipt-title refinement.
- `cancellation-ui-final-process-20261010.txt`: rebuilt production binary passed
  the CLI and TUI process tests after those last changes, including 140×40,
  80×24 and 48×18 reviews, stale preview replies, scope change, duplicates,
  uncertain receipts, forced exit and terminal restoration. SSH was synthetic.
- `cancellation-ui-check-r2-20261010.txt`: final full race suite, vet and
  formatting passed. `cancellation-ui-final-unit-r3-20261010.txt` additionally
  records targeted return-context coverage for a different job, a new attempt
  with the same ID, disappearance and unchanged paused-log scroll.
- `cancellation-ui-mcp-20261010.txt`: MCP wire checks passed with the embedded
  usage guidance. MCP remains read-only.
- Reviewed Ioskeley emulator renders include
  `output/terminal/cancel-ui-hummingbird-running-array-802803_0-2-review.png`
  and `output/terminal/cancel-ui-unknown-48.png`. These establish the reviewed
  PTY layout, not native Terminal.app visual acceptance.

Earlier failures are preserved: a compile error in an initial test, a compact
review overflow/demo array-count defect corrected before passing process runs,
and a return-context test that changed only the ID without normalizing the
selected row. The corrected test covers the actual model transition.

Remaining: native appearance/interaction acceptance; the TUI does not yet select
one unmaterialized task from a grouped pending range (the CLI supports that on
tested Slurm 25.11.4). Cancellation receipts are session-local plus exit text,
not a durable journal. Release archives predate this work and require rebuilding.
The broader readiness gates remain open; this milestone is not full-goal or
release acceptance.

## Packaged workspace acceptance · October 10

The `workspace-20261010` candidates replace the earlier `polish-20261010`
artifacts for review. They contain the current setup screen, allocation shells,
cancellation reviews and receipts, footer controls, navigation fixes, CLI and
embedded MCP guide. README now describes the workload preparation and action
workflows as well as monitoring. This is a local candidate, not a publication.

The portable runtime harness now also checks offline array preparation, shell
review, cancellation at 48×18, Enter not confirming cancellation, demo receipts
and receipt reopening. Each binary runs both workspaces with CSS pointers, X11
pointers and mouse disabled, then verifies terminal attributes, alternate-screen
restoration and pointer reset. These are real PTY processes using synthetic
workloads; they do not establish actual remote mutations or native appearance.

Runtime results:

| Target | Execution environment | Evidence |
| --- | --- | --- |
| macOS ARM | Native on this Mac | `release-workspace-runtime-darwin-arm64.json` |
| macOS x86 | Existing Rosetta compatibility | `release-workspace-runtime-darwin-amd64.json` |
| Linux x86 | Native Hummingbird compute job 802838 | `release-workspace-native-linux-amd64.json` |
| Linux ARM | Existing OrbStack/course-image compatibility environment | `release-workspace-runtime-linux-arm64.json` |

All paths above are under `output/validation`. The Linux ARM container used the
existing x86 course image, no network, no image pull, a read-only project/root
filesystem and a temporary `/tmp`. Its Python interpreter reports x86; the
harness independently identifies and hashes the ARM ELF it executes. This is
compatibility evidence, not native ARM hardware acceptance. The disposable
container was removed and OrbStack was stopped after testing.

`tools/test_release_live.py` stages only the executable and two standard-library
test scripts plus their wrapper in a private home directory. The explicit
`--submit` run requested one CPU, 256 MiB and three minutes. Job 802838 completed
with exit `0:0` in 18 seconds, the result's executable hash matched the archive,
and the staging directory was removed after collection. No software was
installed, and no pre-existing workload was modified. The test uses demo data
inside the allocation; it is packaged-runtime acceptance, not another live
Slurm action test. The receipt records queue absence and accounting separately.

Packaging checks verify all four archive checksums, executable metadata,
documentation and examples against source, internal links, original Chip assets,
and dependency notices against each binary's embedded Go module graph. Negative
notice tests reject changed license text and stale dependency versions. Docs
were refreshed after runtime testing and repackaged; the final summary compares
each archive's binary hash to the corresponding runtime report, so documentation
updates cannot silently substitute an untested executable.

Evidence: `release-workspace-build.txt`, `release-workspace-build-final.txt`,
`release-workspace-archives-final.json`, `release-workspace-notices.txt`, and
`release-workspace-summary.json`, plus the runtime reports above. The prior
full race/vet/format and terminal regression results remain recorded in the
cancellation milestone; this packaging-only change does not replace them.

Remaining: native font/Chip/pointer appearance and interaction acceptance,
real interactive authentication/MFA and notification delivery, actual NRP access,
project/artwork licensing, and the broader requirement audit. Runtime and archive
checks alone do not close those gates or declare the full goal complete.

## Discoverable job actions · October 10

The Jobs and History selection footer now exposes **a actions**, also reachable
through `a`, `/actions`, or the general command palette. A compact card groups
output, scripts, resource measurements, allocation-shell review, failed-task
copying and cancellation reviews. On a wide terminal the card stays 70 columns
wide; at 48×18 it uses the inactive command bar's space to show five actions.
Typed filtering, keyboard selection, mouse targets and scroll position share
the same layout calculation. The footer says **review** for shells/cancellation,
**copy** for failed IDs, and **open** for read-only views.

The card names its opening job and remains pinned to that attempt. If refresh
replaces the selected job, restarts it, or removes it, activation is refused
with an explanation until the menu is closed and reopened. Normal availability
checks remain shared with shortcuts and the general palette. Selecting an action
never bypasses a shell or cancellation review. Escape preserves the underlying
view, filter, selection and paused output position. Ctrl+k opens the full
workspace palette independently of the last job-menu scope.

The design pass reviewed the initial oversized frame and one-row minimum-size
layout, then replaced them with the bounded card and five-row compact version.
Final Ioskeley emulator images are under `output/terminal/job-actions-menu-140.png`,
`job-actions-menu-48.png`, `job-actions-resized-last.png` and
`job-actions-unavailable-80.png`. These are actual PTY renders; native visual
acceptance remains separate. Chip's artwork and animation are unchanged.

Validation so far: `output/validation/job-actions-card-check.txt` passed the
full race suite, vet and formatting. `job-actions-card-unit.txt` covers exact
row hit geometry, hover without activation, missing/replacement/restarted job
guards, contextual availability, review-only activation and paused-log return.
`job-actions-card-pty.txt` passed wide, medium and minimum-size flows, resizing
while the menu is open, keyboard and pointer activation, filtering, a whole-array
review without sending cancellation, unavailable actions and terminal cleanup.
`job-actions-terminal-full.txt` passed the full terminal regression suite on
the preceding menu layout; the final compact-card geometry has the targeted
follow-up checks recorded below.

The earlier `workspace-20261010` packaged candidates remain valid evidence for
their recorded executable hashes, but predate this new menu. Use the rebuilt
development `bin/perch` for the current design; repackaging remains part of the
final delivery audit. The overall goal is still active.

Final rebuilt-binary checks passed: `job-actions-card-final-pty.txt`,
`job-actions-palette-regression.txt`, `job-actions-footer-regression.txt`,
`job-actions-hover-regression.txt`, and `job-actions-mcp.txt` under
`output/validation`. These cover the final five-row compact geometry, existing
global palette, clickable workspace controls, CSS/X11 pointer restoration,
mouse-disabled fallback and the updated embedded guidance. No live workload was
submitted for this menu change; the existing shell/cancellation adapters and
their explicit confirmation paths are reused.


---

## Archived readiness audit before scheduler-preview consolidation

Preserved verbatim on October 10, 2026 after verification of the scheduler preview.
The wording below describes earlier checkpoints; use READINESS.md for current status.

# Perch readiness

Current audit: October 10, 2026. **The build goal remains active.** Perch has a
working Hummingbird workspace, CLI, read-only MCP server and explicit workload
actions. Automated checks and bounded live tests cover substantial behavior;
they do not establish complete native visual, authentication or notification
acceptance. NRP is an experimental, partially implemented stretch goal.

Development after the workspace candidate adds clearer pending-job guidance and
more reading space in compact inspectors. The archived candidate below remains
unchanged; its runtime evidence does not cover these later presentation changes.
Focused evidence is in `output/validation/pending-guidance-summary.json`.
Further development adds scoped queue GPU requests to job details, summaries,
CLI/MCP output and optional cached metadata. Evidence is recorded in
`output/validation/job-gpu-summary.json`; historical requests remain unknown
when accounting omits them. This is request visibility, not GPU-shell acceptance.
The subsequent summary pass preserves whole GPU constraints at narrow widths
and shows a counted overflow hint when height is limited; its focused resize,
pointer-navigation and render evidence is in
`output/validation/gpu-summary-wrap-summary.json`.
Priority reports now include separately observed controller fair-share weight in
the TUI, CLI and MCP. Optional configuration failures preserve job scores and
leave the weight unknown. Compact priority reading reclaims the inactive input
rows. See `output/validation/priority-policy-summary.json` for current unit,
protocol, navigation and live read-only evidence; this development is also
newer than the archived candidate.

The next local candidate is `preview-20261010-scheduler`, incorporating the
pending guidance, scoped GPU requests, complete summary wrapping and observed
fair-share policy described above. Its validation is recorded separately in
`output/validation/scheduler-release-summary.json` when available. Do not infer
runtime acceptance from packaging or from the preceding candidate's checks.

The preceding verified local candidate is `preview-20261010-workspace`. It includes the
navigation candidate below plus module-command copy, static-reader and resize
fixes, script availability and search, job sorting/resource columns, and NRP
Job dry-run validation with explicit recovery outcomes. Candidate verification
is recorded outside the archives in `output/validation/workspace-release-summary.json`;
match its binary hashes before relying on a platform's runtime result. Linux ARM
runtime, native visual/authentication/notification acceptance and real NRP access
remain open. The dated development sections below retain their original
checkpoint scope; the navigation archives are historical, not this candidate.

The `preview-20261010-navigation` local candidates include retained-output search,
compact log reading, node GPU assignments/filtering and offline NRP Job manifests,
alongside experimental NRP Pod metrics, job-draft correction, software browsing
and the existing Hummingbird reports and workload actions.
Mac ARM, Mac Intel through Rosetta, and native Linux x86 passed the expanded
candidate runtime checks. Linux ARM is cross-built and archive-verified; this
candidate has no Linux ARM runtime result. Evidence is bound to exact binary
hashes. Nothing has been published, and no project or artwork license has been
selected. The older `preview-20261010` and `preview-20261010-polish` reports remain
historical evidence for those earlier executables.

The focused development evidence below predates the newest candidate packaging.
The final candidate section binds runtime results to its exact packaged hashes;
older packaged-runtime reports do not cover the new features.
Development after this candidate adds copying a module's load command from its
detail view, cached wrapping for long static text, compact script reading,
a resize repaint and clearer script availability/errors. These changes have
separate focused evidence below and are not
in the packaged `preview-20261010-navigation` executables.
The submitted-script reader also now supports local find, match navigation and
clickable search controls, sharing the output reader's search implementation.
Jobs and History now have independent session sort choices, clickable headings
and resource columns when sorting by CPUs or requested memory.
Experimental NRP development also adds an explicit server Job dry-run command;
its local TLS coverage does not establish live NRP access or Pod admission.

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
| Beautiful, calm interface inspired by Claude Code and superfile | Warm charcoal/cream/peach, top tabs, persistent command input, rounded boxes, responsive lists and inspectors; Ioskeley previews. `internal/ui`; latest visual/process evidence below. User accepted the border fix and described the 12-pt tight Chip view as “quite good.” | Final interactive visual review of current screens. The wordmark remains deliberately deferred. Quality comparable to the inspirations is not proved by passing tests. |
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
| Mac/Linux executable and eventual distribution | Four current candidate archives, checksums, doc/artwork comparison, binary target/dependency validation and runtime hash matching. Current Mac ARM and Linux x86 tested natively; Mac Intel tested under Rosetta. Older Linux ARM/OrbStack evidence applies only to that older binary. | Current Linux ARM runtime acceptance; repackage after further source changes; select licensing before publication. Public repository, GitHub release and Homebrew tap are not created or published. |
| NRP support (stretch) | Explicit namespace/context, typed Job/Pod/container identity, bounded HTTPS reads and log snapshots, explicit credential handling, CLI and namespace UI. Synthetic TLS/plugin/PTY coverage. See [NRP](NRP.md). | No configured context was found in the fresh local check. Quota reads, exact quantities and partial failures now have synthetic CLI/HTTPS/PTY coverage. On-demand Pod CPU/memory metrics also have synthetic parser, CLI, HTTPS and PTY coverage. Real namespace/RBAC/authentication and Metrics API acceptance, real fixtures and reviewed workload actions remain open. |

## Current checks and their limits

Evidence paths below are relative to the repository, not files bundled into
every release package. A passing log is cited only for what its harness tests.

- `output/validation/job-actions-card-check.txt`: full Go race suite, vet and
  formatting passed for that application checkpoint. Test definitions live
  alongside the implementation; live opt-in tests are not implied by this run.
- `job-actions-terminal-full.txt`: full terminal suite passed on the preceding
  job-menu geometry. The final geometry and rebuilt executable passed
  `job-actions-card-final-pty.txt`, `job-actions-palette-regression.txt`,
  `job-actions-footer-regression.txt`, `job-actions-hover-regression.txt` and
  `job-actions-mcp.txt`, all under `output/validation`.
- `output/terminal/job-actions-menu-140.png`, `job-actions-menu-48.png`,
  `job-actions-resized-last.png` and `job-actions-unavailable-80.png`: reviewed
  final Ioskeley emulator renders. The minimum screen displays five actions;
  a replacement/restarted job cannot become an open menu's target.
- `output/validation/readiness-current-chip.txt`: fresh all-state relative
  cadence, pause, aligned stepping, single running sample and terminal cleanup
  passed. `assets/bird/artwork_test.go` protects detailed originals and
  `octant_test.go` covers cell widths, footprint and static frames.
- `output/validation/readiness-current-hummingbird.txt`: fresh read-only probe
  and refresh passed: Slurm 25.11.4/v0.0.44, zero queue records, 117 accounting
  records and three partitions. One completed job provided 768 output bytes and
  a 1,137-byte accounting-backed script. Content was not printed. **No running
  job existed**, so this run did not exercise live resources or incremental logs.
- `output/validation/readiness-current-nrp-access.json`: local context listing
  succeeded with zero configured contexts; the OIDC login plugin was not found
  on PATH. No cluster was contacted and no configuration changed. This does not
  establish whether the user already has an account or namespace membership.
- `output/validation/cancellation-ui-production-live-20261010.json`: actual
  TUI reviews and confirmation on new held job 802802, running array 802803 and
  mixed-state array 802813. Single-task scope preserved its sibling; final
  queue checks and accounting were recorded. All probes were removed.
- `output/validation/release-workspace-summary.json`: all four final candidate
  hashes match their runtime reports. Native Hummingbird job 802838 completed
  in 18 seconds with exit 0 and its staging directory was removed. The job used
  demo data; it is not a live scheduler-action test. OrbStack was stopped after
  the compatibility check. These packages predate the job-menu change.

Earlier launch, log lifecycle, resource, notification-reconciliation, shell,
transport, NRP and parser observations remain in the validation history and
feature-specific documents. A historical note without a retained process result
is weaker evidence than an inspected result; it must not silently close a new
acceptance requirement.

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
4. **NRP stretch goal.** Pod metrics now have synthetic parser/CLI/TUI coverage;
   continue workload-design work and complete
   actual namespace authentication and permission checks when access is available.
   No kubeconfig, context or credential was changed during this audit.
5. **Delivery.** Current preview candidates are checked below; repeat packaging
   and artifact/runtime identity checks after further accepted source changes.
   Complete current Linux ARM runtime acceptance. Resolve project/artwork licensing before
   publication. Publishing or contacting administrators requires authorization;
   neither has been inferred from the build goal.
6. **Final completion audit.** Revisit every row above and the authoritative
   architecture/feature documents. Unverified native behavior, partial NRP work
   and unimplemented requirements cannot be closed by a clean build.

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

## Two-node CPU shell acceptance, October 10

- `shell-capacity-after-preview.json` recorded the changed CPU/memory allocation
  context before a new bounded submission. Job **803078** obtained node-19 and
  node-22. `shell-two-node-hummingbird-20261010-c.json` binds results to the tested
  development executable's hash and verifies both CLI previews, outside-node
  refusal, CLI entry on the second node and TUI mouse entry on both nodes.
- All three shell steps completed with exit 0. Ctrl-C returned to the shell;
  exit restored the prior inspector/filter and terminal attributes. The parent
  remained running until explicit harness cleanup, then was cancelled. Queue
  absence and remote directory removal were verified.
- Reviewed `shell-ui-hummingbird-preview-node-2.png` and
  `shell-ui-hummingbird-returned-node-2.png` under `output/terminal`. These are
  real PTY emulator renders. Native visual/pointer acceptance and GPU behavior
  remain open. Earlier queued/not-exercised reports are preserved unchanged.

## Previous preview packages, October 10

- `output/validation/release-preview-summary.json` binds all four archive and
  binary hashes to their available runtime results. `release-preview-archives.json`
  checks current documentation/artwork bytes, dependency notices, executable
  mode, safe archive members and each binary's actual OS/architecture. Negative
  checks reject a wrong target, changed notice bytes and stale dependency versions.
- `release-preview-runtime-darwin-arm64.json` passed natively;
  `release-preview-runtime-darwin-amd64.json` passed under Rosetta on this ARM Mac.
  The portable test covers newer reports, Job actions, Activity, warning settings
  and NRP quotas alongside CLI, 15-tool MCP, review flows and PTY restoration.
- `release-preview-native-linux-amd64.json`: native Hummingbird compute job
  **803074** completed in 24 seconds, exit 0, no stderr; exact binary hash matched
  and staging directory cleanup succeeded. This is an offline synthetic-data
  runtime check on a compute node, not a live scheduler mutation acceptance test.
- `release-preview-setup.txt`: extracted Mac ARM executable passed first-run
  destination entry, explicit preference persistence, cancellation, error
  recovery and 48-column layout with synthetic SSH and isolated preferences.
  Reviewed `output/terminal/setup-remember-80.png` and `setup-compact-48.png`.
- Current Linux ARM runtime is **not tested**. OrbStack was not started. Native
  fonts, cursor appearance, interactive MFA and desktop banners remain separate
  acceptance requirements. These local previews are not a public release.

## Activity validation, October 10

- `output/validation/activity-check.txt`: full race suite, vet and formatting;
  includes failure/cancellation/finish rendering, every origin tab, time-warning
  navigation, selection retention, 100-event bound, unavailable/restarted attempts.
- `output/validation/activity-terminal-full.txt`: full terminal suite passed,
  including new `tools/test_activity.py` at 140×40, 80×24 and 48×18. Existing
  keyboard/mouse, job actions, shell/cancellation, setup, logs, cache and preferences
  checks remained green. No live workload was submitted for this UI change.
- Reviewed `output/terminal/activity-warnings-140.png`,
  `activity-finished-80.png` and `activity-warnings-48.png`: actual PTY emulator
  renders, not native Terminal.app screenshots. Chip and its assets are unchanged.
- The feed adds no cluster polling or on-disk activity storage. Notification
  settings are independent. CLI/MCP history retains its scheduler-history meaning.


## Time-warning validation, October 10

- `output/validation/warnings-check.txt`: full race suite, vet and formatting.
  `internal/timewarning`, `app/time_warnings_test.go`, UI/preferences/config tests
  and notification-message tests cover limits, measured time, crossing/dedup,
  preference edits, old checkpoint keys, restart/requeue/limit changes and
  checkpoint compaction across 5,000 jobs with five thresholds.
- `tools/test_warnings.py`: real PTY at 140×40, 80×24 and 48×18, typed validation,
  draft cancellation, mouse Apply/Save, resize, session off, isolated explicit
  persistence, flag/no-config precedence and private permissions. Its SSH is
  synthetic; no live cluster workload or desktop notification is involved.
- Reviewed `output/terminal/warnings-editor-140.png` and
  `warnings-editor-48.png`; these are emulator renders. The form uses a centered
  card, preserves the calling view/filter, and does not change Chip or fonts.
- Activity records and desktop warning messages include measured remaining time.
  Job highlights use the configured policy; compact remaining-time copy says
  recorded. These observations are never advanced by a wall-clock countdown.

- `output/validation/warnings-terminal-full.txt`: full terminal regression suite
  passed. That run precedes only the final measured-remaining activity label,
  compact recorded-time label and save-error wording; targeted final-binary
  warnings/activity/glance checks are recorded separately.

- Final rebuilt executable: `warnings-mcp.txt`, `warnings-pty.txt`,
  `warnings-activity.txt` and `warnings-glance.txt` passed the MCP protocol and
  focused CLI/PTY regressions after those final display-copy changes. The latest
  `activity-warnings-48.png` was also reviewed for measured remaining-time copy.


## Multi-node shell probe, October 10

`output/validation/shell-two-node-hummingbird-20261010.json` records a new,
owned two-node CPU probe, job 802978. It stayed PENDING / Priority during the
70-second observation window; no shell was opened. Accounting confirmed
CANCELLED with zero runtime and no assigned nodes. The harness verified queue
absence and removed its temporary directory. The original report's `failed`
stage is preserved; this result does not demonstrate a shell implementation
failure or close the live multi-node gate.

The harness now supports `--nodes 2 --perch-tui`, an explicit bounded
`--wait-seconds`, recorded scheduling reasons and a `not-exercised` result for
future queue-wait expirations. On allocation it checks actual hostnames for
CLI node selection and both TUI node choices; these new paths have not yet
executed live. The concurrent partition snapshot reported all four GPUs in use;
no GPU probe was submitted. No automatic resubmission is scheduled.


## Namespace quota validation, October 10

- `internal/nautilus/quota.go` and `quota_test.go`: typed ResourceQuota identity,
  scope selectors, exact Kubernetes quantities, missing versus zero usage,
  over-limit/at-limit state, controller lag and bounded malformed input. The
  existing complete-list limits, pagination and HTTPS scope checks also apply.
- `output/validation/nrp-quota-final-check.txt`: full Go race suite, vet and
  formatting passed after the compact-header refinement. The official quantity
  dependency is pinned and its runtime notices are included.
- `output/validation/nrp-quota-final-nrp.txt`: full NRP CLI/HTTPS/plugin/PTY
  regression passed before only the final compact header and stale-status label
  changes. `nrp-quota-compact-pty.txt` checks the final executable at 140×40,
  80×24 and 48×18, including quota warnings visible without initial scrolling,
  every scoped/unknown/updating section, resize and filtered-list return.
- Reviewed `output/terminal/nrp-quota-140.png`, `nrp-quota-80.png` and
  `nrp-quota-48.png`. These are Ioskeley emulator renders from a real PTY, not
  native Terminal.app acceptance or live NRP namespace data.
- `output/validation/nrp-quota-final-https.txt`: the final executable passed
  the local TLS/API/credential-plugin process suite, including quota permission
  loss, retained stale values, recovery and unchanged global configuration.
- No NRP account, context, RBAC or live quota acceptance is implied. Metrics and
  workload actions remain open. The Hummingbird shell breadth gate above is
  independent of this implementation.

- Final executable regression: `output/validation/nrp-quota-terminal-full.txt`
  completed the full Hummingbird terminal suite; `nrp-quota-final-pty.txt`
  passed the final NRP quota navigation/render checks; `nrp-quota-mcp.txt`
  passed stdio protocol checks. `nrp-quota-final-notices.txt` verified all four
  runtime dependency graphs (42 components / 58 notice texts). The executable
  checksum is saved in `nrp-quota-binary.sha256`. Release archives were not
  rebuilt or published by this change.


## Priority report validation, October 10

- The production development executable includes TUI `/priority`, CLI `priority`
  and the fourteenth read-only MCP tool, `perch_priority`. The parser bounds and
  validates explicit `sprio` fields and retains the reported total independently
  of potentially rounded components. No recurring priority polling is added.
- `output/validation/priority-check.txt`: full Go race suite, vet and formatting
  passed. The later test-only additions passed focused race checks in
  `priority-recovery.txt` and `priority-rounding.txt`.
- `priority-pty.txt`: actual PTY checks passed at 140×40, 80×24 and 48×18 for
  command/palette entry, score rendering, scroll, refresh/back clicks, resize,
  hidden-selection guards and returning to filtered History. The compact guard
  message was shortened after its first failed run exposed clipping.
- `priority-mcp.txt`: built-binary protocol discovery, structured results,
  priority pagination and EOF cleanup passed. This is synthetic demo evidence,
  not a specific desktop MCP client's UI acceptance.
- `priority-empty-live.json`, `priority-held-live.json` and
  `priority-eligible-live.json` record production CLI reads on Hummingbird. The
  held probe returned no factors; the eligible probe returned score 10010022.
  Both owned probes were cancelled with queue absence verified. The follow-up
  `priority-probe-accounting.txt` was empty; accounting does not add confirmation
  of an execution or terminal state here.
- Native visual review remains open. Release archives have not been rebuilt for
  this change; `bin/perch` is the development executable under validation.

- `priority-terminal-full.txt`: full terminal regression suite passed, including
  setup, navigation, hover, help, logs, workload review, shells/cancellation,
  preferences, cache, diagnostics and attempt identity. This preceded only the
  final removal of duplicate report shortcuts and timestamp styling.
- Final source passed `priority-final-check.txt` (race suite, vet, formatting),
  `priority-final-pty.txt` (all three sizes and report navigation), and
  `priority-final-mcp.txt` (built-binary wire protocol). The development binary
  hash is retained in `priority-binary.sha256`.
- Reviewed final `output/terminal/priority-{140,80,48}.png` and
  `priority-bottom-48.png`. The wide view now fits its own observation timestamp;
  compact views retain scroll indicators and fully visible guarded-action
  feedback. These are Ioskeley emulator renders of PTY output, not native captures.


## Additional live acceptance, October 10

- `priority-ui-live.json` and `priority-ui-live.txt`: production TUI/SSH loaded
  the real pending probe's factors, then Escape returned to Jobs and terminal
  cleanup passed. Reviewed `output/terminal/priority-hummingbird-live.png`, an
  emulator render of the actual live session, not a native Terminal screenshot.
- `priority-mcp-live.txt`: the actual command entrypoint and official SDK client
  returned one live priority record through `perch_priority`, with clean shutdown.
  Live MCP tests now have named subtests so a single capability can be checked
  without repeating unrelated cluster reads.
- `shell-two-node-hummingbird-20261010-b.json`: second bounded two-node probe,
  job 803043, remained pending for Priority for 180 seconds. Classified
  `not-exercised`; cancelled with zero runtime, queue absence and remote-directory
  cleanup verified. Multi-node shell acceptance remains open. No GPU probe ran.
- `shell-capacity-current.json` and `shell-two-node-scheduler-context.txt` retain
  the capacity/estimate observation. Idle CPUs are not a scheduling promise;
  allocated memory and scheduler policy still matter.


## Report return navigation, October 10

- Reports now retain a reading bookmark for Partitions and Connection, alongside
  the existing job-list identity/filter return. Refreshing or opening a second
  report preserves the original caller. Nearby unique text is used after reflow;
  missing content falls back to the previous offset, clamped by the viewport.
- `report-return-check.txt`: full Go race suite, vet and formatting passed.
  `report-return-ui-final.txt`: uncached UI race suite passed after adding the
  removed-content regression. Tests exercise changed counts, a newly inserted
  partition, wrapping at 48/80/140 columns, and removed source content.
- `report-return-pty.txt`: real PTY keyboard/mouse entry, refresh, report chaining,
  resize, Escape and terminal cleanup passed for both cluster panels at three
  widths. `report-return-nodes.txt` and `report-return-priority.txt` passed the
  existing production CLI/PTY report regressions.
- Reviewed `output/terminal/report-return-tab3-48.png` and
  `report-return-tab4-80.png`. These are emulator renders, not native-terminal
  acceptance. Development binary rebuilt; release archives remain older.

## Node allocation report, October 10

- Added on-demand TUI `/nodes` and Partitions' clickable `n nodes`, CLI `nodes`
  and MCP `perch_nodes`. The report distinguishes configured/job-allocated/system-
  reserved/unallocated memory from OS free memory. State flags and GRES remain
  visible; neither idle CPUs nor unallocated memory is a scheduling promise.
- `nodes-final-check.txt`: full race suite, vet and formatting passed. Parser
  tests cover the projected live fixture, unknown quantities, system reservations,
  negative headroom, raw overflow, schema errors, duplicate nodes and query scope.
  UI tests cover first-content-row wheel scrolling and report age independent of
  subsequent queue refresh, as well as return to Partitions at all three sizes.
- `nodes-cli-live.json`: production CLI returned 20 visible Hummingbird nodes.
  `nodes-mcp-live.txt` passed the actual entrypoint/SDK live `perch_nodes` query.
  `nodes-mcp.txt` passed the built executable's independent protocol checks,
  including the fifteenth tool's pagination and retained zero memory headroom.
- `nodes-ui-live.json` / `nodes-ui-live.txt`: production TUI over SSH passed node
  entry from the Partitions footer, independent report age, 48-column memory
  display, scroll to observation time, return to Partitions and terminal cleanup.
  Reviewed `output/terminal/nodes-hummingbird-live.png` and
  `nodes-hummingbird-live-48.png`. They are emulator renders of actual PTY output;
  native appearance is not thereby certified. No cluster jobs were submitted.
- `nodes-pty.txt` passed the preceding layout's CLI/demo/pointer/resize checks;
  the subsequent report-age change is exercised by the live check and final
  broad terminal suite. Release archives still predate these source changes.

- `nodes-terminal-full.txt`: final full terminal suite passed, including the new
  node report at 140×40, 80×24 and 48×18, all priority regressions, mouse/hover,
  workload actions, log/resource views, preferences/cache, diagnostics and attempt
  identity. Reviewed the final 80-column demo render alongside the live wide and
  compact images. `nodes-binary.sha256` records the development executable tested.


## Pod resource metrics, October 10

The experimental NRP workspace now reads a selected Pod's CPU and memory sample
on demand. `u resources` / `/metrics` opens it; a Job first offers its owned Pods.
The main application container comes first. At 48×18, its CPU and memory fit
above the scroll indicator, with sample age/window kept below the input.
Escape restores the prior filter, inspector or Pod picker. The CLI equivalent is
`perch nrp metrics POD --context CONTEXT --namespace NAMESPACE [--json]`.

- `nrp-metrics-check-final.txt`: full race suite, vet and formatting passed before
  the final attribution and retained-sample hint refinements.
- `nrp-metrics-final-race.txt`: adapter, NRP UI and CLI race tests passed after
  attribution refinement. `nrp-metrics-retained-hint-race.txt` and
  `nrp-metrics-retained-vet.txt` cover the final rendered-hint correction.
- `nrp-metrics-full-ui-final.txt`: the full NRP PTY suite passed, including metrics,
  quotas, navigation, hover, clickable footer, logs and synthetic TLS/login.
  This precedes the final retained-sample hint correction.
- `nrp-metrics-final-pty.txt`: metrics CLI and PTY checks passed at 140×40, 80×24
  and 48×18, including exact units, mouse entry/scroll/reread/back, Job/Pod origins,
  resize, filtered return and terminal cleanup. This also precedes the hint fix.
- `nrp-metrics-development-runtime.json`: the development binary passed the
  portable Mac ARM runtime harness, covering CLI, MCP, both workspace PTYs,
  pointer requests and terminal restoration. The report contains its exact
  binary hash; it predates the final retained-sample hint correction.
- `nrp-metrics-retained-https-ui.txt`: the corrected executable passed the local
  TLS/PTY flow, including metrics permission loss, retained values and sample
  age, successful reread, return to the filtered Pod, log recovery and credential
  handoff/terminal restoration. `nrp-metrics-binary.sha256` identifies that build.
- Reviewed `nrp-metrics-48.png`, `nrp-metrics-140.png` and
  `nrp-metrics-stale-https.png` under `output/terminal`. These are emulator renders
  of real PTYs, not native Terminal.app acceptance or live NRP measurements.

The first HTTPS failure-path extension exposed a generic error hint overwriting
metrics sample age. The final view keeps the detailed failure in the panel and
“Read failed” plus sample age/window in the persistent hint. An assertion on the
rendered view now protects this behavior. No real NRP context, permission or
Metrics API availability is established by these synthetic fixtures.


## Job draft correction, October 10

Invalid job inputs now return focus to the field that needs correction. A
wrapped explanation is shown beside the input, with the other values preserved.
The compact correction view borrows the inactive progress bar's space, so long
path and launcher errors fit at 48×18. It does not bypass the existing review,
remote validation or explicit submission steps. Structured `launch.FieldError`
values provide the same domain validation to CLI and UI callers; error text is
not used to choose a field.

- `launch-field-focus-check.txt`: full Go race suite, vet and formatting passed.
- `launch-field-focus-race.txt`: launch, UI and CLI package race tests passed.
  Cases cover parsing and range errors across all fourteen form inputs, intact
  drafts, single-input focus, visible correction/navigation and pointer targets.
- `launch-field-focus-pty.txt`: production demo PTYs passed at 48×18, 80×24 and
  140×40. Actual input, click, resize and correction return to review with the
  original command/directory; multi-node launcher correction preserves topology.
  Existing array/dependency, validation, explicit simulated submission and
  terminal-restoration checks also passed. No remote submission occurred.
- `launch-field-focus-footer.txt`: existing wide/compact clickable-footer,
  navigation, log, help and demo launch checks passed.
- Reviewed `output/terminal/launch-correction-path-48.png`,
  `launch-correction-topology-48.png` and `launch-correction-cpu-80.png`.
  These are emulator renders, not native visual acceptance.

A separate read-only Hummingbird node observation at 14:07 local time reported
all four P100s allocated (`shell-gpu-capacity-20261010.json`). No GPU probe was
submitted. GPU shell/resource acceptance remains open; the capacity observation
is neither a queue prediction nor a scheduler result for a proposed job.


## Selectable software catalog, October 10

`/software` now offers arrow/page/home/end selection, wheel movement, hover and
click-to-open setup. Enter also opens the selected module. `f` searches within
the catalog without changing the job filter. Escape returns to the same module
and query, then to the original workspace/filter/selection; catalog refresh
keeps a still-present module by name rather than by old row number. Empty,
failed and loading catalogs cannot activate the hidden job selection. The
instructions viewport now uses the whole panel, including at 48×18.

- `software-catalog-check.txt`: full race suite, vet and formatting passed before
  the final status-line refinement. `software-catalog-final-race.txt`,
  `software-catalog-final-vet.txt` and `software-catalog-final-format.txt` cover
  that final change. Unit cases include name-preserving refresh after reordering,
  cancellation/late responses, failed and empty reads, catalog search and return.
- `software-catalog-final-pty.txt`: actual keyboard/mouse/footer navigation passed
  at 48×18, 80×24 and 140×40, including hover without activation, read failure,
  resize, wheel, refresh, empty results and retained caller context.
- `software-catalog-terminal.txt` and `software-catalog-hover.txt`: existing
  main-workspace and CSS/X11 pointer/cleanup regressions passed on the final build.
  `software-catalog-report-return.txt` passed the cluster-report bookmark checks
  before the status-only refinement.
- `software-catalog-live-final.txt`: the final executable passed a read-only
  Hummingbird SSH/PTY catalog → CUDA setup → catalog → Jobs round trip, including
  48-column resize. The live search returned CUDA 12.8.1 and 13.1.1. No job was
  submitted and no module was loaded.
- Reviewed `software-catalog-{48,80}.png`, `software-module-48.png` and live
  `software-catalog-hummingbird{,-48}.png` / `software-module-hummingbird.png`
  under `output/terminal`. These are emulator renders, not native acceptance.
  `software-catalog-binary.sha256` identifies the final development executable.

Software discovery still makes only explicit, bounded read requests. Availability
means presence in the query's login environment; it is not a compatibility claim.
No module is loaded and no displayed setup command is executed by browsing.


## Current polish preview packages, October 10

Version: `preview-20261010-polish`. Four local archives and their checksum manifest
are under `dist`. `release-polish-final-summary.json` binds archive and executable
hashes to the exact runtime reports. The archives include current documentation,
examples, dependency notices and the unchanged original Chip assets. They are
local review artifacts; nothing was published or installed on Hummingbird.

- `release-polish-final-archives.json`: checksums, OS/architecture, executable
  mode, safe members, linked docs, current documentation bytes, original artwork
  and exact binary/dependency notice matching passed on all four targets.
  `release-polish-final-notices.txt` records rejection of altered notice bytes,
  stale versions and a wrong-target executable.
- `release-polish-final-runtime-darwin-arm64.json`: packaged Mac ARM binary passed
  natively. `release-polish-final-runtime-darwin-amd64.json`: packaged Intel Mac
  binary passed through Rosetta on the ARM Mac, not on native Intel hardware.
- `release-polish-final-native-linux-amd64.json`: Hummingbird compute job
  **803122** completed in 29 seconds, exit 0, with empty stderr. The remote
  executable hash matched the package, queue absence was verified, and the
  private staging directory was removed. This is an offline demo/CLI/MCP/PTY
  executable test on a compute node; it does not establish live GPU behavior.
- The portable checks now include Pod metrics, catalog selection/setup/return
  and field-directed draft correction, alongside existing CLI, MCP, mouse
  protocol, resize, action-review and terminal-restoration checks. All run with
  isolated local preferences and synthetic workload data.
- `release-polish-final-setup.txt`: the extracted Mac ARM binary passed the
  first-run destination/persistence/cancel/recovery/layout checks with synthetic
  SSH. Normal preferences were not changed.
- Current Linux ARM runtime remains **untested**. No VM was started. Native
  font/animation appearance, actual mouse pointer shape, real MFA and native
  desktop banners remain separate acceptance requirements.

After recording these results, the same candidates were repackaged with this
updated documentation. Archive validation was rerun and all four executable
hashes remained identical, so the runtime evidence applies to their final
packaged executables. The earlier archive report is retained as
`release-polish-final-archives-before-docs.json`.


## Offline NRP Job preparation · October 10

Implemented local review, full-draft JSON and raw Job export through
`perch nrp prepare`. Explicit CPU/memory/local-disk budgets, finite deadline,
optional GPU resource and existing PVC are available. CPU-only placement,
original argv preservation, no failure retries and review boundaries are tested.
This moves workload preparation forward without changing the live-submission
acceptance gate or claiming full NRP support.

Evidence: `nrp-prepare-check.txt` (full Go race/vet/format),
`nrp-prepare-process.json` (built CLI and no external command execution),
`nrp-prepare-schema.json` (official v1.33.9 schema, three manifest variants and
negative control), `nrp-prepare-notices.txt` (regenerated dependency notices).
Terminal review renders use the emulator, not native Terminal.app. Current
release archives remain the earlier `preview-20261010-polish` executables.

Final development evidence is summarized in `nrp-prepare-summary.json`.
Existing synthetic HTTPS/auth queries and CLI allocation-shell/cancellation
regressions passed on that binary. Reviewed 48- and 80-column PTY previews;
word wrapping preserves readable prose without changing JSON exports.


## Retained-output search and compact reading · October 10

Added local literal/case-insensitive log search, match highlighting, next/previous
navigation, cancellation of edits and explicit End-to-follow behavior. Queries
stay separate from the job filter and preserve return to activity. A compact
reading mode makes six log rows available at 48×18, restoring the command input
when editing. Detailed original Chip assets and animation data were unchanged.

Focused evidence: `log-search-check.txt` (full Go race/vet/format),
`log-search-pty.txt` (three-size production-adapter/PTY exercise with synthetic
SSH), and the `log-search-*-regression.txt` logs for follow, retained failures,
job actions, footer, hover and help. The expanded activity PTY check also follows
an event into output, searches, clears the query and returns to the activity feed.
Reviewed `log-search-first-{80,48}.png` emulator renders. These checks do not close
native font, pointer, MFA or notification acceptance gates.

Long output is now prewrapped once per content/query/width change. A newline-free
256-KiB line exposed repeated scanning in viewport wrapping; regression coverage
now includes that case. Highlight runs also preserve joined emoji and combining
marks instead of inserting style changes between their constituent characters.

`log-search-performance-final.txt` measured both a 256-KiB multiline tail with
capped matches and a newline-free tail on this M4: about 12 ms per reindex and
11–12 ms per update/render. This is a local stress measurement, not a
cross-platform latency guarantee.
The older release-candidate hashes do not cover this development change.
`log-search-summary.json` records the current development binary and evidence
hashes. The final long-line PTY capture waits for the complete committed query;
`log-search-long-line.png` was reviewed alongside the compact 48-column view.

## Node GPU assignments and navigation · October 10

The node report now exposes explicit GPU GRES counts by type: configured,
allocated and unallocated. CLI text and JSON, MCP and the TUI share the same
parsed values. Missing, malformed, ambiguous and overallocated descriptions keep
their raw strings and unavailable counts; device index lists are not counted
again. GPU resources can represent MIG instances, so these are not advertised as
physical-card counts or measured utilization.

The clickable `g GPUs` control filters the existing node snapshot, preserves that
scope on refresh and returns to all nodes with `g all nodes`. Small windows show
the allocation count before CPU/memory details. Node/priority reports and direct
module inspection also have contextual hints instead of inheriting the software
catalog's setup shortcut.

The live read in `output/validation/node-gpu-live.json` returned 20 nodes and
node-24 with four configured/four allocated P100 GRES units. No workload was
submitted; this observation does not close GPU allocation-shell or utilization
acceptance. GPU count parsing covers typed, untyped, mixed-type and MIG names,
commas inside device-index annotations, explicit zero, missing values,
duplicates and inconsistent counts. `node-gpu-check.txt` records full Go
race/vet/format checks. Production CLI/MCP and three-size PTY evidence is recorded
in `node-gpu-pty.txt`, `node-gpu-mcp.txt`, `node-gpu-return.txt` and
`node-gpu-software.txt`. All are development evidence; packaged release hashes
remain older. Emulator renders do not certify native pointer/font appearance.
`node-gpu-summary.json` binds these checks and the live read to the rebuilt
development binary. The final compact render includes the corrected report hint.

## Navigation candidate acceptance · October 10

`preview-20261010-navigation` packages the current development changes for Mac
and Linux, ARM64 and x86-64. The full Hummingbird terminal suite passed in
`polish-current-terminal.txt`; the full NRP UI/synthetic-auth suite passed in
`polish-current-nrp-ui.txt`. These exercise real PTY input/output and reconstructed
screens, not native window appearance. Recent compact job-actions and NRP metric
renders were reviewed alongside the GPU and search views.

The expanded portable acceptance harness checks output search/return to follow,
the GPU filter and offline NRP Job manifest export, as well as the existing CLI,
MCP, six mouse/pointer-mode TUI sessions and terminal cleanup. Results:

- Mac ARM64: `navigation-runtime-darwin-arm64.json` passed natively.
- Mac x86-64: `navigation-runtime-darwin-amd64-retest.json` passed under Rosetta.
  The first run missed a raw-stream match-count assertion during per-character
  redraws. The isolated X11 repeat passed. The portable test now pastes its query
  atomically; separate screen-based ordinary typing passed at 140/80/48 columns
  in `navigation-rosetta-search-screen-retest.json`. The earlier failed reports
  remain available rather than being overwritten. No application change was
  needed. The screen-based test also corrects its initial expectation: a compact
  following view starts at the tail, so it must search to reveal an earlier line.
- Linux x86-64: owned Hummingbird job **803191** completed in **31 seconds** with
  exit **0:0**. `navigation-runtime-linux-amd64.json` records the native runtime,
  accounting, matching binary hash and removal of the private staging directory.
  This job ran offline demo/CLI/MCP/PTY checks; it was not a GPU test.
- Linux ARM64: archive/target/dependency checks passed; current runtime remains
  untested. No VM was started.

Archive integrity, linked documentation, original artwork and dependency-notice
checks passed for all four targets. Adversarial notice checks rejected changed
license bytes, stale versions and incorrect target binaries. Final documentation
was repackaged after these results; executable hashes must remain identical to
the verified runtime reports. `navigation-release-summary.json` records that
comparison and the final archive evidence. Nothing was published. Native visual,
MFA, notification, wider GPU and real NRP acceptance gates remain open.

## Module command copy and compact setup · October 10

Module details put the exact load command at the top, immediately after the
prerequisite reminder. The clickable `y copy load` action writes that command to
the local clipboard without loading the module or running a remote command.
Typed successful details must match the current requested module; loading,
failed rereads or mismatched details cannot supply clipboard content. The global
array-copy palette entry and `/failed` retain their array-specific scope.

The existing bounded clipboard helper is shared with failed-array copying.
Tests use private fake helpers; the user's clipboard was not modified. Evidence:
`module-copy-check.txt` (Go race/vet/format), `module-copy-pty.txt` (three sizes,
mouse copy, exact bytes, helper failure/timeout/recovery, duplicate suppression,
scope and failed-read guards), plus `module-copy-software.txt`,
`module-copy-arrays.txt`, `module-copy-palette.txt`, `module-copy-footer.txt` and
`module-copy-help.txt`. The compact emulator render `module-copy-48.png` was
reviewed; both the command and full success message fit. The final help entry is
grouped with Hummingbird tools, separately from job-specific array copying.
This development change postdates the navigation preview packages; native
clipboard/window acceptance is not established by fake-helper tests.

## Static text readers and resize repaint · October 10

Submitted scripts hard-wrap once per content/width change, preserving whitespace
and grapheme boundaries. Software reports reuse word-wrapped text and hard-wrap
unbroken paths/tokens. This avoids repeated long-prefix scans on every animation
frame. At 48×18, the script reader now shows six lines instead of two; `/`
restores command input, Escape returns to reading, and hidden input cannot take
mouse clicks. The original script bytes and detailed artwork are unchanged.

The near-256 KiB stress test exposed a resize artifact: the captured stream
painted the previous wider frame at the new terminal size before an incremental
redraw, leaving old text fragments. A full repaint after applying new geometry
clears them. The strengthened PTY test checks title, border, navigation uniqueness,
Home/End, command-input restoration and return to filtered jobs at three sizes.
The earlier captures and failed initial fake-SSH test remain as diagnostic
evidence; the latter incorrectly treated direct script reads as batched requests.

`static-readers-resize-check-go.txt` records full Go race/vet/format checks;
`static-readers-resize-retest.txt` records the production-adapter stress test with
synthetic SSH and animation enabled. The corrected 48-column script render was
reviewed. Local M4 microbenchmarks in `static-readers-final-benchmark.txt` measured
about 1 ms/frame for 8 KiB unbroken lines, versus 9–14 ms before caching, and about
10–12 ms/frame at 256 KiB. These are short local measurements, not cross-platform
latency guarantees. The summary binds final navigation, reader and mouse checks
to the development binary; preview archives still predate these changes. PTY
screens do not establish native-window acceptance.

## Script availability and recovery · October 10

Historical script queries now use `sacct --batch-script --noheader`. Successful
but empty responses and the historical `NONE` sentinel produce an explicit
unavailable result instead of a blank viewer or a pretend script. Other valid
script content is preserved after control-sequence sanitization. Command failures
retain a bounded sanitized stderr diagnostic and discard partial stdout;
network/authentication classifications remain intact. The TUI wraps the error
as prose, reflows it on resize and exposes `r retry` in the footer. The existing
two-second manual-refresh cooldown still applies.

This behavior is grounded in the [Slurm sacct documentation](https://slurm.schedmd.com/sacct.html#OPT_batch-script)
and the [25.11.4 implementation](https://github.com/SchedMD/slurm/blob/slurm-25-11-4-1/src/sacct/options.c#L1180).
It does not infer whether the job was interactive, storage was disabled, or a
retention policy removed the script; the response alone cannot establish that.

The read-only Hummingbird check in `script-outcomes-live.json` retrieved the
136-byte stored script of earlier owned test job **803191**, starting with a
shebang and no report header. Script contents were not saved; no job was
submitted and no script was executed. `script-outcomes-final-check.txt` records
Go race/vet/format checks. Focused production CLI, MCP and three-size PTY checks
exercise absent/empty/failed/successful reads, partial-output rejection, retry,
resize, caller return and cleanup. Initial harness failures (refresh cooldown
and an error phrase wrapping across lines) remain recorded. See
`script-outcomes-summary.json` for the final binary and evidence hashes.
These changes remain development-only; packaged candidates are older.

## NRP server Job dry-run · October 10

`nrp validate JOB.json --context NAME --namespace NAME` extends the local
preparer with a non-persisting server check. The exact named Job JSON and
namespace are validated before credentials are loaded. One POST always uses
`dryRun=All` and `fieldValidation=Strict`; there is no real-create fallback,
automatic retry or redirect following. The report includes a manifest hash,
Job admission result and an independent ResourceQuota snapshot. It does not
claim Pod admission, available hardware, image/PVC access or execution.

`nrp-validation-checked-errors.txt` records final Go race/vet/format checks.
`nrp-validation-checked-process.txt` exercises the built CLI with a local TLS
server: exact request bytes and route, partial quota failure, permission/name
conflict/admission failures, JSON status and no token/server-error leakage.
`nrp-validation-final-process.txt` includes the existing NRP UI, credentials,
reconnect, log and terminal-cleanup regressions before the final error-kind
redaction adjustment. `nrp-validation-prepare.txt` verifies that preparation
remains offline. `nrp-validation-summary.json` binds source, binary and evidence.
No real NRP request, credential setup or workload submission was performed.

The same turn's Hummingbird capacity observation is saved in
`gpu-acceptance-nodes.json`: node-24 reports four of four P100 GRES units
allocated. No GPU probe was queued. GPU shell/resource acceptance remains open;
idle CPU count on that node is not an available GPU.

## NRP validation outcomes and recovery · October 10

The CLI now distinguishes `accepted`, `rejected` and `unverified` admission
results in text and JSON. A timeout, authentication failure or malformed reply
cannot become a claimed rejection. Explicit API denials retain their
classification, and each result includes a next step: check destination/access,
sign in explicitly, correct or rename the manifest, or retry the dry-run.
Quota success/failure remains independent of the admission result.

`nrp-outcome-check.txt` records full Go race, vet and formatting checks.
`nrp-outcome-process.txt` records built-CLI synthetic TLS checks of text, JSON,
exit status and exactly one POST per attempt for 401, 403, 404, 409, 422 and 500
responses, alongside the existing CLI/authentication regressions. Unit tests
cover cancellation, timeout, TLS failure and invalid-response classification.
`nrp-outcome-summary.json` binds this development binary and source to the
evidence. No UI layout changed, no real NRP server was contacted and no workload
was submitted. The earlier validation summary describes the prior checkpoint;
the packaged navigation candidate is still older.

## Submitted-script find and navigation · October 10

The script reader shares the output reader's literal, case-insensitive search.
`f` edits the query, `[` / `]` visit and wrap matches, Enter keeps the search,
and Escape cancels an edit or clears a committed search before returning to the
calling job/activity view. Footer controls are clickable. At 48×18 a committed
query remains in the header above six readable rows. Resize keeps the selected
match visible. End scrolls to the script's bottom without clearing its search;
logs retain their separate End-to-follow behavior.

An unchanged reread preserves match selection; changed displayed script text resets the
ordinal, and a failed read clears script/search rather than presenting stale
script content as current. Switching readers/jobs clears search. The original
script text, job selection and job filter are not modified. Search is local to
the already bounded content, with the same 100-character/4,096-match limits as
output search; it never executes the script or issues a new remote search.

`script-search-check.txt` records full Go race/vet/format checks.
`script-search-process.txt` records production-adapter PTYs at 140×40, 80×24
and 48×18 with animation enabled, mouse search controls, resize, refresh,
failure/retry, a near-limit unbroken line and return to the filtered job list.
The initial two harness failures advanced on an intermediate search result
before the Enter key was consumed; the harness now waits for the committed
search controls. Those failed logs are preserved. Separate log-search,
static-reader, help, hover, script-outcome and footer regressions passed.

The final `script-search-first-{140,80,48}.png` emulator renders and the compact
no-match editing screen were reviewed. `script-search-summary.json` binds
source, executable, checks and renders. Native Terminal.app acceptance remains
separate; no terminal profile, original artwork or cluster workload changed.
These changes are in the development build, not the older preview archives.

## Job-list sorting and resource comparison · October 10

`/sort` opens the existing command palette filtered to seven choices: default,
name, state, elapsed, CPUs, requested memory and submission time. Direct commands
accept optional `asc`/`desc`. Visible Name, State and metric headings use shared
drawing/hit geometry; clicking the active heading reverses direction. Jobs and
History retain independent choices during the session. Sorting preserves the
selected job ID and filter, including when refreshed values move that row.

The sorted projection is separate from the reconciled scheduler store. Ties
retain the base order, unavailable values stay last in both directions, and
unobserved array-gap rows do not acquire resource values. Grouped array ranges
remain grouped. Memory is per-record requested memory, not usage or an array
aggregate. CPU/memory sorting shows that metric in the table and reserves room
for it in compact rows. A filtered-empty view has no invisible heading targets.

The final Go checks are in `job-sort-verified-check.txt`; three-size keyboard and
mouse acceptance is in `job-sort-verified-process.txt`. The final palette and
expanded 5,000-task sorting checks are in `job-sort-verified-palette.txt` and
`job-sort-verified-arrays.txt`. Earlier mouse, hover, help and job-action
regressions passed before the final chooser-copy refinement. Initial failures
and corrections remain recorded: compact metric clipping, a refresh test that
mistakenly reused retained demo array history, and the word “newest” matching
the existing “new job” palette search. No test expectation was weakened to
retain those issues.

Reviewed emulator renders include the 140-column memory table, 80-column CPU
table, and the 48-column memory row and final chooser. `job-sort-summary.json`
records the binary, source and evidence hashes. Native pointer/font acceptance
remains separate; these checks use demo/synthetic data and made no cluster
request or priority change. The preview archives remain older development
checkpoints.

## Contradictory accounting outcome guard · October 10

An audit reproduced two different terminal states for the same scoped job
attempt being accepted, leaving downstream row order to choose an outcome.
The parser now rejects the entire accounting component with an explicit error;
queue and partition results remain usable. The probe calls this an invalid
response rather than an unsupported capability. Exact duplicates, retries and
reused IDs with different submission times remain accepted.

Full Go race/vet/format checks passed. The production CLI/PTY regression covers
both conflicting row orders, usable queue data, visible connection error and
reconnect recovery, as well as existing mixed-array requeue behavior. An app
regression confirms rejected history emits no outcome and a later consistent
exact-attempt completion still emits once. The emulator screenshot was reviewed.
A read-only live query accepted 128 current Hummingbird accounting rows; no job
was submitted. Evidence and executable hashes are recorded in
`output/validation/accounting-conflicts-summary.json`.

The development binary contains this change; scheduler preview archives do not.
This closes only the contradictory same-attempt terminal-state case. Missing
identity evidence and conflicting overlapping range records remain under audit;
native and real NRP acceptance remain open.

## Ambiguous accounting identity guard · October 10

The follow-up audit reproduced duplicate logical IDs being accepted when one
record lacks a submission time, and one scoped attempt carrying different raw
allocation IDs. Both cases now reject the accounting component before any
historical selection or log/script read. Identity validation uses structured
cluster/user/logical-ID/submission/restart keys and linear passes over records.
Single missing timestamps and identical unknown-time duplicates remain missing,
not invented or rejected; distinct known submissions and retries remain valid.

Full Go race/vet/format checks and the production CLI/PTY attempts regression
passed. Both row orders refuse job/log/script/resource reads for the ambiguous
submission case. Existing contradiction/reconnect and real-fixture mixed-requeue
regressions still pass. A read-only query accepted 128 live Hummingbird accounting
records. Its first harness timed out before the CLI deadline; the leftover private
SSH master was closed and the retry used an internal 20-second deadline and passed.
No cluster jobs were submitted. Exact source/executable hashes and evidence are in
`output/validation/accounting-identity-summary.json`. Scheduler archives remain
unchanged; they do not include either subsequent accounting guard.

Conflicting overlapping array records remain under audit, alongside the existing
native experience, authentication, alerts, GPU, Linux ARM and real NRP gates.

## Array range precedence and conflict handling · October 10

Reproduced a current pending range being classified as failed when a larger
accounting range sorted first. The store now marks current queue membership;
summaries apply it before accounting and subtract only compact progressions.
A failed queue read preserves that stale presentation; a successful empty queue
releases it. The optional display cache preserves precedence without becoming
authority for reads, actions or notifications.

Same-attempt terminal ranges with contradictory overlapping outcomes now produce
an unavailable summary and refuse failed-task copying. Exact stepped intersections
are checked with bounded work/fragmentation; duplicates, disjoint residues and
different attempts retain their semantics. The keyboard/palette copy guard now
reports the conflict explicitly. Individual records still refine aggregate ranges.

Full Go race/vet/format checks passed, including 300 independent enumerated
membership/conflict cases and billion-task compact stride cases. Production
CLI/PTY array-copy tests passed at140/48 columns: the overlap case copies only
`--array=20`, and conflicting histories in either order leave the private fake
clipboard untouched. Retry/accounting and snapshot-cache PTY regressions passed.
Wide/narrow and conflict renders were reviewed. A read-only live query returned
16 Hummingbird array summaries successfully; no jobs were submitted.

Exact evidence and executable hashes: `output/validation/array-overlap-summary.json`.
Scheduler preview archives remain unchanged. Cross-poll array notification
transitions and mixed identity evidence across range IDs remain under audit, as
do the native and external-access acceptance gates.


## Partial array outcome activity and accounting freshness · October 10

Aggregate outcome activity now counts only the disjoint, previously observed
membership confirmed by a refresh. Checkpoints remove that membership so later
expanded task records cannot notify again. Successive subsets retain distinct
activity keys, confirmed task counts and the original scheduler attempt for
inspection. Notification grouping sums tasks instead of counting aggregate rows.
Contradictory ranges suppress aggregate events until a consistent read arrives.

The production terminal regression exposed an additional freshness boundary:
reconnect preserves the normal accounting cadence, so a successful queue read can
arrive while accounting is retained. A unit regression then reproduced eleven
failed tasks and an incorrect full-range copy when only one had been confirmed.
Previously unfinished identities overlapping retained terminal evidence now stay
UNKNOWN with temporary accounting precedence. Current queue records still win;
fresh accounting releases the temporary precedence. Expanded task rows and
standalone jobs follow the same rule. Display caching retains this provenance.
No missing membership is promoted to success or failure merely by leaving queue.

Full Go race, vet and formatting checks passed. Regressions cover checkpoint
restore, grouped-to-individual transitions, one then ten outcomes, billion-task
compact membership, stale-accounting suppression and cache roundtrip. Production
CLI/PTY array-copy and snapshot-cache regressions passed. The activity test uses
synthetic SSH and a private clipboard; it waits for the real accounting cadence,
checks one confirmed failure plus ten unresolved tasks, then fresh confirmation.
Reviewed the awaiting-accounting render: the unknown range remains selectable,
progress reads one of eleven finished, and copying yields only --array=10.

Evidence: output/validation/range-events-summary.json and
output/validation/accounting-freshness-summary.json. These are development-build
checks, not new scheduler preview archives. No cluster jobs or native desktop
notifications were sent. Native terminal appearance/pointer/MFA/alerts, GPU-shell,
current Linux ARM and real NRP gates remain open. Mixed or missing attempt identity
across different range IDs remains under audit.


## Missing submission evidence across array records · October 10

Reproduced overlapping failed/completed ranges being assigned outcomes by sort
order when one submission time was missing. Array evidence now checks scoped
membership before ordering records, with exact compact progression intersections
and bounded fragments/work. Unknown time cannot stand in for an earlier attempt.
Disjoint task sets, separate scopes, all-missing evidence without a contradictory
outcome, and fully known retry ordering retain their existing behavior.

The accounting parser refuses a component with overlapping mixed submission
evidence, even when range IDs differ. Combined queue/accounting summaries also
report the ambiguity before display-family filtering can discard the missing-time
row. Failed-task copying refuses that result, aggregate events remain suppressed,
and ambiguous evidence cannot retire the previously observed array minimum.
Fresh complete submission evidence restores ordinary progress/copy behavior.

Full Go race, vet and format checks passed. Four hundred independently enumerated
membership cases, billion-task disjoint strides, range/task identity, record order,
parser rejection and display-filter/recovery checks passed. The production
CLI/PTY array-copy regression exercises both accounting row orders and a separate
queue/accounting conflict; wide and 48-column views show an unavailable summary
and leave the private fake clipboard unchanged. Render review exposed a clipped
explanation in the wide glance panel; the shared array progress renderer now wraps
that message to the pane width while the bounded glance keeps its details shortcut.
Word-preservation/width checks cover 32, 44, 55 and 80 columns. Reviewed both
updated renders. A read-only
query accepted 128 actual Hummingbird history records with the rebuilt binary;
no jobs were submitted.

Evidence: output/validation/array-identity-summary.json. The scheduler preview
archives remain unchanged. Synthetic SSH, clipboard and emulator renders do not
close native appearance/pointer, MFA, desktop-notification, GPU-shell, current
Linux ARM or real NRP acceptance gates.


## Reconciled local preview · October 10

Prepared preview-20261010-reconciled with the accounting identity/outcome guards,
array overlap/freshness handling, observed-range activity and wrapped explanations.
All 48 terminal regression scripts passed. Chip's six-state playback, relative
cadence, pause/step alignment and faster running sample passed. Reviewed its
emulator sheet and packaged compact sorting/script-search screens. Application,
fixture, tool and artwork hashes stayed unchanged through verification; only
readiness and validation documentation was finalized before the final packaging.

The extracted Mac ARM executable passed CLI/MCP, six Hummingbird/NRP mouse/pointer
and keyboard PTY modes, plus search/sorting at 140, 80 and 48 columns. The Mac Intel
executable passed the same checks through Rosetta, not on native Intel hardware.
The Linux x86 executable passed on Hummingbird in owned job 803524, COMPLETED 0:0
in 35 seconds. Its exact temporary stage was removed after confirmed success.
Linux ARM was cross-built and archive-verified, not runtime-tested. No VM started.

Packaged Mac ARM NRP tests passed against local synthetic HTTPS, including plugin
authentication/recovery and terminal flows. Dependency-notice negative checks
rejected altered architecture/license/version evidence. Final four-archive checks
cover checksums, target/dependency metadata, matching linked documentation and
unaltered original artwork. Runtime hashes are compared with final archive hashes
so documentation repackaging cannot silently substitute an untested executable.

A fresh read of node-24 still showed all four P100s allocated, so no new GPU probe
was submitted. Native fonts/pointer, interactive MFA and desktop notifications,
GPU shell, current Linux ARM runtime and real NRP access remain open. Nothing was
published or installed. Complete evidence: output/validation/reconciled-release-summary.json.


## October 10, 2026 — private repository and reference-profile handoff

The user approved the supplied Ioskeley Mono 12 pt / 0.90 line-spacing profile
(`docs/terminal/Perch-Ioskeley.terminal`). Its embedded font archive was checked:
Ioskeley-Mono at 12 pt, width spacing 1.0, height spacing 0.9. The distribution
profile omits the old demo startup command and contains no local executable path.
This is reference-profile appearance acceptance, not proof of every interaction,
all animation states, native authentication or notification banners.

The user removed NRP from active scope and explicitly requested preservation of
its existing work. Source, tests, fixtures and documentation remain intact; no
further NRP access is required to prepare the Hummingbird release.

The prior full README is preserved as `docs/GUIDE.md`, with links adjusted for its
new location and NRP marked deferred. A short README and current offline demo
render replace the long front page. The render comes from a real PTY with
`--demo --no-config --reduced-motion` at 140 by 44 cells; it is an emulator image,
not a native screenshot. Original detailed Chip assets are unchanged.
