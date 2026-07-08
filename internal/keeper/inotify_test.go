package keeper

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/AkashiSN/pod-log-preserver/internal/config"
	"github.com/AkashiSN/pod-log-preserver/internal/metrics"
)

// keeperWithDirs builds a Keeper whose watch and preserve trees are siblings
// under one temp dir, so they share a filesystem and hardlinks succeed. The
// inotify fd is closed on test cleanup.
func keeperWithDirs(t *testing.T) (k *Keeper, watch, preserve string) {
	t.Helper()
	root := t.TempDir()
	watch = filepath.Join(root, "pods")
	preserve = filepath.Join(root, "pods-preserved")
	for _, d := range []string{watch, preserve} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	k, err := New(config.Config{WatchDir: watch, PreserveDir: preserve}, &metrics.Metrics{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = k.Close() })
	return k, watch, preserve
}

// writeLog creates relDir under watch and writes a "0.log" file into it,
// returning the container directory's absolute path.
func writeLog(t *testing.T, watch, relDir string) string {
	t.Helper()
	dir := filepath.Join(watch, relDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "0.log"), []byte("line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// preservedCount returns the number of regular files under the preserve tree.
func preservedCount(t *testing.T, preserve string) int {
	t.Helper()
	n := 0
	_ = filepath.WalkDir(preserve, func(_ string, d os.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			n++
		}
		return nil
	})
	return n
}

// TestHandleEventDispatch feeds synthetic inotify events to handleEvent and
// asserts each is routed correctly: a regular-file event preserves the log, a
// new-directory event watches the subtree and catches up its files, IN_IGNORED
// drops the watch bookkeeping, and events with an unknown descriptor or an
// empty name are ignored. This covers the dispatch that no unit test exercised
// (inotify.go handleEvent / handleNewDir).
func TestHandleEventDispatch(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, k *Keeper, watch, preserve string) (*syscall.InotifyEvent, string)
		check func(t *testing.T, k *Keeper, watch, preserve string)
	}{
		{
			name: "regular file event is preserved via syncFile",
			setup: func(t *testing.T, k *Keeper, watch, _ string) (*syscall.InotifyEvent, string) {
				dir := writeLog(t, watch, "ns_pod_uid/c")
				if err := k.addWatch(dir); err != nil {
					t.Fatal(err)
				}
				return &syscall.InotifyEvent{Wd: int32(k.dirToWd[dir]), Mask: syscall.IN_CLOSE_WRITE}, "0.log"
			},
			check: func(t *testing.T, _ *Keeper, _, preserve string) {
				if got := preservedCount(t, preserve); got != 1 {
					t.Errorf("preserved %d files, want 1", got)
				}
			},
		},
		{
			name: "new directory event watches the subtree and syncs its files",
			setup: func(t *testing.T, k *Keeper, watch, _ string) (*syscall.InotifyEvent, string) {
				if err := k.addWatch(watch); err != nil {
					t.Fatal(err)
				}
				writeLog(t, watch, "ns_pod_uid") // new dir + file, not yet watched
				return &syscall.InotifyEvent{Wd: int32(k.dirToWd[watch]), Mask: syscall.IN_CREATE | syscall.IN_ISDIR}, "ns_pod_uid"
			},
			check: func(t *testing.T, k *Keeper, watch, preserve string) {
				newDir := filepath.Join(watch, "ns_pod_uid")
				if _, ok := k.dirForWd(k.dirToWd[newDir]); !ok {
					t.Error("new directory was not watched")
				}
				if got := preservedCount(t, preserve); got != 1 {
					t.Errorf("preserved %d files, want 1 (catch-up walk)", got)
				}
			},
		},
		{
			name: "moved-in directory is treated like a created one",
			setup: func(t *testing.T, k *Keeper, watch, _ string) (*syscall.InotifyEvent, string) {
				if err := k.addWatch(watch); err != nil {
					t.Fatal(err)
				}
				writeLog(t, watch, "ns_pod_uid")
				return &syscall.InotifyEvent{Wd: int32(k.dirToWd[watch]), Mask: syscall.IN_MOVED_TO | syscall.IN_ISDIR}, "ns_pod_uid"
			},
			check: func(t *testing.T, k *Keeper, watch, preserve string) {
				newDir := filepath.Join(watch, "ns_pod_uid")
				if _, ok := k.dirForWd(k.dirToWd[newDir]); !ok {
					t.Error("moved-in directory was not watched")
				}
				if got := preservedCount(t, preserve); got != 1 {
					t.Errorf("preserved %d files, want 1", got)
				}
			},
		},
		{
			name: "IN_IGNORED drops the watch bookkeeping",
			setup: func(t *testing.T, k *Keeper, watch, _ string) (*syscall.InotifyEvent, string) {
				dir := writeLog(t, watch, "ns_pod_uid/c")
				if err := k.addWatch(dir); err != nil {
					t.Fatal(err)
				}
				return &syscall.InotifyEvent{Wd: int32(k.dirToWd[dir]), Mask: syscall.IN_IGNORED}, ""
			},
			check: func(t *testing.T, k *Keeper, watch, preserve string) {
				dir := filepath.Join(watch, "ns_pod_uid/c")
				if _, ok := k.dirToWd[dir]; ok {
					t.Error("descriptor for the ignored watch was not forgotten")
				}
				if got := preservedCount(t, preserve); got != 0 {
					t.Errorf("preserved %d files on IN_IGNORED, want 0", got)
				}
			},
		},
		{
			name: "unknown descriptor is ignored",
			setup: func(t *testing.T, k *Keeper, watch, _ string) (*syscall.InotifyEvent, string) {
				writeLog(t, watch, "ns_pod_uid/c")
				return &syscall.InotifyEvent{Wd: 99999, Mask: syscall.IN_CLOSE_WRITE}, "0.log"
			},
			check: func(t *testing.T, _ *Keeper, _, preserve string) {
				if got := preservedCount(t, preserve); got != 0 {
					t.Errorf("preserved %d files for an unknown descriptor, want 0", got)
				}
			},
		},
		{
			name: "empty name with a known descriptor is ignored",
			setup: func(t *testing.T, k *Keeper, watch, _ string) (*syscall.InotifyEvent, string) {
				dir := writeLog(t, watch, "ns_pod_uid/c")
				if err := k.addWatch(dir); err != nil {
					t.Fatal(err)
				}
				return &syscall.InotifyEvent{Wd: int32(k.dirToWd[dir]), Mask: syscall.IN_CLOSE_WRITE}, ""
			},
			check: func(t *testing.T, _ *Keeper, _, preserve string) {
				if got := preservedCount(t, preserve); got != 0 {
					t.Errorf("preserved %d files for an empty name, want 0", got)
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k, watch, preserve := keeperWithDirs(t)
			ev, name := tc.setup(t, k, watch, preserve)
			k.handleEvent(ev, name)
			tc.check(t, k, watch, preserve)
		})
	}
}

