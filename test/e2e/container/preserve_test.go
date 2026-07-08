//go:build e2e

package container

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestPreserveOnCreate: a new pod log is hardlinked into the preserve tree
// sharing its inode, and the hardlink counter increments.
func TestPreserveOnCreate(t *testing.T) {
	work := newWorkDir(t)
	plp := startPLP(t, work, nil)

	const key, ctr = "ns1_pod1_uid1", "app"
	src := podLogDir(work, key, ctr)
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(src, "0.log")
	if err := os.WriteFile(logPath, []byte("hello e2e\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	preserved := findPreserved(t, preservedDir(work, key, ctr))
	if !sameInode(t, logPath, preserved) {
		t.Fatalf("preserved %s is not a hardlink of %s", preserved, logPath)
	}
	if n := nlink(t, logPath); n < 2 {
		t.Fatalf("Nlink = %d, want >= 2 (a hardlink should exist)", n)
	}
	waitFor(t, 10*time.Second, "hardlinks_created_total >= 1", func() bool {
		return readCounter(t, plp.MetricsURL, "pod_log_preserver_hardlinks_created_total") >= 1
	})
}
