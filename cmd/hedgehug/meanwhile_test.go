package main

import (
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/store"
)

// meanwhile makes fn write to the store once the next command has read its
// index and before it writes, as another process could.
func (te *testEnv) meanwhile(fn func(tx *store.Tx) error) {
	te.t.Helper()
	var restore func()
	restore = ops.SetBeforeUpdate(func() {
		restore()
		s, err := store.Open(te.storeDir("me"))
		if err != nil {
			te.t.Fatal(err)
		}
		if _, err := s.Update(store.Op{Name: "update", Author: gitrepo.Signature{Name: "T", Email: "t@example.com"}}, fn); err != nil {
			te.t.Fatal(err)
		}
	})
	te.t.Cleanup(restore)
}

// putChild returns a write of a new task under parent, in its workspace and
// deleted when it is, and points *ulid at the new task.
func putChild(parent string, ulid *model.ULID) func(*store.Tx) error {
	return func(tx *store.Tx) error {
		parentULID, err := model.ParseULID(parent)
		if err != nil {
			return err
		}
		p, err := tx.Task(parentULID)
		if err != nil {
			return err
		}
		c := &model.Task{ULID: model.NewULID(), Workspace: p.Workspace, Parent: &p.ULID, Kind: "task", Title: "meanwhile", Status: "todo", DeletedAt: p.DeletedAt}
		*ulid = c.ULID
		return tx.PutTask(c)
	}
}

func TestMvWorkspaceMovesAChildAddedMeanwhile(t *testing.T) {
	te := newStoreEnv(t)
	te.ok("workspace", "add", "api", "--prefix", "API")
	p := te.addJSON("p", "--workspace", "me")
	var c model.ULID
	te.meanwhile(putChild(p.ULID, &c))
	out := te.ok("mv", p.ID, "--workspace", "api")
	if got := te.task(c.String()); got.ID != "API-2" {
		t.Errorf("the child added meanwhile: id %s", got.ID)
	}
	if !strings.Contains(out, "ME-2 → API-2") {
		t.Errorf("mv =\n%s", out)
	}
}

func TestWorkspacePrefixRenumbersATaskAddedMeanwhile(t *testing.T) {
	te := newStoreEnv(t)
	p := te.addJSON("p")
	var c model.ULID
	te.meanwhile(putChild(p.ULID, &c))
	out := te.ok("workspace", "prefix", "me", "NEW")
	if got := te.task(c.String()); got.ID != "NEW-2" {
		t.Errorf("the task added meanwhile: id %s", got.ID)
	}
	if !strings.Contains(out, "renumbered 2 tasks") || !strings.Contains(out, "ME-2 → NEW-2") {
		t.Errorf("prefix =\n%s", out)
	}
}

func TestRmDeletesAChildAddedMeanwhile(t *testing.T) {
	te := newStoreEnv(t)
	p := te.addJSON("p")
	var c model.ULID
	te.meanwhile(putChild(p.ULID, &c))
	if out := te.ok("rm", p.ID); !strings.Contains(out, "and 1 task under it") {
		t.Errorf("rm = %s", out)
	}
	if got := te.task(c.String()); got.DeletedAt == nil {
		t.Error("the child added meanwhile is not deleted")
	}
}

func TestRestoreRestoresAChildDeletedMeanwhile(t *testing.T) {
	te := newStoreEnv(t)
	p := te.addJSON("p")
	te.ok("rm", p.ID)
	var c model.ULID
	te.meanwhile(putChild(p.ULID, &c)) // deleted at the same time as p
	te.ok("restore", p.ID)
	if got := te.task(c.String()); got.DeletedAt != nil {
		t.Error("the child deleted with p meanwhile is not restored")
	}
}

