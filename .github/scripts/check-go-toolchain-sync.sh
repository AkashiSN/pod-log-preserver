#!/usr/bin/env bash
# Verify the Go toolchain version is identical across every place it is pinned.
#
# The version is hand-duplicated in three build inputs that must stay in lockstep
# (see the comments on each pin). Renovate manages them in separate managers, so
# nothing but this guard catches a drift — including the case where one pin is
# bumped and the others are forgotten.
#
# Usage: check-go-toolchain-sync.sh
set -euo pipefail

repo_root="${REPO_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"

fail=0
error() {
  echo "::error::$*" >&2
  fail=1
}

# extract <file> — echoes the Go version pinned in that file, or empty string.
extract() {
  local file="${repo_root}/$1"
  [[ -f "$file" ]] || { error "$1: file not found"; return; }
  case "$1" in
    go.mod)
      # the `go <version>` directive (first bare `go <version>` line).
      awk '$1=="go" && $2 ~ /^[0-9]/ {print $2; exit}' "$file"
      ;;
    aqua.yaml)
      # the `- name: golang/go@go<version>` pin.
      sed -n 's/.*golang\/go@go\([0-9][0-9.]*\).*/\1/p' "$file" | head -n1
      ;;
    Dockerfile)
      # the `FROM …golang:<version>-bookworm@sha256:…` builder image.
      sed -n 's/.*golang:\([0-9][0-9.]*\)-bookworm.*/\1/p' "$file" | head -n1
      ;;
    *)
      error "$1: no extractor defined"
      ;;
  esac
}

files=(
  go.mod
  aqua.yaml
  Dockerfile
)

reference=""
reference_file=""
for f in "${files[@]}"; do
  version="$(extract "$f")"
  if [[ -z "$version" ]]; then
    error "$f: could not extract a Go toolchain version"
    continue
  fi
  printf '  %-12s %s\n' "$f" "$version" >&2
  if [[ -z "$reference" ]]; then
    reference="$version"
    reference_file="$f"
  elif [[ "$version" != "$reference" ]]; then
    error "$f pins Go ${version}, but ${reference_file} pins ${reference} — all three must match"
  fi
done

if [[ "$fail" -ne 0 ]]; then
  echo "Bump the Go toolchain in lockstep across go.mod, aqua.yaml, and the Dockerfile." >&2
  exit 1
fi

echo "Go toolchain is synchronized at ${reference}"
