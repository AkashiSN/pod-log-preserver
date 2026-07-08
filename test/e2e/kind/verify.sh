#!/usr/bin/env bash
# Assert that pod-log-preserver hardlinked a real pod's kubelet-written log into
# the preserve tree on the node. Runs against the current kubectl context.
set -euo pipefail

NS="${E2E_NS:-plp-e2e}"
REL="${E2E_RELEASE:-pod-log-preserver}"
SYS_NS="${E2E_SYS_NS:-default}"

# A workload that writes identifiable log lines.
kubectl -n "$NS" run logger --image=busybox --restart=Never -- \
  sh -c 'for i in $(seq 1 50); do echo "plp-e2e-marker line $i"; done; sleep 3600'
kubectl -n "$NS" wait --for=condition=Ready pod/logger --timeout=120s

# Find a plp DaemonSet pod (one per node; single-node cluster -> one).
plp_pod="$(kubectl -n "$SYS_NS" get pods -l "app.kubernetes.io/name=pod-log-preserver" \
  -o jsonpath='{.items[0].metadata.name}')"
[ -n "$plp_pod" ] || { echo "::error::no pod-log-preserver pod found"; exit 1; }

# The plp pod mounts the preserve tree; assert the logger pod's log was hardlinked
# there. Poll for up to 60s (preservation is event-driven + resync).
for _ in $(seq 1 30); do
  if kubectl -n "$SYS_NS" exec "$plp_pod" -- \
      sh -c 'ls -R /var/log/pods-preserved 2>/dev/null | grep -q logger'; then
    echo "ok: logger pod log preserved under /var/log/pods-preserved"
    exit 0
  fi
  sleep 2
done
echo "::error::logger pod log was not preserved within timeout"
kubectl -n "$SYS_NS" exec "$plp_pod" -- sh -c 'ls -R /var/log/pods-preserved' || true
exit 1
