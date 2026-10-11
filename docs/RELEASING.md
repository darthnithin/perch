# Preparing a Perch release

Perch is a Hummingbird terminal workspace in preview. The current product scope
excludes NRP: its existing source, fixtures and tests are preserved as deferred
experiments. Nothing in this checklist authorizes publishing the repository.

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
`ubuntu-24.04-arm` runner. It downloads an existing private release archive;
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

Push reviewed source to the private repository. Verify the remote is private and
its commit matches the local checkout. A private GitHub draft release can be
prepared with checked archives and checksums; distinguish a draft from a published
release. Record incomplete native or platform checks in its notes.

## Before public release

The code and documentation use the root MIT license; Chip artwork uses the
reserved terms in `assets/bird/LICENSE`. Include both with every archive and
Homebrew installation, along with third-party notices. Finish the applicable
acceptance checks, select a version, and review the exact release contents.
Public visibility, public releases and a Homebrew tap require a separate user
instruction. Rebuild the candidate to include the selected licensing notices
and anonymized current examples when those inputs change; preview 3 already
includes both. The user chose to keep the existing repository
and Git history; do not rename it or rewrite historical username references.

### Try the private draft without building

Repository collaborators with push access can download the existing draft through
an authenticated GitHub CLI. For Apple Silicon, run this from an empty directory:

```sh
gh release download v0.1.0-preview.3 --repo darthnithin/perch \
  --pattern 'perch-v0.1.0-preview.3-darwin-arm64.tar.gz' \
  --pattern 'SHA256SUMS-v0.1.0-preview.3.txt'
awk '$2 == "perch-v0.1.0-preview.3-darwin-arm64.tar.gz" {print}' \
  SHA256SUMS-v0.1.0-preview.3.txt | shasum -a 256 -c -
# Continue only after the checksum check reports OK.
tar -xzf perch-v0.1.0-preview.3-darwin-arm64.tar.gz
./perch --demo --no-config
```

Use the matching archive suffix from the README for other platforms. This path
was checked using the uploaded Mac ARM archive, extracted outside the source
checkout; its bytes matched the independently verified candidate. It requires no
Go or Python. It does not exercise browser-download quarantine or Gatekeeper.
The release remains a draft, so these are collaborator instructions rather than
public installation instructions.

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

After distribution is authorized, publish accessible release assets and place
the generated file at `Formula/perch.rb` in `darthnithin/homebrew-tap`. The
intended command is `brew install darthnithin/tap/perch`; that tap is not yet
published. Draft release URLs are not working public downloads. Validate the
full installation and upgrade lifecycle against the published assets before
advertising the command. See the [Homebrew tap guide](https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap).
