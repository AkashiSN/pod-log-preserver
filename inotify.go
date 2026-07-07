package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
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
// Per-directory failures are logged and skipped so one unreadable subtree does
// not abort the walk.
func (k *Keeper) AddWatchRecursive(root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if e := k.addWatch(path); e != nil {
				logWarn("addWatch %s: %v", path, e)
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
		logWarn("watch new dir %s: %v", dir, err)
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

// Run reads inotify events until ctx is cancelled or the fd is closed. It
// blocks in Read (via the poller); a Close on the fd during shutdown surfaces
// as a read error, treated as a clean stop once ctx is done. On IN_Q_OVERFLOW
// it re-establishes watches and does a full resync to recover missed events.
func (k *Keeper) Run(ctx context.Context) error {
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
				logWarn("inotify queue overflow; resyncing watch tree")
				_ = k.AddWatchRecursive(k.cfg.WatchDir)
				k.walkAndSync(k.cfg.WatchDir)
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
