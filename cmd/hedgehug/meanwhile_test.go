package main

import (
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// meanwhile makes fn write to the store once the next command has read its
// index and before it writes, as another process could.
func (te *testEnv) meanwhile(fn func(tx *store.Tx) error) {
	te.t.Helper()
	beforeUpdate = func() {
		beforeUpdate = func() {}
		s, err := store.Open(te.storeDir("me"))
		if err != nil {
			te.t.Fatal(err)
		}
		if _, err := s.Update(store.Op{Name: "update", Author: gitrepo.Signature{Name: "T", Email: "t@example.com"}}, fn); err != nil {
			te.t.Fatal(err)
		}
	}
	te.t.Cleanup(func() { beforeUpdate = func() {} })
}

// putChild returns a write of a new task under parent, in its workspace and
// deleted when it is, and points *id at the new task.
func putChild(parent string, id *model.ID) func(*store.Tx) error {
	return func(tx *store.Tx) error {
		pid, err := model.ParseID(parent)
		if err != nil {
			return err
		}
		p, err := tx.Task(pid)
		if err != nil {
			return err
		}
		c := &model.Task{ID: model.NewID(), Workspace: p.Workspace, Parent: &p.ID, Kind: "task", Title: "meanwhile", Status: "todo", DeletedAt: p.DeletedAt}
		*id = c.ID
		return tx.PutTask(c)
	}
}

func TestMvWorkspaceMovesAChildAddedMeanwhile(t *testing.T) {
	te := newStoreEnv(t)
	te.ok("workspace", "add", "api", "--prefix", "API")
	p := te.addJSON("p", "--workspace", "me")
	var c model.ID
	te.meanwhile(putChild(p.ID, &c))
	out := te.ok("mv", p.Key, "--workspace", "api")
	if got := te.task(c.String()); got.Key != "API-2" {
		t.Errorf("the child added meanwhile: key %s", got.Key)
	}
	if !strings.Contains(out, "ME-2 → API-2") {
		t.Errorf("mv =\n%s", out)
	}
}

func TestWorkspacePrefixRenumbersATaskAddedMeanwhile(t *testing.T) {
	te := newStoreEnv(t)
	p := te.addJSON("p")
	var c model.ID
	te.meanwhile(putChild(p.ID, &c))
	out := te.ok("workspace", "prefix", "me", "NEW")
	if got := te.task(c.String()); got.Key != "NEW-2" {
		t.Errorf("the task added meanwhile: key %s", got.Key)
	}
	if !strings.Contains(out, "renumbered 2 tasks") || !strings.Contains(out, "ME-2 → NEW-2") {
		t.Errorf("prefix =\n%s", out)
	}
}

func TestRmDeletesAChildAddedMeanwhile(t *testing.T) {
	te := newStoreEnv(t)
	p := te.addJSON("p")
	var c model.ID
	te.meanwhile(putChild(p.ID, &c))
	if out := te.ok("rm", p.Key); !strings.Contains(out, "and 1 task under it") {
		t.Errorf("rm = %s", out)
	}
	if got := te.task(c.String()); got.DeletedAt == nil {
		t.Error("the child added meanwhile is not deleted")
	}
}

func TestRestoreRestoresAChildDeletedMeanwhile(t *testing.T) {
	te := newStoreEnv(t)
	p := te.addJSON("p")
	te.ok("rm", p.Key)
	var c model.ID
	te.meanwhile(putChild(p.ID, &c)) // deleted at the same time as p
	te.ok("restore", p.Key)
	if got := te.task(c.String()); got.DeletedAt != nil {
		t.Error("the child deleted with p meanwhile is not restored")
	}
}

func TestPurgeSeesChildrenAddedMeanwhile(t *testing.T) {
	te := newStoreEnv(t)
	p := te.addJSON("p")
	te.ok("rm", p.Key)
	var c model.ID
	te.meanwhile(putChild(p.ID, &c)) // deleted, so it goes with p
	te.ok("purge", p.Key, "--yes")
	s, err := store.Open(te.storeDir("me"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Task(c); err == nil {
		t.Error("the deleted child added meanwhile is not purged")
	}

	q := te.addJSON("q")
	te.ok("rm", q.Key)
	te.meanwhile(func(tx *store.Tx) error {
		if err := putChild(q.ID, &c)(tx); err != nil {
			return err
		}
		live, err := tx.Task(c)
		if err != nil {
			return err
		}
		live.DeletedAt = nil
		return tx.PutTask(live)
	})
	if code, _, stderr := te.run("purge", q.Key, "--yes"); code != 1 || !strings.Contains(stderr, "not deleted") {
		t.Errorf("purge over a live child added meanwhile: exit %d, stderr %q", code, stderr)
	}

	// After a confirmation, a change to what is purged stops the purge.
	r := te.addJSON("r")
	te.ok("rm", r.Key)
	te.terminal, te.stdin = true, "y\n"
	te.meanwhile(putChild(r.ID, &c))
	if code, _, stderr := te.run("purge", r.Key); code != 1 || !strings.Contains(stderr, "run purge again") {
		t.Errorf("purge after a change: exit %d, stderr %q", code, stderr)
	}
	if _, err := s.Task(c); err != nil {
		t.Errorf("a task that was not confirmed is purged: %v", err)
	}
}
