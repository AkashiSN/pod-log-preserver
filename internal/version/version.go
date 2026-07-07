// Package version exposes the release version, embedded at build time from the
// VERSION file colocated with this package. It must match the git tag a release
// is cut from (see the release CI).
package version

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var raw string

// Version is the release version with surrounding whitespace trimmed.
var Version = strings.TrimSpace(raw)
