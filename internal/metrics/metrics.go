package metrics

import "sync/atomic"

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
