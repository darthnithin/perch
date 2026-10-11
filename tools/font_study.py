#!/usr/bin/env python3
"""Render equal-size font specimens from locally installed fonts, without installing any."""
import argparse
from pathlib import Path
from PIL import Image, ImageDraw, ImageFont

parser = argparse.ArgumentParser()
parser.add_argument('--scale', type=int, choices=range(1,5), default=1)
SCALE = parser.parse_args().scale
def scaled(values): return tuple(v*SCALE for v in values)
suffix = f'-{SCALE}x' if SCALE > 1 else ''

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / 'output' / 'design'
OUT.mkdir(parents=True, exist_ok=True)
FONTS = Path.home() / 'Library' / 'Fonts'
BG, INK, MUTED, PEACH, GREEN = '#1C1B1A', '#ECE6DF', '#A39B93', '#E6AD8F', '#A8CA9A'
families = [
    ('A', 'Ioskeley Mono', FONTS/'IoskeleyMono-Regular.ttf', FONTS/'IoskeleyMono-Bold.ttf'),
    ('B', 'JetBrains Mono', FONTS/'JetBrainsMono[wght].ttf', FONTS/'JetBrainsMono[wght].ttf'),
    ('C', 'IBM Plex Mono', FONTS/'IBMPlexMono-Regular.otf', FONTS/'IBMPlexMono-Bold.otf'),
]
label = ImageFont.truetype('/System/Library/Fonts/Menlo.ttc', 17*SCALE)
board = Image.new('RGB', scaled((1180, 1004)), BG)
draw = ImageDraw.Draw(board)
draw.text(scaled((32, 20)), 'PERCH / FONT STUDY     Same 22 px size, regular + bold, same colors', font=label, fill=MUTED)
for index, (letter, name, regular_path, bold_path) in enumerate(families):
    top = 68 + index*306
    regular = ImageFont.truetype(str(regular_path), 22*SCALE)
    bold = ImageFont.truetype(str(bold_path), 22*SCALE)
    if name == 'JetBrains Mono':
        regular.set_variation_by_axes([400])
        bold.set_variation_by_axes([700])
    draw.line(scaled((32, top, 1148, top)), fill='#57514B', width=SCALE)
    draw.text(scaled((32, top+16)), f'{letter}   {name}', font=label, fill=MUTED)
    x, y = 46, top+52
    draw.text(scaled((x,y)), 'perch', font=bold, fill=PEACH)
    draw.text(scaled((x+105,y)), 'Hummingbird workspace', font=regular, fill=INK)
    draw.text(scaled((x,y+39)), 'My jobs', font=bold, fill=INK)
    draw.text(scaled((x,y+78)), '  JOB        NAME                 STATE      ELAPSED', font=regular, fill=MUTED)
    draw.rectangle(scaled((x-8,y+111,1138,y+142)), fill='#48382F')
    draw.text(scaled((x,y+113)), '> 9100_11    parameter-sweep       running    43m 41s', font=regular, fill=INK)
    draw.text(scaled((x,y+146)), '  9231       embedding-model      queued      0m 00s', font=regular, fill=INK)
    draw.text(scaled((x,y+191)), '> /  Find a command or jump to a job', font=regular, fill=MUTED)
    draw.text(scaled((x,y+228)), '0 O  1 l I  {} [] ()  /home/researcher/run_01.log', font=regular, fill=MUTED)
    board.crop(scaled((0,top,1180,top+306))).save(OUT/f'font-{letter.lower()}{suffix}.png')
board.save(OUT/f'font-study{suffix}.png')
print(OUT/f'font-study{suffix}.png')
