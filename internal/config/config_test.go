package config

import (
	"reflect"
	"testing"
)

// TestLoadConfigDefaults asserts every field falls back to its documented
// default (spec §5.4) when the environment does not set it. Empty values
// exercise the fallback path (envStr/envInt treat "" as unset), so the result
// is deterministic regardless of the ambient environment.
func TestLoadConfigDefaults(t *testing.T) {
	for _, k := range configEnvKeys {
		t.Setenv(k, "")
	}

	cfg := Load()

	want := Config{
		WatchDir:           "/var/log/pods",
		PreserveDir:        "/var/log/pods-preserved",
		CleanupIntervalSec: 60,
		CleanupMaxAgeMin:   5,
		CleanupGzMaxAgeMin: 60,
		ResyncIntervalSec:  30,
		NamespaceFilter:    nil,
		LogLevel:           "info",
		MetricsPort:        9113,
		PreservedLogDBGlob: "/var/lib/fluent-bit/flb_kube*.db",
		PodNamespace:       "",
		PodName:            "",
		PodUID:             "",
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("Load() =\n  %+v\nwant\n  %+v", cfg, want)
	}
}

// TestLoadConfigOverrides asserts every env key overrides its default.
func TestLoadConfigOverrides(t *testing.T) {
	for _, k := range configEnvKeys {
		t.Setenv(k, "")
	}
	t.Setenv("WATCH_DIR", "/w")
	t.Setenv("PRESERVE_DIR", "/p")
	t.Setenv("CLEANUP_INTERVAL_SEC", "10")
	t.Setenv("CLEANUP_MAX_AGE_MIN", "11")
	t.Setenv("CLEANUP_GZ_MAX_AGE_MIN", "12")
	t.Setenv("RESYNC_INTERVAL_SEC", "13")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("METRICS_PORT", "9999")
	t.Setenv("PRESERVED_LOG_DB_GLOB", "/x/*.db")
	t.Setenv("POD_NAMESPACE", "kube-system")
	t.Setenv("POD_NAME", "pod-log-preserver-abcde")
	t.Setenv("POD_UID", "1234-5678")

	cfg := Load()

	if cfg.PodNamespace != "kube-system" || cfg.PodName != "pod-log-preserver-abcde" ||
		cfg.PodUID != "1234-5678" {
		t.Errorf("pod identity = %q, %q, %q", cfg.PodNamespace, cfg.PodName, cfg.PodUID)
	}
	if cfg.WatchDir != "/w" || cfg.PreserveDir != "/p" {
		t.Errorf("dirs = %q, %q", cfg.WatchDir, cfg.PreserveDir)
	}
	if cfg.CleanupIntervalSec != 10 || cfg.CleanupMaxAgeMin != 11 ||
		cfg.CleanupGzMaxAgeMin != 12 || cfg.ResyncIntervalSec != 13 {
		t.Errorf("ints = %d, %d, %d, %d", cfg.CleanupIntervalSec,
			cfg.CleanupMaxAgeMin, cfg.CleanupGzMaxAgeMin, cfg.ResyncIntervalSec)
	}
	if cfg.LogLevel != "debug" || cfg.MetricsPort != 9999 ||
		cfg.PreservedLogDBGlob != "/x/*.db" {
		t.Errorf("misc = %q, %d, %q", cfg.LogLevel, cfg.MetricsPort, cfg.PreservedLogDBGlob)
	}
}

// TestLoadConfigInvalidIntFallsBack asserts a non-numeric int env value is
// ignored in favor of the default rather than causing a startup error.
func TestLoadConfigInvalidIntFallsBack(t *testing.T) {
	t.Setenv("METRICS_PORT", "not-a-number")
	cfg := Load()
	if cfg.MetricsPort != 9113 {
		t.Errorf("MetricsPort = %d, want default 9113 on invalid int", cfg.MetricsPort)
	}
}

// TestNamespaceFilterParsing covers the comma-separated glob-pattern parsing,
// including the empty=all (nil) case and whitespace/empty-segment handling.
func TestNamespaceFilterParsing(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want []string
	}{
		{"empty is nil (all namespaces)", "", nil},
		{"single", "kube-system", []string{"kube-system"}},
		{"multiple", "a,b,c", []string{"a", "b", "c"}},
		{"trims whitespace", " a , b ", []string{"a", "b"}},
		{"skips empty segments", "a,,b,", []string{"a", "b"}},
		{"glob pattern preserved", "cdx-*", []string{"cdx-*"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("NAMESPACE_FILTER", tc.val)
			cfg := Load()
			if !reflect.DeepEqual(cfg.NamespaceFilter, tc.want) {
				t.Errorf("NamespaceFilter = %#v, want %#v", cfg.NamespaceFilter, tc.want)
			}
		})
	}
}

// TestEnvStrAndEnvInt covers the two primitives directly: a set value wins, an
// empty or invalid value yields the fallback.
func TestEnvStrAndEnvInt(t *testing.T) {
	t.Setenv("PLP_STR", "val")
	if got := envStr("PLP_STR", "fb"); got != "val" {
		t.Errorf("envStr(set) = %q, want %q", got, "val")
	}
	t.Setenv("PLP_STR", "")
	if got := envStr("PLP_STR", "fb"); got != "fb" {
		t.Errorf("envStr(empty) = %q, want fallback %q", got, "fb")
	}

	t.Setenv("PLP_INT", "42")
	if got := envInt("PLP_INT", 7); got != 42 {
		t.Errorf("envInt(set) = %d, want 42", got)
	}
	t.Setenv("PLP_INT", "nope")
	if got := envInt("PLP_INT", 7); got != 7 {
		t.Errorf("envInt(invalid) = %d, want fallback 7", got)
	}
}