func TestPurgeSeesChildrenAddedMeanwhile(t *testing.T) {
	te := newStoreEnv(t)
	p := te.addJSON("p")
	te.ok("rm", p.ID)
	var c model.ULID
	te.meanwhile(putChild(p.ULID, &c)) // deleted, so it goes with p
	te.ok("purge", p.ID, "--yes")
	s, err := store.Open(te.storeDir("me"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Task(c); err == nil {
		t.Error("the deleted child added meanwhile is not purged")
	}

	q := te.addJSON("q")
	te.ok("rm", q.ID)
	te.meanwhile(func(tx *store.Tx) error {
		if err := putChild(q.ULID, &c)(tx); err != nil {
			return err
		}
		live, err := tx.Task(c)
		if err != nil {
			return err
		}
		live.DeletedAt = nil
		return tx.PutTask(live)
	})
	if code, _, stderr := te.run("purge", q.ID, "--yes"); code != 1 || !strings.Contains(stderr, "not deleted") {
		t.Errorf("purge over a live child added meanwhile: exit %d, stderr %q", code, stderr)
	}

	// After a confirmation, a change to what is purged stops the purge.
	r := te.addJSON("r")
	te.ok("rm", r.ID)
	te.terminal, te.stdin = true, "y\n"
	te.meanwhile(putChild(r.ULID, &c))
	if code, _, stderr := te.run("purge", r.ID); code != 1 || !strings.Contains(stderr, "run purge again") {
		t.Errorf("purge after a change: exit %d, stderr %q", code, stderr)
	}
	if _, err := s.Task(c); err != nil {
		t.Errorf("a task that was not confirmed is purged: %v", err)
	}
}

// changeTask returns a write that changes the task ulid with fn.
func changeTask(ulid string, fn func(*model.Task)) func(*store.Tx) error {
	return func(tx *store.Tx) error {
		u, err := model.ParseULID(ulid)
		if err != nil {
			return err
		}
		t, err := tx.Task(u)
		if err != nil {
			return err
		}
		fn(t)
		return tx.PutTask(t)
	}
}

func TestRmRefusesATaskDeletedMeanwhile(t *testing.T) {
	te := newStoreEnv(t)
	p := te.addJSON("p")
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	te.meanwhile(changeTask(p.ULID, func(t *model.Task) { t.DeletedAt = &at }))
	if code, _, stderr := te.run("rm", p.ID); code != 1 || !strings.Contains(stderr, "already deleted") {
		t.Errorf("rm: exit %d, stderr %q", code, stderr)
	}
	if got := te.task(p.ULID); got.DeletedAt == nil || !got.DeletedAt.Equal(at) {
		t.Errorf("deleted_at = %v, want %v", got.DeletedAt, at)
	}
}

func TestRestoreRefusesATaskRestoredMeanwhile(t *testing.T) {
	te := newStoreEnv(t)
	p := te.addJSON("p")
	te.ok("rm", p.ID)
	te.meanwhile(changeTask(p.ULID, func(t *model.Task) { t.DeletedAt = nil }))
	if code, _, stderr := te.run("restore", p.ID); code != 1 || !strings.Contains(stderr, "not deleted") {
		t.Errorf("restore: exit %d, stderr %q", code, stderr)
	}
}

func TestPurgeRefusesATaskRestoredMeanwhile(t *testing.T) {
	te := newStoreEnv(t)
	p := te.addJSON("p")
	te.ok("rm", p.ID)
	te.meanwhile(changeTask(p.ULID, func(t *model.Task) { t.DeletedAt = nil }))
	if code, _, stderr := te.run("purge", p.ID, "--yes"); code != 1 || !strings.Contains(stderr, "not deleted") {
		t.Errorf("purge: exit %d, stderr %q", code, stderr)
	}
	if got := te.task(p.ULID); got.DeletedAt != nil {
		t.Error("the task restored meanwhile is changed")
	}
}

func TestMvSeesAWorkspaceMoveMeanwhile(t *testing.T) {
	te := newStoreEnv(t)
	var api workspaceJSON
	te.wsJSON(&api, "add", "api", "--prefix", "API")
	apiULID := mustULID(t, api.ULID)
	p := te.addJSON("p", "--workspace", "me")
	c := te.addJSON("c", "--parent", p.ID)
	// Another process moves p and its child to api; mv then moves p back.
	te.meanwhile(func(tx *store.Tx) error {
		for _, u := range []string{p.ULID, c.ULID} {
			if err := changeTask(u, func(t *model.Task) { t.Workspace = apiULID })(tx); err != nil {
				return err
			}
		}
		return nil
	})
	te.ok("mv", p.ULID, "--workspace", "me")
	if got := te.task(c.ULID); got.Workspace == apiULID || !strings.HasPrefix(got.ID, "ME-") {
		t.Errorf("the child is left in workspace %s with id %s", got.Workspace, got.ID)
	}
}

func TestMvParentInAnotherWorkspaceMovesAChildAddedMeanwhile(t *testing.T) {
	te := newStoreEnv(t)
	te.ok("workspace", "add", "api", "--prefix", "API")
	q := te.addJSON("q", "--workspace", "api")
	p := te.addJSON("p", "--workspace", "me")
	var c model.ULID
	te.meanwhile(putChild(p.ULID, &c))
	te.ok("mv", p.ID, "--parent", q.ID)
	if got := te.task(c.String()); !strings.HasPrefix(got.ID, "API-") {
		t.Errorf("the child added meanwhile: id %s", got.ID)
	}
}

func TestWorkspaceSubjectsSeeARenameMeanwhile(t *testing.T) {
	te := newStoreEnv(t)
	me := mustULID(t, te.addJSON("a").Workspace.ULID)
	var tmp workspaceJSON
	te.wsJSON(&tmp, "add", "tmp", "--prefix", "TMP")
	rename := func(ulid model.ULID, name string) func(*store.Tx) error {
		return func(tx *store.Tx) error {
			w, err := tx.Workspace(ulid)
			if err != nil {
				return err
			}
			w.Name = name
			return tx.PutWorkspace(w)
		}
	}
	te.meanwhile(rename(me, "mid"))
	te.ok("workspace", "rename", "me", "last")
	if subj, _ := te.lastCommit(); subj != "hedgehug: workspace rename mid → last (ME)" {
		t.Errorf("rename subject = %q", subj)
	}
	te.meanwhile(rename(me, "other"))
	te.ok("workspace", "prefix", "last", "NEW")
	if subj, _ := te.lastCommit(); subj != "hedgehug: workspace prefix other ME → NEW" {
		t.Errorf("prefix subject = %q", subj)
	}
	te.meanwhile(rename(mustULID(t, tmp.ULID), "tmp2"))
	te.ok("workspace", "rm", "tmp")
	if subj, _ := te.lastCommit(); subj != "hedgehug: workspace rm tmp2 (TMP)" {
		t.Errorf("rm subject = %q", subj)
	}
}

func TestWorkspacePrefixToAPrefixSetMeanwhileChangesNothing(t *testing.T) {
	te := newStoreEnv(t)
	te.addJSON("p")
	var ls []map[string]any
	te.wsJSON(&ls, "ls")
	ulid, err := model.ParseULID(ls[0]["ulid"].(string))
	if err != nil {
		t.Fatal(err)
	}
	var set time.Time
	te.meanwhile(func(tx *store.Tx) error {
		w, err := tx.Workspace(ulid)
		if err != nil {
			return err
		}
		w.IDPrefix = "NEW"
		w.PrefixSet = time.Now()
		set = w.PrefixSet
		return tx.PutWorkspace(w)
	})
	before := len(te.log("me"))
	out := te.ok("workspace", "prefix", "me", "NEW")
	if !strings.HasPrefix(out, "No changes") || strings.Contains(out, "→") {
		t.Errorf("prefix =\n%s", out)
	}
	// Only the commit made meanwhile is new.
	if n := len(te.log("me")); n != before+1 {
		t.Errorf("commits = %d, want %d", n, before+1)
	}
	if subject, _ := te.lastCommit(); strings.Contains(subject, "workspace prefix") {
		t.Errorf("last commit = %q, want the one made meanwhile", subject)
	}
	s, err := store.Open(te.storeDir("me"))
	if err != nil {
		t.Fatal(err)
	}
	w, err := s.Workspace(ulid)
	if err != nil {
		t.Fatal(err)
	}
	if w.IDPrefix != "NEW" || !w.PrefixSet.Equal(set) {
		t.Errorf("workspace = %s set %v, want NEW set %v", w.IDPrefix, w.PrefixSet, set)
	}
}
