#!/usr/bin/env python3
"""Run Perch in a real PTY, inspect its VT screen, and verify terminal cleanup.

This does not control Terminal.app. PNGs are emulator renders of actual PTY
output. Install tools/requirements.txt into a development virtual environment.
"""
import codecs
import fcntl
import json
import os
import pathlib
import pty
import re
import select
import signal
import struct
import subprocess
import sys
import termios
import time

import pyte
from PIL import Image, ImageDraw, ImageFont

ROOT = pathlib.Path(__file__).resolve().parents[1]
OUT = ROOT / 'output' / 'terminal'
OUT.mkdir(parents=True, exist_ok=True)
os.chmod(OUT, 0o700)

def draw_box_cell(draw, glyph, x, y, w, h, fill, scale):
    """Join box strokes at cell boundaries, as terminal box rendering does.

    Font fallback glyph metrics do not span our configurable line height.
    This is emulator rendering, not a claim about native Terminal pixels.
    """
    if glyph not in '─━│╭╮╰╯': return False
    cx,cy=x+w//2,y+h//2
    weight=scale*(2 if glyph=='━' else 1)
    if glyph in '─━': draw.line((x,cy,x+w-1,cy),fill=fill,width=weight)
    elif glyph=='│': draw.line((cx,y,cx,y+h-1),fill=fill,width=weight)
    else:
        r=max(scale,min(w,h)//2-scale)
        if glyph=='╭':
            draw.arc((cx,cy,cx+2*r,cy+2*r),180,270,fill=fill,width=weight)
            draw.line((cx,cy+r,cx,y+h-1),fill=fill,width=weight)
            draw.line((cx+r,cy,x+w-1,cy),fill=fill,width=weight)
        elif glyph=='╮':
            draw.arc((cx-2*r,cy,cx,cy+2*r),270,360,fill=fill,width=weight)
            draw.line((cx,cy+r,cx,y+h-1),fill=fill,width=weight)
            draw.line((x,cy,cx-r,cy),fill=fill,width=weight)
        elif glyph=='╰':
            draw.arc((cx,cy-2*r,cx+2*r,cy),90,180,fill=fill,width=weight)
            draw.line((cx,y,cx,cy-r),fill=fill,width=weight)
            draw.line((cx+r,cy,x+w-1,cy),fill=fill,width=weight)
        else:
            draw.arc((cx-2*r,cy-2*r,cx,cy),0,90,fill=fill,width=weight)
            draw.line((cx,y,cx,cy-r),fill=fill,width=weight)
            draw.line((x,cy,cx-r,cy),fill=fill,width=weight)
    return True

def draw_block_cell(draw, glyph, x, y, w, h, fill):
    """Render Unicode block elements using their specified cell fractions."""
    quadrants = ' ▘▝▀▖▌▞▛▗▚▐▜▄▙▟█'
    def rect(l, t, r, b):
        draw.rectangle((x+round(w*l), y+round(h*t),
                        x+round(w*r)-1, y+round(h*b)-1), fill=fill)
    if glyph in quadrants and len(glyph)==1:
        mask=quadrants.index(glyph)
        for bit in range(4):
            if mask & (1<<bit):
                l,t=(bit%2)/2,(bit//2)/2
                rect(l,t,l+.5,t+.5)
    elif glyph in '▁▂▃▄▅▆▇' and len(glyph)==1:
        rect(0,1-('▁▂▃▄▅▆▇'.index(glyph)+1)/8,1,1)
    elif glyph in '▏▎▍▌▋▊▉' and len(glyph)==1:
        rect(0,0,('▏▎▍▌▋▊▉'.index(glyph)+1)/8,1)
    elif glyph=='▔': rect(0,0,1,1/8)
    elif glyph=='▕': rect(7/8,0,1,1)
    else: return False
    return True

class Screen(pyte.Screen):
    """Add scroll and backward-tab operations emitted by Bubble Tea v2.

    pyte 0.8 ignores CSI S/T/Z, leaving stale lines after optimized redraws.
    These operate on the current scrolling region and preserve cursor position.
    """
    def scroll_up(self, count=1):
        saved = self.cursor.y
        self.cursor.y = self.margins.bottom if self.margins else self.lines-1
        for _ in range(count or 1): self.index()
        self.cursor.y = saved
    def scroll_down(self, count=1):
        saved = self.cursor.y
        self.cursor.y = self.margins.top if self.margins else 0
        for _ in range(count or 1): self.reverse_index()
        self.cursor.y = saved
    def back_tab(self, count=1):
        for _ in range(count or 1):
            self.cursor.x = max((x for x in self.tabstops if x < self.cursor.x), default=0)

class Stream(pyte.Stream):
    csi = {**pyte.Stream.csi, 'S':'scroll_up', 'T':'scroll_down', 'Z':'back_tab'}
    events = pyte.Stream.events | {'scroll_up', 'scroll_down', 'back_tab'}
    def feed(self, data):
        # pyte mistakes xterm's modifyOtherKeys CSI > 4 ; n m for SGR,
        # inventing underline/dim attributes. These are input-mode controls.
        controls = ('\x1b[>4m', '\x1b[>4;0m', '\x1b[>4;1m', '\x1b[>4;2m')
        data = getattr(self, '_keymode_tail', '') + data
        self._keymode_tail = ''
        for control in controls:
            data = data.replace(control, '')
        for count in range(min(len(data), 7), 0, -1):
            if any(control.startswith(data[-count:]) for control in controls):
                self._keymode_tail, data = data[-count:], data[:-count]
                break
        super().feed(data)


class Session:
    def __init__(self, arguments, width=160, height=44, env=None, binary=None):
        self.master, self.slave = pty.openpty()
        self.before = termios.tcgetattr(self.slave)
        self.width, self.height = width, height
        fcntl.ioctl(self.slave, termios.TIOCSWINSZ, struct.pack('HHHH', height, width, 0, 0))
        self.screen = Screen(width, height)
        self.stream = Stream(self.screen)
        self.decoder = codecs.getincrementaldecoder('utf-8')('replace')
        self.raw = bytearray()
        environment = {**os.environ, 'TERM':'xterm-256color', 'COLORTERM':'truecolor'}
        environment.update(env or {})
        environment.pop('NO_COLOR', None)
        self.process = subprocess.Popen([str(binary or ROOT/'bin/perch'), *arguments], stdin=self.slave, stdout=self.slave, stderr=self.slave, start_new_session=True, env=environment)
    def pump(self, duration=.1):
        deadline = time.monotonic()+duration
        while time.monotonic()<deadline:
            if not select.select([self.master],[],[],min(.05,max(0,deadline-time.monotonic())))[0]: continue
            data = os.read(self.master,65536)
            if not data: break
            self.raw.extend(data)
            self.stream.feed(self.decoder.decode(data))
            if b'\x1b[6n' in data: os.write(self.master,b'\x1b[1;1R')
            if b'\x1b]11;?' in data: os.write(self.master,b'\x1b]11;rgb:1616/1a1a/1c1c\x1b\\')
    @property
    def text(self): return '\n'.join(self.screen.display)
    def wait(self, text, timeout=12):
        deadline=time.monotonic()+timeout
        while text not in self.text:
            self.pump()
            if self.process.poll() is not None: raise AssertionError(f'Perch exited {self.process.returncode}: {self.text}')
            if time.monotonic()>deadline: raise AssertionError(f'Missing {text!r}:\n{self.text}')
    def key(self, value): os.write(self.master,value.encode()); self.pump(.15)
    def resize(self,width,height):
        self.width,self.height=width,height
        self.screen.resize(height,width)
        fcntl.ioctl(self.slave,termios.TIOCSWINSZ,struct.pack('HHHH',height,width,0,0))
        os.kill(self.process.pid,signal.SIGWINCH);self.pump(.2)
    def screenshot(self,name,font_path=None,bold_path=None,font_size=16,scale=1):
        assert isinstance(scale,int) and 1 <= scale <= 4
        # Match Perch's approved preview font when installed. This only affects the
        # emulator image, never the user's terminal profile or font settings.
        preferred = pathlib.Path.home() / 'Library/Fonts/IoskeleyMono-Regular.ttf'
        preferred_bold = preferred.with_name('IoskeleyMono-Bold.ttf')
        if font_path is None and preferred.is_file():
            font_path = str(preferred)
            if bold_path is None and preferred_bold.is_file():
                bold_path = str(preferred_bold)
        base_font=ImageFont.truetype(font_path or '/System/Library/Fonts/Menlo.ttc',font_size)
        cw,ch=round(base_font.getlength('M'))*scale,round(font_size*1.375)*scale
        font=ImageFont.truetype(font_path or '/System/Library/Fonts/Menlo.ttc',font_size*scale)
        bold_font=ImageFont.truetype(bold_path,font_size*scale) if bold_path else (font if font_path else ImageFont.truetype('/System/Library/Fonts/Menlo.ttc',font_size*scale,index=1))
        symbol_font=ImageFont.truetype('/System/Library/Fonts/Menlo.ttc',font_size*scale)
        default_bg=(28,27,26);default_fg=(236,230,223)
        # Bubble Tea sets canvas colors through OSC 10/11 rather than SGR
        # backgrounds on every cell. pyte ignores these window operations.
        for code in (10, 11):
            matches = re.findall(rb'\x1b\]' + str(code).encode() + rb';#([0-9a-fA-F]{6})(?:\x07|\x1b\\)', self.raw)
            if matches:
                rgb = tuple(int(matches[-1][i:i+2], 16) for i in (0, 2, 4))
                if code == 10: default_fg = rgb
                else: default_bg = rgb
        canvas_bg = self.screen.buffer[0][0].bg
        if len(canvas_bg)==6:
            try: default_bg=tuple(int(canvas_bg[i:i+2],16) for i in (0,2,4))
            except ValueError: pass
        img=Image.new('RGB',(self.width*cw+32*scale,self.height*ch+32*scale),default_bg);draw=ImageDraw.Draw(img)
        def color(value,default):
            if len(value)==6:
                try: return tuple(int(value[i:i+2],16) for i in (0,2,4))
                except ValueError: pass
            return default
        for y in range(self.height):
            for x in range(self.width):
                char=self.screen.buffer[y][x];fg=color(char.fg,default_fg);bg=color(char.bg,default_bg)
                if char.reverse: fg,bg=bg,fg
                left,top=16*scale+x*cw,16*scale+y*ch
                draw.rectangle((left,top,left+cw-1,top+ch-1),fill=bg)
                if draw_box_cell(draw,char.data,left,top,cw,ch,fg,scale):pass
                elif draw_block_cell(draw,char.data,left,top,cw,ch,fg):pass
                elif len(char.data)==1 and 0x2800 <= ord(char.data) <= 0x28ff:
                    # Pillow does not perform macOS font fallback for Braille.
                    # Render the encoded 2x4 dot grid rather than missing glyphs.
                    mask = ord(char.data)-0x2800
                    for bit, (dx,dy) in enumerate(((0,0),(0,1),(0,2),(1,0),(1,1),(1,2),(0,3),(1,3))):
                        if mask & (1<<bit):
                            px,py=left+round(cw*.2)+dx*round(cw*.5),top+round(ch*.14)+dy*round(ch*.23)
                            draw.ellipse((px,py,px+scale,py+scale),fill=fg)
                elif char.data!=' ':
                    # Custom font studies use a terminal-style symbol fallback.
                    # No bold is fabricated when the requested family lacks it.
                    face=bold_font if char.bold else font
                    if font_path and any(0x2190<=ord(c)<=0x25ff for c in char.data):face=symbol_font
                    draw.text((left,top+scale),char.data,font=face,fill=fg)
                if char.underscore:
                    draw.line((left,top+ch-3*scale,left+cw-1,top+ch-3*scale),fill=fg,width=scale)
        path=OUT/(name+'.png');img.save(path);os.chmod(path,0o600)
        (OUT/(name+'.txt')).write_text(self.text)
    def close(self, exit_key='q'):
        self.key(exit_key)
        deadline=time.monotonic()+4
        while self.process.poll() is None and time.monotonic()<deadline:self.pump()
        assert self.process.poll()==0, 'Perch did not exit cleanly'
        self.pump()
        after=termios.tcgetattr(self.slave)
        # Darwin's kernel can set PENDIN while reprocessing pending input. It is
        # not a persistent mode changed by the application. Compare every other
        # flag, speed and control character, including ICANON/ECHO/ISIG.
        pending = getattr(termios, 'PENDIN', 0)
        after[3] &= ~pending
        self.before[3] &= ~pending
        assert after==self.before, f'terminal attributes were not restored: before={self.before!r} after={after!r}'
        assert b'\x1b[?1049l' in self.raw, 'alternate screen was not exited'
        assert b'\x1b[?25h' in self.raw, 'cursor was not restored'
        os.close(self.master);os.close(self.slave)
    def abort(self):
        # Drain the PTY while Bubble Tea restores the terminal. Waiting without
        # reading can deadlock shutdown behind a partially written first frame.
        try:
            if self.process.poll() is None:
                self.process.terminate()
                deadline=time.monotonic()+5
                while self.process.poll() is None and time.monotonic()<deadline:
                    try:self.pump(.1)
                    except OSError:break
                if self.process.poll() is None:
                    self.process.kill()
                self.process.wait(timeout=5)
        finally:
            os.close(self.master);os.close(self.slave)

def main():
    s=Session(['--demo','--reduced-motion'])
    try:
        s.wait('parameter-sweep');s.screenshot('demo-wide')
        s.key('l');s.wait('Dataset ready');s.screenshot('demo-output')
        s.key('e');s.wait('Standard error')
        s.key('\x1b');s.key('s');s.wait('Synthetic demo script');s.screenshot('demo-script')
        s.key('\x1b');s.key('2');s.wait('Recent history')
        s.key('3');s.wait('Allocation is scheduler capacity');s.screenshot('demo-partitions')
        s.key('1');s.key('/software cuda\r');s.wait('cuda/12.8.1');s.screenshot('demo-software')
        s.key('/module cuda/12.8.1\r');s.wait('module load cuda/12.8.1')
        s.key('\x1b');s.key('/find embedding\r');s.wait('embedding-model')
        assert 'parameter-sweep' not in s.text, 'filter did not exclude unrelated jobs'
        s.resize(80,24);s.screenshot('demo-narrow')
        s.key('\r');s.wait('Waiting for priority');s.wait('Scheduler: Priority')
        s.key('\x1b');s.key('?');s.wait('Move around')
        s.key('\x1b');s.key('\x0b');s.wait('Type to find a command')
        s.key('logs');s.wait('Read job output')
        assert 'Prepare a new job' not in s.text, 'palette search left unrelated commands'
        s.screenshot('palette-search-narrow')
        s.key('\r');s.wait('Output appears when this task starts.')
        s.key('\x0b');s.key('zz-no-command');s.wait('No matching commands.')
        s.key('\r');s.wait('No matching commands.')
        s.key('\x1b');s.key('/find embedding\r');s.wait('embedding-model')
        s.key('2');s.wait('Recent history')
        s.key('/find OUT_OF_MEMORY\r');s.wait('9100_7')
        s.key('1');s.wait('embedding-model')
        assert 'parameter-sweep' not in s.text, 'tab switch lost Jobs filter'
        s.key('2');s.wait('9100_7')
        assert '9100_6' not in s.text, 'tab switch lost History filter'
        s.resize(140,40);s.key('\x0b');s.key('new job');s.wait('Prepare a new job')
        s.screenshot('palette-search-wide')
        s.key('\x1b');s.key('\x1b');s.key('c');s.wait('demo · offline')
        s.close()
    except BaseException:
        s.screenshot('failure');s.abort();raise

    result={'demo':'passed','checks':['jobs','output','stderr switch','script','history','partitions','software search','module instructions','filter','80x24 resize','inspection','help','searchable command palette','empty command search','per-tab filter restoration','demo reconnect','terminal attributes restored','alternate screen exited','cursor restored'],'screenshots':'Emulator renders from real PTY output, not native Terminal.app captures.'}
    (OUT/'results.json').write_text(json.dumps(result,indent=2)+'\n')
    print(json.dumps(result,indent=2))

if __name__ == '__main__':
    main()
