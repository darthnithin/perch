#!/usr/bin/env python3
"""Regression checks for terminal operations missing from stock pyte."""
import unittest
import os
import sys
from test_terminal import Screen, Stream, Session

class EmulatorTests(unittest.TestCase):
    def test_scroll_within_region_preserves_outer_rows_and_cursor(self):
        screen = Screen(12, 6)
        stream = Stream(screen)
        for row, letter in enumerate('ABCDEF', 1):
            stream.feed(f'\x1b[{row};1H{letter}')
        stream.feed('\x1b[2;5r\x1b[3;4H\x1b[2T')
        self.assertEqual([r[0] for r in screen.display], ['A',' ',' ','B','C','F'])
        self.assertEqual((screen.cursor.x,screen.cursor.y), (3,2))
        stream.feed('\x1b[2S')
        self.assertEqual([r[0] for r in screen.display], ['A','B','C',' ',' ','F'])
        self.assertEqual((screen.cursor.x,screen.cursor.y), (3,2))

    def test_keyboard_mode_controls_do_not_create_underlines(self):
        screen = Screen(40, 4)
        stream = Stream(screen)
        for chunk in ('\x1b[>', '4;', '2', 'm', '\x1b[>4', 'm', 'normal', '\x1b[4m', 'underlined', '\x1b[24m', 'plain'):
            stream.feed(chunk)
        self.assertEqual(screen.display[0].strip(), 'normalunderlinedplain')
        self.assertFalse(screen.buffer[0][0].underscore)
        self.assertTrue(screen.buffer[0][6].underscore)
        self.assertFalse(screen.buffer[0][16].underscore)

    def test_backward_tab(self):
        screen = Screen(40, 4)
        stream = Stream(screen)
        stream.feed('\x1b[1;25H\x1b[2Z')
        self.assertEqual(screen.cursor.x,8)
        stream.feed('\x1b[9Z')
        self.assertEqual(screen.cursor.x,0)

class SessionCleanupTests(unittest.TestCase):
    def test_abort_drains_a_childs_final_output_and_reaps_it(self):
        program = '''import signal, sys, time
def stop(*args):
    sys.stdout.write("x" * (128 * 1024))
    sys.stdout.flush()
    sys.exit(0)
signal.signal(signal.SIGTERM, stop)
print("READY", flush=True)
while True: time.sleep(1)
'''
        s = Session(['-c', program], 48, 18, binary=sys.executable)
        try:
            s.wait('READY')
            s.abort()
            self.assertEqual(s.process.returncode, 0, 'graceful exit was blocked behind output')
            with self.assertRaises(OSError): os.fstat(s.master)
            with self.assertRaises(OSError): os.fstat(s.slave)
        finally:
            if s.process.poll() is None:
                s.process.kill()
                s.process.wait(timeout=5)

if __name__ == '__main__':
    unittest.main()
