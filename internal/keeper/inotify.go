package keeper

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/AkashiSN/pod-log-preserver/internal/logging"
)

// watchMask is the set of inotify events each directory watch subscribes to.
// IN_CREATE catches new files/subdirectories, IN_CLOSE_WRITE catches a writer
// finishing an active log, and IN_MOVED_TO catches kubelet's rotation rename.
const watchMask = syscall.IN_CREATE | syscall.IN_CLOSE_WRITE | syscall.IN_MOVED_TO

// addWatch registers an inotify watch on dir and records the descriptor. An
// existing watch on the same path is updated in place (same descriptor), so
// re-adding during a resync is idempotent.
func (k *Keeper) addWatch(dir string) error {
	wd, err := syscall.InotifyAddWatch(k.fd, dir, watchMask)
	if err != nil {
		return err
	}
	k.mu.Lock()
	k.wdToDir[wd] = dir
	k.dirToWd[dir] = wd
	k.mu.Unlock()
	return nil
}

// AddWatchRecursive establishes a watch on root and every directory beneath it.
// A failure at the root — the directory is missing or cannot be watched — is
// fatal (returned), since proceeding would leave the event loop blocked with no
// watches. Failures on individual subtrees are logged and skipped so one
// unreadable directory does not abort the walk.
func (k *Keeper) AddWatchRecursive(root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if path == root {
				return err
			}
			return nil // skip unreadable subtree, keep walking
		}
		if d.IsDir() {
			if e := k.addWatch(path); e != nil {
				if path == root {
					return e
				}
				logging.Warn("addWatch %s: %v", path, e)
			}
		}
		return nil
	})
}

// dirForWd returns the directory a watch descriptor refers to.
func (k *Keeper) dirForWd(wd int) (string, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	dir, ok := k.wdToDir[wd]
	return dir, ok
}

// forgetWatch drops the bookkeeping for a descriptor the kernel has retired
// (IN_IGNORED), e.g. after the watched directory was deleted.
func (k *Keeper) forgetWatch(wd int) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if dir, ok := k.wdToDir[wd]; ok {
		delete(k.dirToWd, dir)
		delete(k.wdToDir, wd)
	}
}

// handleNewDir reacts to a directory appearing under the watch tree: it watches
// the new subtree and syncs any files that were created in the window between
// the mkdir and the watch being established.
func (k *Keeper) handleNewDir(dir string) {
	if err := k.AddWatchRecursive(dir); err != nil {
		logging.Warn("watch new dir %s: %v", dir, err)
	}
	k.walkAndSync(dir)
}

// handleEvent dispatches a single parsed inotify event. name is the entry the
// event concerns, relative to the watched directory (empty for watch-scoped
// events like IN_IGNORED).
func (k *Keeper) handleEvent(ev *syscall.InotifyEvent, name string) {
	if ev.Mask&syscall.IN_IGNORED != 0 {
		k.forgetWatch(int(ev.Wd))
		return
	}
	dir, ok := k.dirForWd(int(ev.Wd))
	if !ok || name == "" {
		return
	}
	full := filepath.Join(dir, name)

	if ev.Mask&syscall.IN_ISDIR != 0 {
		if ev.Mask&(syscall.IN_CREATE|syscall.IN_MOVED_TO) != 0 {
			k.handleNewDir(full)
		}
		return
	}
	k.syncFile(full)
}

// handleOverflow recovers from an IN_Q_OVERFLOW: the kernel dropped an unknown
// number of events, so it re-establishes the watch tree (new directories that
// appeared during the gap may be unwatched) and runs a full resync to preserve
// any files whose events were lost. This is the implementation of spec §7
// risk #4. The unconditional periodic resync uses the same walkAndSync, so a
// missed overflow only delays preservation by at most RESYNC_INTERVAL_SEC.
func (k *Keeper) handleOverflow() {
	logging.Warn("inotify queue overflow; resyncing watch tree")
	_ = k.AddWatchRecursive(k.cfg.WatchDir)
	k.walkAndSync(k.cfg.WatchDir)
}

// Run reads inotify events until ctx is cancelled or the fd is closed. It
// blocks in Read (via the poller); a Close on the fd during shutdown surfaces
// as a read error, treated as a clean stop once ctx is done. On IN_Q_OVERFLOW
// it re-establishes watches and does a full resync to recover missed events.
func (k *Keeper) eventLoop(ctx context.Context) error {
	buf := make([]byte, 64*1024)
	for {
		n, err := k.file.Read(buf)
		if err != nil {
			if ctx.Err() != nil {
				return nil // fd closed for shutdown
			}
			return err
		}
		for offset := 0; offset+syscall.SizeofInotifyEvent <= n; {
			raw := (*syscall.InotifyEvent)(unsafe.Pointer(&buf[offset]))
			if raw.Mask&syscall.IN_Q_OVERFLOW != 0 {
				k.handleOverflow()
			}
			name := ""
			if raw.Len > 0 {
				start := offset + syscall.SizeofInotifyEvent
				name = string(bytes.TrimRight(buf[start:start+int(raw.Len)], "\x00"))
			}
			k.handleEvent(raw, name)
			offset += syscall.SizeofInotifyEvent + int(raw.Len)
		}
	}
}
