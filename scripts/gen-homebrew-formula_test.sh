#!/usr/bin/env bash
# Fails if gen-homebrew-formula.sh drops a platform sha256.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
fixture="$(mktemp)"
trap 'rm -f "${fixture}"' EXIT
cat >"${fixture}" <<'EOF'
aaa111  dist/terradrift_darwin_amd64.tar.gz
bbb222  dist/terradrift_darwin_arm64.tar.gz
ccc333  dist/terradrift_linux_amd64.tar.gz
ddd444  dist/terradrift_linux_arm64.tar.gz
EOF
out="$("${root}/scripts/gen-homebrew-formula.sh" v9.9.9 "${fixture}")"
printf '%s\n' "${out}" | grep -q 'version "9.9.9"'
printf '%s\n' "${out}" | grep -q 'sha256 "aaa111"'
printf '%s\n' "${out}" | grep -q 'sha256 "bbb222"'
printf '%s\n' "${out}" | grep -q 'sha256 "ccc333"'
printf '%s\n' "${out}" | grep -q 'sha256 "ddd444"'
if "${root}/scripts/gen-homebrew-formula.sh" v9.9.9 /dev/null >/dev/null 2>&1; then
  echo "expected missing checksums to fail" >&2
  exit 1
fi
echo "gen-homebrew-formula.sh ok"
