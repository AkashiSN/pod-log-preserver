package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/AkashiSN/pod-log-preserver/internal/config"
	"github.com/AkashiSN/pod-log-preserver/internal/keeper"
	"github.com/AkashiSN/pod-log-preserver/internal/logging"
	"github.com/AkashiSN/pod-log-preserver/internal/metrics"
	"github.com/AkashiSN/pod-log-preserver/internal/validate"
	"github.com/AkashiSN/pod-log-preserver/internal/version"
)

// main loads configuration, logs the effective settings, serves the metrics
// endpoint, and runs the preservation path: an initial sync, a recursive
// inotify watch tree, a periodic resync, and the tail-DB-confirmed cleanup
// loop.
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
	if cfg.PodNamespace != "" || cfg.PodName != "" || cfg.PodUID != "" {
		logging.Info(cfg, "POD=%s/%s (uid %s)", cfg.PodNamespace, cfg.PodName, cfg.PodUID)
	} else {
		logging.Info(cfg, "POD=(downward API not injected)")
	}

	if err := run(cfg); err != nil {
		log.Fatalf("[ERROR] %v", err)
	}
}

// run wires up the daemon and drives the preservation path until a
// SIGTERM/SIGINT arrives. It runs the startup filesystem validation (which also
// creates the preserve directory), starts the metrics server, constructs the
// Keeper, and blocks in Keeper.Run until the signal-derived context is
// cancelled — at which point the metrics server is shut down too.
func run(cfg config.Config) error {
	// Fail fast on out-of-range numeric config before any ticker is built from
	// it (a non-positive interval would panic time.NewTicker at runtime).
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	m := &metrics.Metrics{}

	// Fail fast if the watch and preserve dirs can't hardlink (spec §4.1 / §5.2);
	// this also creates the preserve directory. A missing own container log warns
	// and skips rather than failing.
	res, err := validate.ValidateFilesystem(cfg.WatchDir, cfg.PreserveDir, cfg.PodNamespace, cfg.PodName, cfg.PodUID)
	if err != nil {
		return err
	}
	if res.Skipped {
		logging.Warn("hardlink validation skipped: %s", res.Reason)
	} else {
		logging.Info(cfg, "hardlink validation passed against own log %s", res.TestedLog)
	}

	k, err := keeper.New(cfg, m)
	if err != nil {
		return err
	}
	defer func() { _ = k.Close() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// Serve Prometheus metrics alongside the preservation loops (spec §4.2 / §5.1).
	// The listener is bound synchronously so a bind failure (e.g. METRICS_PORT
	// already in use) fails startup fast instead of leaving the daemon running
	// without the endpoint the spec promises.
	srv, err := startMetrics(cfg, m)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()

	return k.Run(ctx)
}

// startMetrics binds the metrics listener synchronously so a bind failure (e.g.
// METRICS_PORT already in use) is returned to the caller and fails startup fast,
// then serves /metrics in the background on the bound listener (spec §4.2). The
// returned server is shut down by the caller on context cancellation.
func startMetrics(cfg config.Config, m *metrics.Metrics) (*http.Server, error) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", m.Handler())

	addr := fmt.Sprintf(":%d", cfg.MetricsPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("metrics server: listen on %s: %w", addr, err)
	}

	srv := &http.Server{Handler: mux}
	go func() {
		logging.Info(cfg, "metrics server listening on :%d/metrics", cfg.MetricsPort)
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logging.Error("metrics server: %v", err)
		}
	}()
	return srv, nil
}
