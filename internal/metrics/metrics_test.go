package metrics

import "testing"

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
