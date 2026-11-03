package history_test

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/history"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// TestInterleavedHandlesShareTheIndex has two handles of one store write in
// turn and open the history after each write, so that they append to the same
// index file. A third handle then reads what a build from nothing reads.
func TestInterleavedHandlesShareTheIndex(t *testing.T) {
	t.Parallel()
	a := newStore(t)
	b, err := store.Open(a.Dir())
	if err != nil {
		t.Fatal(err)
	}
	tasks := []model.ULID{model.NewULID(), model.NewULID(), model.NewULID()}
	for i := range 6 {
		s := a
		if i%2 == 1 {
			s = b
		}
		writeTask(t, s, tasks[i%len(tasks)], "t"+string(rune('a'+i)))
		if _, err := history.Open(s, history.Options{}); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}

	c, err := store.Open(a.Dir())
	if err != nil {
		t.Fatal(err)
	}
	got, err := history.Open(c, history.Options{})
	if err != nil {
		t.Fatal(err)
	}
	removeIndex(t, c)
	want, err := history.Open(c, history.Options{})
	if err != nil {
		t.Fatal(err)
	}

	same := func(label string, g, w []history.Commit) {
		t.Helper()
		if !slices.EqualFunc(g, w, func(x, y history.Commit) bool {
			return x.Hash == y.Hash && x.Task == y.Task && x.Author == y.Author && x.Op == y.Op && x.Workspace == y.Workspace
		}) {
			t.Errorf("%s\n got %v\nwant %v", label, g, w)
		}
	}
	if len(want.All()) < 7 {
		t.Fatalf("the history has %d commits, want the 6 writes and the first", len(want.All()))
	}
	same("All", got.All(), want.All())
	for _, u := range tasks {
		same("Task "+u.String(), got.Task(u), want.Task(u))
	}
}
