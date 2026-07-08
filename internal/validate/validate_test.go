package validate

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// watchAndPreserve returns an existing watch directory and a not-yet-created
// preserve directory under a fresh temp dir, so both resolve to the same
// filesystem in the common case.
func watchAndPreserve(t *testing.T) (watch, preserve string) {
	t.Helper()
	dir := t.TempDir()
	watch = filepath.Join(dir, "pods")
	preserve = filepath.Join(dir, "preserved")
	if err := os.MkdirAll(watch, 0o755); err != nil {
		t.Fatal(err)
	}
	return watch, preserve
}

// TestValidateFilesystemPassesOnSameFilesystem verifies the gate creates the
// preserve directory, passes when watch and preserve share a filesystem, and
// leaves no probe files behind — with no dependency on pod identity or any
// pre-existing log.
func TestValidateFilesystemPassesOnSameFilesystem(t *testing.T) {
	watch, preserve := watchAndPreserve(t)

	if err := ValidateFilesystem(watch, preserve); err != nil {
		t.Fatalf("same-filesystem validation should pass: %v", err)
	}
	if _, err := os.Stat(preserve); err != nil {
		t.Fatalf("preserve directory was not created: %v", err)
	}
	entries, err := os.ReadDir(preserve)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("probe files left behind in preserve dir: %v", entries)
	}
}

// TestValidateFilesystemFailsOnCrossFilesystem verifies the gate fails fast when
// the watch and preserve directories live on different filesystems (distinct
// st_dev), which is the misconfiguration the invariant exists to catch.
func TestValidateFilesystemFailsOnCrossFilesystem(t *testing.T) {
	watch, preserve := watchAndPreserve(t)

	orig := statDev
	t.Cleanup(func() { statDev = orig })
	statDev = func(path string) (uint64, error) {
		if path == watch {
			return 1, nil
		}
		return 2, nil
	}

	if err := ValidateFilesystem(watch, preserve); err == nil {
		t.Fatal("validation should fail fast when watch and preserve are on different filesystems")
	}
}

// TestValidateFilesystemFailsWhenHardlinkUnsupported verifies the gate fails
// fast when the filesystem shares st_dev but rejects hardlinks (e.g. EOPNOTSUPP)
// — the gap that the self-hardlink probe closes. Deterministic regardless of the
// test's uid.
func TestValidateFilesystemFailsWhenHardlinkUnsupported(t *testing.T) {
	watch, preserve := watchAndPreserve(t)

	orig := linkFile
	t.Cleanup(func() { linkFile = orig })
	linkFile = func(oldname, newname string) error {
		return &os.LinkError{Op: "link", Old: oldname, New: newname, Err: syscall.EOPNOTSUPP}
	}

	if err := ValidateFilesystem(watch, preserve); err == nil {
		t.Fatal("validation should fail fast when the filesystem does not support hardlinks")
	}
	// The failed probe must not leave a source file behind in the preserve dir.
	entries, err := os.ReadDir(preserve)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("probe files left behind after a failed hardlink: %v", entries)
	}
}

// TestValidateFilesystemFailsWhenPreserveDirUncreatable verifies the gate fails
// fast when the preserve directory cannot be created (its parent is a file).
func TestValidateFilesystemFailsWhenPreserveDirUncreatable(t *testing.T) {
	watch, _ := watchAndPreserve(t)

	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	preserve := filepath.Join(blocker, "preserved")

	if err := ValidateFilesystem(watch, preserve); err == nil {
		t.Fatal("validation should fail fast when the preserve dir cannot be created")
	}
}

// TestValidateFilesystemFailsWhenWatchDirMissing verifies the gate fails fast
// when the watch directory does not exist, rather than silently proceeding.
func TestValidateFilesystemFailsWhenWatchDirMissing(t *testing.T) {
	dir := t.TempDir()
	watch := filepath.Join(dir, "does-not-exist")
	preserve := filepath.Join(dir, "preserved")

	if err := ValidateFilesystem(watch, preserve); err == nil {
		t.Fatal("validation should fail fast when the watch dir does not exist")
	}
}