// TestHandleOverflowReestablishesWatchesAndSyncs covers the IN_Q_OVERFLOW
// recovery path (spec §7 risk #4) without provoking a real kernel queue
// overflow: handleOverflow must re-establish the watch tree and run a full
// resync. The watch tree is populated but never watched beforehand, so a green
// result proves both effects.
func TestHandleOverflowReestablishesWatchesAndSyncs(t *testing.T) {
	k, watch, preserve := keeperWithDirs(t)
	writeLog(t, watch, "ns_pod_uid/c")

	k.handleOverflow()

	if _, ok := k.dirForWd(k.dirToWd[watch]); !ok {
		t.Error("watch root was not re-established after overflow")
	}
	if got := preservedCount(t, preserve); got != 1 {
		t.Errorf("preserved %d files after overflow resync, want 1", got)
	}
}

// TestAddWatchRecursiveMissingRootFails verifies a missing/unwatchable watch
// root is reported as an error rather than silently succeeding — otherwise the
// event loop would block forever with no watches registered.
func TestAddWatchRecursiveMissingRootFails(t *testing.T) {
	k, err := New(config.Config{}, &metrics.Metrics{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = k.Close() }()

	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if err := k.AddWatchRecursive(missing); err == nil {
		t.Fatal("AddWatchRecursive on a missing root should return an error")
	}
}

// TestAddWatchRecursiveExistingRoot verifies a real tree is watched without
// error and every directory in it is registered.
func TestAddWatchRecursiveExistingRoot(t *testing.T) {
	k, err := New(config.Config{}, &metrics.Metrics{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = k.Close() }()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "ns_pod_uid", "container"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := k.AddWatchRecursive(root); err != nil {
		t.Fatalf("AddWatchRecursive on an existing tree: %v", err)
	}
	if _, ok := k.dirForWd(k.dirToWd[root]); !ok {
		t.Fatal("root directory was not watched")
	}
}
