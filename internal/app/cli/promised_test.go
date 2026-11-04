package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestStoreInitConfigWriteFailureLeavesNoStore(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	sc := schema.Default()
	sc.Workflow.Transitions = schema.Lists{{Key: "todo", Values: []string{"in_progress"}}}
	te.uc().MustEdit("me", "update", usecasetest.AliceAuthor, func(tx *usecasetest.Tx) error { return tx.PutSchema(sc) })
	before := len(te.log("me"))
	code, _, stderr := te.run("status", a.ID, "done")
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
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	b := te.addJSON("Beta")
	te.ok("dep", "add", a.ID, "--blocked-by", b.ID)
	before := len(te.log("me"))
	code, _, stderr := te.run("dep", "add", b.ID, "--blocked-by", a.ID)
	if code != 1 || !strings.Contains(stderr, "leads back") {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
}

func TestDepAddSelfIsAnError(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	before := len(te.log("me"))
	code, _, stderr := te.run("dep", "add", a.ID, "--blocked-by", a.ID)
	if code == 0 || !strings.HasPrefix(stderr, "hedgehug: ") {
		t.Errorf("exit %d, stderr %q, want an error", code, stderr)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
}

func TestMvAgainstItself(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	te.addJSON("Beta")
	before := len(te.log("me"))
	for _, flag := range []string{"--before", "--after"} {
		code, _, stderr := te.run("mv", a.ID, flag, a.ID)
		if code == 0 || !strings.HasPrefix(stderr, "hedgehug: ") {
			t.Errorf("mv %s itself: exit %d, stderr %q, want an error", flag, code, stderr)
		}
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
}

func TestMvDeletedTask(t *testing.T) {
	t.Parallel()
	te := newStoreEnv(t)
	a := te.addJSON("Alpha")
	te.addJSON("Beta")
	te.ok("rm", a.ID)
	before := len(te.log("me"))
	code, _, stderr := te.run("mv", a.ID, "--root")
	if code != 1 || !strings.Contains(stderr, "restore") {
		t.Errorf("exit %d, stderr %q, want 1 and a pointer to restore", code, stderr)
	}
	if n := len(te.log("me")); n != before {
		t.Errorf("commits = %d, want %d", n, before)
	}
}
