# test/

Integration and end-to-end tests that exercise the built binary or span multiple
packages.

Unit tests live **beside the code they test**, in the same package (for example
`internal/keeper/preserve_test.go`), following Go convention — they assert
package-internal behavior and must compile with their package. Only tests that
do not belong to a single `internal/` package (integration/e2e) go here.

## Subdirectories

- `e2e/container/` — the container harness: runs the binary against synthetic
  pod logs and a real fluent-bit tail DB, exercising the full preserve →
  confirm → cleanup loop.
- `e2e/kind/` — the kind smoke test: deploys the Helm chart on a single-node
  kind cluster and asserts that hardlinking works end-to-end.
