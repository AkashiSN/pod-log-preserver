package keeper

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/AkashiSN/pod-log-preserver/internal/config"
	"github.com/AkashiSN/pod-log-preserver/internal/metrics"
)

// mkfile writes content to path, creating parent directories as needed.
func mkfile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestPeriodicResyncPreservesNewFilesAndStops verifies the resync loop picks up
// a file that appears after startup and returns when its context is cancelled.
func TestPeriodicResyncPreservesNewFiles(t *testing.T) {
	dir := t.TempDir()
	watch := filepath.Join(dir, "pods")
	preserve := filepath.Join(dir, "preserved")
	if err := os.MkdirAll(watch, 0o755); err != nil {
		t.Fatal(err)
	}
	k := &Keeper{cfg: config.Config{WatchDir: watch, PreserveDir: preserve}, m: &metrics.Metrics{}}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		k.periodicResync(ctx, 5*time.Millisecond)
		close(done)
	}()

	// File appears after the loop is already running.
	src := filepath.Join(watch, "ns_p_u", "c", "0.log.20231001-120000")
	mkfile(t, src, "late")

	dst := filepath.Join(preserve, "ns_p_u", "c", "0.log.20231001-120000")
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(dst); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("resync did not preserve the new file within 2s")
		}
		time.Sleep(5 * time.Millisecond)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("periodicResync did not return after context cancel")
	}
}

// TestMaxAgeForPath verifies compressed logs get the longer .gz threshold and
// everything else the plain threshold.
func TestMaxAgeForPath(t *testing.T) {
	k := &Keeper{cfg: config.Config{CleanupMaxAgeMin: 5, CleanupGzMaxAgeMin: 60}, m: &metrics.Metrics{}}
	if got := k.maxAgeForPath("/p/ns_pod_uid/c/0.log.20231001-120000"); got != 5*time.Minute {
		t.Fatalf("rotated maxAge = %v, want 5m", got)
	}
	if got := k.maxAgeForPath("/p/ns_pod_uid/c/0.log.20231001-120000.gz"); got != 60*time.Minute {
		t.Fatalf("gz maxAge = %v, want 60m", got)
	}
}

// TestInitialSync verifies a full walk of the watch tree preserves every
// matching log found, across namespaces.
func TestInitialSync(t *testing.T) {
	dir := t.TempDir()
	watch := filepath.Join(dir, "pods")
	preserve := filepath.Join(dir, "preserved")
	k := &Keeper{cfg: config.Config{WatchDir: watch, PreserveDir: preserve}, m: &metrics.Metrics{}}

	mkfile(t, filepath.Join(watch, "a_p1_u1", "c", "0.log"), "a")
	mkfile(t, filepath.Join(watch, "b_p2_u2", "c", "0.log.20231001-120000"), "b")
	mkfile(t, filepath.Join(watch, "b_p2_u2", "c", "0.log.20231001-120000.gz"), "bz")
	mkfile(t, filepath.Join(watch, "b_p2_u2", "c", "ignore.me"), "n")

	k.initialSync()

	if n := k.m.HardlinksCreated.Load(); n != 3 {
		t.Fatalf("hardlinksCreated = %d, want 3", n)
	}
	if _, err := os.Stat(filepath.Join(preserve, "b_p2_u2", "c", "0.log.20231001-120000.gz")); err != nil {
		t.Fatalf("compressed log not preserved: %v", err)
	}
}

