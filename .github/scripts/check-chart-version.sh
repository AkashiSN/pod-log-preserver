#!/usr/bin/env bash
# Assert the release tag agrees with the chart's committed version/appVersion.
# The git tag is the source of truth for a release; Chart.yaml is the
# human-readable record on main and the two must agree (spec §6.1). Unlike the
# sibling repo, this chart keeps the leading "v" on appVersion (so it matches
# internal/version/VERSION and the image tag), while version is bare SemVer.
# Usage: check-chart-version.sh <tag>   e.g. check-chart-version.sh v0.5.0
# On success prints "version=<x.y.z>" (GITHUB_OUTPUT-shaped) and exits 0;
# on mismatch exits 1 with an actionable ::error::.
set -euo pipefail

tag="${1:?usage: check-chart-version.sh <tag>}"
chart_dir="${CHART_DIR:-charts/pod-log-preserver}"
chart_file="${chart_dir}/Chart.yaml"

version="${tag#v}"   # strip a leading v: v0.5.0 -> 0.5.0

chart_ver="$(grep -E '^version:' "$chart_file" | awk '{print $2}')"
app_ver="$(grep -E '^appVersion:' "$chart_file" | awk '{gsub(/"/,"",$2); print $2}')"

fail=0
if [ "$chart_ver" != "$version" ]; then
  echo "::error::tag $tag (version $version) != Chart.yaml version $chart_ver" >&2
  fail=1
fi
if [ "$app_ver" != "$tag" ]; then
  echo "::error::tag $tag != Chart.yaml appVersion $app_ver" >&2
  fail=1
fi
if [ "$fail" -ne 0 ]; then
  echo "Bump Chart.yaml version to $version and appVersion to $tag (or tag the right version) and retry." >&2
  exit 1
fi

echo "version=$version"
