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

`tools/chip_wing_study.py` now provides one isolated figure-eight study. It keeps
the original body, head, bill, tail and hover cadence, with a 640 ms wingbeat
(half the shipped speed). The projected wing chord narrows through the turn.
This is a stylized periodic path, not a physical spring or flight simulation.
All 16 stroke positions were reviewed in an Ioskeley contact sheet; the wing
becomes nearly hidden by the head at the close end, which needs user judgment.
The full 2.56-second GIF was decoded and compared against every rendered frame.
PTY checks passed for quitting, SIGTERM and the narrow-window guard, including
terminal restoration. Source artwork hashes remained unchanged. These checks
do not establish aesthetic approval or native Terminal rendering acceptance.

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
`output/design/wing-arc-study/`. The latest study is under
`output/design/wing-eight-study/`, including `preview.gif`, `poses.png` and
`terminal-checks.json`.
