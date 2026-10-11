# October 11 acceptance update

This update supplements the earlier [readiness audit](READINESS.md). The goal
remains open: the user reported that the wings look uneven, and interactive
authentication and GPU-shell acceptance still need evidence.

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
The shipped source still cycles through eight wing-tip positions at 40 ms per
frame and shifts the body by one pixel during part of the loop.

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
Original art is retained. This refinement is still a review study.

To inspect in the supplied Ioskeley Terminal profile:

```sh
.venv/bin/python tools/chip_wing_study.py --play
```

Space pauses, arrow keys step and `q` exits. To recreate the GIF and pose sheet
on macOS, use `--render`. The study uses the existing local Pillow environment
and CoreText renderer. The production animation is not replaced by this tool.

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
tests this source change; its final conclusion is not yet recorded here.
The private draft still packages `c89be60` and does not include this newer layout.
