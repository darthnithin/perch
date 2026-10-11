# Workspace preview assets

`workspace.gif` captures the real demo TUI through a PTY: 64 samples at 20 ms,
then rasterizes its cells with installed Ioskeley regular/bold through macOS
CoreText. Font size and line spacing come from the approved Terminal profile
(12 pt, 0.90; 7.2 by 14.4 point cells). It is an emulator rendering, not a native
Terminal.app screenshot. No application interface or font settings are changed.

The 2080 by 1332 GIF loops every 1.28 seconds, using a shared palette and optimized
frame differences. Identical samples may be coalesced while preserving duration.
`workspace.png` is a 3120 by 1998 still captured with reduced motion. Both keep
Chip's original source proportions and actual font glyphs.

To regenerate on macOS with Ioskeley installed:

```sh
.venv/bin/python tools/render_readme.py --animate
.venv/bin/python tools/render_readme.py
```

Review the generated assets in `output/readme/animated` and `output/readme/static`
before copying them into this directory. The original detailed Chip artwork in
`assets/bird` remains unchanged.

The slower curved flight was accepted by the user in the reference Terminal
profile on October 11, 2026. Flight uses a separate terminal sprite strip; all
original detailed artwork is preserved.
