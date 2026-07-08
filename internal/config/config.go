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

	// invalidInts collects the parse errors for integer env vars that were set
	// to a non-numeric value. Load records them here (the field itself falls
	// back to its default so startup logging stays coherent) and Validate
	// surfaces them, so a typo like CLEANUP_INTERVAL_SEC=5m is a fail-fast
	// naming the key rather than a silently ignored setting (spec §5.4).
	invalidInts []error
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
}

// Load reads configuration from the environment, applying the documented
// default when a key is unset or empty. A key set to a non-numeric integer
// still falls back to its default here (so startup logging is coherent) but is
// recorded so Validate can reject it — a typo is a fail-fast, not a silently
// ignored setting (spec §5.4).
func Load() Config {
	var invalid []error
	intOf := func(key string, fallback int) int {
		n, err := envInt(key, fallback)
		if err != nil {
			invalid = append(invalid, err)
		}
		return n
	}
	cfg := Config{
		WatchDir:           envStr("WATCH_DIR", "/var/log/pods"),
		PreserveDir:        envStr("PRESERVE_DIR", "/var/log/pods-preserved"),
		CleanupIntervalSec: intOf("CLEANUP_INTERVAL_SEC", 60),
		CleanupMaxAgeMin:   intOf("CLEANUP_MAX_AGE_MIN", 5),
		CleanupGzMaxAgeMin: intOf("CLEANUP_GZ_MAX_AGE_MIN", 60),
		ResyncIntervalSec:  intOf("RESYNC_INTERVAL_SEC", 30),
		LogLevel:           envStr("LOG_LEVEL", "info"),
		MetricsPort:        intOf("METRICS_PORT", 9113),
		PreservedLogDBGlob: envStr("PRESERVED_LOG_DB_GLOB", "/var/lib/fluent-bit/flb_kube*.db"),
	}
	cfg.invalidInts = invalid

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
	// Non-numeric integer env values are rejected first, naming each key; the
	// affected fields fell back to valid defaults, so they will not also trip
	// the positive-integer checks below.
	errs = append(errs, c.invalidInts...)
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

// envInt returns key parsed as an int, or fallback when it is unset or empty.
// When the value is set but not a valid integer it returns fallback together
// with an error naming the key and quoting the offending value, so the caller
// can turn the typo into a fail-fast instead of a silent default.
func envInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback, fmt.Errorf("%s must be an integer, got %q", key, v)
	}
	return n, nil
}
