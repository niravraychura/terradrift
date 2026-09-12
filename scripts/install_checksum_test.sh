#!/usr/bin/env bash
# Fails if install.sh cannot resolve bare or dist/-prefixed checksum lines.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"

hash_for() {
  awk -v a="terradrift_linux_amd64.tar.gz" '$2 == a || $2 == "dist/" a { print $1; found=1; exit } END { if (!found) exit 1 }' "$1"
}

dir="$(mktemp -d)"
trap 'rm -rf "${dir}"' EXIT
printf 'payload\n' >"${dir}/terradrift_linux_amd64.tar.gz"
hash="$( (cd "${dir}" && shasum -a 256 terradrift_linux_amd64.tar.gz | awk '{print $1}') )"

printf '%s  terradrift_linux_amd64.tar.gz\n' "${hash}" >"${dir}/bare.txt"
printf '%s  dist/terradrift_linux_amd64.tar.gz\n' "${hash}" >"${dir}/prefixed.txt"
printf '%s  other.tar.gz\n' "${hash}" >"${dir}/missing.txt"

test "$(hash_for "${dir}/bare.txt")" = "${hash}"
test "$(hash_for "${dir}/prefixed.txt")" = "${hash}"
if hash_for "${dir}/missing.txt" >/dev/null 2>&1; then
  echo "expected missing archive name to fail" >&2
  exit 1
fi

# Keep the matcher in install.sh and the generator in release.yml.
grep -qE '\(dist/\)\?' "${root}/scripts/install.sh"
grep -q '(cd dist && sha256sum \*.tar.gz > checksums.txt)' "${root}/.github/workflows/release.yml"

echo "install checksum lines ok"
