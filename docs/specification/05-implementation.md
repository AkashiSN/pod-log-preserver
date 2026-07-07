# 5. Implementation

## 5.1 Architecture

A single Go binary running as a DaemonSet. The implementation is one
`package main` split across concern-focused files (config, logging, metrics,
inotify watching, preservation, tail-DB read, cleanup, validation) rather than
a literal single file. Three concurrent loops share in-memory metric counters
and coordinate shutdown via a context:

1. **Event loop** — an `inotify` watch tree over the watch directory reacts to
   new files and directories, creating hardlinks as logs appear/rotate.
2. **Periodic resync** — a full walk of the watch directory on a fixed interval,
   catching anything inotify missed (e.g. a queue overflow).
3. **Cleanup loop** — a periodic walk of the preserve directory that removes
   confirmed or aged-out orphans and prunes empty directories.

A metrics HTTP server runs alongside. SIGTERM/SIGINT cancels the context and
closes the inotify fd to unblock the event loop for a clean shutdown.

## 5.2 Startup sequence

1. Load configuration from environment variables (§5.4).
2. Create the preserve directory; run the **hardlink validation test** (§4.1)
   against the pod's own container log — fail fast if it cannot hardlink.
3. Initial sync: walk the watch directory and hardlink all existing matching
   logs.
4. Establish the recursive inotify watch tree.
5. Start the metrics server and the resync/cleanup loops; enter the event loop.

## 5.3 Tail DB read

Each cleanup cycle opens every DB matching the glob with a read-only,
single-connection SQLite handle and issues one query
(`SELECT inode, offset, name FROM in_tail_files`), building an
inode → (offset, name) map per DB. A failed DB is logged and skipped, never
fatal. The runtime driver is pure-Go `modernc.org/sqlite` (no CGO) so the image
can be distroless static; the read-only DSN uses `mode=ro` and a
`busy_timeout` pragma.

## 5.4 Configuration schema

All configuration is via environment variables:

| Env var | Default | Meaning |
|---------|---------|---------|
| `WATCH_DIR` | `/var/log/pods` | Directory tree to watch for pod logs |
| `PRESERVE_DIR` | `/var/log/pods-preserved` | Where hardlinks are created |
| `CLEANUP_INTERVAL_SEC` | `60` | Cleanup loop period |
| `CLEANUP_MAX_AGE_MIN` | `5` | Age threshold for non-`.gz` orphans |
| `CLEANUP_GZ_MAX_AGE_MIN` | `60` | Age threshold for `.gz` orphans |
| `RESYNC_INTERVAL_SEC` | `30` | Periodic full-resync period |
| `NAMESPACE_FILTER` | (empty = all) | Comma-separated namespace glob patterns |
| `LOG_LEVEL` | `info` | `debug` or `info` |
| `METRICS_PORT` | `9113` | Prometheus metrics port |
| `PRESERVED_LOG_DB_GLOB` | `/var/lib/fluent-bit/flb_kube*.db` | Tail DB glob; empty disables DB-aware cleanup |
