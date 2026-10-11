"""Review and export the terminal figure-eight stroke, preserving original art.

Render: .venv/bin/python tools/chip_wing_study.py --render
Terminal: .venv/bin/python tools/chip_wing_study.py --play
Export: .venv/bin/python tools/chip_wing_study.py --export
The terminal preview uses the current font; use the supplied Ioskeley profile.
"""
import argparse
import ast
from collections import Counter
import hashlib
import json
import math
import os
from pathlib import Path
import re
import select
import signal
import subprocess
import sys
import tempfile
import termios
import time
import tty

from PIL import Image, ImageDraw

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "output/design/wing-eight-study-v2"
COUNT, FRAME_MS = 64, 20
BG = "1c1b1a"


def original_art():
    # The original authoring script writes files at import time. Evaluate only
    # its palette and the three drawing functions, never its export statements.
    namespace = {"Image": Image, "ImageDraw": ImageDraw}
    tree = ast.parse((ROOT / "tools/draw_bird.py").read_text())
    for node in tree.body:
        if isinstance(node, ast.Assign) and isinstance(node.targets[0], ast.Tuple):
            if any(isinstance(n, ast.Name) and n.id in ("P", "INK") for n in node.targets[0].elts):
                exec(compile(ast.Module([node], type_ignores=[]), "palette", "exec"), namespace)
        if isinstance(node, ast.FunctionDef) and node.name in ("blank", "head", "flying"):
            exec(compile(ast.Module([node], type_ignores=[]), "sprite", "exec"), namespace)
            if node.name == "flying":
                flying = node
    with Image.open(ROOT / "assets/bird/running-strip.png") as source:
        assert all(namespace["flying"](i).tobytes() == source.crop((i * 32, 0, i * 32 + 32, 32)).tobytes()
                   for i in range(64)), "Source art changed; recheck the study's body extraction."
    # Remove wing calls, identified by their use of the tip coordinates. All
    # original body, face, bill and tail drawing commands remain verbatim.
    body = ast.parse(ast.unparse(flying)).body[0]
    body.name = "body"
    body.body = [node for node in body.body if not (
        isinstance(node, ast.Expr) and
        any(isinstance(n, ast.Name) and n.id in ("x", "y") for n in ast.walk(node)))]
    exec(compile(ast.Module([body], type_ignores=[]), "body", "exec"), namespace)
    return namespace


def tip_at(phase):
    # A flattened figure eight in this stylized side projection. Sines give a
    # continuous stroke and a gradual sweep reversal, not linear pose stepping.
    angle = 2 * math.pi * phase
    # Keep the close turn left of the face so its edge-on silhouette remains
    # readable instead of disappearing into the head for several poses.
    return 8.5 - 5.5 * math.cos(angle), 10 + 3 * math.sin(2 * angle)


def wing(layer, root, tip, width, fill, edge, shadow):
    dx, dy = tip[0] - root[0], tip[1] - root[1]
    length = math.hypot(dx, dy)
    normal = (-dy / length, dx / length)
    def p(distance, across=0):
        return (round(root[0] + dx * distance + normal[0] * across),
                round(root[1] + dy * distance + normal[1] * across))
    # A tapered blade rotates about the shoulder. Its projected chord narrows
    # through each reversal, suggesting wing roll rather than a rubber paddle.
    d = ImageDraw.Draw(layer)
    d.polygon([p(0, -.5), p(.55, -width * .35), p(1),
               p(.88, width * .5), p(.38, width), p(0, 1)], fill=fill)
    d.line([p(0), p(.55, -width * .35), p(1)], fill=edge)
    if width > 1.4:
        d.line([p(.2, width * .7), p(.7, width * .5)], fill=shadow)


def sprites(art):
    body = art["body"](0)
    face = art["blank"]()
    art["head"](ImageDraw.Draw(face), x=21, y=9)
    frames = []
    for f in range(COUNT):
        phase = (f % 32) / 32  # 640 ms per beat: half the shipped beat speed.
        tip = tip_at(phase)
        width = 1 + 2 * abs(math.sin(2 * math.pi * phase))
        im = art["blank"]()
        far_tip = (18 + .8 * (tip[0] - 17), 16 + .8 * (tip[1] - 18))
        wing(im, (18, 17), far_tip, width * .7, art["S"], art["P"], art["S"])
        im.alpha_composite(body)
        wing(im, (17, 18), tip, width, art["P"], art["H"], art["S"])
        # Keep the approved face intact even when the wing passes behind it.
        im.alpha_composite(face)
        shifted = art["blank"]()
        # Preserve the original one-pixel hover and its wall-clock cadence.
        hover_phase = (f * FRAME_MS / 40) % 32
        shifted.alpha_composite(im, (0, -1 if 12 <= hover_phase < 24 else 0))
        assert shifted.getbbox()[0] >= 2 and shifted.getbbox()[3] <= 28
        frames.append(shifted)
    return frames


