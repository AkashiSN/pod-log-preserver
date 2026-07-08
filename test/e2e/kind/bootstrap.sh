#!/usr/bin/env bash
# kind smoke for pod-log-preserver: cluster up -> load PR image -> helm install
# the chart -> wait Ready -> verify preservation -> teardown. Tools (kind,
# kubectl, helm) come from aqua/$PATH. Set E2E_KIND_KEEP=true to keep the cluster.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "$here/../../.." && pwd)"
cluster="${E2E_KIND_CLUSTER:-plp-e2e}"
image="${E2E_IMAGE:-pod-log-preserver:e2e}"
ns="${E2E_NS:-plp-e2e}"
release="${E2E_RELEASE:-pod-log-preserver}"

cleanup() {
  if [ "${E2E_KIND_KEEP:-false}" != "true" ]; then
    kind delete cluster --name "$cluster" || true
  fi
}
trap cleanup EXIT

kind create cluster --name "$cluster" --config "$here/kind.yaml" --wait 120s
kind load docker-image "$image" --name "$cluster"

# Install the chart with the loaded image. repo:tag are split from $image.
repo="${image%:*}"; tag="${image##*:}"
helm install "$release" "$root/charts/pod-log-preserver" \
  --set "image.repository=${repo}" \
  --set "image.tag=${tag}" \
  --set "image.pullPolicy=Never" \
  --wait --timeout 180s

kubectl rollout status "daemonset/${release}" --timeout=180s

kubectl create namespace "$ns" 2>/dev/null || true
E2E_NS="$ns" E2E_RELEASE="$release" bash "$here/verify.sh"
