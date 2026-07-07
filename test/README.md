# test/

Reserved for integration and end-to-end tests that exercise the built binary
or span multiple packages.

Unit tests live **beside the code they test**, in the same package (for example
`internal/keeper/preserve_test.go`), following Go convention — they assert
package-internal behavior and must compile with their package. Only tests that
do not belong to a single `internal/` package (integration/e2e) go here.

There are no integration tests yet; this directory is a placeholder documenting
where they will land.
