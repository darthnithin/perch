# Chip — Perch's mascot

The little bird is named **Chip**. Perch is the application; Chip is its animated companion.

The user selected the **round second-pass set** on October 9, 2026, after comparing it against slimmer variants. This directory is the canonical artwork for Perch. Later flight experiments are not part of the approved set.

The art is manually plotted pixel art. It was not made with image generation. The earlier overall TUI concept image was generated, but is not included as a product asset here.

## Files

- `index.html`: offline animation preview with a motion toggle and initial reduced-motion preference support.
- `manifest.json`: frame dimensions, timing, poster indices, and intended playback.
- `STATE-strip.png`: transparent horizontal sprite sheet for each of six states.
- `STATE-static.png`: transparent static fallback/poster.
- `STATE.gif`: enlarged animated preview on a charcoal background.
- `perch-v2.gif` / `perch-v2.png`: animated and static presentation sheets.
- `motion-review.png`: selected frames for inspecting wingbeats, completion, and head turns.

Each sprite is 32 × 32 pixels. Each strip contains 64 frames; source frame duration is 80 ms. GIF encoders may combine identical frames while retaining their duration. Use the sprite sheet and manifest for application playback.

| State | Behavior |
| --- | --- |
| idle | Perched, occasional blink |
| running | Hovering wingbeat |
| queued | Dozing |
| complete | Crouch, hop, sparkles, settle |
| attention | Alert pose with a visible marker |
| reconnecting | Head turns and activity dots |

The completion preview loops to aid review. In the application, play it once for an observed completion event, then derive the appropriate current state. Other-state priority and a compact terminal rendition are implementation work.

## Regeneration

`tools/draw_bird.py` contains the self-contained second-pass drawing and export code. It requires Python and Pillow for artwork development only; this does not introduce a Python runtime dependency into the planned Go app.

From the repository root on macOS, with Pillow available:

```sh
python3 tools/draw_bird.py
```

It regenerates sprites and preview images in this directory, preserving the separately maintained HTML preview. The presentation sheets currently use `/System/Library/Fonts/Menlo.ttc`; adapt that font path before regenerating on another platform. The sprite artwork itself is font-independent.

The approved files were copied from the original design workspace and checked byte-for-byte using SHA-256. Only the preview's link back to the discarded first-pass comparison was removed. Original design files remain in the source workspace.

No font files are bundled. Software is covered by the root MIT license;
Chip artwork is reserved under the [artwork terms](LICENSE).

## Separate terminal artwork

Keep every detailed PNG strip, static image and animation above unchanged. These
remain the canonical assets for future web pages and other image-capable surfaces.
Terminal constraints are not a reason to replace or simplify those source files.

`octant.go` renders the original sprite silhouette using Unicode 16.0 block
mosaics, with the stronger wing shading selected as **D** in the native terminal
comparison. It fits two source pixels horizontally and four vertically in each
cell, without changing source geometry. The eye becomes negative space; each
cell uses one foreground color, favoring the folded-wing shade. No cell
background colors are emitted. Ioskeley Mono contains these characters; font
fallback and native line spacing can still introduce seams.

The rejected hand-redrawn terminal version is archived under
`output/design/chip-lab/rejected-terminal-drawing.go.txt`, outside the application
package. It must not replace the approved detailed assets or the selected D
conversion.

Run `go run ./tools/chip-preview` to compare all six selected terminal states.
Space pauses playback, arrow keys step frames, and `q` exits. Completion loops in
this review tool only. `--static` starts paused; `--json` exports ANSI review frames.
The main TUI plays completion once before returning to the current workload state.
`--reduced-motion` uses static posters, and compact/no-color views omit Chip.

The workspace uses `terminal-running-strip.png` for flight: a separate 64-frame
strip derived from the slower figure-eight study. A stroke takes 640 ms with
32 poses sampled at 20 ms intervals. The original body, face and 1.28-second
hover cadence remain intact. Other states still use their original strips at
80 ms. The original `running-strip.png` and web/GIF files remain unchanged.
Regenerate only the terminal flight strip with:

```sh
.venv/bin/python tools/chip_wing_study.py --export
```

The developer review now uses the same playback intervals as the workspace.
Each state is labelled with its frame index and cadence. Pausing freezes both
clocks; arrow keys align all states to one source frame for direct comparisons.
Single-state `--sample --state running` uses the same slower stroke.
These terminal playback choices do not alter the original web/GIF assets.

`make chip-test` verifies playback, pause, aligned stepping, a running-state
sample, and terminal restoration in a real PTY. Screens saved under
`output/terminal/chip-cadence-*` are emulator renderings; native terminal font
and spacing acceptance still requires a separate visual check.
