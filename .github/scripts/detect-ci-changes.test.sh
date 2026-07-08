#!/usr/bin/env bash
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
script="$here/detect-ci-changes.sh"
fail=0

# assert <name> <expected 5-line block> <input paths...>
assert() {
  local name="$1" expected="$2"; shift 2
  local got; got="$(printf '%s\n' "$@" | "$script")"
  if [ "$got" != "$expected" ]; then
    echo "FAIL: $name"; echo "  expected: $(echo "$expected" | tr '\n' ' ')"; echo "  got:      $(echo "$got" | tr '\n' ' ')"; fail=1
  else echo "ok: $name"; fi
}

ALL_FALSE=$'go=false\nchart=false\ndocker=false\ndocs=false\ninfra=false'
assert "docs-only"       $'go=false\nchart=false\ndocker=false\ndocs=true\ninfra=false'   "docs/specification/01-overview.md" "README.md" "README.ja.md"
assert "chart-only"      $'go=false\nchart=true\ndocker=false\ndocs=false\ninfra=false'   "charts/pod-log-preserver/values.yaml"
assert "go-only"         $'go=true\nchart=false\ndocker=false\ndocs=false\ninfra=false'   "internal/cleanup/foo.go"
assert "version-is-go"   $'go=true\nchart=false\ndocker=false\ndocs=false\ninfra=false'   "internal/version/VERSION"
assert "gomod"           $'go=true\nchart=false\ndocker=false\ndocs=false\ninfra=false'   "go.mod"
assert "golangci-config" $'go=true\nchart=false\ndocker=false\ndocs=false\ninfra=false'   ".golangci.yml"
assert "dockerfile"      $'go=false\nchart=false\ndocker=true\ndocs=false\ninfra=false'   "Dockerfile"
assert "dockerignore"    $'go=false\nchart=false\ndocker=true\ndocs=false\ninfra=false'   ".dockerignore"
assert "package-is-docs" $'go=false\nchart=false\ndocker=false\ndocs=true\ninfra=false'   "package.json"
assert "infra-makefile"  $'go=false\nchart=false\ndocker=false\ndocs=false\ninfra=true'   "Makefile"
assert "infra-aqua"      $'go=false\nchart=false\ndocker=false\ndocs=false\ninfra=true'   "aqua.yaml"
assert "infra-detector"  $'go=false\nchart=false\ndocker=false\ndocs=false\ninfra=true'   ".github/scripts/detect-ci-changes.sh"
assert "infra-ci-yaml"   $'go=false\nchart=false\ndocker=false\ndocs=false\ninfra=true'   ".github/workflows/ci.yaml"
assert "go-and-chart"    $'go=true\nchart=true\ndocker=false\ndocs=false\ninfra=false'    "internal/x.go" "charts/pod-log-preserver/templates/daemonset.yaml"
assert "empty-input"     "$ALL_FALSE"                                                     ""

[ "$fail" -eq 0 ] && echo "ALL PASS" || { echo "SOME FAILED"; exit 1; }
