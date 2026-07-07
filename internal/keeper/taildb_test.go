package keeper

import (
	"crypto/sha256"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// buildTailDB creates a fluent-bit-style in_tail_files SQLite database at path
// in WAL mode, inserts the given rows, and checkpoints so the main file is
// self-contained. Each row is {inode, offset, name}.
func buildTailDB(t *testing.T, path string, rows []dbEntry, inodes []uint64) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`CREATE TABLE in_tail_files (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		offset INTEGER,
		inode INTEGER,
		created INTEGER,
		rotated INTEGER DEFAULT 0
	)`); err != nil {
		t.Fatal(err)
	}
	for i, r := range rows {
		if _, err := db.Exec(
			"INSERT INTO in_tail_files (name, offset, inode) VALUES (?, ?, ?)",
			r.name, r.offset, inodes[i],
		); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		t.Fatal(err)
	}
}

func sha256File(t *testing.T, path string) [32]byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(b)
}

// TestReadTailDBReproducesRows asserts readTailDB returns exactly the inserted
// rows (per-inode offset and name) against a WAL fixture.
func TestReadTailDBReproducesRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flb_kube.db")
	inodes := []uint64{100, 200, 300}
	rows := []dbEntry{
		{offset: 10, name: "/var/log/pods-preserved/ns_pod_uid/c/0.log.20240101-000000"},
		{offset: 4096, name: "/var/log/pods-preserved/ns_pod_uid/c/0.log.20240101-010000"},
		{offset: 0, name: "/var/log/pods-preserved/ns_pod_uid/c/0.log.20240101-020000"},
	}
	buildTailDB(t, path, rows, inodes)

	got, err := readTailDB(path)
	if err != nil {
		t.Fatalf("readTailDB: %v", err)
	}
	if len(got) != len(rows) {
		t.Fatalf("got %d rows, want %d", len(got), len(rows))
	}
	for i, ino := range inodes {
		e, ok := got[ino]
		if !ok {
			t.Fatalf("inode %d missing from result", ino)
		}
		if e.offset != rows[i].offset || e.name != rows[i].name {
			t.Errorf("inode %d = {%d, %q}, want {%d, %q}", ino, e.offset, e.name, rows[i].offset, rows[i].name)
		}
	}
}

// TestReadTailDBIsReadOnly asserts opening with mode=ro leaves the DB file
// bytes unchanged.
func TestReadTailDBIsReadOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flb_kube.db")
	buildTailDB(t, path,
		[]dbEntry{{offset: 5, name: "/p/ns_pod_uid/c/0.log.20240101-000000"}},
		[]uint64{42},
	)

	before := sha256File(t, path)
	if _, err := readTailDB(path); err != nil {
		t.Fatalf("readTailDB: %v", err)
	}
	after := sha256File(t, path)
	if before != after {
		t.Errorf("readTailDB modified the DB file")
	}
}

// TestDBConfirmedConsumed covers the confirmation predicate: inode match, the
// "/"+relPath suffix anchor, and offset >= size across every matching DB.
func TestDBConfirmedConsumed(t *testing.T) {
	const (
		inode   = uint64(777)
		relPath = "ns_pod_uid/c/0.log.20240101-000000"
		size    = int64(100)
	)
	fullName := "/var/log/pods-preserved/" + relPath

	tests := []struct {
		name string
		dbs  []map[uint64]dbEntry
		want bool
	}{
		{
			name: "absent in all DBs is not confirmed",
			dbs: []map[uint64]dbEntry{
				{999: {offset: 100, name: "/other/file"}},
			},
			want: false,
		},
		{
			name: "inode collision with mismatched name is not confirmed",
			dbs: []map[uint64]dbEntry{
				{inode: {offset: 100, name: "/var/log/pods-preserved/other_pod/c/0.log.20240101-000000"}},
			},
			want: false,
		},
		{
			name: "offset below size is not confirmed",
			dbs: []map[uint64]dbEntry{
				{inode: {offset: 99, name: fullName}},
			},
			want: false,
		},
		{
			name: "offset at size in the only matching DB is confirmed",
			dbs: []map[uint64]dbEntry{
				{inode: {offset: 100, name: fullName}},
			},
			want: true,
		},
		{
			name: "one DB finished, another has no row, is confirmed",
			dbs: []map[uint64]dbEntry{
				{inode: {offset: 120, name: fullName}},
				{999: {offset: 0, name: "/unrelated"}},
			},
			want: true,
		},
		{
			name: "one DB finished, another matching DB not finished, is not confirmed",
			dbs: []map[uint64]dbEntry{
				{inode: {offset: 100, name: fullName}},
				{inode: {offset: 50, name: fullName}},
			},
			want: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := dbConfirmedConsumed(tc.dbs, inode, relPath, size); got != tc.want {
				t.Errorf("dbConfirmedConsumed = %v, want %v", got, tc.want)
			}
		})
	}
}
