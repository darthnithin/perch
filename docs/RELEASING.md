# Preparing a Perch release

Perch is a private Hummingbird development preview. The current product scope
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

## Private preview handoff

Push reviewed source to the private repository. Verify the remote is private and
its commit matches the local checkout. A private GitHub draft release can be
prepared with checked archives and checksums; distinguish a draft from a published
release. Record incomplete native or platform checks in its notes.

## Before public release

Choose the project and artwork licenses explicitly, finish the remaining
acceptance gates, select a version, and review the exact release contents.
Third-party notices do not license Perch itself. Public visibility, public
releases and a Homebrew tap require a separate user instruction.
