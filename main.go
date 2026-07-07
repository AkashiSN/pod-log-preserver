package main

import (
	"context"
	_ "embed"
	"log"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

//go:embed VERSION
var versionRaw string

// version is the release version, embedded at build time from the VERSION
// file. It must match the git tag a release is cut from (see the release CI).
var version = strings.TrimSpace(versionRaw)

// main loads configuration, logs the effective settings, and runs the
// preservation path: an initial sync, a recursive inotify watch tree, and a
// periodic resync. The cleanup loop and metrics server are wired up in later
// issues of the v0.5 milestone.
func main() {
	log.SetFlags(0) // timestamps handled by container runtime

	cfg := loadConfig()

	logInfo(cfg, "=== pod-log-preserver %s starting ===", version)
	logInfo(cfg, "WATCH_DIR=%s", cfg.WatchDir)
	logInfo(cfg, "PRESERVE_DIR=%s", cfg.PreserveDir)
	logInfo(cfg, "CLEANUP_INTERVAL_SEC=%d", cfg.CleanupIntervalSec)
	logInfo(cfg, "CLEANUP_MAX_AGE_MIN=%d", cfg.CleanupMaxAgeMin)
	logInfo(cfg, "CLEANUP_GZ_MAX_AGE_MIN=%d", cfg.CleanupGzMaxAgeMin)
	logInfo(cfg, "RESYNC_INTERVAL_SEC=%d", cfg.ResyncIntervalSec)
	if cfg.PreservedLogDBGlob != "" {
		logInfo(cfg, "PRESERVED_LOG_DB_GLOB=%s", cfg.PreservedLogDBGlob)
	} else {
		logInfo(cfg, "PRESERVED_LOG_DB_GLOB=(disabled)")
	}
	if cfg.NamespaceFilter != nil {
		logInfo(cfg, "NAMESPACE_FILTER=%s", strings.Join(cfg.NamespaceFilter, ","))
	} else {
		logInfo(cfg, "NAMESPACE_FILTER=(all)")
	}
	logInfo(cfg, "LOG_LEVEL=%s", cfg.LogLevel)
	logInfo(cfg, "METRICS_PORT=%d", cfg.MetricsPort)

	if err := run(cfg); err != nil {
		log.Fatalf("[ERROR] %v", err)
	}
}

// run drives the preservation path until a SIGTERM/SIGINT arrives. It creates
// the preserve directory, does the startup sync, watches the tree, starts the
// periodic resync, and blocks in the inotify event loop. Shutdown cancels the
// context (stopping resync) and closes the inotify fd (unblocking the loop).
func run(cfg Config) error {
	m := &metrics{}

	// Fail fast if the watch and preserve dirs can't hardlink (spec §4.1); this
	// also creates the preserve directory.
	if err := validateHardlink(cfg.WatchDir, cfg.PreserveDir); err != nil {
		return err
	}

	keeper, err := NewKeeper(cfg, m)
	if err != nil {
		return err
	}
	defer func() { _ = keeper.Close() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	logInfo(cfg, "initial sync of %s", cfg.WatchDir)
	keeper.initialSync()

	if err := keeper.AddWatchRecursive(cfg.WatchDir); err != nil {
		return err
	}

	go keeper.periodicResync(ctx, time.Duration(cfg.ResyncIntervalSec)*time.Second)

	// Close the fd when the context is cancelled so Run's blocking Read returns.
	go func() {
		<-ctx.Done()
		_ = keeper.Close()
	}()

	logInfo(cfg, "watching for log events")
	return keeper.Run(ctx)
}
