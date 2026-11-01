package index

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
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
