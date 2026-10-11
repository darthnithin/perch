#!/usr/bin/env python3
"""Render Perch's captured ANSI frame for layout review, not a native screenshot.

Requires Pillow. Only SGR colors/styles are expected in model View() captures.
"""
import pathlib
import re
import sys
from PIL import Image, ImageDraw, ImageFont

font_path = '/System/Library/Fonts/Menlo.ttc'
font = ImageFont.truetype(font_path, 17)
cell_w, cell_h = 11, 23
background = (22, 26, 28)
foreground = (231, 221, 210)
sequence = re.compile(r'\x1b\[([0-9;]*)m')

for name in sys.argv[1:]:
    path = pathlib.Path(name)
    lines = path.read_text().splitlines()
    cols = max(len(sequence.sub('', line)) for line in lines)
    image = Image.new('RGB', (cols * cell_w + 32, len(lines) * cell_h + 32), background)
    draw = ImageDraw.Draw(image)
    fg, bg, bold = foreground, background, False
    for row, line in enumerate(lines):
        pos, column = 0, 0
        while pos < len(line):
            match = sequence.match(line, pos)
            if match:
                codes = [int(x) if x else 0 for x in match[1].split(';')]
                i = 0
                while i < len(codes):
                    code = codes[i]
                    if code == 0: fg, bg, bold = foreground, background, False
                    elif code == 1: bold = True
                    elif code == 22: bold = False
                    elif code == 39: fg = foreground
                    elif code == 49: bg = background
                    elif code in (38, 48) and codes[i+1:i+2] == [2]:
                        color = tuple(codes[i+2:i+5])
                        if code == 38: fg = color
                        else: bg = color
                        i += 4
                    i += 1
                pos = match.end()
                continue
            char = line[pos]
            x, y = 16 + column * cell_w, 16 + row * cell_h
            draw.rectangle((x, y, x + cell_w - 1, y + cell_h - 1), fill=bg)
            if char == '▀':
                draw.rectangle((x, y, x + cell_w - 1, y + cell_h // 2 - 1), fill=fg)
            elif char != ' ':
                draw.text((x, y + 1), char, font=font, fill=fg, stroke_width=0)
            column += 1
            pos += 1
    dest = path.with_suffix('.png')
    image.save(dest)
    print(dest)
