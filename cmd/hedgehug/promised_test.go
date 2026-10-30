package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

func TestStoreInitConfigWriteFailureLeavesNoStore(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permissions do not restrict writes on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	te := newTestEnv(t)
	if err := os.MkdirAll(filepath.Join(te.hhHome, "stores"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(te.hhHome, 0o500); err != nil { //nolint:gosec // a read-only directory is the point of the test
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(te.hhHome, 0o750) }) //nolint:gosec // let TempDir remove it

	code, _, stderr := te.run("store", "init", "me", "--prefix", "ME")
	if code != 1 {
		t.Errorf("exit = %d, want 1 (stderr %q)", code, stderr)
	}
	if _, err := os.Stat(te.storeDir("me")); err == nil {
		t.Error("the store directory was left behind")
	}
}

func TestStatusRejectedByTransitionAddsNoCommit(t *testing.T) {
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	s, err := store.Open(te.storeDir("me"))
	if err != nil {
		t.Fatal(err)
	}
	sc := schema.Default()
	sc.Workflow.Transitions = schema.Lists{{Key: "todo", Values: []string{"in_progress"}}}
	author := gitrepo.Signature{Name: "Alice", Email: "alice@example.com"}
	if _, err := s.Update(store.Op{Name: "update", Author: author}, func(tx *store.Tx) error { return tx.PutSchema(sc) }); err != nil {
		t.Fatal(err)
	}
	before := len(te.log("me"))
	code, _, stderr := te.run("status", a.Key, "done")
	if code != 1 || !strings.HasPrefix(stderr, "hedgehug: ") {
		t.Errorf("exit %d, stderr %q, want 1 and an error", code, stderr)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
	if st := te.task(a.ULID).Status; st != "todo" {
		t.Errorf("status = %q, want todo", st)
	}
}

func TestDepAddCycleAddsNoCommit(t *testing.T) {
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	b := te.addJSON("Beta")
	te.ok("dep", "add", a.Key, "--blocked-by", b.Key)
	before := len(te.log("me"))
	code, _, stderr := te.run("dep", "add", b.Key, "--blocked-by", a.Key)
	if code != 1 || !strings.Contains(stderr, "leads back") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
}

func TestDepAddSelfIsAnError(t *testing.T) {
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	before := len(te.log("me"))
	code, _, stderr := te.run("dep", "add", a.Key, "--blocked-by", a.Key)
	if code == 0 || !strings.HasPrefix(stderr, "hedgehug: ") {
		t.Errorf("exit %d, stderr %q, want an error", code, stderr)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
}

func TestMvAgainstItself(t *testing.T) {
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	te.addJSON("Beta")
	before := len(te.log("me"))
	for _, flag := range []string{"--before", "--after"} {
		code, _, stderr := te.run("mv", a.Key, flag, a.Key)
		if code == 0 || !strings.HasPrefix(stderr, "hedgehug: ") {
			t.Errorf("mv %s itself: exit %d, stderr %q, want an error", flag, code, stderr)
		}
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
}

func TestMvDeletedTask(t *testing.T) {
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	te.addJSON("Beta")
	te.ok("rm", a.Key)
	before := len(te.log("me"))
	code, _, stderr := te.run("mv", a.Key, "--root")
	if code != 1 || !strings.Contains(stderr, "restore") {
		t.Errorf("exit %d, stderr %q, want 1 and a pointer to restore", code, stderr)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
}
