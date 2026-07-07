## Project

`pod-log-preserver` preserves kubelet-rotated pod logs on EKS Auto Mode. On EKS
Auto Mode the kubelet's `containerLogMaxSize` (10MB) and `containerLogMaxFiles`
(5) cannot be customized, so a rotated log can be deleted before a log agent has
collected it. `pod-log-preserver` hardlinks `/var/log/pods/` logs into
`/var/log/pods-preserved/`, then cleans them up only after fluent-bit's
read-only tail DB confirms a full read (falling back to an age threshold when
unconfirmed). It runs as a DaemonSet; the implementation is a single-file Go
program with SQLite as its only external dependency.

The **source of truth for design** is [`docs/specification/`](docs/specification/)
(Japanese translation: [`docs/ja/specification/`](docs/ja/specification/)).
Read it before making design-affecting changes.

The project is in its **initial OSS bootstrap** phase: this repository currently
carries the development-process foundation and the specification. The
implementation (the Go program, `Dockerfile`, Helm chart, and release CI) is
built later, issue-first, targeting the first public release `v0.5.0` (feature
parity with the internal predecessor, on a new runtime: `modernc.org/sqlite` +
distroless static). The specification is the source of truth — keep code and
spec in sync (see *Specification rules* below).

## Development process

- **Design changes require an Issue first.** Anything that changes behavior,
  the configuration schema (env keys), metric names, or the public surface must
  start as a GitHub Issue and reach agreement before implementation.
- **Branch naming**: `feat/<issue#>-<topic>`, `fix/<issue#>-<topic>`,
  `docs/<topic>`, `chore/<topic>`, `refactor/<topic>`.
- **One PR = one concern.** Keep PRs focused and reviewable.
- Every PR body must reference its issue with `Closes #<issue>` (or `Refs #<issue>`).
- **`main` is protected**: PR-only, CI must be green, squash merge.
- **Conventional Commits** for commit messages and PR titles:
  `type(scope): subject` where type ∈ {feat, fix, docs, chore, refactor, test, perf}.
  Examples:
  - `feat(cleanup): add db-confirmed orphan removal`
  - `fix(inotify): handle queue overflow resync`
  - `docs(spec): clarify tail-DB read-only semantics`
- **Milestones** (`v0.5`, …) group issues toward each release.
- **Test-Driven Development.** Implementation work is written test-first: a
  failing test, the minimal code to pass it, then refactor.

## Parallel development with worktrees

- **One Issue = one PR = one branch = one git worktree.** Each unit of work
  lives in its own `git worktree` so concurrent work streams never share a
  working tree and cannot interfere with one another.
- **Place worktrees under `.worktrees/`.** Create each worktree inside the repo
  at `.worktrees/<branch-topic>` (e.g.
  `git worktree add -b chore/foo .worktrees/foo origin/main`). The directory is
  git-ignored, so the working tree stays inside the repo (visible to editors)
  without being tracked.
- **Tear down after merge.** Once a PR is squash-merged, remove its worktree
  (`git worktree remove`) and delete the branch. Worktrees are disposable and
  must not accumulate.
- **No stacked branches.** Do not branch off another in-flight feature branch.
  Land the base change first, then start dependent work on a fresh branch off
  the updated `main`. Keep every branch rooted at `main`.

## Specification rules

- `docs/specification/` (English) is the canonical spec. `docs/ja/specification/`
  is a translation and **must be kept in sync** — update both in the same PR.
- English is the default language for code, comments, docs, issues, and PRs.
  Japanese content lives only under `docs/ja/`.
- The spec leads the implementation. Do not let code and spec diverge; if a PR
  changes behavior, update the spec in the same PR.

## Architectural invariants (do not break without an ADR/Issue)

- **Read-only against fluent-bit's tail DB.** The DB is opened with `mode=ro`
  and the pool pinned to a single connection; `pod-log-preserver` never writes
  fluent-bit's rows. (The volume is still mounted read-write because a WAL
  reader must register in the `-shm` wal-index.)
- **Preservation is via hardlinks, never copies.** `/var/log/pods` and
  `/var/log/pods-preserved` must be on the same filesystem; a hardlink test
  gates startup.
- **Deletion is confirmation-first, age-second.** An orphaned preserved file
  (`Nlink==1`) is removed immediately when a tail DB confirms fluent-bit read it
  fully; otherwise it waits for the age threshold. A file absent from all DBs is
  never treated as "finished".
- **All state is on the filesystem and fluent-bit's DB** — no external
  datastore, no Kubernetes API dependency.

## Must not

- Do not mix a design decision into an unrelated PR — open an Issue for it.
- Do not introduce organization-specific, internal, or proprietary information
  (company names, internal hostnames, business-cycle details, account IDs, IAM
  names, etc.). This is a public, vendor-neutral OSS project.
- Do not reintroduce a CGO SQLite driver — the runtime is pure-Go
  `modernc.org/sqlite` so the image can be distroless static.

## Contributor docs

Human-facing process lives in [`CONTRIBUTING.md`](CONTRIBUTING.md). This file
is the single source of truth for process; `CLAUDE.md` only adds AI-specific
emphasis and points back to it.
