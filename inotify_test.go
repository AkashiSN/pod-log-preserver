package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAddWatchRecursiveMissingRootFails verifies a missing/unwatchable watch
// root is reported as an error rather than silently succeeding — otherwise the
// event loop would block forever with no watches registered.
func TestAddWatchRecursiveMissingRootFails(t *testing.T) {
	k, err := NewKeeper(Config{}, &metrics{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = k.Close() }()

	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if err := k.AddWatchRecursive(missing); err == nil {
		t.Fatal("AddWatchRecursive on a missing root should return an error")
	}
}

// TestAddWatchRecursiveExistingRoot verifies a real tree is watched without
// error and every directory in it is registered.
func TestAddWatchRecursiveExistingRoot(t *testing.T) {
	k, err := NewKeeper(Config{}, &metrics{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = k.Close() }()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "ns_pod_uid", "container"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := k.AddWatchRecursive(root); err != nil {
		t.Fatalf("AddWatchRecursive on an existing tree: %v", err)
	}
	if _, ok := k.dirForWd(k.dirToWd[root]); !ok {
		t.Fatal("root directory was not watched")
	}
}
