package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all runtime configuration, sourced entirely from environment
// variables (spec §5.4). The compatibility surface is these env keys; changing
// a key or default is a versioned change.
type Config struct {
	WatchDir           string
	PreserveDir        string
	CleanupIntervalSec int
	CleanupMaxAgeMin   int
	CleanupGzMaxAgeMin int
	ResyncIntervalSec  int
	NamespaceFilter    []string // nil = all namespaces; entries are glob patterns (e.g. "team-*")
	LogLevel           string
	MetricsPort        int
	// PreservedLogDBGlob is the glob for the fluent-bit tail DBs that track the
	// preserved log tree. Empty disables DB-aware cleanup. The default matches
	// flb_kube*.db so DBs of other fluent-bit inputs are never read by mistake.
	PreservedLogDBGlob string
	// PodNamespace, PodName, and PodUID identify this DaemonSet pod, injected via
	// the Kubernetes downward API. They locate the pod's own container log under
	// WatchDir (`<PodNamespace>_<PodName>_<PodUID>/`) for the startup hardlink
	// validation test (spec §5.2). Empty (unset downward API) makes the test
	// warn-and-skip rather than fail. This is not a Kubernetes API dependency —
	// the values arrive as environment variables set by the kubelet.
	PodNamespace string
	PodName      string
	PodUID       string
}

// configEnvKeys lists every environment variable Load reads. It exists so
// tests can neutralize the ambient environment before asserting defaults.
var configEnvKeys = []string{
	"WATCH_DIR",
	"PRESERVE_DIR",
	"CLEANUP_INTERVAL_SEC",
	"CLEANUP_MAX_AGE_MIN",
	"CLEANUP_GZ_MAX_AGE_MIN",
	"RESYNC_INTERVAL_SEC",
	"NAMESPACE_FILTER",
	"LOG_LEVEL",
	"METRICS_PORT",
	"PRESERVED_LOG_DB_GLOB",
	"POD_NAMESPACE",
	"POD_NAME",
	"POD_UID",
}

// Load reads configuration from the environment, applying the documented
// default when a key is unset, empty, or (for integers) non-numeric.
func Load() Config {
	cfg := Config{
		WatchDir:           envStr("WATCH_DIR", "/var/log/pods"),
		PreserveDir:        envStr("PRESERVE_DIR", "/var/log/pods-preserved"),
		CleanupIntervalSec: envInt("CLEANUP_INTERVAL_SEC", 60),
		CleanupMaxAgeMin:   envInt("CLEANUP_MAX_AGE_MIN", 5),
		CleanupGzMaxAgeMin: envInt("CLEANUP_GZ_MAX_AGE_MIN", 60),
		ResyncIntervalSec:  envInt("RESYNC_INTERVAL_SEC", 30),
		LogLevel:           envStr("LOG_LEVEL", "info"),
		MetricsPort:        envInt("METRICS_PORT", 9113),
		PreservedLogDBGlob: envStr("PRESERVED_LOG_DB_GLOB", "/var/lib/fluent-bit/flb_kube*.db"),
		PodNamespace:       envStr("POD_NAMESPACE", ""),
		PodName:            envStr("POD_NAME", ""),
		PodUID:             envStr("POD_UID", ""),
	}

	if filter := envStr("NAMESPACE_FILTER", ""); filter != "" {
		for _, ns := range strings.Split(filter, ",") {
			if ns = strings.TrimSpace(ns); ns != "" {
				cfg.NamespaceFilter = append(cfg.NamespaceFilter, ns)
			}
		}
	}

	return cfg
}

// Validate checks the numeric configuration for values that would otherwise
// fault at runtime — the interval and age fields become time.Ticker /
// time.Duration inputs (a non-positive duration panics time.NewTicker), and
// MetricsPort must be a bindable TCP port. It accumulates every offending key
// so a single run surfaces all misconfigurations, and returns nil when the
// configuration is usable. Callers should treat a non-nil result as fatal.
func (c Config) Validate() error {
	var errs []error
	positive := func(key string, v int) {
		if v <= 0 {
			errs = append(errs, fmt.Errorf("%s must be a positive integer, got %d", key, v))
		}
	}
	positive("CLEANUP_INTERVAL_SEC", c.CleanupIntervalSec)
	positive("CLEANUP_MAX_AGE_MIN", c.CleanupMaxAgeMin)
	positive("CLEANUP_GZ_MAX_AGE_MIN", c.CleanupGzMaxAgeMin)
	positive("RESYNC_INTERVAL_SEC", c.ResyncIntervalSec)
	if c.MetricsPort < 1 || c.MetricsPort > 65535 {
		errs = append(errs, fmt.Errorf("METRICS_PORT must be in 1..65535, got %d", c.MetricsPort))
	}
	return errors.Join(errs...)
}

// envStr returns the value of key, or fallback when it is unset or empty.
func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envInt returns key parsed as an int, or fallback when it is unset, empty, or
// not a valid integer.
func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
