#!/usr/bin/env bash
# Checksum-verifying installer for TerraDrift GitHub Release archives.
# Usage: TERRADRIFT_VERSION=v1.1.1 PREFIX=/usr/local ./scripts/install.sh
set -euo pipefail

VERSION="${TERRADRIFT_VERSION:-v1.1.1}"
PREFIX="${PREFIX:-/usr/local}"
REPO="${TERRADRIFT_REPO:-niravraychura/terradrift}"
BASE="https://github.com/${REPO}/releases/download/${VERSION}"

uname_s="$(uname -s | tr '[:upper:]' '[:lower:]')"
uname_m="$(uname -m)"
case "${uname_s}" in
  linux) os=linux ;;
  darwin) os=darwin ;;
  *)
    echo "unsupported OS: ${uname_s}" >&2
    exit 1
    ;;
esac
case "${uname_m}" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *)
    echo "unsupported architecture: ${uname_m}" >&2
    exit 1
    ;;
esac

archive="terradrift_${os}_${arch}.tar.gz"
tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

curl -fsSL -o "${tmpdir}/checksums.txt" "${BASE}/checksums.txt"
curl -fsSL -o "${tmpdir}/${archive}" "${BASE}/${archive}"

(
  cd "${tmpdir}"
  # Older releases list `dist/terradrift_*.tar.gz`; rewrite to the downloaded basename.
  hash="$(awk -v a="${archive}" '$2 == a || $2 == "dist/" a { print $1; found=1; exit } END { if (!found) exit 1 }' checksums.txt)" || {
    echo "no checksum line for ${archive}" >&2
    exit 1
  }
  if command -v sha256sum >/dev/null 2>&1; then
    printf '%s  %s\n' "${hash}" "${archive}" | sha256sum -c -
  else
    printf '%s  %s\n' "${hash}" "${archive}" | shasum -a 256 -c -
  fi
)

tar -xzf "${tmpdir}/${archive}" -C "${tmpdir}" terradrift
mkdir -p "${PREFIX}/bin"
install -m 0755 "${tmpdir}/terradrift" "${PREFIX}/bin/terradrift"
echo "installed ${PREFIX}/bin/terradrift (${VERSION} ${os}/${arch})"
