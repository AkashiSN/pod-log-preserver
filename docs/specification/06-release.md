# 6. Release

## 6.1 Versioning and release

- Semantic Versioning (`vMAJOR.MINOR.PATCH`).
- Pre-1.0 (`v0.x.y`) while the configuration schema and metric names stabilize.
- The compatibility surface is: environment-variable configuration keys,
  Prometheus metric names, and the preserved-log directory layout.
- Distribution: a multi-arch image at
  `ghcr.io/akashisn/pod-log-preserver` and an OCI Helm chart at
  `oci://ghcr.io/akashisn/charts/pod-log-preserver`, published by a
  tag-triggered release workflow. The binary embeds its version, which must
  match the git tag.
- Licensed under the Apache License 2.0.

## 6.2 Roadmap

- **v0.5.0 — first public release.** Feature parity with the internal
  predecessor, re-platformed onto a pure-Go SQLite driver
  (`modernc.org/sqlite`) and a distroless static image. Deliverables: the Go
  program, `Dockerfile`, Helm chart, and the tag-triggered release CI, each
  landed issue-first with tests.
- **Post-v0.5** — hardening from real-cluster soak (edge cases in tail-DB
  confirmation under multiple inputs), and any configuration additions surfaced
  by adopters. A path to v1.0 is declared once the schema and metrics are
  considered stable.
