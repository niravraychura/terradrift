#!/usr/bin/env bash
# Print a Homebrew formula for a TerraDrift GitHub Release.
# Usage: scripts/gen-homebrew-formula.sh v1.1.1 [checksums.txt]
# If checksums.txt is omitted, it is downloaded from the matching GitHub Release.
set -euo pipefail

raw="${1:?usage: gen-homebrew-formula.sh vX.Y.Z [checksums.txt]}"
case "${raw}" in
  v*) tag="${raw}"; ver="${raw#v}" ;;
  *) tag="v${raw}"; ver="${raw}" ;;
esac

checksums="${2:-}"
tmpdir=""
if [[ -z "${checksums}" ]]; then
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "${tmpdir}"' EXIT
  checksums="${tmpdir}/checksums.txt"
  curl -fsSL -o "${checksums}" "https://github.com/niravraychura/terradrift/releases/download/${tag}/checksums.txt"
fi

sha_for() {
  local name="$1" hash
  if ! hash="$(awk -v n="${name}" '$2 ~ n"$" { print $1; found=1 } END { if (!found) exit 1 }' "${checksums}")"; then
    echo "missing sha256 for ${name} in ${checksums}" >&2
    exit 1
  fi
  printf '%s' "${hash}"
}

darwin_amd64="$(sha_for terradrift_darwin_amd64.tar.gz)"
darwin_arm64="$(sha_for terradrift_darwin_arm64.tar.gz)"
linux_amd64="$(sha_for terradrift_linux_amd64.tar.gz)"
linux_arm64="$(sha_for terradrift_linux_arm64.tar.gz)"

cat <<EOF
# frozen_string_literal: true

# Homebrew formula for the TerraDrift CLI (GitHub Release archives).
class Terradrift < Formula
  desc "Plan-based Terraform and OpenTofu drift detection CLI"
  homepage "https://github.com/niravraychura/terradrift"
  version "${ver}"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/niravraychura/terradrift/releases/download/v#{version}/terradrift_darwin_arm64.tar.gz"
      sha256 "${darwin_arm64}"
    end
    on_intel do
      url "https://github.com/niravraychura/terradrift/releases/download/v#{version}/terradrift_darwin_amd64.tar.gz"
      sha256 "${darwin_amd64}"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/niravraychura/terradrift/releases/download/v#{version}/terradrift_linux_arm64.tar.gz"
      sha256 "${linux_arm64}"
    end
    on_intel do
      url "https://github.com/niravraychura/terradrift/releases/download/v#{version}/terradrift_linux_amd64.tar.gz"
      sha256 "${linux_amd64}"
    end
  end

  def install
    bin.install "terradrift"
    generate_completions_from_executable(bin/"terradrift", "completion")
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/terradrift --version")
  end
end
EOF
