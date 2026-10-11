#!/bin/sh
# Build local release candidates. This script never publishes or installs.
set -eu
version=${1:-dev}
case "$version" in *[!A-Za-z0-9._-]*|'') printf 'Invalid version\n' >&2; exit 2;; esac
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"
python3 tools/notices.py --check
mkdir -p dist
stage=''
trap '[ -z "$stage" ] || rm -rf -- "$stage"' EXIT HUP INT TERM
set --
for target in darwin/arm64 darwin/amd64 linux/arm64 linux/amd64; do
  system=${target%/*}
  architecture=${target#*/}
  name="perch-${version}-${system}-${architecture}.tar.gz"
  stage=$(mktemp -d "dist/.perch-stage.XXXXXX")
  mkdir -p "$stage/package/assets"
  CGO_ENABLED=0 GOOS="$system" GOARCH="$architecture" go build -trimpath -ldflags "-s -w -X main.version=$version" -o "$stage/package/perch" ./cmd/perch
  cp README.md ARCHITECTURE.md THIRD_PARTY_NOTICES.md "$stage/package/"
  cp -R docs examples third_party "$stage/package/"
  cp -R assets/bird "$stage/package/assets/"
  COPYFILE_DISABLE=1 tar --format=ustar -czf "$stage/archive.tar.gz" -C "$stage/package" perch README.md ARCHITECTURE.md THIRD_PARTY_NOTICES.md third_party docs examples assets
  mv -f "$stage/archive.tar.gz" "dist/$name"
  rm -rf -- "$stage"
  stage=''
  set -- "$@" "$name"
done
# Basenames make the checksum file usable after downloading/extracting dist.
(
  cd dist
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$@" > "SHA256SUMS-$version.txt"
  else
    sha256sum "$@" > "SHA256SUMS-$version.txt"
  fi
)
printf 'Local candidates built with linked documentation and original artwork. Runtime verification and license review are still required before publication.\n'
