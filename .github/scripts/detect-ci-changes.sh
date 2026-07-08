#!/usr/bin/env bash
# Classify a list of changed file paths (newline-separated on stdin) into the
# five CI concern flags. Pure: no git, no GitHub Actions context — the workflow
# feeds it `git diff --name-only` output. Unit-tested by detect-ci-changes.test.sh.
#
# `infra` is the shared build machinery (Makefile, aqua.yaml, the CI workflows,
# these scripts) that can affect any job; each consumer gates on
# `<their-flag> || infra`, a deliberately conservative over-trigger. The release
# workflow change triggers docker and chart jobs for smoke-building before tagging.
# Unlike node-rotation-controller there is no api/, config/, aqua-policy.yaml, or
# local aqua registry here, so those patterns are absent.
set -euo pipefail

changed="$(cat)"

has() { grep -qE "$1" <<<"$changed"; }   # here-string: no pipe, so no SIGPIPE under pipefail

go=false; chart=false; docker=false; docs=false; infra=false
if has '(\.go$|^go\.(mod|sum)$|^internal/version/VERSION$|^\.golangci\.ya?ml$)'; then go=true; fi
if has '(^charts/|^\.github/workflows/release\.yaml$)'; then chart=true; fi
if has '(^Dockerfile$|^\.dockerignore$|^\.github/workflows/release\.yaml$)'; then docker=true; fi
if has '(^docs/|^README(\.ja)?\.md$|^package(-lock)?\.json$)'; then docs=true; fi
if has '(^Makefile$|^aqua\.yaml$|^\.github/workflows/ci\.yaml$|^\.github/workflows/docs-lint\.yaml$|^\.github/scripts/)'; then infra=true; fi

printf 'go=%s\nchart=%s\ndocker=%s\ndocs=%s\ninfra=%s\n' "$go" "$chart" "$docker" "$docs" "$infra"
