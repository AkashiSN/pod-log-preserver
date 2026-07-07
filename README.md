# pod-log-preserver

Preserve kubelet-rotated pod logs on EKS Auto Mode until a log agent has
collected them — then reclaim the disk automatically.

## Why

On EKS Auto Mode the kubelet's `containerLogMaxSize` (10MB) and
`containerLogMaxFiles` (5) cannot be customized. A container that logs faster
than a log agent collects can have a rotated log deleted by the kubelet before
it was ever read, losing those lines. `pod-log-preserver` closes that gap.

## How it works

Running as a DaemonSet, it watches `/var/log/pods` and **hardlinks** each pod
log into a preserve directory on the same filesystem, keeping the bytes alive
after the kubelet deletes the original. A cleanup loop reads the log agent's
(fluent-bit) tail DB **read-only** and deletes a preserved file only once the
agent has read it fully; files not yet confirmed fall back to an age threshold.
See the [specification](docs/specification/) for the full design.

## Install (Helm)

> **Planned — ships with the first release, `v0.5.0`.** The image
> (`ghcr.io/akashisn/pod-log-preserver`) and OCI Helm chart
> (`oci://ghcr.io/akashisn/charts/pod-log-preserver`) are published by the
> release workflow, added with the implementation. Until then, this repo
> carries the specification and development process only.

```bash
helm install pod-log-preserver \
  oci://ghcr.io/akashisn/charts/pod-log-preserver --version 0.5.0 \
  --namespace kube-system
```

## Configuration

Configured entirely via environment variables (see
[spec §5.4](docs/specification/05-implementation.md#54-configuration-schema)):

| Env var | Default | Meaning |
|---------|---------|---------|
| `WATCH_DIR` | `/var/log/pods` | Directory tree to watch |
| `PRESERVE_DIR` | `/var/log/pods-preserved` | Where hardlinks are created |
| `CLEANUP_INTERVAL_SEC` | `60` | Cleanup loop period |
| `CLEANUP_MAX_AGE_MIN` | `5` | Age threshold for non-`.gz` orphans |
| `CLEANUP_GZ_MAX_AGE_MIN` | `60` | Age threshold for `.gz` orphans |
| `RESYNC_INTERVAL_SEC` | `30` | Periodic full-resync period |
| `NAMESPACE_FILTER` | (all) | Comma-separated namespace glob patterns |
| `LOG_LEVEL` | `info` | `debug` or `info` |
| `METRICS_PORT` | `9113` | Prometheus metrics port |
| `PRESERVED_LOG_DB_GLOB` | `/var/lib/fluent-bit/flb_kube*.db` | Tail DB glob; empty disables DB-aware cleanup |

## Metrics

A Prometheus endpoint on `METRICS_PORT` at `/metrics` exposes
`pod_log_preserver_preserved_files`, `..._orphaned_files`,
`..._preserved_bytes`, `..._hardlinks_created_total`,
`..._orphans_removed_total`, `..._db_confirmed_removed_total`, and
`..._fluentbit_db_errors_total`. See
[spec §4.2](docs/specification/04-operations.md#42-observability).

## Requirements / Caveats

- **Same filesystem**: the watch and preserve directories must share a
  filesystem (hardlinks cannot cross filesystems); a startup test enforces this.
- **Root required**: reading kubelet-owned logs and creating hardlinks need
  uid 0 — the distroless `nonroot` tag is not usable.
- **Tail DB is read-only but rw-mounted**: fluent-bit uses WAL, and a WAL reader
  must register in the `-shm` index, which needs write access to the DB
  directory.

See [spec §4](docs/specification/04-operations.md) for details.

## License

[Apache License 2.0](LICENSE).
