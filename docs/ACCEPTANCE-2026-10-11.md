# October 11 acceptance update

This update supplements the [readiness audit](READINESS.md). The user accepted
the slower curved flight, ordinary connection/reconnect and quit, completion
banners, mouse navigation, output scrolling and compact resizing. The current
tested draft is **`v0.1.0-preview.2`**, built from `efb9d02`; all four packaged
platforms have runtime evidence, with Mac Intel tested through Rosetta.

Actual password/MFA-expiry recovery and GPU-shell behavior remain unverified.
The user selected MIT for code and reserved Chip artwork. Public distribution
awaits licensed replacement packages and explicit publication approval. The accepted
native checks should not be repeated without a relevant change.

The sections below are a chronological record. Statements about pending motion
approval and preview 1 describe earlier checkpoints; the final preview-2 section
records the current packaged result. Tooling added after that source commit does
not silently replace the tested archives.

## Release and platform checks

The private draft `v0.1.0-preview.1` packages source commit
`c89be60b0b1652995733c6b3d2041c69292de9ba`. All four uploaded archive digests
matched the independently checked local files. Packaged README media and the
Ioskeley terminal profile also matched the source bytes. Later workflow-only
changes did not replace these executables.

- [Go/MCP, all 48 terminal scripts and Chip playback passed](https://github.com/darthnithin/perch/actions/runs/38170883928).
  That run tests `6d67a5c`; `c89be60` only updates readiness/history documentation.
- Packaged Mac ARM passed natively; packaged Mac Intel passed under Rosetta.
- Packaged Linux x86 passed on Hummingbird in job **810392**, COMPLETED 0:0,
  35 seconds. Its temporary stage was removed.
- [Packaged Linux ARM passed on a native aarch64 GitHub runner](https://github.com/darthnithin/perch/actions/runs/38172042532).
  Archive and binary hashes were checked before execution. CLI, MCP, PTY,
  pointer-protocol and terminal cleanup checks passed. OrbStack was not started.

Linux ARM executable SHA-256:
`5275ad29ab35014ca7ec8d4abd2700e6ca7924a7b233ad52cb1b91cf9dc9b82b`.
The manually dispatched workflow and its permission requirements are documented
in [releasing](RELEASING.md). Native font acceptance remains separate from PTY
protocol testing.

## User-observed Terminal checks

These observations used the local development executable (SHA-256
`9698c05c7432076d63e47ece9b207921c7a8393b6ed832b7012e1ff5d96616c4`),
not the separately hashed packaged Mac executable. They establish the observed
native flow, not a native acceptance test of every packaged artifact.

In the supplied Ioskeley profile, the user confirmed:

1. The live Hummingbird jobs/workspace loaded.
2. Quitting returned to a normal shell with a visible cursor and normal typing.
3. With `--notify desktop`, Activity recorded test job **810406** completing.
4. Connection showed desktop alerts enabled, without a delivery error.
5. The completion message reached **Notification Center**, attributed to
   **Script Editor**. After enabling Banners for Script Editor, the user confirmed
   seeing the pop-up completion banner for repeat job **810452**.
6. Pressing `c` returned cleanly to Perch without an authentication prompt.

Job 810406 used one CPU and 128 MiB, completed 0:0 in 61 seconds, and printed the
expected marker. Its two temporary log files and directory were removed after
collection. This verifies a real completion notification reaching Notification
Center. Repeat job 810452 also completed 0:0 in 61 seconds; its expected output
was verified and temporary files were removed. The user saw its completion
banner. This establishes banner delivery under the tested settings, not every
notification/Focus configuration or failure-banner delivery.

The successful connection does not establish that a password or MFA prompt was
shown. Normal reconnect and screen restoration were user-verified; expired
credentials and actual password/MFA prompt handling remain unverified.

## Wing motion remains under review

The user rejected the fixed-body and extra-position comparisons, then clarified
that the up-and-down hover is welcome: the wing movement feels too linear.
They requested a slower preview and suggested a spring or figure-eight stroke.
The private packaged preview cycles through eight wing-tip positions at 40 ms
per frame and shifts the body by one pixel during part of the loop.

The first isolated figure-eight study kept
the original body, head, bill, tail and hover cadence, with a 640 ms wingbeat
(half the shipped speed). The projected wing chord narrows through the turn.
This is a stylized periodic path, not a physical spring or flight simulation.
All 16 stroke positions were reviewed in an Ioskeley contact sheet; the wing
became nearly hidden by the head at the close end.
The full 2.56-second GIF was decoded and compared against every rendered frame.
PTY checks passed for quitting, SIGTERM and the narrow-window guard, including
terminal restoration. Source artwork hashes remained unchanged. These checks
do not establish native Terminal rendering acceptance. The user then selected
**“Closer—refine this motion.”**

The current `tools/chip_wing_study.py` keeps the close turn left of the face so
the blade stays visible. It samples 32 poses per 640 ms wingbeat (20 ms frames)
without changing the beat speed or the original 1.28-second hover cadence.
The new 1.28-second GIF was decoded and compared against every render; the
contact sheet shows every other pose through a complete stroke. Original face
pixels remain identical across every frame after accounting for the hover.
The loop boundary changes fewer pixels than the largest ordinary adjacent
transition; that is a continuity check, not proof of natural-looking motion.
The 20 ms terminal preview passed quit, SIGTERM and narrow-window cleanup checks.
Original art is retained. The refined stroke is now integrated into the local
application for review, as described below; visual acceptance remains open.

To inspect in the supplied Ioskeley Terminal profile:

```sh
.venv/bin/python tools/chip_wing_study.py --play
```

Space pauses, arrow keys step and `q` exits. To recreate the GIF and pose sheet
on macOS, use `--render`. The study uses the existing local Pillow environment
and CoreText renderer. `--export` writes only the separate terminal flight strip;
`--play` and `--render` do not change application artwork.

Local evidence is retained under `output/validation/` (not shipped):
`oct11-private-publication-summary.json`, `oct11-private-linux-arm-runtime/runtime.json`,
`native-acceptance-oct11.json`, and the native notification probe receipts/logs.
Earlier comparisons remain under `output/design/wing-motion-study/` and
`output/design/wing-arc-study/`. The first figure-eight clip remains unchanged at
`output/design/wing-eight-study/`. Its refinement is under
`output/design/wing-eight-study-v2/`, including `preview.gif`, `poses.png`,
`motion-checks.json` and `terminal-checks.json`.

## Compact resource view refinement

Source commit `2decc8b` lets the Resources panel reclaim four rows from the idle
command box in terminals at most 24 rows tall, matching the existing logs and
inspector behavior. At 80×24 the first-sample explanation and the sampled RSS
plot are now visible without scrolling. `/` restores command entry; Escape
restores the resource space and selection. Taller layouts retain the command box.

UI/bird Go tests, UI vet and the build passed. Resource-step, live-step and footer
PTY checks passed, including mouse step selection, compact resizing and cleanup.
The Ioskeley before/after renders are in `output/design/interface-review-oct11/`.
[CI run 38175760369](https://github.com/darthnithin/perch/actions/runs/38175760369)
completed successfully for this source change, including the full terminal and
Chip checks.
The private draft still packages `c89be60` and does not include this newer layout.

## Refined flight integrated into the local application

The local source now uses `assets/bird/terminal-running-strip.png` for running
Chip in both the workspace and developer preview. The terminal strip contains
64 frames at 20 ms: two 640 ms strokes and one unchanged 1.28-second body hover.
All 21 checksum-protected original images remain unchanged. The terminal export
is reproducible with `tools/chip_wing_study.py --export`; the other five states
retain their original artwork and 80 ms interval. This follows the user's
“Closer—refine this motion” direction; it does not claim final visual approval.

The developer preview now advances the other states every four flight samples,
retaining their actual timing. Its clock unit tests and six-state PTY test pass,
including pause, aligned stepping, single-state playback and terminal cleanup.
The first full check exposed unit tests that still referenced the old half-tick
clock; those tests were updated for the four-sample cadence and passed.
UI/bird tests, static-reader navigation, general workspace keyboard navigation
and mouse/resize PTYs passed with the new animation enabled. The full Go race
suite, vet and formatting checks passed. An ignored `output/go.mod` separates
generated release copies and cached dependencies from the application packages;
without that local artifact boundary, vet also scanned a cached dependency
benchmark and reported unused return values outside Perch source.

The full workspace was captured from the actual app in a PTY, then rendered with
Ioskeley Mono 12 pt, 0.90 spacing and a 2× CoreText raster. The review clip at
`output/design/wing-eight-workspace/workspace.gif` is 2080×1332, 55 encoded
frames over 1.28 seconds (64 captured samples). It is an emulator/CoreText
preview, not a native Terminal.app capture. The README media and private release
archives still contain their earlier animation and have not been replaced.

`output/design/wing-eight-workspace/validation.json` binds the reviewed binary,
terminal strip, authoring script, profile and GIF to SHA-256 hashes. All 64
exported frames matched the study, and all 21 original-artwork checksums passed.

## User accepted the integrated flight · October 11

After being asked to run the updated demo in the Ioskeley Terminal profile, the
user chose **“Keep this motion.”** This accepts the slower curved running motion;
it does not imply acceptance of every other state, view or authentication path.
The README GIF and still now use the reviewed motion and reference font/spacing.
The private `v0.1.0-preview.1` remains unchanged; a new candidate is being prepared.
Source CI for `40c84d5` passed in run 38176491104, including all 48 terminal
scripts and six-state Chip playback.

The 14:28 PDT read of Hummingbird again showed all four P100 GPUs allocated.
No further GPU probe was submitted. The user then supplied the Tailscale SSH
endpoint after disconnecting the campus VPN. Live checks should use that explicit
endpoint; do not silently change the product's public Hummingbird default.

## Native navigation accepted; private preview 2

The user confirmed that History/Jobs clicks, changing the selected job, opening
output, scrolling, Escape return and smaller-window resizing all behaved normally
in their terminal. They then said **“Dude i think it's all good.”** The hands-on
review is accepted; do not keep requesting repeated native checks. This does not
retroactively claim that an actual password/MFA prompt or GPU shell was exercised.

`v0.1.0-preview.2` is a separate private draft built from clean source
`efb9d026fa655f6711c10ae49946273d0416c15e`. All four archive architectures,
checksums, docs, original artwork, terminal flight strip and dependency notices
passed verification. All five uploaded digests matched local bytes. The repository
remained private. The approved README GIF and still were also retrieved from the
published commit and matched byte-for-byte.

Exact packaged runtime checks passed on native Mac ARM, Mac Intel through
Rosetta, native Linux x86 and native Linux ARM. Hummingbird job **810653** used
one CPU/256 MiB, completed 0:0 in 36 seconds, and its temporary stage was removed.
It used the user's supplied Tailscale SSH route after VPN disconnection. Linux
ARM ran on GitHub's native aarch64 runner in
[run 38177035642](https://github.com/darthnithin/perch/actions/runs/38177035642).
Both the downloaded archive and executed binary were hash-verified.

Local evidence: `output/validation/preview2-source.json`, `preview2-archives.json`,
`preview2-upload.json`, `preview2-runtime-arm64.json`, `preview2-runtime-amd64.json`,
`preview2-linux-live.json`, `preview2-linux-arm-runtime/runtime.json`, and
`oct11-approved-media-publication.json`. Source CI run 38176491104 completed successfully; its Go/MCP, 48 terminal
scripts and Chip results supplement the exact packaged runtime evidence.
The older preview 1 remains unchanged. No public release or VM was created.

## Homebrew preparation and publication review

Commit `98a38ec` adds a checksum-verified formula generator, six offline tests
for its release selection/failure handling, and an isolated check using
Homebrew's Formula DSL and install helpers. All four platforms selected the
correct archive/checksum and installed the executable, documentation, profile,
artwork and notices into temporary directories. Both Mac executables passed
version and demo-JSON smoke checks after that installation; Intel used Rosetta.
The Linux binaries were not executed in this Mac-hosted check; their separate
packaged runtime evidence is recorded above.

`output/validation/preview2-homebrew-install.json` binds those results to the
archive and executable hashes. This is not evidence for the full `brew install`,
linking, upgrade, uninstall or `brew test` lifecycle. No tap was registered or
published, and no global installation or terminal preference was changed.
See [release preparation](RELEASING.md#prepare-a-homebrew-tap-formula).

A bounded credential-pattern scan inspected 498 text blobs reachable from local
Git refs. Its two matches were dummy credential-bearing URLs in NRP rejection
tests, not live credentials. No private Tailscale route or absolute Mac home path
matched. This is not a comprehensive secret audit. The user's UCSC username
was found in test and validation documents/history. The user asked to remove it
before publication; the current files now use a synthetic `researcher` identity.
The user then chose to keep the existing history and repository: only the current
examples are anonymized. No hosted history was rewritten and no repository was
renamed. Older username references remain in historical commits by that decision.
Replacement packages include the new licensing notices and current examples.
The repository remains private. The user selected MIT for code and reserved
artwork terms for Chip; both are now included by the packaging tools.

The 14:55 PDT read of node-24 again showed all four P100 GPUs allocated; no new
GPU probe was submitted. Evidence is retained in
`output/validation/oct11-gpu-capacity-distribution.json` and
`output/validation/publication-history-scan.json`.

The complete CI run for Homebrew preparation commit `98a38ec` passed:
[run 38178511754](https://github.com/darthnithin/perch/actions/runs/38178511754).
It includes Go race tests, vet/formatting, MCP, dependency notices, the six
Homebrew generator checks, all 48 terminal scripts and six-state Chip playback.
The later hostname-only test update also passed `go test ./cmd/perch` locally.
