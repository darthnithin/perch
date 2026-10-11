# Install Perch

Perch is one executable for macOS or Linux. No Go or Python runtime is needed.
Use your own computer and connect to Hummingbird over OpenSSH; nothing needs to
be installed on the cluster. The current release is **v0.1.0-preview.3**.

## Homebrew

With [Homebrew](https://brew.sh) already installed, on macOS or Linux:

```sh
brew install darthnithin/tap/perch
perch --demo --no-config
perch setup
```

Homebrew selects the prebuilt archive for your operating system and CPU, verifies
its checksum, and installs the executable and documentation. This is the
[Perch tap](https://github.com/darthnithin/homebrew-tap), not Homebrew core.
The font and terminal profile are optional and are not installed automatically.

```sh
# Update
brew update
brew upgrade darthnithin/tap/perch

# Remove the executable and bundled documentation
brew uninstall darthnithin/tap/perch
```

Uninstalling leaves saved Perch preferences alone. `perch config path` shows their
location while Perch is installed.

## Linux without Homebrew

Requires `curl`, `tar`, `sha256sum` and standard Linux file utilities. This installs
under your home directory, with no sudo. It supports x86-64 and ARM64; other CPUs
are rejected before downloading. Run the whole block in a shell:

```sh
(
  set -eu
  case "$(uname -m)" in
    x86_64) perch_arch=amd64 ;;
    aarch64|arm64) perch_arch=arm64 ;;
    *) echo "Perch supports Linux x86-64 and ARM64." >&2; exit 1 ;;
  esac
  perch_prefix=${PERCH_INSTALL_PREFIX:-"$HOME/.local"}
  perch_version=v0.1.0-preview.3
  perch_archive="perch-${perch_version}-linux-${perch_arch}.tar.gz"
  perch_url="https://github.com/darthnithin/perch/releases/download/${perch_version}"
  perch_tmp=$(mktemp -d)
  trap 'rm -rf "$perch_tmp"' EXIT
  cd "$perch_tmp"
  curl -fL --retry 3 "$perch_url/$perch_archive" -o "$perch_archive"
  curl -fL --retry 3 "$perch_url/SHA256SUMS-${perch_version}.txt" -o SHA256SUMS
  awk -v name="$perch_archive" '$2 == name {print; found++} END {if (found != 1) exit 1}' SHA256SUMS > selected.sha256
  sha256sum -c selected.sha256
  tar -xzf "$perch_archive"
  mkdir -p "$perch_prefix/bin" "$perch_prefix/share/perch"
  install -m 755 perch "$perch_prefix/bin/perch"
  cp -R README.md ARCHITECTURE.md LICENSE THIRD_PARTY_NOTICES.md docs examples assets third_party "$perch_prefix/share/perch/"
)
```

For a custom installation directory, set `PERCH_INSTALL_PREFIX` before running
the block. By default, add the executable directory to your current shell, then
try the offline demo:

```sh
export PATH="$HOME/.local/bin:$PATH"
perch --demo --no-config
perch setup
```

If that directory is not already on your PATH, add the `export` line to your shell
startup file (for example `~/.bashrc` for interactive Bash). Documentation and
license notices are retained in `~/.local/share/perch`. Updating uses the same
steps with the desired release version. To uninstall this user-local installation:

```sh
rm "$HOME/.local/bin/perch"
rm -r "$HOME/.local/share/perch"
```

Saved preferences are separate and are not removed by these commands.

## macOS without Homebrew

Download the matching archive and checksum file from
[the release page](https://github.com/darthnithin/perch/releases/tag/v0.1.0-preview.3):

| Computer | Archive |
| --- | --- |
| Apple Silicon | `perch-v0.1.0-preview.3-darwin-arm64.tar.gz` |
| Intel | `perch-v0.1.0-preview.3-darwin-amd64.tar.gz` |

In Terminal, change into the download directory. For Apple Silicon:

```sh
awk '$2 == "perch-v0.1.0-preview.3-darwin-arm64.tar.gz" {print}' \
  SHA256SUMS-v0.1.0-preview.3.txt | shasum -a 256 -c -
# Continue only after the checksum check reports OK.
mkdir perch-preview
tar -xzf perch-v0.1.0-preview.3-darwin-arm64.tar.gz -C perch-preview
cd perch-preview
./perch --demo --no-config
./perch setup
```

For Intel, substitute `amd64` for `arm64`. Keep the extracted directory for its
licenses, documentation and optional terminal profile. These binaries are not
Developer ID signed or notarized. If macOS blocks a browser-downloaded executable,
review it through System Settings → Privacy & Security; do not disable Gatekeeper.
Homebrew is the simpler installation route.

## Connect and customize

`perch setup` asks for `your-cruzid@hb.ucsc.edu` or an existing SSH alias. Account
and network access must already work. Press `c` inside the workspace when sign-in
is needed. Passwords are handled by SSH.

For the approved appearance, use Ioskeley Mono. The macOS reference is 12 pt,
0.90 line spacing; other terminals control spacing differently. See
[font and terminal setup](terminal/README.md). With Homebrew on macOS, the profile is at:

```sh
open "$(brew --prefix perch)/share/doc/perch/docs/terminal/Perch-Ioskeley.terminal"
```

[MCP setup](MCP.md) uses the same installed executable. Find its full path with
`command -v perch`. [The user guide](GUIDE.md) covers jobs, arrays, scripts,
resources, submission and allocation shells.
