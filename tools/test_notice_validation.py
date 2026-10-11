#!/usr/bin/env python3
"""Prove that archive notice validation rejects tampering and stale versions."""
import argparse
import copy
import io
import json
from pathlib import Path
import tarfile
import tempfile

from test_release_archives import check_notices


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('archive', type=Path)
    args = parser.parse_args()
    with tarfile.open(args.archive) as source:
        original = {m.name: source.extractfile(m).read() for m in source.getmembers()
                    if m.isfile() and (m.name in ('perch', 'THIRD_PARTY_NOTICES.md') or m.name.startswith('third_party/'))}
    manifest = json.loads(original['third_party/manifest.json'])
    component = next(c for c in manifest['components'] if c['component'].startswith('charm.land/'))
    target = next(t for t in manifest['targets']
                  if args.archive.name.endswith(t.replace('/', '-')+'.tar.gz'))

    def verify(files, requested_target=target):
        # The source mirror matches the modified archive. Only independent
        # notice hashes and binary build metadata can detect these mutations.
        with tempfile.TemporaryDirectory(prefix='perch-notice-test-') as directory:
            root = Path(directory)
            buffer = io.BytesIO()
            with tarfile.open(fileobj=buffer, mode='w', format=tarfile.USTAR_FORMAT) as tar:
                for name, data in files.items():
                    entry = tarfile.TarInfo(name)
                    entry.size = len(data)
                    tar.addfile(entry, io.BytesIO(data))
                    if name != 'perch':
                        path = root/name
                        path.parent.mkdir(parents=True, exist_ok=True)
                        path.write_bytes(data)
            buffer.seek(0)
            with tarfile.open(fileobj=buffer) as tar:
                return check_notices(tar, root, requested_target)

    assert verify(original) > 0
    for other_target in manifest['targets']:
        if other_target == target:
            continue
        try:
            verify(original, other_target)
        except AssertionError as error:
            assert 'binary target mismatch' in str(error), error
        else:
            raise AssertionError('wrong platform/architecture passed: '+other_target)
    corrupt = dict(original)
    notice = component['notices'][0]['file']
    corrupt[notice] += b'\nALTERED\n'
    try:
        verify(corrupt)
    except AssertionError as error:
        assert str(error) == notice, error
    else:
        raise AssertionError('altered upstream text passed')

    stale = dict(original)
    changed = copy.deepcopy(manifest)
    next(c for c in changed['components'] if c['component'] == component['component'])['version'] += '-stale'
    stale['third_party/manifest.json'] = json.dumps(changed).encode()
    try:
        verify(stale)
    except AssertionError as error:
        assert 'binary/notice dependency mismatch' in str(error), error
    else:
        raise AssertionError('stale module version passed')
    print('Archive validation: original passed; wrong OS/architecture, changed license bytes and stale dependency version rejected against independent evidence.')


if __name__ == '__main__':
    main()
