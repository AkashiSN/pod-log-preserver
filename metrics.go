package main

import "sync/atomic"

// metrics holds the process-wide counters the three loops share. They are
// exported over HTTP by the metrics endpoint (added in a later issue). The
// package-level instance and the increment sites are introduced alongside the
// preservation and cleanup loops that own each counter.
type metrics struct {
	preservedFiles     atomic.Int64 // gauge: files currently in the preserve tree
	orphanedFiles      atomic.Int64 // gauge: preserved files with Nlink==1
	preservedBytes     atomic.Int64 // gauge: total bytes in the preserve tree
	hardlinksCreated   atomic.Int64 // counter: hardlinks created
	orphansRemoved     atomic.Int64 // counter: orphaned links removed
	dbConfirmedRemoved atomic.Int64 // counter: removed after a tail-DB confirmed a full read
	fluentbitDBErrors  atomic.Int64 // counter: tail-DB read errors
}
