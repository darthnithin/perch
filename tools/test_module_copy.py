#!/usr/bin/env python3
"""Module-copy navigation with a private fake clipboard; no module execution."""
import pathlib
import sys
import tempfile
from test_terminal import Session
from test_footer import activate
from test_array_copy import CLIPBOARD,await_calls


def main():
    with tempfile.TemporaryDirectory(prefix='perch-module-copy-') as tmp:
        root=pathlib.Path(tmp);mode=root/'mode';mode.write_text('ok')
        calls=root/'calls';copied=root/'copied'
        for name in ['pbcopy','wl-copy','xclip']:
            p=root/name;p.write_text('#!'+sys.executable+'\n'+CLIPBOARD);p.chmod(0o700)
        env={'PATH':str(root)+':/usr/bin:/bin','PERCH_TEST_CLIPBOARD':str(root),'WAYLAND_DISPLAY':'perch-test','DISPLAY':':perch-test'}
        count=0
        for width,height in [(140,40),(80,24),(48,18)]:
            session=Session(['--demo','--no-config','--reduced-motion'],width,height,env=env)
            try:
                session.wait('parameter-sweep');session.key('/software cuda\r');session.wait('cuda/12.8.1')
                session.key('\r');session.wait('module load cuda/12.8.1')
                activate(session,'y copy load');session.wait('Copied: module load cuda/12.8.1')
                count+=1;await_calls(session,calls,count)
                assert copied.read_text()=='module load cuda/12.8.1'
                session.screenshot(f'module-copy-{width}')
                # The global array command must retain its meaning in module view.
                session.key('/failed\r');session.wait('Choose a job in Jobs or History first.')
                assert len(calls.read_text().splitlines())==count
                if width==80:
                    mode.write_text('fail');session.key('y');session.wait('Clipboard unavailable')
                    count+=1;await_calls(session,calls,count)
                    mode.write_text('timeout');session.key('y');session.wait('Copying module command')
                    count+=1;await_calls(session,calls,count)
                    session.key('yyy');session.wait('Clipboard copy in progress')
                    session.wait('Clipboard unavailable',timeout=6)
                    assert len(calls.read_text().splitlines())==count
                    mode.write_text('ok');session.key('y');session.wait('Copied: module load cuda/12.8.1')
                    count+=1;await_calls(session,calls,count)
                session.key('\x1b');session.wait('login environment')
                session.key('/module absent/1\r');session.wait('Unable to read this view')
                session.key('y');session.wait('Read module details before copying')
                assert len(calls.read_text().splitlines())==count
                session.close()
            except BaseException:
                print(session.text);session.screenshot('module-copy-failure');session.abort();raise
    print('Module copy PTY passed: exact command, footer click, three sizes, copy failure/timeout/recovery, duplicate suppression, array-command scope, failed-read guard, catalog return and cleanup. Private fake clipboard only.')


if __name__=='__main__':main()
