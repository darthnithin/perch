"""Perch v2: hand-plotted layered pixel art and deliberate animation timing."""
from pathlib import Path
import json
from PIL import Image, ImageDraw, ImageFont

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "assets" / "bird"
OUT.mkdir(exist_ok=True)
BG, INK = "#191B20", "#302A2A"
P, H, S = "#DCA184", "#F3C9A5", "#A86F59"
GREEN, RED, MUTED = "#ABD4A0", "#E78979", "#8993A7"
N, MS = 64, 80


def blank():
    return Image.new("RGBA", (32, 32))


def head(d, x=18, y=9, facing="right", closed=False):
    # Keep the same head volume in all poses, anchored to the neck.
    d.polygon([(x-2,y), (x+1,y), (x+3,y+2), (x+3,y+5),
               (x+1,y+7), (x-3,y+6), (x-4,y+3)], fill=P)
    d.line([(x-2,y), (x+1,y)], fill=H)
    if facing == "front":
        d.point((x-2,y+3), fill=INK)
        d.point((x+1,y+3), fill=INK)
        d.line((x,y+4,x,y+5), fill=S)
    else:
        left = facing == "left"
        eye = x-2 if left else x+1
        if closed:
            d.line((eye-1,y+4,eye+1,y+4), fill=INK)
        else:
            d.point((eye,y+3), fill=INK)
        d.line((x-9 if left else x+3,y+3,x-3 if left else x+9,y+3), fill=H)


