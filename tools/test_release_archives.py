#!/usr/bin/env python3
"""Check all four local candidates, linked documentation and unchanged artwork."""
import argparse
import hashlib
import json
from pathlib import Path
import posixpath
import re
import subprocess
import tarfile
import tempfile

from test_release_runtime import architecture


def check_notices(tar, root, target):
    """Compare notice inventory with the actual binary's recorded dependencies."""
    manifest = json.loads(tar.extractfile('third_party/manifest.json').read())
    assert manifest['schema_version'] == 1 and target in manifest['targets']
    components = manifest['components']
    identities = {(c['component'], c['version']) for c in components}
    assert len(identities) == len(components), 'duplicate component'
    listed = {'third_party/manifest.json'}
    for c in components:
        assert c['notices'], 'component has no notice: '+c['component']
        for notice in c['notices']:
            filename = notice['file']
            assert filename.startswith('third_party/') and '..' not in Path(filename).parts
            data = tar.extractfile(filename).read()
            assert hashlib.sha256(data).hexdigest() == notice['sha256'], filename
            listed.add(filename)
    archived = {m.name for m in tar.getmembers() if m.isfile() and m.name.startswith('third_party/')}
    assert archived == listed, 'extra or missing notice payload'
    for filename in listed | {'THIRD_PARTY_NOTICES.md'}:
        assert tar.extractfile(filename).read() == (root/filename).read_bytes(), filename
    with tempfile.TemporaryDirectory(prefix='perch-buildinfo-') as directory:
        binary = Path(directory)/'perch'
        binary.write_bytes(tar.extractfile('perch').read())
        # go version reads build metadata for all architectures; no execution.
        info = subprocess.check_output(['go', 'version', '-m', str(binary)], text=True)
        system, machine = target.split('/')
        expected = {'format': 'Mach-O' if system == 'darwin' else 'ELF', 'architecture': machine}
        assert architecture(binary) == expected, 'binary target mismatch: '+target
        settings = dict(line.strip().split('=', 1) for line in info.splitlines()
                        if line.strip().startswith(('build\tGOOS=', 'build\tGOARCH=')))
        assert settings.get('build\tGOOS') == system and settings.get('build\tGOARCH') == machine, 'binary build target mismatch: '+target
    binary_deps = set()
    for line in info.splitlines():
        fields = line.split()
        if fields and fields[0] == 'dep':
            binary_deps.add((fields[1], fields[2]))
        assert not (fields and fields[0] == '=>'), 'replaced binary dependency requires review'
    supplied = {(c['component'], c['version']) for c in components
                if target in c['targets'] and c['component'] not in ('Unicode data', 'Go runtime and standard library')}
    assert binary_deps and binary_deps == supplied, ('binary/notice dependency mismatch', binary_deps ^ supplied)
    toolchain = info.splitlines()[0].split()[-1]
    assert ('Go runtime and standard library', toolchain) in identities, 'wrong Go runtime notice'
    return len(binary_deps)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('version')
    parser.add_argument('--extract-to', type=Path)
    args = parser.parse_args()
    assert re.fullmatch(r'[A-Za-z0-9._-]+', args.version), 'invalid version'
    root = Path(__file__).resolve().parents[1]
    prefix = 'perch-'+args.version+'-'
    sums = root/'dist'/('SHA256SUMS-'+args.version+'.txt')
    checks = {line.split()[1]: line.split()[0] for line in sums.read_text().splitlines()}
    expected = {prefix+target+'.tar.gz' for target in ['darwin-arm64','darwin-amd64','linux-arm64','linux-amd64']}
    assert set(checks) == expected, 'checksum manifest has wrong targets or nonportable paths'
    reports = []
    for filename in sorted(expected):
        archive = root/'dist'/filename
        assert hashlib.sha256(archive.read_bytes()).hexdigest() == checks[filename], filename
        with tarfile.open(archive) as tar:
            members = tar.getmembers()
            names = {m.name for m in members}
            assert len(names) == len(members), 'duplicate archive entry'
            for m in members:
                assert not Path(m.name).is_absolute() and '..' not in Path(m.name).parts, m.name
                assert m.isfile() or m.isdir(), 'unexpected link/device: '+m.name
                assert not any(part.startswith('._') or part == '.DS_Store' for part in Path(m.name).parts), m.name
                assert not m.pax_headers, 'nonportable extended metadata'
            for name in ['perch','README.md','ARCHITECTURE.md','LICENSE','assets/bird/LICENSE','docs/MCP.md',
                         'examples/mcp-config.json','assets/bird/manifest.json',
                         'THIRD_PARTY_NOTICES.md','third_party/manifest.json']:
                assert name in names, name
            assert tar.getmember('perch').mode & 0o111, 'binary is not executable'
            for m in members:
                if m.isfile() and (m.name in ('README.md', 'ARCHITECTURE.md', 'LICENSE') or
                                   m.name.startswith(('docs/', 'examples/'))):
                    assert tar.extractfile(m).read() == (root/m.name).read_bytes(), 'stale packaged documentation: '+m.name
                if m.isfile() and m.name.endswith('.md'):
                    content = tar.extractfile(m).read().decode('utf-8')
                    for link in re.findall(r'\]\(([^)]+)\)', content):
                        if '://' not in link and not link.startswith('#'):
                            target = posixpath.normpath(str(Path(m.name).parent/link.split('#')[0]))
                            assert target in names, (m.name, link)
                if m.isfile() and m.name.startswith('assets/bird/'):
                    assert tar.extractfile(m).read() == (root/m.name).read_bytes(), m.name
            binary_hash = hashlib.sha256(tar.extractfile('perch').read()).hexdigest()
            target_name = filename[len(prefix):-len('.tar.gz')]
            dependency_count = check_notices(tar, root, target_name.replace('-', '/'))
            if args.extract_to:
                target = args.extract_to/filename[len(prefix):-len('.tar.gz')]
                target.mkdir(parents=True, exist_ok=True)
                tar.extractall(target)
        reports.append({'archive':filename,'sha256':checks[filename],'binary_sha256':binary_hash,
                        'linked_docs':'passed','docs_match_source':'passed','artwork_matches_source':'passed',
                        'dependency_notices':'passed','binary_dependency_count':dependency_count,
                        'binary_target':'passed',
                        'members':len(members)})
    print(json.dumps(reports, indent=2))


if __name__ == '__main__':
    main()
