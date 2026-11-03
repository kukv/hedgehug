package index

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/history"
)

// A cold build finds the ids of several forwards in one walk, and the answer
// is the one the search of each alone gives: a plain forward, one that was
// forwarded twice, one with former ids, and one with no history of a task.
func TestColdBuildResolvesSeveralForwardsInOnePass(t *testing.T) {
	t.Parallel()
	s, dir := newStore(t)
	a := putWithID(t, s, newTask("a"), "OLD-1")
	b := putWithID(t, s, newTask("b"))
	c := putWithID(t, s, newTask("c"))
	put(t, s, newTask("stays"))
	commitRaw(t, s, dir, model.TaskPath(a.ULID), forwardOf(a.ULID, destStore))
	commitRaw(t, s, dir, model.TaskPath(c.ULID), forwardOf(c.ULID, destStore))
	commitRaw(t, s, dir, model.TaskPath(b.ULID), forwardOf(b.ULID, destStore))
	commitRaw(t, s, dir, model.TaskPath(b.ULID), forwardOf(b.ULID, destStore2)) // forward after forward
	lone := model.NewULID()
	commitRaw(t, s, dir, model.TaskPath(lone), forwardOf(lone, destStore))

	ix := mustOpen(t, s)
	for _, want := range []*model.Task{a, b, c} {
		f, ok := ix.Forward(want.ULID)
		if !ok || f.ID != want.ID || !slices.Equal(f.FormerIDs, want.FormerIDs) {
			t.Errorf("forward of %s = %+v, %v; want id %s former %v", want.Title, f, ok, want.ID, want.FormerIDs)
		}
		one, err := ix.lastTask(want.ULID)
		if err != nil || one == nil || one.ID != want.ID {
			t.Errorf("lastTask(%s) = %+v, %v", want.Title, one, err)
		}
	}
	if f, ok := ix.Forward(b.ULID); !ok || f.MovedTo.String() != destStore2 {
		t.Errorf("b moved_to = %+v", f)
	}
	if f, ok := ix.Forward(lone); !ok || f.ID != "" || len(f.FormerIDs) != 0 {
		t.Errorf("a forward with no history = %+v, %v", f, ok)
	}
}

// Comments written after the last edit of a task and before it moved are not
// changes of the task file: the id of the forward is still the task's.
func TestForwardIDIgnoresCommitsOfOtherFiles(t *testing.T) {
	t.Parallel()
	s, dir := newStore(t)
	a := putWithID(t, s, newTask("a"), "OLD-1")
	commitRaw(t, s, dir, model.CommentPath(a.ULID, model.NewULID()), "---\n---\nnote\n")
	put(t, s, newTask("other"))
	commitRaw(t, s, dir, model.TaskPath(a.ULID), forwardOf(a.ULID, destStore))

	ix := mustOpen(t, s)
	f, ok := ix.Forward(a.ULID)
	if !ok || f.ID != a.ID || !slices.Equal(f.FormerIDs, a.FormerIDs) {
		t.Errorf("forward = %+v, %v; want id %s former %v", f, ok, a.ID, a.FormerIDs)
	}
	if one, err := ix.lastTask(a.ULID); err != nil || one == nil || one.ID != a.ID {
		t.Errorf("lastTask = %+v, %v", one, err)
	}
}

// The history index is a cache: a commit it lacks, or an index that cannot be
// opened, still gives the last change by walking; and it is opened once for
// all the lookups of an update.
func TestLastChangerFallsBackToTheWalk(t *testing.T) {
	t.Parallel()
	s, dir := newStore(t)
	a := putWithID(t, s, newTask("a"))
	stale, err := history.Open(s, history.Options{}) // before the next commit
	if err != nil {
		t.Fatal(err)
	}
	commitRaw(t, s, dir, model.TaskPath(a.ULID), rawTask(a.ULID, "title: changed\n"))
	head, err := s.Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	ix := mustOpen(t, s)
	opens := 0
	for name, open := range map[string]func() (*history.History, error){
		"commit not in the index": func() (*history.History, error) { opens++; return stale, nil },
		"index cannot be opened":  func() (*history.History, error) { opens++; return nil, errors.New("broken") },
	} {
		opens = 0
		ix.openHistory = open
		lc := ix.lastChanger()
		for range 3 {
			e, ok, err := lc(head, model.TaskPath(a.ULID))
			if err != nil || !ok || e.Hash != head {
				t.Errorf("%s: last change = %v, %v, %v; want %s", name, e.Hash, ok, err, head)
			}
		}
		if opens != 1 {
			t.Errorf("%s: opened %d times, want 1", name, opens)
		}
	}
	// lastTask shares one lastChanger between its calls.
	opens, ix.lc = 0, nil
	for range 2 {
		if _, err := ix.lastTask(a.ULID); err != nil {
			t.Fatal(err)
		}
	}
	if opens != 1 {
		t.Errorf("lastTask opened the index %d times, want 1", opens)
	}
}
