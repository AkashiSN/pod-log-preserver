# 4. Operations

## 4.1 Requirements / Caveats

| Concern | Treatment |
|---------|-----------|
| Same filesystem | Watch and preserve dirs must share a filesystem; a startup hardlink test fails fast otherwise. |
| Root required | Reading kubelet-owned logs and creating hardlinks need uid 0; the distroless `nonroot` tag is not usable. |
| hostPath mounts | `/var/log` (rw) and the agent's DB dir (e.g. `/var/lib/fluent-bit`) are mounted from the host. |
| Tail DB is rw-mounted | Even though the DB is opened `mode=ro`, fluent-bit uses WAL, and a WAL reader must register in the `-shm` wal-index, which needs write access to the DB directory. |

## 4.2 Observability

A Prometheus endpoint is served on `METRICS_PORT` (default 9113) at `/metrics`:

| Metric | Type | Meaning |
|--------|------|---------|
| `pod_log_preserver_preserved_files` | gauge | Files currently in the preserve directory |
| `pod_log_preserver_orphaned_files` | gauge | Preserved files with link count 1 |
| `pod_log_preserver_preserved_bytes` | gauge | Total bytes under the preserve directory |
| `pod_log_preserver_hardlinks_created_total` | counter | Hardlinks created |
| `pod_log_preserver_orphans_removed_total` | counter | Orphaned files removed |
| `pod_log_preserver_db_confirmed_removed_total` | counter | Orphans removed after a tail DB confirmed a full read |
| `pod_log_preserver_fluentbit_db_errors_total` | counter | Tail DB read errors |

The listener is bound synchronously at startup; if `METRICS_PORT` cannot be
bound (e.g. already in use), startup fails fast rather than running without the
endpoint.

Log verbosity is controlled by `LOG_LEVEL` (`debug` / `info`).

## 4.3 RBAC and security

- No Kubernetes API access is required — the program operates purely on the
  node filesystem, so it needs **no ServiceAccount permissions / RBAC**.
- It requires **root** and **hostPath** access to the node's log and agent-DB
  directories; this is the minimum for hardlinking kubelet logs.
- The agent's tail DB is only ever **read** (`mode=ro`, single connection).

## 4.4 Cost

The steady-state footprint is small (a request=limit of ~50m CPU / ~32Mi memory
is a reasonable default). Preservation is by hardlink, so it consumes no extra
data blocks — only inode/directory-entry overhead — and disk is reclaimed as
soon as collection is confirmed.