def cell(text=" ", foreground="ece6df"):
    return dict(text=text, foreground=foreground, background=BG, bold=False, underline=False)


def octants(im):
    glyphs = re.findall("'([^']+)'", (ROOT / "assets/bird/octant_map.go").read_text())
    assert len(glyphs) == 256
    rows = []
    for y in range(0, 28, 4):
        row = []
        for x in range(2, 32, 2):
            mask, colors = 0, Counter()
            for dy in range(4):
                for dx in range(2):
                    r, g, b, a = im.getpixel((x + dx, y + dy))
                    if not a or (r, g, b) == (48, 42, 42):
                        continue
                    mask |= 1 << (dx + 2 * dy)
                    colors[r, g, b] += 9 if (r, g, b) == (168, 111, 89) else 5
            color = min(colors, key=lambda c: (-colors[c], c)) if colors else (236, 230, 223)
            row.append(cell(glyphs[mask], "%02x%02x%02x" % color))
        rows.append(row)
    return rows


def screens(frames):
    result = []
    for im in frames:
        grid = [[cell() for _ in range(54)] for _ in range(19)]
        def label(x, y, text, color="aaa096"):
            for i, ch in enumerate(text):
                grid[y][x + i] = cell(ch, color)
        label(1, 0, "Chip / a slower figure-eight stroke", "e6ad8f")
        for y, row in enumerate(octants(im)):
            grid[3 + y][7:22] = row
        label(1, 10, "╭" + "─" * 49 + "╮", "e6ad8f")
        for y, text in ((11, "  My jobs"), (12, ""),
                        (13, "  9100   parameter-sweep             ● running")):
            label(1, y, "│" + text.ljust(49) + "│")
        label(1, 14, "╰" + "─" * 49 + "╯", "e6ad8f")
        label(1, 16, "Same body hover / half-speed wingbeat")
        label(1, 17, "Terminal motion / original web artwork preserved")
        assert all(len(row) == 54 for row in grid)
        result.append(dict(columns=54, rows=19, cells=grid))
    return result


def play(data):
    if not sys.stdin.isatty() or not sys.stdout.isatty():
        raise SystemExit("Run --play in your Ioskeley Terminal window.")
    frames = []
    for screen in data:
        rows = []
        for row in screen["cells"]:
            parts, previous = [], None
            for c in row:
                if c["foreground"] != previous:
                    previous = c["foreground"]
                    parts.append("\033[38;2;%d;%d;%dm" % tuple(int(previous[i:i+2], 16) for i in (0, 2, 4)))
                parts.append(c["text"])
            rows.append("".join(parts) + "\033[0m")
        frames.append(rows)
    fd = sys.stdin.fileno()
    saved = termios.tcgetattr(fd)
    def stop(*_):
        raise KeyboardInterrupt
    old_term = signal.signal(signal.SIGTERM, stop)
    frame, paused, previous, dimensions = 0, False, [], None
    interval = FRAME_MS / 1000
    next_frame = time.monotonic() + interval
    try:
        tty.setraw(fd)
        sys.stdout.write("\033[?1049h\033[?25l\033[2J")
        while True:
            size = os.get_terminal_size(sys.stdout.fileno())
            if size != dimensions:
                dimensions, previous = size, []
                sys.stdout.write("\033[2J")
            lines = frames[frame] + ["Space pause/play | arrows step | q quit"]
            if size.columns < 55 or size.lines < 21:
                lines = ["Resize to 55 columns / 21 rows; q exits."[:max(1, size.columns - 1)]]
            for y, line in enumerate(lines):
                if y >= len(previous) or line != previous[y]:
                    sys.stdout.write(f"\033[{y+1};1H" + line + "\033[K")
            previous = lines
            sys.stdout.flush()
            timeout = max(0, next_frame - time.monotonic()) if not paused else .1
            if select.select([fd], [], [], min(timeout, .1))[0]:
                key = os.read(fd, 32)
                if key in (b"q", b"\x03", b"\x04"):
                    break
                if key == b" ":
                    paused = not paused
                    next_frame = time.monotonic() + interval
                if key in (b"\x1b[C", b"\x1b[D"):
                    paused = True
                    frame = (frame + (1 if key.endswith(b"C") else -1)) % COUNT
            now = time.monotonic()
            if not paused and now >= next_frame:
                elapsed = max(1, int((now - next_frame) / interval) + 1)
                frame = (frame + elapsed) % COUNT
                next_frame += elapsed * interval
    except KeyboardInterrupt:
        pass
    finally:
        sys.stdout.write("\033[0m\033[?25h\033[?1049l")
        sys.stdout.flush()
        termios.tcsetattr(fd, termios.TCSADRAIN, saved)
        signal.signal(signal.SIGTERM, old_term)


