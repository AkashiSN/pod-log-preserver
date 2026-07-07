package validate

import (
	"os"
	"path/filepath"
	"testing"
)

// TestValidateHardlinkSameFilesystem verifies the startup gate passes when the
// watch and preserve directories share a filesystem, creates the preserve
// directory, and leaves no probe files behind.
func TestValidateHardlinkSameFilesystem(t *testing.T) {
	dir := t.TempDir()
	watch := filepath.Join(dir, "pods")
	preserve := filepath.Join(dir, "preserved")
	if err := os.MkdirAll(watch, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := ValidateHardlink(watch, preserve); err != nil {
		t.Fatalf("same-filesystem validation should pass: %v", err)
	}

	if _, err := os.Stat(preserve); err != nil {
		t.Fatalf("preserve directory was not created: %v", err)
	}
	for _, d := range []string{watch, preserve} {
		entries, err := os.ReadDir(d)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatalf("probe file left behind in %s: %v", d, entries)
		}
	}
}

// TestValidateHardlinkMissingWatchDirFails verifies the gate fails fast when the
// watch directory does not exist (the probe cannot be created there).
func TestValidateHardlinkMissingWatchDirFails(t *testing.T) {
	dir := t.TempDir()
	watch := filepath.Join(dir, "does-not-exist")
	preserve := filepath.Join(dir, "preserved")

	if err := ValidateHardlink(watch, preserve); err == nil {
		t.Fatal("validation should fail fast when the watch dir is missing")
	}
}
