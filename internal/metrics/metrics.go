package metrics

import (
	"net/http"
	"strconv"
	"sync/atomic"
)

// Metrics holds the process-wide counters the three loops share. They are
// exported over HTTP by the metrics endpoint (added in a later issue). The
// increment sites are introduced alongside the preservation and cleanup loops
// that own each counter.
type Metrics struct {
	PreservedFiles     atomic.Int64 // gauge: files currently in the preserve tree
	OrphanedFiles      atomic.Int64 // gauge: preserved files with Nlink==1
	PreservedBytes     atomic.Int64 // gauge: total bytes in the preserve tree
	HardlinksCreated   atomic.Int64 // counter: hardlinks created
	OrphansRemoved     atomic.Int64 // counter: orphaned links removed
	DBConfirmedRemoved atomic.Int64 // counter: removed after a tail-DB confirmed a full read
	FluentbitDBErrors  atomic.Int64 // counter: tail-DB read errors
}

// series describes one exported metric: its name, Prometheus type, HELP text,
// and how to read its current value from a Metrics.
type series struct {
	name string
	typ  string // "gauge" or "counter"
	help string
	read func(*Metrics) int64
}

// exported is the metric set served at /metrics, in the order the spec §4.2
// table lists them (gauges first, then counters).
var exported = []series{
	{"pod_log_preserver_preserved_files", "gauge", "Files currently in the preserve directory", func(m *Metrics) int64 { return m.PreservedFiles.Load() }},
	{"pod_log_preserver_orphaned_files", "gauge", "Preserved files with link count 1", func(m *Metrics) int64 { return m.OrphanedFiles.Load() }},
	{"pod_log_preserver_preserved_bytes", "gauge", "Total bytes under the preserve directory", func(m *Metrics) int64 { return m.PreservedBytes.Load() }},
	{"pod_log_preserver_hardlinks_created_total", "counter", "Hardlinks created", func(m *Metrics) int64 { return m.HardlinksCreated.Load() }},
	{"pod_log_preserver_orphans_removed_total", "counter", "Orphaned files removed", func(m *Metrics) int64 { return m.OrphansRemoved.Load() }},
	{"pod_log_preserver_db_confirmed_removed_total", "counter", "Orphans removed after a tail DB confirmed a full read", func(m *Metrics) int64 { return m.DBConfirmedRemoved.Load() }},
	{"pod_log_preserver_fluentbit_db_errors_total", "counter", "Tail DB read errors", func(m *Metrics) int64 { return m.FluentbitDBErrors.Load() }},
}

// Handler returns an http.Handler serving the metric set in Prometheus text
// exposition format (v0.0.4). Each metric emits a HELP line, a TYPE line, and a
// single unlabeled sample carrying its current value.
func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		for _, s := range exported {
			_, _ = w.Write([]byte("# HELP " + s.name + " " + s.help + "\n"))
			_, _ = w.Write([]byte("# TYPE " + s.name + " " + s.typ + "\n"))
			_, _ = w.Write([]byte(s.name + " " + strconv.FormatInt(s.read(m), 10) + "\n"))
		}
	})
}
