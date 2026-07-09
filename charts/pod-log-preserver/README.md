# pod-log-preserver (Helm chart)

Deploy [`pod-log-preserver`](https://github.com/AkashiSN/pod-log-preserver) as a
DaemonSet. It preserves kubelet-rotated pod logs on EKS Auto Mode until a log
agent has collected them, then reclaims the disk automatically.

On EKS Auto Mode the kubelet's `containerLogMaxSize` (10MB) and
`containerLogMaxFiles` (5) cannot be customized, so a container that logs faster
than the agent collects can have a rotated log deleted before it was ever read.
This chart runs a per-node agent that **hardlinks** each pod log into a preserve
directory on the same filesystem and deletes a preserved file only once
fluent-bit's tail DB confirms a full read (falling back to an age threshold).
See the [project README](https://github.com/AkashiSN/pod-log-preserver#readme)
and the [specification](https://github.com/AkashiSN/pod-log-preserver/tree/main/docs/specification)
for the full design.

## TL;DR

```bash
helm install pod-log-preserver \
  oci://ghcr.io/akashisn/charts/pod-log-preserver --version 0.5.0 \
  --namespace kube-system
```

## Prerequisites

- Kubernetes cluster (built for **EKS Auto Mode**, but works anywhere the
  kubelet writes `/var/log/pods` and fluent-bit tails it).
- Helm 3.8+ (for OCI registry support).
- A log agent — **fluent-bit** — running on the same nodes, writing a tail DB
  under `hostPaths.fluentBitDBDir` (default `/var/lib/fluent-bit`).
- The node's watch and preserve directories must live on the **same
  filesystem** (both under `/var/log` by default); a startup test enforces this.

## Installing

The OCI Helm chart and the multi-arch image are published to GHCR by the release
workflow.

```bash
# From the published OCI registry (recommended)
helm install pod-log-preserver \
  oci://ghcr.io/akashisn/charts/pod-log-preserver --version 0.5.0 \
  --namespace kube-system

# From a local checkout
helm install pod-log-preserver ./charts/pod-log-preserver \
  --namespace kube-system
```

The chart has no namespaced RBAC or ServiceAccount token; pick the install
namespace with `--namespace`.

The DaemonSet renders its own `metadata.namespace` from the release namespace, so
rendering the chart yourself needs `--namespace` too — `helm template` does not
inject it the way `helm install` does:

```bash
helm template pod-log-preserver ./charts/pod-log-preserver \
  --namespace kube-system | kubectl apply -f -
```

Argo CD (`destination.namespace`), Flux (`HelmRelease.spec.targetNamespace`), and
helmfile all set the release namespace themselves, so they need no extra wiring.

## Uninstalling

```bash
helm uninstall pod-log-preserver --namespace kube-system
```

Uninstalling removes the DaemonSet. Files already hardlinked into the preserve
directory on each node are left in place; they are plain filesystem entries and
can be removed manually if needed.

## Configuration

Every runtime setting lives under `config.*` and maps to the environment
variable of the same purpose (see
[spec §5.4](https://github.com/AkashiSN/pod-log-preserver/blob/main/docs/specification/05-implementation.md#54-configuration-schema)).
An empty string falls back to the binary's built-in default. Override with
`--set config.<key>=<value>` or a values file.

### Runtime config (`config.*`)

| Key | Env var | Default | Meaning |
|-----|---------|---------|---------|
| `config.watchDir` | `WATCH_DIR` | `/var/log/pods` | Directory tree to watch |
| `config.preserveDir` | `PRESERVE_DIR` | `/var/log/pods-preserved` | Where hardlinks are created |
| `config.cleanupIntervalSec` | `CLEANUP_INTERVAL_SEC` | `60` | Cleanup loop period (seconds) |
| `config.cleanupMaxAgeMin` | `CLEANUP_MAX_AGE_MIN` | `5` | Age threshold for non-`.gz` orphans (minutes) |
| `config.cleanupGzMaxAgeMin` | `CLEANUP_GZ_MAX_AGE_MIN` | `60` | Age threshold for `.gz` orphans (minutes) |
| `config.resyncIntervalSec` | `RESYNC_INTERVAL_SEC` | `30` | Periodic full-resync period (seconds) |
| `config.namespaceFilter` | `NAMESPACE_FILTER` | `""` (all) | Comma-separated namespace glob patterns |
| `config.logLevel` | `LOG_LEVEL` | `info` | `debug` or `info` |
| `config.metricsPort` | `METRICS_PORT` | `9113` | Prometheus metrics port |
| `config.preservedLogDBGlob` | `PRESERVED_LOG_DB_GLOB` | `/var/lib/fluent-bit/flb_kube*.db` | Tail DB glob; empty disables DB-aware cleanup |

### Image, scheduling, and packaging

| Key | Default | Meaning |
|-----|---------|---------|
| `image.repository` | `ghcr.io/akashisn/pod-log-preserver` | Image repository |
| `image.tag` | `""` | Image tag; empty defaults to the chart `appVersion` |
| `image.pullPolicy` | `IfNotPresent` | Image pull policy |
| `imagePullSecrets` | `[]` | Secrets for pulling the image |
| `nameOverride` | `""` | Override the chart name portion of resource names |
| `fullnameOverride` | `""` | Override the full resource name |
| `extraEnv` | `[]` | Extra `EnvVar`s appended verbatim to the container |
| `resources` | `50m` CPU / `32Mi` mem (request == limit) | Container resource requests/limits |
| `updateStrategy` | `RollingUpdate`, `maxUnavailable: 1` | DaemonSet update strategy |
| `tolerations` | `[{operator: Exists}]` | Land on every node, including tainted ones |
| `nodeSelector` | `{}` | Restrict which nodes run the pod |
| `affinity` | `{}` | Pod affinity/anti-affinity rules |
| `priorityClassName` | `""` | PriorityClass for the pods |
| `automountServiceAccountToken` | `false` | No Kubernetes API access is needed (spec §4.3) |

### Security context

The pod runs as **root (uid/gid 0)** by default: reading kubelet-owned logs and
creating hardlinks both require it, so the distroless `nonroot` image tag is not
usable.

| Key | Default | Meaning |
|-----|---------|---------|
| `podSecurityContext` | `{}` | Pod-level security context |
| `securityContext` | `runAsUser: 0`, `runAsGroup: 0` | Container security context |

### Host paths

`hostLogDir` must contain both `config.watchDir` and `config.preserveDir` so
preservation hardlinks stay on one filesystem. `fluentBitDBDir` is mounted
read-write because a `mode=ro` WAL reader must still register in the `-shm`
wal-index.

| Key | Default | Meaning |
|-----|---------|---------|
| `hostPaths.hostLogDir` | `/var/log` | Host log dir mounted (rw) into the pod |
| `hostPaths.fluentBitDBDir` | `/var/lib/fluent-bit` | fluent-bit tail DB dir mounted (rw) |

### Labels, annotations, and Prometheus scraping

When `prometheusScrape` is `true` (default), the chart injects annotation-based
scrape annotations (`prometheus.io/scrape`, `/path`, and `/port` derived from
`config.metricsPort`) so they always match the served endpoint. User-supplied
`podAnnotations` win on conflict, so a key there can override or drop a
generated one.

| Key | Default | Meaning |
|-----|---------|---------|
| `prometheusScrape` | `true` | Inject Prometheus scrape annotations |
| `podAnnotations` | `{}` | Extra pod annotations (override generated ones) |
| `podLabels` | `{}` | Extra pod labels merged onto selector labels |
| `commonAnnotations` | `{}` | Annotations on the DaemonSet object itself |
| `commonLabels` | `{}` | Extra labels on every rendered resource |

## Metrics

A Prometheus endpoint is served on `config.metricsPort` (default `9113`) at
`/metrics`; the chart enables annotation-based scraping by default. See
[spec §4.2](https://github.com/AkashiSN/pod-log-preserver/blob/main/docs/specification/04-operations.md#42-observability).

| Metric | Type | Meaning |
|--------|------|---------|
| `pod_log_preserver_preserved_files` | gauge | Files currently in the preserve directory |
| `pod_log_preserver_orphaned_files` | gauge | Preserved files with link count 1 |
| `pod_log_preserver_preserved_bytes` | gauge | Total bytes under the preserve directory |
| `pod_log_preserver_hardlinks_created_total` | counter | Hardlinks created |
| `pod_log_preserver_orphans_removed_total` | counter | Orphaned files removed |
| `pod_log_preserver_db_confirmed_removed_total` | counter | Orphans removed after a tail DB confirmed a full read |
| `pod_log_preserver_fluentbit_db_errors_total` | counter | Tail DB read errors |

## Caveats

- **Same filesystem**: the watch and preserve directories must share a
  filesystem (hardlinks cannot cross filesystems); a startup test enforces this.
- **Root required**: reading kubelet-owned logs and creating hardlinks need
  uid 0 — the distroless `nonroot` tag is not usable.
- **hostPath mounts**: the node's `/var/log` (rw) and the fluent-bit DB
  directory (rw) are mounted from the host.
- **Tail DB is read-only but rw-mounted**: fluent-bit uses WAL, and a WAL reader
  must register in the `-shm` index, which needs write access to the DB
  directory.

See [spec §4](https://github.com/AkashiSN/pod-log-preserver/blob/main/docs/specification/04-operations.md)
for details.

## License

[Apache License 2.0](https://github.com/AkashiSN/pod-log-preserver/blob/main/LICENSE).
