package main

import (
	"os"
	"sync"
	"syscall"
)

// Keeper owns the preservation side of the daemon: the inotify watch tree over
// the watch directory and the hardlinking of matching logs into the preserve
// tree. It holds the shared configuration and metric counters, plus the
// bookkeeping that maps inotify watch descriptors to the directories they
// watch.
type Keeper struct {
	cfg Config
	m   *metrics

	fd        int       // raw inotify fd, used for InotifyAddWatch
	file      *os.File  // the same fd wrapped so reads go through Go's poller
	closeOnce sync.Once // guards Close against the shutdown goroutine and the defer

	mu      sync.Mutex     // guards the watch maps
	wdToDir map[int]string // watch descriptor -> directory path
	dirToWd map[string]int // directory path -> watch descriptor
}

// NewKeeper creates a Keeper with an initialized inotify instance. The fd is
// non-blocking and close-on-exec and is wrapped in an *os.File so Run's Read
// goes through the runtime poller; Close then reliably unblocks a blocked Read
// for a clean shutdown (spec §5.1) — closing a raw fd does not.
func NewKeeper(cfg Config, m *metrics) (*Keeper, error) {
	fd, err := syscall.InotifyInit1(syscall.IN_NONBLOCK | syscall.IN_CLOEXEC)
	if err != nil {
		return nil, err
	}
	return &Keeper{
		cfg:     cfg,
		m:       m,
		fd:      fd,
		file:    os.NewFile(uintptr(fd), "inotify"),
		wdToDir: make(map[int]string),
		dirToWd: make(map[string]int),
	}, nil
}

// Close releases the inotify file descriptor, unblocking a Run loop waiting on
// it. It is safe to call more than once and from multiple goroutines.
func (k *Keeper) Close() error {
	var err error
	k.closeOnce.Do(func() {
		err = k.file.Close()
	})
	return err
}
