package main

import (
	_ "embed"
	"log"
	"strings"
)

//go:embed VERSION
var versionRaw string

// version is the release version, embedded at build time from the VERSION
// file. It must match the git tag a release is cut from (see the release CI).
var version = strings.TrimSpace(versionRaw)

// main loads configuration and logs the effective settings. The preservation,
// cleanup, and metrics loops are wired up in later issues of the v0.5
// milestone.
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
}
