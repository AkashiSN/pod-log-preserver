package main

import (
	"strings"
	"testing"
)

// TestVersionEmbedded asserts the version is populated from the embedded
// VERSION file. The exact value is intentionally not pinned here so a version
// bump does not require a test edit; the invariants are that it is present and
// carries no surrounding whitespace.
func TestVersionEmbedded(t *testing.T) {
	if version == "" {
		t.Fatal("version is empty; VERSION file was not embedded")
	}
	if version != strings.TrimSpace(version) {
		t.Fatalf("version has surrounding whitespace: %q", version)
	}
}
