package keeper

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AkashiSN/pod-log-preserver/internal/config"
	"github.com/AkashiSN/pod-log-preserver/internal/metrics"
)

// newCleanupKeeper builds a Keeper over a fresh preserve tree with the standard
// age thresholds: 5 min for plain orphans, 60 min for .gz orphans.
func newCleanupKeeper(t *testing.T) (*Keeper, string) {
	t.Helper()
	dir := t.TempDir()
	preserve := filepath.Join(dir, "preserved")
	if err := os.MkdirAll(preserve, 0o755); err != nil {
		t.Fatal(err)
	}
	k := &Keeper{
		cfg: config.Config{
			WatchDir:           filepath.Join(dir, "pods"),
			PreserveDir:        preserve,
			CleanupMaxAgeMin:   5,
			CleanupGzMaxAgeMin: 60,
		},
		m: &metrics.Metrics{},
	}
	return k, preserve
}

// inodeOfPath returns the inode number of the file at path.
func inodeOfPath(t *testing.T, path string) uint64 {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	ino, ok := inodeOf(fi)
	if !ok {
		t.Fatal("cannot read inode")
	}
	return ino
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// TestCleanupSkipsStillLinkedFile leaves a preserved file whose original still
// exists (Nlink > 1) in place, and reflects it in the gauges.
func TestCleanupSkipsStillLinkedFile(t *testing.T) {
	k, preserve := newCleanupKeeper(t)
	orig := filepath.Join(k.cfg.WatchDir, "0.log.20240101-000000")
	mkfile(t, orig, "hello")
	link := filepath.Join(preserve, "ns_pod_uid", "c", "0.log.20240101-000000")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(orig, link); err != nil {
		t.Fatal(err)
	}

	k.cleanupOrphans(nil, time.Now())

	if !exists(link) {
		t.Error("still-linked file was removed")
	}
	if got := k.m.PreservedFiles.Load(); got != 1 {
		t.Errorf("PreservedFiles = %d, want 1", got)
	}
	if got := k.m.OrphanedFiles.Load(); got != 0 {
		t.Errorf("OrphanedFiles = %d, want 0", got)
	}
	if got := k.m.OrphansRemoved.Load(); got != 0 {
		t.Errorf("OrphansRemoved = %d, want 0", got)
	}
}

// TestCleanupRemovesDBConfirmedOrphan removes an orphaned rotated log the
// moment a tail DB confirms fluent-bit read it fully, regardless of age.
func TestCleanupRemovesDBConfirmedOrphan(t *testing.T) {
	k, preserve := newCleanupKeeper(t)
	rel := filepath.Join("ns_pod_uid", "c", "0.log.20240101-000000")
	path := filepath.Join(preserve, rel)
	mkfile(t, path, "0123456789") // size 10, Nlink == 1 (orphan)
	ino := inodeOfPath(t, path)

	dbs := []map[uint64]dbEntry{
		{ino: {offset: 10, name: "/var/log/pods-preserved/" + filepath.ToSlash(rel)}},
	}
	// mtime is "now" so age-based cleanup would NOT fire; only DB confirms it.
	k.cleanupOrphans(dbs, time.Now())

	if exists(path) {
		t.Error("db-confirmed orphan was not removed")
	}
	if got := k.m.OrphansRemoved.Load(); got != 1 {
		t.Errorf("OrphansRemoved = %d, want 1", got)
	}
	if got := k.m.DBConfirmedRemoved.Load(); got != 1 {
		t.Errorf("DBConfirmedRemoved = %d, want 1", got)
	}
}

// TestCleanupKeepsUnconfirmedRecentOrphan keeps an orphan that no DB confirms
// and that is younger than the age threshold.
func TestCleanupKeepsUnconfirmedRecentOrphan(t *testing.T) {
	k, preserve := newCleanupKeeper(t)
	path := filepath.Join(preserve, "ns_pod_uid", "c", "0.log.20240101-000000")
	mkfile(t, path, "0123456789")
	ino := inodeOfPath(t, path)

	// offset below size => not confirmed.
	dbs := []map[uint64]dbEntry{
		{ino: {offset: 3, name: "/var/log/pods-preserved/ns_pod_uid/c/0.log.20240101-000000"}},
	}
	k.cleanupOrphans(dbs, time.Now())

	if !exists(path) {
		t.Error("unconfirmed recent orphan was removed")
	}
	if got := k.m.OrphanedFiles.Load(); got != 1 {
		t.Errorf("OrphanedFiles = %d, want 1", got)
	}
}

// TestCleanupAgeThresholdNonGz removes an unconfirmed plain rotated orphan once
// it is older than CLEANUP_MAX_AGE_MIN.
func TestCleanupAgeThresholdNonGz(t *testing.T) {
	k, preserve := newCleanupKeeper(t)
	path := filepath.Join(preserve, "ns_pod_uid", "c", "0.log.20240101-000000")
	mkfile(t, path, "data")

	now := time.Now()
	old := now.Add(-6 * time.Minute) // older than 5 min threshold
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}

	k.cleanupOrphans(nil, now)

	if exists(path) {
		t.Error("aged non-gz orphan was not removed")
	}
	if got := k.m.OrphansRemoved.Load(); got != 1 {
		t.Errorf("OrphansRemoved = %d, want 1", got)
	}
	if got := k.m.DBConfirmedRemoved.Load(); got != 0 {
		t.Errorf("DBConfirmedRemoved = %d, want 0 (age, not DB)", got)
	}
}

