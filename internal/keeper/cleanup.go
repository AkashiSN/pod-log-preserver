package keeper

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/AkashiSN/pod-log-preserver/internal/logging"
)

// cleanupLoop runs a cleanup cycle on the given interval until ctx is
// cancelled (spec §5.2 step 5).
func (k *Keeper) cleanupLoop(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			k.runCleanupCycle()
		}
	}
}

// runCleanupCycle loads the tail DBs once, removes finished orphans, and prunes
// empty directories (spec §3.2–§3.3). Tail DBs are read read-only and any
// unreadable DB is skipped, so a cycle never fails.
func (k *Keeper) runCleanupCycle() {
	dbs := loadTailDBs(k.cfg.PreservedLogDBGlob, k.m)
	k.cleanupOrphans(dbs, time.Now())
	k.pruneEmptyDirs()
}

// cleanupOrphans walks the preserve tree once, deciding each regular file's
// fate and refreshing the gauges (spec §3.2–§3.3):
//
//   - Nlink > 1: the original still exists; keep it.
//   - Nlink == 1 (orphan): remove it immediately if a tail DB confirms a full
//     read, otherwise once it is older than its age threshold; keep it while it
//     is still unread and young.
//
// The preserved-files, preserved-bytes, and orphaned-files gauges are set to
// the post-cycle counts (files that survive this pass).
func (k *Keeper) cleanupOrphans(dbs []map[uint64][]dbEntry, now time.Time) {
	var files, bytes, orphans int64
	_ = filepath.WalkDir(k.cfg.PreserveDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil // skip unreadable entries and directories, keep walking
		}
		info, statErr := os.Lstat(path)
		if statErr != nil || !info.Mode().IsRegular() {
			return nil
		}
		if stillLinked(info) {
			files++
			bytes += info.Size()
			return nil
		}
		if k.tryRemoveOrphan(path, info, dbs, now) {
			return nil
		}
		files++
		bytes += info.Size()
		orphans++
		return nil
	})
	k.m.PreservedFiles.Store(files)
	k.m.PreservedBytes.Store(bytes)
	k.m.OrphanedFiles.Store(orphans)
}

// tryRemoveOrphan removes an orphaned preserved file when it is finished, and
// reports whether it did. Any orphan is removed the instant a tail DB confirms
// fluent-bit read it fully (confirmation-first) and is otherwise removed once
// older than its age threshold (age-second). A file no DB has finished and that
// is still young is kept.
//
// The DB check is attempted for every orphan, not just rotated/compressed
// names: an active log is preserved as a snapshot (0.log.<UnixNano>) whose only
// preserved link survives kubelet's rotation via inode dedup, so it too must be
// confirmable. The DB predicate anchors by preserve-relative path and requires
// offset >= size, so a name that is not tailed simply falls through to age.
func (k *Keeper) tryRemoveOrphan(path string, info os.FileInfo, dbs []map[uint64][]dbEntry, now time.Time) bool {
	if ino, ok := inodeOf(info); ok {
		if rel, err := filepath.Rel(k.cfg.PreserveDir, path); err == nil {
			if dbConfirmedConsumed(dbs, ino, filepath.ToSlash(rel), info.Size()) {
				if k.removeOrphan(path) {
					k.m.DBConfirmedRemoved.Add(1)
					return true
				}
				return false
			}
		}
	}
	if now.Sub(info.ModTime()) > k.maxAgeForPath(path) {
		return k.removeOrphan(path)
	}
	return false
}

// removeOrphan deletes a preserved orphan, counting the removal on success. A
// failed removal is logged and never fatal.
func (k *Keeper) removeOrphan(path string) bool {
	if err := os.Remove(path); err != nil {
		logging.Warn("remove orphan %s: %v", path, err)
		return false
	}
	k.m.OrphansRemoved.Add(1)
	return true
}

// pruneEmptyDirs removes empty directories under the preserve tree, deepest
// first so a directory emptied by pruning its children is itself removed. The
// preserve root is never removed. Removal errors are ignored (spec §3.3).
func (k *Keeper) pruneEmptyDirs() {
	var dirs []string
	_ = filepath.WalkDir(k.cfg.PreserveDir, func(path string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() && path != k.cfg.PreserveDir {
			dirs = append(dirs, path)
		}
		return nil
	})
	// Longer paths are deeper, so removing in descending length order visits a
	// child before its parent.
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, dir := range dirs {
		if entries, err := os.ReadDir(dir); err == nil && len(entries) == 0 {
			_ = os.Remove(dir)
		}
	}
}

// stillLinked reports whether the file described by fi still has its original
// hardlink (Nlink > 1) and so is not an orphan. When the stat cannot be read it
// returns true, so an indeterminate file is never treated as a removable
// orphan. The comparison avoids naming Stat_t.Nlink's platform-dependent type
// (uint64 on amd64, uint32 on arm64).
func stillLinked(fi os.FileInfo) bool {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return true
	}
	return st.Nlink > 1
}