def render(data):
    OUT.mkdir(parents=True, exist_ok=True)
    (OUT / "screens.json").write_text(json.dumps(data))
    with tempfile.TemporaryDirectory(prefix="perch-wing-coretext-") as cache:
        subprocess.run(["swift", "-module-cache-path", cache, str(ROOT / "tools/render_readme.swift"),
                        str(OUT / "screens.json"), str(ROOT / "docs/terminal/Perch-Ioskeley.terminal"),
                        str(OUT / "frames"), "2"], check=True)
    images = [Image.open(OUT / "frames" / f"frame-{i:03d}.png").convert("RGB") for i in range(COUNT)]
    palette = images[0].quantize(colors=256)
    indexed = [im.quantize(palette=palette, dither=Image.Dither.NONE) for im in images]
    indexed[0].save(OUT / "preview.gif", save_all=True, append_images=indexed[1:],
                    duration=FRAME_MS, loop=0, disposal=1, optimize=True)
    images[0].save(OUT / "preview.png")
    # Actual Ioskeley glyphs, every other pose of the 32-position beat.
    sheet = Image.new("RGB", (4 * 360, 4 * 260), images[0].getpixel((0, 0)))
    d = ImageDraw.Draw(sheet)
    for i in range(16):
        x, y = (i % 4) * 360, (i // 4) * 260
        crop = images[i * 2].crop((110, 100, 420, 340))
        sheet.paste(crop, (x, y + 20))
        d.text((x + 20, y + 5), f"{i * 2:02d} / {i * 2 * FRAME_MS} ms", fill="#e6ad8f")
    sheet.save(OUT / "poses.png")
    # Verify the encoded loop against every frame, including GIF coalescing.
    with Image.open(OUT / "preview.gif") as gif:
        elapsed = 0
        for i in range(gif.n_frames):
            gif.seek(i)
            duration = gif.info["duration"]
            for sample in range(elapsed // FRAME_MS, (elapsed + duration) // FRAME_MS):
                assert gif.convert("RGB").tobytes() == indexed[sample].convert("RGB").tobytes()
            elapsed += duration
        assert elapsed == COUNT * FRAME_MS
    print(f"Rendered {OUT}; {COUNT * FRAME_MS} ms loop, 640 ms wingbeat. Original art preserved.")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--render", action="store_true")
    parser.add_argument("--play", action="store_true")
    parser.add_argument("--export", action="store_true", help="write the separate terminal flight strip")
    args = parser.parse_args()
    if not (args.render or args.play or args.export):
        parser.error("choose --render, --play or --export")
    target = ROOT / "assets/bird/terminal-running-strip.png"
    sources = [p for p in (ROOT / "assets/bird").glob("*.png") if p != target]
    before = {p: hashlib.sha256(p.read_bytes()).digest() for p in sources}
    frames = sprites(original_art())
    if args.export:
        strip = Image.new("RGBA", (32 * COUNT, 32))
        for i, frame in enumerate(frames):
            strip.paste(frame, (32 * i, 0))
        strip.save(target)
        print(f"Exported {target}; {FRAME_MS} ms frames, 640 ms wingbeat")
    data = screens(frames)
    if args.render:
        render(data)
    if args.play:
        play(data)
    assert before == {p: hashlib.sha256(p.read_bytes()).digest() for p in sources}


if __name__ == "__main__":
    main()
