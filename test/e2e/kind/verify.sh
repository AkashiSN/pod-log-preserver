#!/usr/bin/env bash
# Assert that pod-log-preserver hardlinked a real pod's kubelet-written log into
# the preserve tree on the node. Runs against the current kubectl context.
set -euo pipefail

NS="${E2E_NS:-plp-e2e}"
REL="${E2E_RELEASE:-pod-log-preserver}"
SYS_NS="${E2E_SYS_NS:-default}"
INSPECTOR="plp-e2e-inspector"

cleanup_inspector() {
  kubectl -n "$SYS_NS" delete pod "$INSPECTOR" --ignore-not-found --now >/dev/null 2>&1 || true
}
trap cleanup_inspector EXIT

# A workload that writes identifiable log lines.
kubectl -n "$NS" run logger --image=busybox:1.37 --restart=Never -- \
  sh -c 'for i in $(seq 1 50); do echo "plp-e2e-marker line $i"; done; sleep 3600'
kubectl -n "$NS" wait --for=condition=Ready pod/logger --timeout=120s

# Find a plp DaemonSet pod (one per node; single-node cluster -> one) and the
# node it runs on, so the inspector below lands on the same node.
plp_pod="$(kubectl -n "$SYS_NS" get pods -l "app.kubernetes.io/name=pod-log-preserver" \
  -o jsonpath='{.items[0].metadata.name}')"
[ -n "$plp_pod" ] || { echo "::error::no pod-log-preserver pod found"; exit 1; }
plp_node="$(kubectl -n "$SYS_NS" get pod "$plp_pod" -o jsonpath='{.spec.nodeName}')"
[ -n "$plp_node" ] || { echo "::error::could not resolve node for $plp_pod"; exit 1; }

# The plp container image is gcr.io/distroless/static-debian12 (see Dockerfile):
# it ships only the static Go binary, with no shell, `ls`, or `grep`. So the
# preserve tree cannot be inspected via `kubectl exec <plp pod> -- sh ...`. Run a
# short-lived busybox pod on the SAME node instead, mounting the node's
# /var/log read-only via hostPath -- the same directory plp mounts read-write,
# with pods-preserved/ as a subdirectory of it (see
# charts/pod-log-preserver/templates/daemonset.yaml) -- and inspect the tree
# from there.
cleanup_inspector
kubectl -n "$SYS_NS" apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: $INSPECTOR
  namespace: $SYS_NS
  labels:
    app: plp-e2e-inspector
spec:
  nodeName: $plp_node
  restartPolicy: Never
  containers:
    - name: inspector
      image: busybox:1.37
      command: ["sh", "-c", "sleep 3600"]
      volumeMounts:
        - name: host-log
          mountPath: /host/var/log
          readOnly: true
  volumes:
    - name: host-log
      hostPath:
        path: /var/log
        type: Directory
EOF
kubectl -n "$SYS_NS" wait --for=condition=Ready "pod/$INSPECTOR" --timeout=60s

# Poll for up to 60s (preservation is event-driven + resync).
for _ in $(seq 1 30); do
  if kubectl -n "$SYS_NS" exec "$INSPECTOR" -- \
      sh -c 'ls -R /host/var/log/pods-preserved 2>/dev/null | grep -q logger'; then
    echo "ok: logger pod log preserved under /var/log/pods-preserved"
    exit 0
  fi
  sleep 2
done
echo "::error::logger pod log was not preserved within timeout"
kubectl -n "$SYS_NS" exec "$INSPECTOR" -- sh -c 'ls -R /host/var/log/pods-preserved' || true
exit 1
