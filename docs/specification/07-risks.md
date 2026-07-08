# 7. Risks & Status

## 7.1 Risks

| # | Risk | Mitigation |
|---|------|-----------|
| 1 | Preserve and watch dirs on different filesystems → hardlinks impossible | Startup hardlink test fails fast before any work begins (§4.1). |
| 2 | A preserved file is deleted before the agent finishes reading it | Deletion is confirmation-first: an orphan is removed only when a tail DB shows offset ≥ size, else it waits for the age threshold (§3.2–3.3). |
| 3 | inode-number collision matches an unrelated file in the tail DB | The recorded `name` must also end with `/<relPath>`, anchoring the match on a path boundary (§3.2). |
| 4 | inotify queue overflow drops events | A queue-overflow event triggers a full resync; a periodic resync also runs unconditionally (§5.1). |
| 5 | A new tail input's slow first discovery races a deletion | A file becomes an orphan only several rotation cycles after hardlinking, by which time a sub-second-refresh agent has discovered it; new inputs must share the DB glob (§3.2). |
| 6 | Running as root with hostPath is a broad privilege | Required for the task; no Kubernetes API access is requested, and the tail DB is read-only (§4.3). |

## 7.2 Validated assumptions

- The internal predecessor ran in production against fluent-bit tail DBs; the
  hardlink-preserve + DB-confirmed-cleanup behavior is proven there.
- **To re-validate for this release:** the pure-Go `modernc.org/sqlite` driver
  reads a real fluent-bit WAL tail DB read-only with results identical to the
  previous CGO driver — verified against a copied real DB during implementation.
- **Automated re-validation:** the hybrid e2e harness (`test/e2e/`) exercises the
  full preserve → fluent-bit-confirms → cleanup loop against the shipped image and
  a real fluent-bit WAL tail DB on every CI run, replacing the one-time manual
  check above with a continuous gate. The harness pins a single fluent-bit
  version (`3.1.9`); the tail-DB schema matrix that makes this generalize across
  fluent-bit 1.x–5.x — and its safe fallback on an incompatible schema — is
  stated in §5.3 and covered by a unit-test support matrix over each major's
  schema.

## 7.3 Open questions

- Whether to expose a "confirmed-only, never age-delete" strict mode for
  operators who prefer to never drop unconfirmed logs.
- Whether to support log agents other than fluent-bit (different tail-DB
  schemas) behind a pluggable reader.
