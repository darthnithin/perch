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
   **Script Editor**. No pop-up banner was observed in the initial check.

Job 810406 used one CPU and 128 MiB, completed 0:0 in 61 seconds, and printed the
expected marker. Its two temporary log files and directory were removed after
collection. This verifies a real completion notification reaching Notification
Center; it does not establish banner visibility under every notification/Focus
setting, nor failure-banner delivery. The user is choosing whether to enable
banners and repeat that part of the check.

The successful connection does not establish that a password or MFA prompt was
shown. Actual interactive reconnect/prompt handling remains unverified.

## Wing motion remains under review

The user said the wings look weird and not smooth. The current source cycles
through eight wing-tip positions at 40 ms per frame and shifts the body by one
pixel during part of the loop. An isolated A/B/C study compares the existing
motion, a steady body, and 16 wing positions at the same wingbeat speed. It uses
the approved Ioskeley font and spacing and preserves body proportions. Original
artwork and the application remain unchanged pending visual review.

Local evidence is retained under `output/validation/` (not shipped):
`oct11-private-publication-summary.json`, `oct11-private-linux-arm-runtime/runtime.json`,
`native-acceptance-oct11.json`, and the native notification probe receipts/logs.
The motion study is under `output/design/wing-motion-study/`.
