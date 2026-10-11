# Preparing a Perch release

Perch is a Hummingbird terminal workspace in preview. The current product scope
excludes NRP: its existing source, fixtures and tests are preserved as deferred
experiments. The user authorized public repository, release and Homebrew
distribution on October 11, 2026. The published preview is v0.1.0-preview.3.

## Validate a candidate

1. From a clean checkout, install Go at the version in `go.mod` and run
   `make check` and `make notices-check`.
2. Create `.venv`, install `tools/requirements.txt`, then run
   `make terminal-test chip-test mcp-test`. These are offline tests. Live test
   scripts require their own explicit opt-in and an owned workload where relevant.
3. Review the UI in the reference profile and check the native acceptance items
   in [readiness](READINESS.md). A PTY screenshot cannot prove native font,
   password/MFA or desktop-notification behavior.
4. Run `VERSION=preview-YYYYMMDD make release-check`. This builds and checks
   macOS/Linux archives for ARM64 and x86-64, documentation, artwork, third-party
   notices and checksums. It neither publishes nor installs them.
5. Test the exact packaged executable on each advertised platform. Record its
   SHA-256 and runtime environment. Hummingbird covers Linux x86-64; it does not
   validate ARM64. Keep untested targets clearly labelled. Starting OrbStack
   requires the user's approval and is not necessary to use Perch on Hummingbird.
6. Preserve original Chip files and their artwork manifest. Include the guide,
   terminal profile and notices in every package. Source-only test evidence under
   `output/` stays local and is not included in Git or distribution archives.

## Native Linux ARM verification

The manual **Packaged Linux ARM runtime** workflow uses GitHub's
`ubuntu-24.04-arm` runner. It downloads an existing release archive;
it does not rebuild the application or start a local VM. Dispatch
`.github/workflows/release-runtime.yml` with the release tag, archive SHA-256 and
packaged executable SHA-256 from your independent local archive checks. GitHub
requires push access to see draft releases, so this manually dispatched job has
`contents: write`; it uses the token only for downloading and never publishes
or changes a release. Checkout does not persist credentials.

The workflow verifies those hashes before execution, asserts a native Linux ARM
host, and runs `tools/test_release_runtime.py` against the extracted executable.
Its `linux-arm-runtime` artifact records CLI, MCP, PTY and cleanup results, the
host architecture and executable hash. Preserve that report with the release
evidence before its 30-day artifact retention expires. A green run covers those
checks, not native desktop fonts or authentication to Hummingbird.

## Preview preparation

Push reviewed source and verify the remote commit. Prepare future versions as
drafts with checked archives and checksums, then publish the reviewed version.
Record incomplete native or platform checks in its notes. Preserve the immutable
preview-3 archives and their tested source identity.

## Before public release

The code and documentation use the root MIT license; Chip artwork uses the
reserved terms in `assets/bird/LICENSE`. Include both with every archive and
Homebrew installation, along with third-party notices. Finish the applicable
acceptance checks, select a version, and review the exact release contents.
Public distribution is authorized for this preview. Rebuild the candidate to include the selected licensing notices
and anonymized current examples when those inputs change; preview 3 already
includes both. The user chose to keep the existing repository
and Git history; do not rename it or rewrite historical username references.

## Public installation

See [installation](INSTALL.md) for Homebrew, Linux user-local commands and macOS
archives. The public preview uses the exact verified preview-3 archives; their
bundled documentation records the earlier private preparation checkpoint. Current
installation instructions live in the repository. Do not replace released archives
just to refresh those historical notes.

## Prepare a Homebrew tap formula

An independently maintained tap can install the prebuilt archives without Go.
Generate its formula from a checked release directory:

```sh
python3 tools/homebrew_formula.py v0.1.0-preview.3 --archives dist
make homebrew-test
HOMEBREW_NO_AUTO_UPDATE=1 brew ruby tools/test_homebrew_formula.rb \
  output/homebrew/perch.rb dist v0.1.0-preview.3
```

The generator verifies all four archive checksums before writing the formula.
The Ruby check uses Homebrew's platform selection and install helpers with a
temporary prefix, verifies the bundled files, and smoke-tests the Mac binaries
on a Mac (Intel via Rosetta on Apple Silicon). It does not exercise the complete
`brew install`, linking, upgrade, uninstall, or `brew test` lifecycle, and it
does not install a tap or modify terminal preferences.

The public tap is [darthnithin/homebrew-tap](https://github.com/darthnithin/homebrew-tap),
with the generated file at `Formula/perch.rb`. Users install through
`brew install darthnithin/tap/perch`. For future updates, publish the verified
release assets before updating formula URLs and checksums. Validate installation
and smoke tests against the public assets, then record the exact version and
runtime scope. See the [Homebrew tap guide](https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap).
