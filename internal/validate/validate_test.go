package validate

import (
	"os"
	"path/filepath"
	"testing"
)

// ownLogTree lays out a watch directory containing this pod's own container log
// at <watch>/<ns>_<name>_<uid>/<container>/0.log and returns the watch and
// preserve paths plus the own-log path.
func ownLogTree(t *testing.T, ns, name, uid string) (watch, preserve, ownLog string) {
	t.Helper()
	dir := t.TempDir()
	watch = filepath.Join(dir, "pods")
	preserve = filepath.Join(dir, "preserved")
	podDir := filepath.Join(watch, ns+"_"+name+"_"+uid, "app")
	if err := os.MkdirAll(podDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ownLog = filepath.Join(podDir, "0.log")
	if err := os.WriteFile(ownLog, []byte("log line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return watch, preserve, ownLog
}

// TestValidateFilesystemSameFilesystemOwnLog verifies the gate passes when a
// hardlink of the pod's own container log into the preserve directory succeeds:
// it creates the preserve dir, reports the tested log, and leaves neither the
// source log nor a validation link behind.
func TestValidateFilesystemSameFilesystemOwnLog(t *testing.T) {
	ns, name, uid := "kube-system", "plp-abcde", "uid-1"
	watch, preserve, ownLog := ownLogTree(t, ns, name, uid)

	res, err := ValidateFilesystem(watch, preserve, ns, name, uid)
	if err != nil {
		t.Fatalf("same-filesystem validation should pass: %v", err)
	}
	if res.Skipped {
		t.Fatalf("validation should not be skipped when own log is present: %+v", res)
	}
	if res.TestedLog != ownLog {
		t.Errorf("TestedLog = %q, want %q", res.TestedLog, ownLog)
	}
	if _, err := os.Stat(preserve); err != nil {
		t.Fatalf("preserve directory was not created: %v", err)
	}
	if _, err := os.Stat(ownLog); err != nil {
		t.Fatalf("own log should be untouched: %v", err)
	}
	entries, err := os.ReadDir(preserve)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("validation link left behind in preserve dir: %v", entries)
	}
}

// TestValidateFilesystemSkipsWhenIdentityUnset verifies the gate warns and
// skips (no error) when the downward-API pod identity is not injected, so the
// pod's own log cannot be located.
func TestValidateFilesystemSkipsWhenIdentityUnset(t *testing.T) {
	dir := t.TempDir()
	watch := filepath.Join(dir, "pods")
	preserve := filepath.Join(dir, "preserved")
	if err := os.MkdirAll(watch, 0o755); err != nil {
		t.Fatal(err)
	}

	res, err := ValidateFilesystem(watch, preserve, "", "", "")
	if err != nil {
		t.Fatalf("missing identity should skip, not error: %v", err)
	}
	if !res.Skipped || res.Reason == "" {
		t.Fatalf("expected a skip with a reason, got %+v", res)
	}
	if _, err := os.Stat(preserve); err != nil {
		t.Fatalf("preserve directory should still be created on skip: %v", err)
	}
}

// TestValidateFilesystemSkipsWhenOwnLogMissing verifies the gate skips when the
// pod identity is set but no matching own container log exists yet under the
// watch directory.
func TestValidateFilesystemSkipsWhenOwnLogMissing(t *testing.T) {
	dir := t.TempDir()
	watch := filepath.Join(dir, "pods")
	preserve := filepath.Join(dir, "preserved")
	if err := os.MkdirAll(watch, 0o755); err != nil {
		t.Fatal(err)
	}

	res, err := ValidateFilesystem(watch, preserve, "kube-system", "plp-abcde", "uid-1")
	if err != nil {
		t.Fatalf("missing own log should skip, not error: %v", err)
	}
	if !res.Skipped || res.Reason == "" {
		t.Fatalf("expected a skip with a reason, got %+v", res)
	}
}

// TestValidateFilesystemFailsWhenPreserveDirUncreatable verifies the gate fails
// fast when the preserve directory cannot be created (its parent is a file).
func TestValidateFilesystemFailsWhenPreserveDirUncreatable(t *testing.T) {
	ns, name, uid := "kube-system", "plp-abcde", "uid-1"
	watch, _, _ := ownLogTree(t, ns, name, uid)

	// A regular file where the preserve dir's parent should be makes MkdirAll fail.
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	preserve := filepath.Join(blocker, "preserved")

	if _, err := ValidateFilesystem(watch, preserve, ns, name, uid); err == nil {
		t.Fatal("validation should fail fast when the preserve dir cannot be created")
	}
}

// TestValidateFilesystemFailsWhenHardlinkImpossible verifies the gate fails
// fast when the hardlink of the own log into the preserve directory is denied
// (simulated with a read-only preserve dir). Skipped under uid 0, which bypasses
// directory permissions.
func TestValidateFilesystemFailsWhenHardlinkImpossible(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory write permissions; cannot simulate link failure")
	}
	ns, name, uid := "kube-system", "plp-abcde", "uid-1"
	watch, preserve, _ := ownLogTree(t, ns, name, uid)

	// Pre-create the preserve dir read-only so MkdirAll is a no-op but os.Link
	// into it is denied.
	if err := os.MkdirAll(preserve, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(preserve, 0o755) })

	if _, err := ValidateFilesystem(watch, preserve, ns, name, uid); err == nil {
		t.Fatal("validation should fail fast when the hardlink test cannot create the link")
	}
}
