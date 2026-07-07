package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

// Kubelet pod-log filename patterns, matched against a file's base name. The
// active log is `<restart>.log`; kubelet rotates it to `<restart>.log.<ts>`
// (timestamp `20060102-150405`) and optionally compresses to `.gz`.
var (
	activeLogRe     = regexp.MustCompile(`^\d+\.log$`)
	rotatedLogRe    = regexp.MustCompile(`^\d+\.log\.\d{8}-\d{6}$`)
	compressedLogRe = regexp.MustCompile(`^\d+\.log\.\d{8}-\d{6}\.gz$`)
)

// shouldProcess reports whether a file at the given watch-relative path passes
// the namespace filter. The path's first segment is `<ns>_<pod>_<uid>`; the
// namespace is the part before its first underscore, matched (glob) against any
// configured filter pattern. An empty filter admits every namespace.
func (k *Keeper) shouldProcess(rel string) bool {
	if len(k.cfg.NamespaceFilter) == 0 {
		return true
	}
	firstSeg := rel
	if i := strings.IndexByte(rel, filepath.Separator); i >= 0 {
		firstSeg = rel[:i]
	}
	ns := firstSeg
	if i := strings.IndexByte(firstSeg, '_'); i >= 0 {
		ns = firstSeg[:i]
	}
	for _, pat := range k.cfg.NamespaceFilter {
		if ok, err := filepath.Match(pat, ns); err == nil && ok {
			return true
		}
	}
	return false
}

// syncFile preserves a single watch-tree path if it is a kubelet log this
// daemon handles and its namespace passes the filter. It mirrors the source's
// watch-relative directory under the preserve tree: active logs are linked with
// a timestamp suffix, rotated/compressed logs keep their name. Non-log files
// and filtered namespaces are ignored. Per-file errors are logged, never fatal.
func (k *Keeper) syncFile(path string) {
	rel, err := filepath.Rel(k.cfg.WatchDir, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return
	}
	if !k.shouldProcess(rel) {
		return
	}
	name := filepath.Base(path)
	dstDir := filepath.Join(k.cfg.PreserveDir, filepath.Dir(rel))

	var addTS bool
	switch {
	case activeLogRe.MatchString(name):
		addTS = true
	case rotatedLogRe.MatchString(name), compressedLogRe.MatchString(name):
		addTS = false
	default:
		return // not a log we preserve
	}

	if _, err := k.createHardlink(path, dstDir, name, addTS); err != nil {
		logWarn("hardlink %s: %v", path, err)
	}
}

// walkAndSync walks root and calls syncFile on every regular file, preserving
// all matching logs it finds. Walk errors on individual entries are skipped.
func (k *Keeper) walkAndSync(root string) {
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries, keep walking
		}
		if d.Type().IsRegular() {
			k.syncFile(path)
		}
		return nil
	})
}

// initialSync preserves every matching log already present in the watch tree
// at startup, before inotify watches are established (spec §5.2 step 3).
func (k *Keeper) initialSync() {
	k.walkAndSync(k.cfg.WatchDir)
}

// periodicResync re-walks the watch tree on the given interval, catching any
// file inotify missed (e.g. after a queue overflow). It returns when ctx is
// cancelled.
func (k *Keeper) periodicResync(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			k.walkAndSync(k.cfg.WatchDir)
		}
	}
}

// maxAgeForPath returns the cleanup age threshold for a preserved path:
// compressed (.gz) logs are kept longer than plain rotated logs.
func (k *Keeper) maxAgeForPath(path string) time.Duration {
	if strings.HasSuffix(path, ".gz") {
		return time.Duration(k.cfg.CleanupGzMaxAgeMin) * time.Minute
	}
	return time.Duration(k.cfg.CleanupMaxAgeMin) * time.Minute
}

// inodeOf returns the inode number of the file at path.
func inodeOf(fi os.FileInfo) (uint64, bool) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return st.Ino, true
}

// createHardlink links src into dstDir. dstName is the base destination file
// name; when addTS is true (active logs), a `.<mtime-unixnano>` suffix is
// appended so successive incarnations of the same active file name don't
// collide. It returns false (without error) when the link is skipped because
//
//   - the destination directory already holds a hardlink to src's inode, or
//   - a file already occupies the destination name.
//
// On a created link it increments m.hardlinksCreated.
func (k *Keeper) createHardlink(src, dstDir, dstName string, addTS bool) (bool, error) {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return false, err
	}
	srcIno, ok := inodeOf(srcInfo)
	if !ok {
		return false, fmt.Errorf("cannot read inode of %s", src)
	}

	// Inode dedup: skip if any entry in dstDir is already the same inode.
	if entries, err := os.ReadDir(dstDir); err == nil {
		for _, e := range entries {
			info, err := e.Info()
			if err != nil {
				continue
			}
			if ino, ok := inodeOf(info); ok && ino == srcIno {
				return false, nil
			}
		}
	}

	if addTS {
		dstName = fmt.Sprintf("%s.%d", dstName, srcInfo.ModTime().UnixNano())
	}
	dst := filepath.Join(dstDir, dstName)

	// Name-collision guard: never overwrite an existing destination.
	if _, err := os.Lstat(dst); err == nil {
		return false, nil
	}

	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return false, err
	}
	if err := os.Link(src, dst); err != nil {
		return false, err
	}
	k.m.hardlinksCreated.Add(1)
	return true, nil
}