// TestCleanupAgeThresholdGz keeps a .gz orphan past the non-gz threshold but
// removes it past the longer gz threshold.
func TestCleanupAgeThresholdGz(t *testing.T) {
	now := time.Now()

	// 30 min old: past the 5-min non-gz threshold, under the 60-min gz one.
	k, preserve := newCleanupKeeper(t)
	gz := filepath.Join(preserve, "ns_pod_uid", "c", "0.log.20240101-000000.gz")
	mkfile(t, gz, "data")
	mid := now.Add(-30 * time.Minute)
	if err := os.Chtimes(gz, mid, mid); err != nil {
		t.Fatal(err)
	}
	k.cleanupOrphans(nil, now)
	if !exists(gz) {
		t.Error(".gz orphan removed before its longer threshold")
	}

	// 61 min old: past the gz threshold.
	k2, preserve2 := newCleanupKeeper(t)
	gz2 := filepath.Join(preserve2, "ns_pod_uid", "c", "0.log.20240101-000000.gz")
	mkfile(t, gz2, "data")
	oldGz := now.Add(-61 * time.Minute)
	if err := os.Chtimes(gz2, oldGz, oldGz); err != nil {
		t.Fatal(err)
	}
	k2.cleanupOrphans(nil, now)
	if exists(gz2) {
		t.Error("aged .gz orphan past gz threshold was not removed")
	}
}

// TestPruneEmptyDirs removes empty directories under the preserve tree while
// keeping populated ones and the preserve root itself.
func TestPruneEmptyDirs(t *testing.T) {
	k, preserve := newCleanupKeeper(t)
	emptyNested := filepath.Join(preserve, "ns_a", "c")
	if err := os.MkdirAll(emptyNested, 0o755); err != nil {
		t.Fatal(err)
	}
	full := filepath.Join(preserve, "ns_b", "c")
	mkfile(t, filepath.Join(full, "0.log.20240101-000000"), "x")

	k.pruneEmptyDirs()

	if exists(emptyNested) || exists(filepath.Join(preserve, "ns_a")) {
		t.Error("empty directories were not pruned")
	}
	if !exists(full) {
		t.Error("populated directory was pruned")
	}
	if !exists(preserve) {
		t.Error("preserve root was pruned")
	}
}
