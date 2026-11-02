package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/store"
)

// storeTaskFile is the path and content of the file of the task id in the store
// name, as it is in the work tree.
func (te *testEnv) storeTaskFile(name, id string) (path string, data []byte) {
	te.t.Helper()
	err := filepath.WalkDir(filepath.Join(te.storeDir(name), "tasks"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p) //nolint:gosec // a file of the test's own store
		if err == nil && strings.Contains(string(b), "\nid: "+id+"\n") {
			path, data = p, b
		}
		return err
	})
	if err != nil || path == "" {
		te.t.Fatalf("no file of %s in %s: %v", id, name, err)
	}
	return path, data
}

// newForwardEnv is newMoveStoreEnv after AA-2 moved to bravo-long (as BB-3):
// alpha holds a forward file.
func newForwardEnv(t *testing.T) *testEnv {
	t.Helper()
	te := newMoveStoreEnv(t)
	te.ok("mv", "AA-2", "--store", "bravo-long", "--yes")
	return te
}

// The forward file a mounted-nowhere store leaves is told of by doctor as an
// info, which does not fail it.
func TestDoctorForwardTargetIsAnInfo(t *testing.T) {
	t.Parallel()
	te := newForwardEnv(t)
	te.ok("store", "remove", "bravo-long", "--keep")
	te.goldenRun("text", 0, "doctor")
	te.goldenRun("json", 0, "doctor", "--json")
}

// The same ULID live in two stores is a warning of doctor and of a sync.
func TestDuplicateAcrossStoresIsAWarning(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	remote := te.bare("shared")
	te.ok("store", "init", "alpha", "--prefix", "AA", "--remote", remote)
	te.ok("store", "init", "bravo-long", "--prefix", "BB")
	te.addJSON("--store", "alpha", "Alpha task")
	path, old := te.storeTaskFile("alpha", "AA-1")
	te.ok("mv", "AA-1", "--store", "bravo-long", "--yes")
	// the loser of two crossed moves: the task is back in alpha
	if err := os.WriteFile(path, old, 0o600); err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(te.storeDir("alpha"), path)
	if err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(te.storeDir("alpha"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Repo().Commit(gitrepo.Commit{Paths: []string{filepath.ToSlash(rel)}, Subject: "raw", Author: gitrepo.Signature{Name: "Alice", Email: "alice@example.com"}, When: te.now}); err != nil {
		t.Fatal(err)
	}
	te.goldenRun("doctor", 1, "doctor")
	te.goldenRun("doctor-json", 1, "doctor", "--json")
	code, _, stderr := te.run("sync", "--store", "alpha")
	if code != 0 || !strings.Contains(stderr, "warning: alpha: ") || !strings.Contains(stderr, " is in alpha (AA-1) and bravo-long (BB-1); remove the one not needed with hedgehug rm and hedgehug purge --older-than 0d --store <store>") {
		t.Errorf("sync: exit %d\nstderr %s", code, stderr)
	}
}

// Two users move the same task to different stores at the same time. The
// duplicate arrives with the sync of the store the other one moved it to.
func TestSyncBringsADuplicateAcrossStores(t *testing.T) {
	t.Parallel()
	alice := newTestEnv(t)
	remoteA, remoteB := alice.bare("alpha"), alice.bare("bravo")
	alice.ok("store", "init", "alpha", "--prefix", "AA", "--remote", remoteA)
	alice.ok("store", "init", "bravo-long", "--prefix", "BB", "--remote", remoteB)
	alice.ok("store", "init", "charlie", "--prefix", "CC")
	alice.ok("add", "--store", "alpha", "Shared")
	alice.ok("sync")
	bob := alice.other()
	bob.ok("store", "add", "alpha", "--remote", remoteA)
	bob.ok("store", "add", "bravo-long", "--remote", remoteB)
	bob.ok("mv", "AA-1", "--store", "bravo-long", "--yes")
	bob.ok("sync")
	alice.ok("mv", "AA-1", "--store", "charlie", "--yes")
	code, _, stderr := alice.run("sync", "--store", "bravo-long")
	want := " is in bravo-long (BB-1) and charlie (CC-1); remove the one not needed with hedgehug rm and hedgehug purge --older-than 0d --store <store>"
	if code != 0 || !strings.Contains(stderr, "warning: bravo-long: ") || !strings.Contains(stderr, want) {
		t.Errorf("sync: exit %d\nstderr %s", code, stderr)
	}
}

// A --fix of a later store can renumber a task that lives in two stores; the
// warning names the id it has after the repairs.
func TestDoctorFixShowsTheRenumberedIDOfADuplicate(t *testing.T) {
	t.Parallel()
	te := newTwoStoreEnv(t)
	te.addJSON("--store", "alpha", "Crossed")
	path, old := te.storeTaskFile("alpha", "AA-3")
	te.ok("mv", "AA-3", "--store", "bravo-long", "--yes")
	if err := os.WriteFile(path, old, 0o600); err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(te.storeDir("alpha"), path)
	if err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(te.storeDir("alpha"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Repo().Commit(gitrepo.Commit{Paths: []string{filepath.ToSlash(rel)}, Subject: "raw", Author: gitrepo.Signature{Name: "Alice", Email: "alice@example.com"}, When: te.now}); err != nil {
		t.Fatal(err)
	}
	// the moved task takes the id of an older one in bravo-long
	te.handEdit("bravo-long", "BB-3", "id: BB-3", "id: BB-1")
	_, out, _ := te.run("doctor", "--fix", "--json")
	var got []struct {
		Store    string `json:"store"`
		Repaired []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"repaired"`
		Findings []struct {
			Check string `json:"check"`
			Msg   string `json:"msg"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if len(got) != 2 || len(got[1].Repaired) != 1 || got[1].Repaired[0].Title != "Crossed" || got[1].Repaired[0].ID == "BB-1" {
		t.Fatalf("doctor --fix = %s; want the task renumbered in bravo-long", out)
	}
	want := "bravo-long (" + got[1].Repaired[0].ID + ")"
	for _, r := range got {
		var n int
		for _, f := range r.Findings {
			if f.Check == "duplicate_across_stores" {
				n++
				if !strings.Contains(f.Msg, want) {
					t.Errorf("%s: %q lacks %q", r.Store, f.Msg, want)
				}
			}
		}
		if n != 1 {
			t.Errorf("%s: %d duplicate findings in %s", r.Store, n, out)
		}
	}
}
