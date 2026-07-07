package main

import (
	"context"
	"log"
	"os/signal"
	"strings"
	"syscall"

	"github.com/AkashiSN/pod-log-preserver/internal/config"
	"github.com/AkashiSN/pod-log-preserver/internal/keeper"
	"github.com/AkashiSN/pod-log-preserver/internal/logging"
	"github.com/AkashiSN/pod-log-preserver/internal/metrics"
	"github.com/AkashiSN/pod-log-preserver/internal/validate"
	"github.com/AkashiSN/pod-log-preserver/internal/version"
)

// main loads configuration, logs the effective settings, and runs the
// preservation path: an initial sync, a recursive inotify watch tree, a
// periodic resync, and the tail-DB-confirmed cleanup loop. The metrics server
// is wired up in a later issue of the v0.5 milestone.
func main() {
	log.SetFlags(0) // timestamps handled by container runtime

	cfg := config.Load()

	logging.Info(cfg, "=== pod-log-preserver %s starting ===", version.Version)
	logging.Info(cfg, "WATCH_DIR=%s", cfg.WatchDir)
	logging.Info(cfg, "PRESERVE_DIR=%s", cfg.PreserveDir)
	logging.Info(cfg, "CLEANUP_INTERVAL_SEC=%d", cfg.CleanupIntervalSec)
	logging.Info(cfg, "CLEANUP_MAX_AGE_MIN=%d", cfg.CleanupMaxAgeMin)
	logging.Info(cfg, "CLEANUP_GZ_MAX_AGE_MIN=%d", cfg.CleanupGzMaxAgeMin)
	logging.Info(cfg, "RESYNC_INTERVAL_SEC=%d", cfg.ResyncIntervalSec)
	if cfg.PreservedLogDBGlob != "" {
		logging.Info(cfg, "PRESERVED_LOG_DB_GLOB=%s", cfg.PreservedLogDBGlob)
	} else {
		logging.Info(cfg, "PRESERVED_LOG_DB_GLOB=(disabled)")
	}
	if cfg.NamespaceFilter != nil {
		logging.Info(cfg, "NAMESPACE_FILTER=%s", strings.Join(cfg.NamespaceFilter, ","))
	} else {
		logging.Info(cfg, "NAMESPACE_FILTER=(all)")
	}
	logging.Info(cfg, "LOG_LEVEL=%s", cfg.LogLevel)
	logging.Info(cfg, "METRICS_PORT=%d", cfg.MetricsPort)

	if err := run(cfg); err != nil {
		log.Fatalf("[ERROR] %v", err)
	}
}

// run wires up the daemon and drives the preservation path until a
// SIGTERM/SIGINT arrives. It runs the startup hardlink gate (which also creates
// the preserve directory), constructs the Keeper, and blocks in Keeper.Run
// until the signal-derived context is cancelled.
func run(cfg config.Config) error {
	m := &metrics.Metrics{}

	// Fail fast if the watch and preserve dirs can't hardlink (spec §4.1); this
	// also creates the preserve directory.
	if err := validate.ValidateHardlink(cfg.WatchDir, cfg.PreserveDir); err != nil {
		return err
	}

	k, err := keeper.New(cfg, m)
	if err != nil {
		return err
	}
	defer func() { _ = k.Close() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	return k.Run(ctx)
}