// TestSyncFilePatternRouting verifies syncFile hardlinks each kubelet log
// pattern to the mirrored preserve path — active logs with a timestamp suffix,
// rotated/compressed logs under their own name — and ignores non-log files.
func TestSyncFilePatternRouting(t *testing.T) {
	dir := t.TempDir()
	watch := filepath.Join(dir, "pods")
	preserve := filepath.Join(dir, "preserved")
	relDir := filepath.Join("ns_pod_uid", "container")
	k := &Keeper{cfg: config.Config{WatchDir: watch, PreserveDir: preserve}, m: &metrics.Metrics{}}

	active := filepath.Join(watch, relDir, "0.log")
	rotated := filepath.Join(watch, relDir, "0.log.20231001-120000")
	compressed := filepath.Join(watch, relDir, "0.log.20231001-120000.gz")
	nonLog := filepath.Join(watch, relDir, "notes.txt")
	for _, p := range []string{active, rotated, compressed, nonLog} {
		mkfile(t, p, "x")
	}

	for _, p := range []string{active, rotated, compressed, nonLog} {
		k.syncFile(p)
	}

	dstDir := filepath.Join(preserve, relDir)
	names := map[string]bool{}
	entries, err := os.ReadDir(dstDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		names[e.Name()] = true
	}
	if !regexp.MustCompile(`^0\.log\.\d+$`).MatchString(activeDstName(t, dstDir)) {
		t.Fatalf("active log not linked with timestamp suffix; entries: %v", names)
	}
	if !names["0.log.20231001-120000"] {
		t.Fatalf("rotated log not preserved under its own name; entries: %v", names)
	}
	if !names["0.log.20231001-120000.gz"] {
		t.Fatalf("compressed log not preserved under its own name; entries: %v", names)
	}
	if names["notes.txt"] {
		t.Fatalf("non-log file was preserved; entries: %v", names)
	}
	if len(entries) != 3 {
		t.Fatalf("want 3 preserved entries, got %d: %v", len(entries), names)
	}
}

// TestSyncFileNamespaceFilterSkips verifies syncFile honors the namespace
// filter and preserves nothing for a filtered-out namespace.
func TestSyncFileNamespaceFilterSkips(t *testing.T) {
	dir := t.TempDir()
	watch := filepath.Join(dir, "pods")
	preserve := filepath.Join(dir, "preserved")
	k := &Keeper{cfg: config.Config{WatchDir: watch, PreserveDir: preserve, NamespaceFilter: []string{"team-*"}}, m: &metrics.Metrics{}}

	src := filepath.Join(watch, "other_pod_uid", "container", "0.log")
	mkfile(t, src, "x")
	k.syncFile(src)

	if _, err := os.Stat(preserve); !os.IsNotExist(err) {
		t.Fatalf("filtered namespace should preserve nothing, but preserve dir exists (err=%v)", err)
	}
	if n := k.m.HardlinksCreated.Load(); n != 0 {
		t.Fatalf("hardlinksCreated = %d, want 0", n)
	}
}

// activeDstName returns the sole timestamped active-log entry name in dstDir,
// or "" if none.
func activeDstName(t *testing.T, dstDir string) string {
	t.Helper()
	entries, err := os.ReadDir(dstDir)
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`^0\.log\.\d+$`)
	for _, e := range entries {
		if re.MatchString(e.Name()) {
			return e.Name()
		}
	}
	return ""
}

// TestShouldProcessNamespaceFilter verifies the namespace glob filter is
// matched against the namespace segment (the part before the first underscore
// of the `<ns>_<pod>_<uid>` directory). An empty filter admits everything.
func TestShouldProcessNamespaceFilter(t *testing.T) {
	cases := []struct {
		name   string
		filter []string
		rel    string
		want   bool
	}{
		{"no filter admits all", nil, "kube-system_pod_uid/c/0.log", true},
		{"glob match", []string{"team-*"}, "team-a_pod_uid/c/0.log", true},
		{"glob non-match", []string{"team-*"}, "other_pod_uid/c/0.log", false},
		{"exact match", []string{"prod"}, "prod_pod_uid/c/0.log", true},
		{"one of several", []string{"a", "team-*"}, "team-b_pod_uid/c/0.log", true},
		{"none of several", []string{"a", "team-*"}, "prod_pod_uid/c/0.log", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k := &Keeper{cfg: config.Config{NamespaceFilter: tc.filter}, m: &metrics.Metrics{}}
			if got := k.shouldProcess(tc.rel); got != tc.want {
				t.Fatalf("shouldProcess(%q) with filter %v = %v, want %v", tc.rel, tc.filter, got, tc.want)
			}
		})
	}
}

