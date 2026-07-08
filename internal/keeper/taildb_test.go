package keeper

import (
	"crypto/sha256"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
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
		es, ok := got[ino]
		if !ok || len(es) != 1 {
			t.Fatalf("inode %d = %d rows, want exactly 1", ino, len(es))
		}
		e := es[0]
		if e.offset != rows[i].offset || e.name != rows[i].name {
			t.Errorf("inode %d = {%d, %q}, want {%d, %q}", ino, e.offset, e.name, rows[i].offset, rows[i].name)
		}
	}
}

// TestReadTailDBKeepsDuplicateInodeRows asserts that when one tail DB holds two
// rows for the same inode — as happens when a single fluent-bit input tails
// both the live tree and the preserved tree, and a preserved hardlink shares
// its original's inode — readTailDB keeps both rows instead of letting the
// unordered SELECT's last row overwrite the other.
func TestReadTailDBKeepsDuplicateInodeRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flb_kube.db")
	const shared = uint64(500)
	rows := []dbEntry{
		{offset: 4096, name: "/var/log/pods/ns_pod_uid/c/0.log"},
		{offset: 4096, name: "/var/log/pods-preserved/ns_pod_uid/c/0.log.20240101-000000"},
	}
	buildTailDB(t, path, rows, []uint64{shared, shared})

	got, err := readTailDB(path)
	if err != nil {
		t.Fatalf("readTailDB: %v", err)
	}
	if len(got[shared]) != 2 {
		t.Fatalf("inode %d has %d rows, want 2 (duplicate rows must not overwrite)", shared, len(got[shared]))
	}
	names := map[string]bool{}
	for _, e := range got[shared] {
		names[e.name] = true
	}
	for _, r := range rows {
		if !names[r.name] {
			t.Errorf("row %q missing from readTailDB result", r.name)
		}
	}
}

// TestDBConfirmedConsumedDuplicateInode covers the confirmation predicate when
// one DB holds two rows on the same inode: the row for the pods/ tree does not
// match the preserved-path anchor and must be ignored, while the preserved-tree
// row confirms. This must hold regardless of which row the SELECT yields last,
// so both insertion orders are exercised.
func TestDBConfirmedConsumedDuplicateInode(t *testing.T) {
	const (
		inode   = uint64(500)
		relPath = "ns_pod_uid/c/0.log.20240101-000000"
		size    = int64(4096)
	)
	preserved := dbEntry{offset: 4096, name: "/var/log/pods-preserved/" + relPath}
	live := dbEntry{offset: 4096, name: "/var/log/pods/ns_pod_uid/c/0.log"}

	for _, order := range []struct {
		name string
		rows []dbEntry
	}{
		{"preserved row last", []dbEntry{live, preserved}},
		{"preserved row first", []dbEntry{preserved, live}},
	} {
		t.Run(order.name, func(t *testing.T) {
			dbs := []map[uint64][]dbEntry{{inode: order.rows}}
			if !dbConfirmedConsumed(dbs, inode, relPath, size) {
				t.Error("dbConfirmedConsumed = false, want true (preserved row confirms regardless of order)")
			}
		})
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

// TestTailDBDSNRejectsWrites proves the DSN readTailDB uses (mode=ro) actually
// rejects writes, not just that readTailDB happens to issue no writes. It opens
// a fixture with the exact same DSN construction (tailDBDSN, shared with the
// implementation so the test cannot drift from it) and asserts an INSERT fails
// with a read-only error. Removing mode=ro from the DSN turns this test red,
// guarding the "read-only against fluent-bit's tail DB" architectural invariant.
func TestTailDBDSNRejectsWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flb_kube.db")
	buildTailDB(t, path,
		[]dbEntry{{offset: 5, name: "/p/ns_pod_uid/c/0.log.20240101-000000"}},
		[]uint64{42},
	)

	db, err := sql.Open("sqlite", tailDBDSN(path))
	if err != nil {
		t.Fatalf("open read-only DSN: %v", err)
	}
	defer func() { _ = db.Close() }()

	_, err = db.Exec("INSERT INTO in_tail_files (name, offset, inode) VALUES ('x', 0, 1)")
	if err == nil {
		t.Fatal("INSERT via the read-only DSN succeeded; the DSN is not read-only")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "readonly") {
		t.Errorf("INSERT error = %v, want a read-only error", err)
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
		dbs  []map[uint64][]dbEntry
		want bool
	}{
		{
			name: "absent in all DBs is not confirmed",
			dbs: []map[uint64][]dbEntry{
				{999: {{offset: 100, name: "/other/file"}}},
			},
			want: false,
		},
		{
			name: "inode collision with mismatched name is not confirmed",
			dbs: []map[uint64][]dbEntry{
				{inode: {{offset: 100, name: "/var/log/pods-preserved/other_pod/c/0.log.20240101-000000"}}},
			},
			want: false,
		},
		{
			name: "offset below size is not confirmed",
			dbs: []map[uint64][]dbEntry{
				{inode: {{offset: 99, name: fullName}}},
			},
			want: false,
		},
		{
			name: "offset at size in the only matching DB is confirmed",
			dbs: []map[uint64][]dbEntry{
				{inode: {{offset: 100, name: fullName}}},
			},
			want: true,
		},
		{
			name: "one DB finished, another has no row, is confirmed",
			dbs: []map[uint64][]dbEntry{
				{inode: {{offset: 120, name: fullName}}},
				{999: {{offset: 0, name: "/unrelated"}}},
			},
			want: true,
		},
		{
			name: "one DB finished, another matching DB not finished, is not confirmed",
			dbs: []map[uint64][]dbEntry{
				{inode: {{offset: 100, name: fullName}}},
				{inode: {{offset: 50, name: fullName}}},
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
