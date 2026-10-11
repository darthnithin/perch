#!/usr/bin/env python3
"""Capture the real offline workspace with the approved font and spacing.

macOS development tool using installed Ioskeley and offline CoreText drawing.
Does not control Terminal.app, change settings, or fabricate glyph geometry.
Run with --animate to capture a 64-frame, 40 ms wingbeat loop as a GIF.
"""
import argparse
import json
from pathlib import Path
import shutil
import subprocess
import tempfile
import time
from PIL import Image
from test_terminal import Session, ROOT

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--animate', action='store_true')
args = parser.parse_args()
out = ROOT / 'output' / 'readme' / ('animated' if args.animate else 'static')
out.mkdir(parents=True, exist_ok=True)
arguments = ['--demo', '--no-config']
if not args.animate:
    arguments.append('--reduced-motion')
s = Session(arguments, width=140, height=44)

def capture():
    def rgb(value, default):
        return value if len(value) == 6 and all(c in '0123456789abcdefABCDEF' for c in value) else default
    rows = []
    for y in range(s.height):
        cells = []
        for x in range(s.width):
            c = s.screen.buffer[y][x]
            fg, bg = rgb(c.fg, 'ece6df'), rgb(c.bg, '1c1b1a')
            if c.reverse:
                fg, bg = bg, fg
            cells.append(dict(text=c.data, foreground=fg, background=bg, bold=c.bold, underline=c.underscore))
        rows.append(cells)
    return dict(columns=s.width, rows=s.height, cells=rows)

try:
    s.wait('parameter-sweep')
    start = time.monotonic()
    screens = []
    for index in range(64 if args.animate else 1):
        s.pump(max(0, start + index * .04 - time.monotonic()))
        screens.append(capture())
    (out / 'screens.json').write_text(json.dumps(screens))
    (out / 'screen.txt').write_text(s.text)
    s.close()
except BaseException:
    s.abort()
    raise
with tempfile.TemporaryDirectory(prefix='perch-coretext-') as cache:
    subprocess.run(['swift', '-module-cache-path', cache, str(ROOT / 'tools/render_readme.swift'),
                    str(out / 'screens.json'), str(ROOT / 'docs/terminal/Perch-Ioskeley.terminal'),
                    str(out), '2' if args.animate else '3'], check=True)
if args.animate:
    frames = [Image.open(out / f'frame-{i:03d}.png').convert('RGB') for i in range(len(screens))]
    # Share one palette across frames to avoid shimmering text and color shifts.
    palette = frames[0].quantize(colors=256)
    indexed = [frame.quantize(palette=palette, dither=Image.Dither.NONE) for frame in frames]
    indexed[0].save(out / 'workspace.gif', save_all=True, append_images=indexed[1:],
                    duration=40, loop=0, optimize=True, disposal=1)
    with Image.open(out / 'workspace.gif') as gif:
        count = gif.n_frames
        duration = 0
        for i in range(count):
            gif.seek(i)
            duration += gif.info['duration']
        assert count > 8 and duration == len(screens) * 40
        print(f'GIF verified: {gif.size}, {count} encoded frames, {duration} ms, {(out / "workspace.gif").stat().st_size} bytes')
else:
    shutil.copyfile(out / 'frame-000.png', out / 'workspace.png')
print(out)
