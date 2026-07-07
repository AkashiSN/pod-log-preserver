package main

import "testing"

// TestMetricsCounters documents the counter set the three loops share and
// exercises every field through an Add/Load round-trip on a fresh value. Each
// counter starts at zero and is independent.
func TestMetricsCounters(t *testing.T) {
	var mm metrics
	cases := []struct {
		name string
		add  func(int64)
		load func() int64
	}{
		{"preservedFiles", func(v int64) { mm.preservedFiles.Add(v) }, func() int64 { return mm.preservedFiles.Load() }},
		{"orphanedFiles", func(v int64) { mm.orphanedFiles.Add(v) }, func() int64 { return mm.orphanedFiles.Load() }},
		{"preservedBytes", func(v int64) { mm.preservedBytes.Add(v) }, func() int64 { return mm.preservedBytes.Load() }},
		{"hardlinksCreated", func(v int64) { mm.hardlinksCreated.Add(v) }, func() int64 { return mm.hardlinksCreated.Load() }},
		{"orphansRemoved", func(v int64) { mm.orphansRemoved.Add(v) }, func() int64 { return mm.orphansRemoved.Load() }},
		{"dbConfirmedRemoved", func(v int64) { mm.dbConfirmedRemoved.Add(v) }, func() int64 { return mm.dbConfirmedRemoved.Load() }},
		{"fluentbitDBErrors", func(v int64) { mm.fluentbitDBErrors.Add(v) }, func() int64 { return mm.fluentbitDBErrors.Load() }},
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
