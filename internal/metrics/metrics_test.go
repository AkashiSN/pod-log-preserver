package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestMetricsCounters documents the counter set the three loops share and
// exercises every field through an Add/Load round-trip on a fresh value. Each
// counter starts at zero and is independent.
func TestMetricsCounters(t *testing.T) {
	var mm Metrics
	cases := []struct {
		name string
		add  func(int64)
		load func() int64
	}{
		{"PreservedFiles", func(v int64) { mm.PreservedFiles.Add(v) }, func() int64 { return mm.PreservedFiles.Load() }},
		{"OrphanedFiles", func(v int64) { mm.OrphanedFiles.Add(v) }, func() int64 { return mm.OrphanedFiles.Load() }},
		{"PreservedBytes", func(v int64) { mm.PreservedBytes.Add(v) }, func() int64 { return mm.PreservedBytes.Load() }},
		{"HardlinksCreated", func(v int64) { mm.HardlinksCreated.Add(v) }, func() int64 { return mm.HardlinksCreated.Load() }},
		{"OrphansRemoved", func(v int64) { mm.OrphansRemoved.Add(v) }, func() int64 { return mm.OrphansRemoved.Load() }},
		{"DBConfirmedRemoved", func(v int64) { mm.DBConfirmedRemoved.Add(v) }, func() int64 { return mm.DBConfirmedRemoved.Load() }},
		{"FluentbitDBErrors", func(v int64) { mm.FluentbitDBErrors.Add(v) }, func() int64 { return mm.FluentbitDBErrors.Load() }},
	}
	for _, c := range cases {
		if got := c.load(); got != 0 {
			t.Errorf("%s initial = %d, want 0", c.name, got)
		}
		c.add(3)
		if got := c.load(); got != 3 {
			t.Errorf("%s after Add(3) = %d, want 3", c.name, got)
		}
	}
}

// TestHandlerExposition asserts the /metrics handler emits every documented
// metric (spec §4.2) in Prometheus text format: a HELP line, a TYPE line with
// the correct gauge/counter type, and a sample line carrying the counter's
// current value.
func TestHandlerExposition(t *testing.T) {
	var m Metrics
	m.PreservedFiles.Store(5)
	m.OrphanedFiles.Store(2)
	m.PreservedBytes.Store(4096)
	m.HardlinksCreated.Store(10)
	m.OrphansRemoved.Store(7)
	m.DBConfirmedRemoved.Store(6)
	m.FluentbitDBErrors.Store(1)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	m.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain...", ct)
	}

	body := rec.Body.String()
	want := []struct {
		name  string
		typ   string
		help  string
		value string
	}{
		{"pod_log_preserver_preserved_files", "gauge", "Files currently in the preserve directory", "5"},
		{"pod_log_preserver_orphaned_files", "gauge", "Preserved files with link count 1", "2"},
		{"pod_log_preserver_preserved_bytes", "gauge", "Total bytes under the preserve directory", "4096"},
		{"pod_log_preserver_hardlinks_created_total", "counter", "Hardlinks created", "10"},
		{"pod_log_preserver_orphans_removed_total", "counter", "Orphaned files removed", "7"},
		{"pod_log_preserver_db_confirmed_removed_total", "counter", "Orphans removed after a tail DB confirmed a full read", "6"},
		{"pod_log_preserver_fluentbit_db_errors_total", "counter", "Tail DB read errors", "1"},
	}
	for _, w := range want {
		for _, line := range []string{
			"# HELP " + w.name + " " + w.help,
			"# TYPE " + w.name + " " + w.typ,
			w.name + " " + w.value,
		} {
			if !strings.Contains(body, line) {
				t.Errorf("body missing line %q\n---\n%s", line, body)
			}
		}
	}
}
