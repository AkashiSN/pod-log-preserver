//go:build e2e

package container

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// orphan creates a preserved-but-orphaned file: write the active log, wait for
// it to be preserved, then rotate + delete the original so only the preserved
// hardlink remains (Nlink == 1). Returns the preserved path.
func makeOrphan(t *testing.T, work, key, ctr, content string) string {
	t.Helper()
	src := podLogDir(work, key, ctr)
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(src, "0.log")
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	preserved := findPreserved(t, preservedDir(work, key, ctr))
	// Simulate kubelet rotation: rename active -> rotated, then delete it so the
	// preserved copy is the sole remaining link.
	rotated := logPath + ".20260101-000000"
	if err := os.Rename(logPath, rotated); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(rotated); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 10*time.Second, "preserved file becomes an orphan (Nlink==1)", func() bool {
		return nlink(t, preserved) == 1
	})
	return preserved
}

// TestConfirmationFirstDelete: fluent-bit reads a preserved orphan fully, so plp
// deletes it immediately (confirmation-first) and the db-confirmed counter ticks.
func TestConfirmationFirstDelete(t *testing.T) {
	work := newWorkDir(t)
	startFluentBit(t, work)
	plp := startPLP(t, work, nil)

	const key, ctr = "ns2_pod2_uid2", "app"
	preserved := makeOrphan(t, work, key, ctr, "line-a\nline-b\nline-c\n")

	waitFor(t, 30*time.Second, "confirmed orphan is deleted", func() bool {
		_, err := os.Stat(preserved)
		return os.IsNotExist(err)
	})
	if got := readCounter(t, plp.MetricsURL, "pod_log_preserver_db_confirmed_removed_total"); got < 1 {
		t.Fatalf("db_confirmed_removed_total = %v, want >= 1", got)
	}
}

// TestAgeFallbackDelete: with DB-aware cleanup disabled (empty glob), an orphan
// survives while young, then is deleted once its mtime is older than the age
// threshold (age fallback), ticking the orphans-removed counter.
func TestAgeFallbackDelete(t *testing.T) {
	work := newWorkDir(t)
	plp := startPLP(t, work, map[string]string{
		"PRESERVED_LOG_DB_GLOB": "/work/none/*.db", // no DB -> pure age path
	})

	const key, ctr = "ns3_pod3_uid3", "app"
	preserved := makeOrphan(t, work, key, ctr, "aged\n")

	// Young + unconfirmed: must still be present after a couple of cleanup cycles.
	time.Sleep(3 * time.Second)
	if _, err := os.Stat(preserved); err != nil {
		t.Fatalf("young unconfirmed orphan was deleted early: %v", err)
	}
	// Backdate mtime past CLEANUP_MAX_AGE_MIN (1 min) -> age fallback deletes it.
	old := time.Now().Add(-2 * time.Minute)
	if err := os.Chtimes(preserved, old, old); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 15*time.Second, "aged orphan is deleted", func() bool {
		_, err := os.Stat(preserved)
		return os.IsNotExist(err)
	})
	if got := readCounter(t, plp.MetricsURL, "pod_log_preserver_orphans_removed_total"); got < 1 {
		t.Fatalf("orphans_removed_total = %v, want >= 1", got)
	}
}

// TestReadOnlyTailDB: plp must not modify fluent-bit's recorded offset for a
// still-live file across a cleanup cycle (the mode=ro invariant, observed).
func TestReadOnlyTailDB(t *testing.T) {
	work := newWorkDir(t)
	startFluentBit(t, work)
	startPLP(t, work, nil)

	const key, ctr = "ns4_pod4_uid4", "app"
	src := podLogDir(work, key, ctr)
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(src, "0.log")
	if err := os.WriteFile(logPath, []byte("live-1\nlive-2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	preserved := findPreserved(t, preservedDir(work, key, ctr)) // live, Nlink==2

	dbPath := filepath.Join(work, "flb", "flb_kube.db")
	var before int64
	waitFor(t, 20*time.Second, "fluent-bit records an offset for the live file", func() bool {
		before = readOffset(t, dbPath, preserved)
		return before > 0
	})
	// Let several plp cleanup cycles run.
	time.Sleep(4 * time.Second)
	after := readOffset(t, dbPath, preserved)
	if after != before {
		t.Fatalf("fluent-bit recorded offset changed under plp: before=%d after=%d", before, after)
	}
}

// readOffset opens fluent-bit's tail DB read-only and returns the recorded
// offset for the row whose name ends with the base name of preserved (0 if none).
func readOffset(t *testing.T, dbPath, preserved string) int64 {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		return 0
	}
	defer db.Close()
	base := filepath.Base(preserved)
	var offset int64
	row := db.QueryRow(
		"SELECT offset FROM in_tail_files WHERE name LIKE ? ORDER BY offset DESC LIMIT 1",
		fmt.Sprintf("%%/%s", base),
	)
	if err := row.Scan(&offset); err != nil {
		return 0
	}
	return offset
}
