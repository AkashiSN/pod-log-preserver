package keeper

import (
	"database/sql"
	"path/filepath"
	"strings"

	"github.com/AkashiSN/pod-log-preserver/internal/logging"
	"github.com/AkashiSN/pod-log-preserver/internal/metrics"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, registered as "sqlite"
)

// dbEntry is one fluent-bit tail-DB row projected to the fields cleanup needs:
// the byte offset the agent has read to and the file name it recorded.
type dbEntry struct {
	offset int64
	name   string
}

// tailDBDSN builds the SQLite DSN used to open a fluent-bit tail DB. It pins
// mode=ro so the agent's rows are never written and a busy_timeout so a
// concurrent writer does not immediately fail the read (spec §5.3,
// architectural invariant: read-only against fluent-bit's tail DB). It is the
// single source of the DSN string so a test can open the same connection the
// implementation does and prove the read-only pin actually rejects writes.
func tailDBDSN(path string) string {
	return "file:" + path + "?mode=ro&_pragma=busy_timeout(5000)"
}

// readTailDB opens a fluent-bit in_tail SQLite tail DB read-only and returns a
// map from inode to the recorded (offset, name) rows. A single DB can hold more
// than one row per inode — a fluent-bit input that tails both the live and the
// preserved tree records the same inode (a shared hardlink) under two names —
// so rows are appended, never overwritten, and every row for an inode is kept.
// The DSN (tailDBDSN) pins mode=ro and a busy_timeout so the agent's rows are never
// written and a concurrent writer does not immediately fail the read (spec
// §5.3, architectural invariant: read-only against fluent-bit's tail DB). The
// pool is pinned to a single connection so the read-only handle registers
// exactly once in the WAL index.
func readTailDB(path string) (map[uint64][]dbEntry, error) {
	db, err := sql.Open("sqlite", tailDBDSN(path))
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)

	rows, err := db.Query("SELECT inode, offset, name FROM in_tail_files")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	entries := make(map[uint64][]dbEntry)
	for rows.Next() {
		var (
			inode  uint64
			offset int64
			name   string
		)
		if err := rows.Scan(&inode, &offset, &name); err != nil {
			return nil, err
		}
		entries[inode] = append(entries[inode], dbEntry{offset: offset, name: name})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

// loadTailDBs reads every tail DB matching glob, returning one inode→entry map
// per readable DB. An empty glob disables DB-aware cleanup (returns nil). A DB
// that cannot be read is logged, counted, and skipped — never fatal — so a
// single corrupt or locked DB can never stall cleanup (spec §5.3).
func loadTailDBs(glob string, m *metrics.Metrics) []map[uint64][]dbEntry {
	if glob == "" {
		return nil
	}
	paths, err := filepath.Glob(glob)
	if err != nil {
		logging.Warn("tail DB glob %q: %v", glob, err)
		return nil
	}
	var dbs []map[uint64][]dbEntry
	for _, p := range paths {
		entries, err := readTailDB(p)
		if err != nil {
			logging.Warn("read tail DB %s: %v", p, err)
			m.FluentbitDBErrors.Add(1)
			continue
		}
		dbs = append(dbs, entries)
	}
	return dbs
}

// dbConfirmedConsumed reports whether every tail DB that tracks this file has
// read it to completion (spec §3.2). A file is confirmed when at least one DB
// has a row for its inode whose recorded name ends with "/"+relPath — the
// leading separator anchors the match on a path boundary, guarding against an
// inode-number collision with an unrelated file — and every such matching row
// has an offset that has reached the file's size. Rows for the inode whose name
// does not match the anchor are ignored (e.g. the live-tree name of a preserved
// hardlink recorded under the same inode), so a single DB tailing both trees
// still confirms deterministically. A DB with no matching row is treated as
// "not discovered yet", not "not finished", so it does not block deletion.
func dbConfirmedConsumed(dbs []map[uint64][]dbEntry, inode uint64, relPath string, size int64) bool {
	anchor := "/" + relPath
	matched := false
	for _, db := range dbs {
		for _, e := range db[inode] {
			if !strings.HasSuffix(e.name, anchor) {
				continue // a different file sharing this inode
			}
			matched = true
			if e.offset < size {
				return false // this agent has not finished reading
			}
		}
	}
	return matched
}
