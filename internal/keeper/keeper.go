// Package keeper owns the preservation side of the daemon: the inotify watch
// tree over the watch directory and the hardlinking of matching logs into the
// preserve tree.
package keeper

import (
	"context"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/AkashiSN/pod-log-preserver/internal/config"
	"github.com/AkashiSN/pod-log-preserver/internal/logging"
	"github.com/AkashiSN/pod-log-preserver/internal/metrics"
)

// Keeper owns the preservation side of the daemon: the inotify watch tree over
// the watch directory and the hardlinking of matching logs into the preserve
// tree. It holds the shared configuration and metric counters, plus the
// bookkeeping that maps inotify watch descriptors to the directories they
// watch.
type Keeper struct {
	cfg config.Config
	m   *metrics.Metrics

	fd        int       // raw inotify fd, used for InotifyAddWatch
	file      *os.File  // the same fd wrapped so reads go through Go's poller
	closeOnce sync.Once // guards Close against the shutdown goroutine and the defer

	mu      sync.Mutex     // guards the watch maps
	wdToDir map[int]string // watch descriptor -> directory path
	dirToWd map[string]int // directory path -> watch descriptor
}

// New creates a Keeper with an initialized inotify instance. The fd is
// non-blocking and close-on-exec and is wrapped in an *os.File so the event
// loop's Read goes through the runtime poller; Close then reliably unblocks a
// blocked Read for a clean shutdown (spec §5.1) — closing a raw fd does not.
func New(cfg config.Config, m *metrics.Metrics) (*Keeper, error) {
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

// Run drives the preservation path until ctx is cancelled. It does the startup
// sync, establishes the recursive inotify watch tree, starts the periodic
// resync and the cleanup loop, and blocks in the inotify event loop. Cancelling
// ctx stops the background loops and closes the inotify fd, which unblocks the
// event loop for a clean shutdown (spec §5.1 / §5.2).
func (k *Keeper) Run(ctx context.Context) error {
	logging.Info(k.cfg, "initial sync of %s", k.cfg.WatchDir)
	k.initialSync()

	if err := k.AddWatchRecursive(k.cfg.WatchDir); err != nil {
		return err
	}

	go k.periodicResync(ctx, time.Duration(k.cfg.ResyncIntervalSec)*time.Second)
	go k.cleanupLoop(ctx, time.Duration(k.cfg.CleanupIntervalSec)*time.Second)

	// Close the fd when the context is cancelled so eventLoop's blocking Read
	// returns.
	go func() {
		<-ctx.Done()
		_ = k.Close()
	}()

	logging.Info(k.cfg, "watching for log events")
	return k.eventLoop(ctx)
}

// Close releases the inotify file descriptor, unblocking an event loop waiting
// on it. It is safe to call more than once and from multiple goroutines.
func (k *Keeper) Close() error {
	var err error
	k.closeOnce.Do(func() {
		err = k.file.Close()
	})
	return err
}
