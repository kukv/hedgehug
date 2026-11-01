package index

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

const (
	destStore  = "01JC0M3R5W7M2X9V4T6R5N8P1B"
	destStore2 = "01JC0M3R5W7M2X9V4T6R5N8P1C"
)

func forwardOf(ulid model.ULID, dest string) string {
	return "---\nulid: " + ulid.String() + "\nmoved_to: " + dest + "\n---\n"
}

// putWithID writes a task and returns it as read back, with its id.
func putWithID(t *testing.T, s *store.Store, task *model.Task, formerIDs ...string) *model.Task {
	t.Helper()
	task.FormerIDs = formerIDs
	put(t, s, task)
	got, err := s.Task(task.ULID)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestForwardsAreNotTasks(t *testing.T) {
	s, dir := newStore(t)
	kept := newTask("kept")
	moved := putWithID(t, s, newTask("moved"), "OLD-3")
	put(t, s, kept)
	commitRaw(t, s, dir, model.TaskPath(moved.ULID), forwardOf(moved.ULID, destStore))

	check := func(t *testing.T, ix *Index) {
		t.Helper()
		if _, ok := ix.Task(moved.ULID); ok {
			t.Error("the forward is in Tasks")
		}
		fs := ix.Forwards()
		if len(fs) != 1 {
			t.Fatalf("forwards = %+v", fs)
		}
		f := fs[0]
		if f.ULID != moved.ULID || f.MovedTo.String() != destStore || f.ID != moved.ID || !slices.Equal(f.FormerIDs, []string{"OLD-3"}) || f.Broken != "" {
			t.Errorf("forward = %+v, want id %s and former id OLD-3", f, moved.ID)
		}
		if g, ok := ix.Forward(moved.ULID); !ok || g != f {
			t.Errorf("Forward = %+v, %v", g, ok)
		}
		if _, err := ix.Resolve(moved.ID); !errors.Is(err, store.ErrNotFound) {
			t.Errorf("Resolve err = %v, want ErrNotFound", err)
		}
		for _, in := range []string{moved.ID, strings.ToLower(moved.ID), "OLD-3", "old-3", moved.ULID.String()} {
			if g, err := ix.ResolveForward(in); err != nil || g.ULID != moved.ULID {
				t.Errorf("ResolveForward(%q) = %+v, %v", in, g, err)
			}
		}
		if _, err := ix.ResolveForward(kept.ULID.String()); !errors.Is(err, store.ErrNotFound) {
			t.Errorf("ResolveForward(task) err = %v, want ErrNotFound", err)
		}
		if _, err := ix.ResolveForward(""); !errors.Is(err, store.ErrNotFound) {
			t.Errorf("ResolveForward(\"\") err = %v, want ErrNotFound", err)
		}
		n, err := ix.IDNumbers()
		if err != nil {
			t.Fatal(err)
		}
		if _, num, _ := model.ParseTaskID(moved.ID); n["TST"] < num || n["OLD"] != 3 {
			t.Errorf("IDNumbers = %v, want TST >= %d and OLD 3", n, num)
		}
	}

	t.Run("open", func(t *testing.T) { check(t, mustOpen(t, s)) })
	t.Run("from the cache", func(t *testing.T) { check(t, mustOpen(t, s)) })
	t.Run("without the cache", func(t *testing.T) {
		if err := os.Remove(filepath.Join(dir, ".hedgehug", "local", cacheName)); err != nil {
			t.Fatal(err)
		}
		check(t, mustOpen(t, s))
	})
}

func TestRefreshTakesInAForward(t *testing.T) {
	s, dir := newStore(t)
	moved := putWithID(t, s, newTask("moved"))
	ix := mustOpen(t, s)
	commitRaw(t, s, dir, model.TaskPath(moved.ULID), forwardOf(moved.ULID, destStore))
	if err := ix.Refresh(); err != nil {
		t.Fatal(err)
	}
	if _, ok := ix.Task(moved.ULID); ok {
		t.Error("the forward is still a task")
	}
	if f, ok := ix.Forward(moved.ULID); !ok || f.ID != moved.ID {
		t.Errorf("forward = %+v, %v; want id %s", f, ok, moved.ID)
	}
	// A forward that is removed leaves the index.
	if _, err := s.Update(store.Op{Name: "purge", Author: testAuthor}, func(tx *store.Tx) error { return tx.RemoveTask(moved.ULID) }); err != nil {
		t.Fatal(err)
	}
	if err := ix.Refresh(); err != nil {
		t.Fatal(err)
	}
	if len(ix.Forwards()) != 0 {
		t.Errorf("forwards = %+v, want none", ix.Forwards())
	}
}

func TestForwardAfterForwardKeepsTheID(t *testing.T) {
	s, dir := newStore(t)
	moved := putWithID(t, s, newTask("moved"))
	commitRaw(t, s, dir, model.TaskPath(moved.ULID), forwardOf(moved.ULID, destStore))
	commitRaw(t, s, dir, model.TaskPath(moved.ULID), forwardOf(moved.ULID, destStore2))
	f, ok := mustOpen(t, s).Forward(moved.ULID)
	if !ok || f.ID != moved.ID || f.MovedTo.String() != destStore2 {
		t.Errorf("forward = %+v, %v", f, ok)
	}
}

func TestForwardWithoutHistoryHasNoID(t *testing.T) {
	s, dir := newStore(t)
	ulid := model.NewULID()
	commitRaw(t, s, dir, model.TaskPath(ulid), forwardOf(ulid, destStore))
	f, ok := mustOpen(t, s).Forward(ulid)
	if !ok || f.ID != "" || len(f.FormerIDs) != 0 || f.Broken != "" {
		t.Errorf("forward = %+v, %v", f, ok)
	}
}

func TestBrokenForward(t *testing.T) {
	s, dir := newStore(t)
	ulid := model.NewULID()
	commitRaw(t, s, dir, model.TaskPath(ulid), "---\nulid: "+ulid.String()+"\nmoved_to: "+destStore+"\ntitle: x\n---\n")
	ix := mustOpen(t, s)
	f, ok := ix.Forward(ulid)
	if !ok || f.Broken == "" || !strings.Contains(f.Broken, "title") {
		t.Fatalf("forward = %+v, %v", f, ok)
	}
	ps := ix.Problems(ulid)
	if len(ps) != 1 || ps[0].Code != CodeBroken {
		t.Errorf("problems = %+v, want one broken", ps)
	}
}