def perch(f=0, mode="idle"):
    im = blank()
    d = ImageDraw.Draw(im)
    d.line((6,27,26,27), fill=S)
    d.line((7,26,27,26), fill=H)
    # A short preparatory crouch, then a single hop with a settled landing.
    hop = 0
    if mode == "complete":
        hop = {5:-1,6:-1,7:1,8:3,9:4,10:4,11:3,12:2,13:1,14:-1}.get(f,0)
    bird = blank()
    b = ImageDraw.Draw(bird)
    b.polygon([(14,20),(17,21),(12,25),(9,26),(11,22)], fill=S)
    b.line((14,22,10,25), fill=P)
    b.polygon([(15,13),(19,12),(22,15),(22,18),(20,21),
               (17,23),(14,22),(12,19),(12,16)], fill=P)
    b.polygon([(20,14),(21,15),(21,18),(19,21),(17,21),(19,17)], fill=H)
    # Folded wing reads independently from the breast.
    b.polygon([(15,14),(17,15),(17,18),(14,21),(13,19),(13,17)], fill=S)
    b.line((15,15,14,18), fill=P)
    if mode == "queued":
        head(b, x=18, y=11, closed=True)
        # Chest rises by one pixel; the feet stay planted.
        if 20 <= f % 40 < 30:
            b.point((21,19), fill=H)
    elif mode == "reconnecting":
        facing = "front" if 14 <= f < 18 or 39 <= f < 43 else "left" if 18 <= f < 39 else "right"
        head(b, facing=facing, closed=f in (48,49))
    elif mode == "attention":
        head(b, y=8, closed=f in (46,47))
    else:
        head(b, closed=f in (44,45))
    if hop > 0:
        b.line((16,23,17,23), fill=S)
        b.line((19,22,20,22), fill=S)
    else:
        b.line((16,23,16,25), fill=S)
        b.line((19,22,19,25), fill=S)
        b.line((15,25,17,25), fill=P)
        b.line((18,25,20,25), fill=P)
    im.alpha_composite(bird, (0,-hop))
    d = ImageDraw.Draw(im)
    if mode == "queued":
        # The sleep symbol stays clear of the bill.
        t = f % 32
        if t < 25:
            x,y = 25,5-t//9
            d.line([(x,y),(x+3,y),(x,y+3),(x+3,y+3)], fill=MUTED)
    if mode == "complete" and 8 <= f <= 24:
        for x,y,start in [(8,9,8),(25,5,11),(26,20,14)]:
            if start <= f < start+10:
                r = 1 if f < start+7 else 0
                d.line((x-r,y,x+r,y), fill=GREEN)
                d.line((x,y-r,x,y+r), fill=GREEN)
    if mode == "attention":
        d.line((26,3,26,6), fill=RED, width=2)
        d.point((26,8), fill=RED)
        d.point((27,8), fill=RED)
    if mode == "reconnecting":
        for i in range(3):
            d.point((23+i*3,4), fill=H if (f//8)%3 == i else S)
    return im


def flying(f):
    im = blank()
    d = ImageDraw.Draw(im)
    # All wing strokes trail the shoulder. The face and bill stay unobscured.
    tips = [(7,3),(4,6),(3,11),(5,16),(9,22),(7,18),(4,12),(5,7)]
    x,y = tips[f%8]
    d.polygon([(18,17),(x+6,max(2,y-2)),(x+7,y+1),(19,19)], fill=S)
    d.line((18,17,x+6,max(2,y-2)), fill=P)
    d.polygon([(16,20),(12,23),(8,26),(10,22),(14,19)], fill=S)
    d.line((13,22,9,25), fill=H)
    d.polygon([(16,14),(20,12),(23,14),(22,18),(19,21),(16,22),(14,20),(14,17)], fill=P)
    d.polygon([(21,15),(22,15),(21,18),(18,20),(17,20),(19,17)], fill=H)
    head(d, x=21, y=9)
    d.polygon([(17,18),(x,y),(x,y+2),(x+3,y+4),(16,20)], fill=P)
    d.line((x,y,16,17), fill=H)
    d.line((x+2,y+3,15,19), fill=S)
    shifted = blank()
    shifted.alpha_composite(im, (0,-1 if 12 <= f%32 < 24 else 0))
    return shifted


STATES = [
    ("idle","IDLE","A blink. A quiet lookout."),
    ("running","RUNNING","A wingbeat with a steady body."),
    ("queued","QUEUED","Head tucked. Patiently dozing."),
    ("complete","COMPLETE","Crouch, hop, tiny celebration."),
    ("attention","NEEDS ATTENTION","Head up. A persistent signal."),
    ("reconnecting","RECONNECTING","A head turn, searching for home."),
]
frames = {state:[flying(f) if state=="running" else perch(f,state) for f in range(N)] for state,*_ in STATES}
posters = {"idle":0,"running":1,"queued":8,"complete":11,"attention":0,"reconnecting":23}
manifest = {"version":2,"size":[32,32],"frame_ms":MS,"frame_count":N,"states":{}}
for state,*_ in STATES:
    strip = Image.new("RGBA",(32*N,32))
    enlarged = []
    for f,im in enumerate(frames[state]):
        strip.alpha_composite(im,(f*32,0))
        bg = Image.new("RGB",(256,256),BG)
        sprite = im.resize((256,256),Image.Resampling.NEAREST)
        bg.paste(sprite,(0,0),sprite)
        enlarged.append(bg)
    strip.save(OUT/f"{state}-strip.png")
    frames[state][posters[state]].save(OUT/f"{state}-static.png")
    enlarged[0].save(OUT/f"{state}.gif",save_all=True,append_images=enlarged[1:],duration=MS,loop=0,disposal=2,optimize=False)
    manifest["states"][state]={"strip":f"{state}-strip.png","poster":posters[state],"playback":"once_then_idle" if state=="complete" else "loop"}
(OUT/"manifest.json").write_text(json.dumps(manifest,indent=2)+"\n")

font = "/System/Library/Fonts/Menlo.ttc"
title = ImageFont.truetype(font,28)
label = ImageFont.truetype(font,17)
small = ImageFont.truetype(font,13)
def board(f=None):
    result=Image.new("RGB",(1200,872),BG)
    d=ImageDraw.Draw(result)
    d.text((36,28),"perch / a little more alive",font=title,fill=H)
    d.text((38,75),"SECOND PASS     HAND-PLOTTED PIXELS     LARGE + SMALL PREVIEWS",font=small,fill=MUTED)
    for i,(state,name,desc) in enumerate(STATES):
        x,y=(i%3)*400,116+(i//3)*356
        if i%3:d.line((x,y+8,x,y+325),fill="#30343C")
        sprite=frames[state][posters[state] if f is None else f]
        big=sprite.resize((224,224),Image.Resampling.NEAREST)
        result.paste(big,(x+88,y),big)
        d.text((x+28,y+230),name,font=label,fill=RED if state=="attention" else GREEN if state=="complete" else H)
        d.text((x+28,y+259),desc,font=small,fill=MUTED)
        # Real pixel sizes, without interpolation.
        for offset,scale in [(28,1),(86,2)]:
            tiny=sprite.resize((32*scale,32*scale),Image.Resampling.NEAREST)
            result.paste(tiny,(x+offset,y+286 if scale==1 else y+270),tiny)
        d.text((x+171,y+297),"32px / 64px",font=small,fill=MUTED)
    d.text((38,838),"ILLUSTRATIVE STATES · COMPLETION REPEATS FOR REVIEW · NO LIVE DATA",font=small,fill=MUTED)
    return result

board().save(OUT/"perch-v2.png")
boards=[board(f) for f in range(N)]
boards[0].save(OUT/"perch-v2.gif",save_all=True,append_images=boards[1:],duration=MS,loop=0,disposal=2,optimize=False)

# The versioned assets/bird/index.html preview is maintained separately.

# Export a frame contact sheet for visual motion review.
contact=Image.new("RGB",(1024,384),BG)
for row,state in enumerate(("running","complete","reconnecting")):
    picks=range(8) if state=="running" else (0,6,8,10,12,14,20,30) if state=="complete" else (0,14,17,18,30,39,42,48)
    for col,f in enumerate(picks):
        sprite=frames[state][f].resize((128,128),Image.Resampling.NEAREST)
        contact.paste(sprite,(128*col,128*row),sprite)
contact.save(OUT/"motion-review.png")

for state,*_ in STATES:
    with Image.open(OUT/f"{state}-strip.png") as im:
        assert im.mode=="RGBA" and im.size==(N*32,32)
    for im in frames[state]:
        assert im.getbbox() and im.getbbox()[0]>0 and im.getbbox()[2]<32
with Image.open(OUT/"perch-v2.gif") as im:
    assert im.n_frames==N
print(f"Verified six sprite strips and {N} animation frames. Saved to {OUT}")
