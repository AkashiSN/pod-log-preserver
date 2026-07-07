# 3. Design

## 3.1 Preservation via hardlinks

When a matching log file appears (via inotify create/close-write/moved-to, or a
periodic full resync), `pod-log-preserver` creates a hardlink under the preserve
directory, mirroring the source's relative path (`<ns>_<pod>_<uid>/<container>/`).

- **Active logs** (`<n>.log`) are linked with a timestamp suffix appended to the
  destination name, so successive rotations of the same active file do not
  collide.
- **Rotated / compressed logs** (`<n>.log.<ts>`, `<n>.log.<ts>.gz`) keep their
  name.
- Duplicate work is avoided two ways: an inode check (a hardlink to the same
  inode already exists in the destination directory) and a destination-name
  collision check.

Because a hardlink shares the inode, the preserved file keeps the log's bytes
alive even after the kubelet deletes the original.

## 3.2 Tail-DB-confirmed cleanup

A cleanup pass runs on a fixed interval. For each preserved file it stats the
link count:

- `Nlink > 1` — the original still exists; skip.
- `Nlink == 1` — an **orphan**; the original was deleted, only the preserved
  hardlink remains. It is a deletion candidate.

For an orphaned rotated log, `pod-log-preserver` consults the log agent's tail
DBs (loaded once per cycle, read-only). A file is **confirmed consumed** when:

1. at least one tail DB has a row for the file's inode whose recorded `name`
   ends with `/<relPath>` (the leading separator anchors the match on a path
   boundary, guarding against an inode-number collision with an unrelated file);
   **and**
2. in every DB that has such a matching row, the recorded offset has reached the
   file's size.

A confirmed file is deleted immediately, releasing the agent's file descriptor.

**Multiple tail inputs:** a DB with no row for the file does not block deletion —
it is treated as "not discovered yet", not "not finished". This is safe because
a file only becomes an orphan several rotation cycles after it was hardlinked, by
which time any agent legitimately tailing the preserved tree (with a
sub-second refresh interval) has long since discovered it. Consequence: a new
tail input must use the same DB glob so its progress is honored here.

## 3.3 Age-based fallback

When DB-aware cleanup is disabled (empty glob), a DB is unreadable, or a file is
not confirmed, deletion falls back to an age threshold on the file's mtime:

- non-`.gz` orphans use `CLEANUP_MAX_AGE_MIN`;
- `.gz` orphans use `CLEANUP_GZ_MAX_AGE_MIN` (compressed logs are kept longer).

Empty directories under the preserve tree are removed opportunistically at the
end of each cycle.

## 3.4 Namespace filtering

An optional comma-separated `NAMESPACE_FILTER` restricts preservation to
matching namespaces, using glob patterns (e.g. `team-*`) matched against the
first path segment (`<ns>` in `<ns>_<pod>_<uid>`). Empty = all namespaces.
