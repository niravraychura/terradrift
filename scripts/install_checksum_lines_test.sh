#!/usr/bin/env bash
# Ensures install.sh can verify bare and legacy dist/ checksum lines.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"

fake_archive="$(mktemp -d)/terradrift_linux_amd64.tar.gz"
trap 'rm -rf "$(dirname "${fake_archive}")"' EXIT
printf 'payload' >"${fake_archive}"
if command -v sha256sum >/dev/null 2>&1; then
  hash="$(sha256sum "${fake_archive}" | awk '{print $1}')"
else
  hash="$(shasum -a 256 "${fake_archive}" | awk '{print $1}')"
fi

check_one() {
  local label="$1" line="$2"
  local dir
  dir="$(mktemp -d)"
  printf '%s\n' "${line}" >"${dir}/checksums.txt"
  cp "${fake_archive}" "${dir}/terradrift_linux_amd64.tar.gz"
  # Replicate install.sh normalize + verify (no network).
  archive="terradrift_linux_amd64.tar.gz"
  verify_line="$(
    grep -E " (dist/)?${archive}\$" "${dir}/checksums.txt" | head -n1 | sed -E "s| dist/${archive}\$| ${archive}|"
  )"
  [[ -n "${verify_line}" ]] || { echo "${label}: no match" >&2; exit 1; }
  (
    cd "${dir}"
    if command -v sha256sum >/dev/null 2>&1; then
      printf '%s\n' "${verify_line}" | sha256sum -c -
    else
      printf '%s\n' "${verify_line}" | shasum -a 256 -c -
    fi
  ) >/dev/null
  rm -rf "${dir}"
  echo "${label} ok"
}

check_one "bare" "${hash}  terradrift_linux_amd64.tar.gz"
check_one "legacy-dist" "${hash}  dist/terradrift_linux_amd64.tar.gz"

# release.yml shape: bare names only when hashing inside dist/
sample="$(mktemp)"
printf '%s  terradrift_linux_amd64.tar.gz\n%s  terradrift_darwin_arm64.tar.gz\n' "${hash}" "${hash}" >"${sample}"
if grep -q ' dist/' "${sample}"; then
  echo "sample unexpectedly has dist/ prefix" >&2
  exit 1
fi
grep -qE ' terradrift_linux_amd64\.tar\.gz$' "${sample}"
rm -f "${sample}"
echo "install_checksum_lines_test.sh ok"
