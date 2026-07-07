# 2. Scope

## 2.1 Supported environments

### Supported

- **EKS Auto Mode** and any Kubernetes node where the kubelet writes pod logs
  under `/var/log/pods/<ns>_<pod>_<uid>/<container>/<n>.log` and rotates them to
  `<n>.log.<timestamp>` / `<n>.log.<timestamp>.gz`.
- Linux nodes only (the event loop uses `inotify`; preservation uses hardlinks).
- `x86_64` and `arm64` (the image is multi-arch).

### Requirements

- The preserve directory and the watch directory must be on the **same
  filesystem** (hardlinks cannot cross filesystems). A startup hardlink test
  enforces this.
- The process runs as **root (uid 0)**: reading kubelet-owned files under
  `/var/log/pods` and creating hardlinks require it. The distroless `nonroot`
  image tag is therefore not usable.

## 2.2 Composition with a log agent (fluent-bit)

`pod-log-preserver` is designed to compose with fluent-bit's `tail` input:

- fluent-bit tails both the live pod logs and the preserved tree.
- `pod-log-preserver` reads fluent-bit's tail DB **read-only** to learn how far
  fluent-bit has read each preserved file (matched by inode), and deletes a
  preserved file once fluent-bit has read it fully.
- If a preserved file is not yet known to any tail DB, it is left to age-based
  cleanup — never deleted on the assumption that "no row" means "finished".

The DB glob defaults to fluent-bit's naming (`flb_kube*.db`) so DBs belonging to
other tail inputs are never read by mistake. The feature is optional: with the
glob empty, cleanup is purely age-based.
