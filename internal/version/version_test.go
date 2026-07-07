package version

import (
	"strings"
	"testing"
)

// TestVersionEmbedded asserts the version is populated from the embedded
// VERSION file. The exact value is intentionally not pinned here so a version
// bump does not require a test edit; the invariants are that it is present and
// carries no surrounding whitespace.
func TestVersionEmbedded(t *testing.T) {
	if Version == "" {
		t.Fatal("Version is empty; VERSION file was not embedded")
	}
	if Version != strings.TrimSpace(Version) {
		t.Fatalf("Version has surrounding whitespace: %q", Version)
	}
}
