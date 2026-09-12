#!/usr/bin/env bash
# Checksum-verifying installer for TerraDrift GitHub Release archives.
# Usage: TERRADRIFT_VERSION=v1.1.0 PREFIX=/usr/local ./scripts/install.sh
set -euo pipefail

VERSION="${TERRADRIFT_VERSION:-v1.1.0}"
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

# Match bare names (current releases) or legacy "dist/<archive>" lines.
# Normalize to the bare name so sha256sum/shasum -c finds the local file.
verify_line="$(
  grep -E " (dist/)?${archive}\$" "${tmpdir}/checksums.txt" | head -n1 | sed -E "s| dist/${archive}\$| ${archive}|"
)"
if [[ -z "${verify_line}" ]]; then
  echo "no checksum line for ${archive} in checksums.txt" >&2
  exit 1
fi

(
  cd "${tmpdir}"
  if command -v sha256sum >/dev/null 2>&1; then
    printf '%s\n' "${verify_line}" | sha256sum -c -
  else
    printf '%s\n' "${verify_line}" | shasum -a 256 -c -
  fi
)

tar -xzf "${tmpdir}/${archive}" -C "${tmpdir}" terradrift
mkdir -p "${PREFIX}/bin"
install -m 0755 "${tmpdir}/terradrift" "${PREFIX}/bin/terradrift"
echo "installed ${PREFIX}/bin/terradrift (${VERSION} ${os}/${arch})"
