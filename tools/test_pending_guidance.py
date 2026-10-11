#!/usr/bin/env python3
"""Pending reasons through the production adapter and actual TUI processes."""
import pathlib
import sys
import tempfile

from test_failure_terminal import FAKE_SSH
from test_terminal import ROOT, Session


QUEUE = '''
    if name == "queue":
        import json
        data = json.loads((fixtures / "queue.json").read_text())
        job = data["jobs"][-1]
        job.update(job_id=9300, name="queued-study", job_state=["PENDING"],
                   state_reason=state, nodes="", array_task_string="")
        for field in ["array_job_id", "array_task_id", "start_time", "end_time"]:
            job[field] = {"set": False, "infinite": False, "number": 0}
        data["jobs"] = [job]
        return json.dumps(data), "", 0
'''


def main():
    with tempfile.TemporaryDirectory(prefix='perch-pending-guidance-') as directory:
        folder = pathlib.Path(directory)
        state = folder/'reason'
        ssh = folder/'ssh'
        needle = '    return (fixtures / (name+".json")).read_text(), "", 0'
        assert needle in FAKE_SSH
        ssh.write_text('#!'+sys.executable+'\n'+FAKE_SSH.replace(needle, QUEUE+needle))
        ssh.chmod(0o700)
        env = {'PATH':str(folder)+':/usr/bin:/bin', 'PERCH_TEST_STATE':str(state),
               'PERCH_TEST_FIXTURES':str(ROOT/'testdata/hummingbird')}
        for width, height in [(140,40),(80,24),(48,18)]:
            state.write_text('JobHeldUser')
            s = Session(['--host','perch-test.invalid','--no-config'], width, height, env=env)
            try:
                s.wait('queued-study'); s.key('/find queued\r'); s.wait('› 9300')
                if width == 140: s.wait('Held by user or coordinator')
                s.key('\r'); s.wait('Held by user or coordinator')
                if width == 48:
                    s.wait('Scheduler: JobHeldUser')
                    assert 'Find a command' not in s.text
                    s.key('/'); s.pump(.2)
                    assert s.text.count('╰') == 2, 'slash did not restore the command box'
                    s.key('\x1b'); s.wait('Scheduler: JobHeldUser')
                s.screenshot(f'pending-held-{width}')
                # Refresh updates the explanation without losing inspector scope.
                for reason, title in [('JobHeldAdmin','Held by administrator'),
                                      ('ReqNodeNotAvail','Required node is unavailable'),
                                      ('SiteSpecificReason','Scheduler: SiteSpecificReason')]:
                    state.write_text(reason); s.pump(2.2); s.key('r'); s.wait(title)
                    if reason == 'ReqNodeNotAvail':
                        s.screenshot(f'pending-node-{width}')
                    assert 'Tentative start:' not in s.text
                s.key('\x1b[F'); s.wait('resources')
                s.resize(48,18); s.key('\x1b[H'); s.wait('Scheduler: SiteSpecificReason')
                s.key('\x1b'); s.wait('› 9300'); s.wait('Filter: queued')
                s.resize(140,40); s.wait('Queued · SiteSpecificReason')
                s.screenshot(f'pending-unknown-return-{width}')
                s.close()
            except BaseException:
                print(s.text); s.screenshot('pending-guidance-failure'); s.abort(); raise
    print('Pending guidance PTY passed: user/coordinator versus administrator holds, unavailable node, unknown reason, refresh, no invented start estimate, three sizes, scroll/resize, retained filter/selection and terminal cleanup. Synthetic SSH only.')


if __name__ == '__main__':
    main()
