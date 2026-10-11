#!/usr/bin/env python3
"""Offline checks for release selection and failed formula generation."""
import hashlib
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

from homebrew_formula import generate, TARGETS


class FormulaGeneration(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="perch-formula-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.version = "v0.1.0-preview.2"
        self.records = []
        for target in TARGETS:
            name = f"perch-{self.version}-{target}.tar.gz"
            payload = target.encode()
            (self.root / name).write_bytes(payload)
            self.records.append(f"{hashlib.sha256(payload).hexdigest()}  {name}")
        self.manifest = self.root / f"SHA256SUMS-{self.version}.txt"
        self.write_records(self.records)

    def write_records(self, records):
        self.manifest.write_text("\n".join(records) + "\n")

    def test_four_verified_release_urls(self):
        formula = generate(self.version, self.root, "example/perch")
        for record in self.records:
            checksum, name = record.split()
            self.assertIn(f'https://github.com/example/perch/releases/download/{self.version}/{name}', formula)
            self.assertIn(f'sha256 "{checksum}"', formula)
        self.assertEqual(formula.count('      url '), 4)

    def test_reject_invalid_identifiers(self):
        for version in ("../bad", 'v1.2.3"', "v1.2.3\n", "dev"):
            with self.subTest(version=version), self.assertRaises(ValueError):
                generate(version, self.root)
        for repository in ("https://github.com/example/perch", 'example/perch"', "../perch", "example/perch/other"):
            with self.subTest(repository=repository), self.assertRaises(ValueError):
                generate(self.version, self.root, repository)

    def test_reject_missing_duplicate_and_unknown_targets(self):
        for records in (self.records[1:], self.records + self.records[:1],
                        [self.records[0].replace("darwin-arm64", "windows-arm64"), *self.records[1:]]):
            with self.subTest(records=records), self.assertRaises(ValueError):
                self.write_records(records)
                generate(self.version, self.root)

    def test_reject_bad_checksum_records(self):
        for record in ("not-a-checksum file", self.records[0] + " extra", ""):
            with self.subTest(record=record), self.assertRaises(ValueError):
                self.write_records([record, *self.records[1:]])
                generate(self.version, self.root)

    def test_reject_missing_and_changed_archive(self):
        archive = self.root / self.records[0].split()[1]
        archive.write_bytes(b"changed")
        with self.assertRaisesRegex(ValueError, "checksum mismatch"):
            generate(self.version, self.root)
        archive.unlink()
        with self.assertRaises(FileNotFoundError):
            generate(self.version, self.root)

    def test_cli_preserves_previous_output_on_failure(self):
        output = self.root / "perch.rb"
        output.write_text("previous verified formula\n")
        self.write_records(self.records[1:])
        result = subprocess.run(
            [sys.executable, str(Path(__file__).with_name("homebrew_formula.py")),
             self.version, "--archives", str(self.root), "--output", str(output)],
            capture_output=True, text=True, check=False)
        self.assertEqual(result.returncode, 1)
        self.assertIn("all four target checksums", result.stderr)
        self.assertEqual(output.read_text(), "previous verified formula\n")


if __name__ == "__main__":
    unittest.main()