// TestCreateHardlinkDedupSameInode verifies the inode check: a second link of a
// file whose inode is already present in the destination directory is skipped,
// independent of the destination name.
func TestCreateHardlinkDedupSameInode(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src", "0.log")
	mkfile(t, src, "hello")
	dstDir := filepath.Join(dir, "dst")
	k := &Keeper{cfg: config.Config{}, m: &metrics.Metrics{}}

	created, err := k.createHardlink(src, dstDir, "0.log", true)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("first link should be created")
	}

	// Change mtime so a fresh timestamp suffix would differ; the inode check
	// must still skip because src's inode is already linked in dstDir.
	older := time.Now().Add(-time.Hour)
	if err := os.Chtimes(src, older, older); err != nil {
		t.Fatal(err)
	}
	created2, err := k.createHardlink(src, dstDir, "0.log", true)
	if err != nil {
		t.Fatal(err)
	}
	if created2 {
		t.Fatal("second link of an already-linked inode should be skipped")
	}

	entries, err := os.ReadDir(dstDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry in dstDir, got %d", len(entries))
	}
	if got := k.m.HardlinksCreated.Load(); got != 1 {
		t.Fatalf("hardlinksCreated = %d, want 1", got)
	}
}

// TestCreateHardlinkNameCollision verifies a pre-existing destination name that
// belongs to a different inode is never overwritten.
func TestCreateHardlinkNameCollision(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src", "0.log")
	mkfile(t, src, "source-bytes")
	dstDir := filepath.Join(dir, "dst")
	name := "0.log.20231001-120000"
	mkfile(t, filepath.Join(dstDir, name), "OTHER")
	k := &Keeper{cfg: config.Config{}, m: &metrics.Metrics{}}

	created, err := k.createHardlink(src, dstDir, name, false)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("link should be skipped on name collision with a different inode")
	}

	got, err := os.ReadFile(filepath.Join(dstDir, name))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "OTHER" {
		t.Fatalf("collision target was overwritten: %q", got)
	}
	if n := k.m.HardlinksCreated.Load(); n != 0 {
		t.Fatalf("hardlinksCreated = %d, want 0", n)
	}
}

// TestCreateHardlinkTimestampSuffix verifies active logs get a timestamp suffix
// so two different inodes sharing the base name "0.log" produce two distinct
// preserved files.
func TestCreateHardlinkTimestampSuffix(t *testing.T) {
	dir := t.TempDir()
	dstDir := filepath.Join(dir, "dst")
	k := &Keeper{cfg: config.Config{}, m: &metrics.Metrics{}}

	srcA := filepath.Join(dir, "a", "0.log")
	mkfile(t, srcA, "a")
	tA := time.Unix(1_700_000_000, 0)
	if err := os.Chtimes(srcA, tA, tA); err != nil {
		t.Fatal(err)
	}
	srcB := filepath.Join(dir, "b", "0.log")
	mkfile(t, srcB, "b")
	tB := time.Unix(1_700_000_123, 0)
	if err := os.Chtimes(srcB, tB, tB); err != nil {
		t.Fatal(err)
	}

	if _, err := k.createHardlink(srcA, dstDir, "0.log", true); err != nil {
		t.Fatal(err)
	}
	if _, err := k.createHardlink(srcB, dstDir, "0.log", true); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dstDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 timestamped entries, got %d", len(entries))
	}
	re := regexp.MustCompile(`^0\.log\.\d+$`)
	for _, e := range entries {
		if !re.MatchString(e.Name()) {
			t.Fatalf("unexpected destination name %q", e.Name())
		}
	}
	if n := k.m.HardlinksCreated.Load(); n != 2 {
		t.Fatalf("hardlinksCreated = %d, want 2", n)
	}
}
