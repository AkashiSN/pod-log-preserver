# kind smoke (test/e2e/kind)

Deploys the Helm chart on a single-node `kind` cluster and asserts a real pod's
kubelet-written log is hardlinked into `/var/log/pods-preserved`. It does not run
fluent-bit or force rotation — that is the container harness's job
(`test/e2e/container`).

## Run

```bash
make e2e-kind                    # build image -> cluster -> install -> verify -> teardown
E2E_KIND_KEEP=true make e2e-kind # leave the cluster up for debugging
```

Requires Docker and, on `PATH`, `kind`/`kubectl`/`helm` (aqua provides them).
