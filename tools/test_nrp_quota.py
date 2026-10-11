#!/usr/bin/env python3
"""Quota navigation/rendering in production TUI with offline synthetic data."""
from test_terminal import Session
from test_mouse import wheel


def main():
    for width,height in [(140,40),(80,24),(48,18)]:
        s=Session(['nrp','ui','--demo'],width,height)
        try:
            s.wait('Jobs current')
            s.key('/find retrying\r')
            s.wait('1–1 of 1')
            s.key('/quotas\r')
            s.wait('Used / limit')
            s.wait('compute')
            s.wait('at limit')
            s.screenshot(f'nrp-quota-{width}')
            # Scroll through long resource names, scoped limits and unknown usage.
            seen=s.text
            for _ in range(22):
                wheel(s,10,6)
                seen+='\n'+s.text
            for text in ['NVIDIA GPU requests','at limit','PriorityClass In','interactive','storage-update','unknown','Configured limit: 2Ti']:
                assert text in seen,(width,text,seen)
            s.key('\x1b[F')
            s.wait('logs.')
            s.screenshot(f'nrp-quota-bottom-{width}')
            s.resize(48,18)
            s.key('\x1b[H')
            s.wait('compute')
            assert 'q quit' in s.text
            s.key('\x1b')
            s.wait('Filter: retrying')
            s.wait('1–1 of 1')
            s.close()
        except BaseException:
            print(s.text)
            s.screenshot('nrp-quota-failure')
            s.abort()
            raise
    print('NRP quota PTY passed: three sizes, scoped/exhausted/pending/missing values, scrolling, resize, filtered-list return, terminal cleanup.')


if __name__=='__main__':
    main()
