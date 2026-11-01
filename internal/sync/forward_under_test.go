package sync

import (
	"testing"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// filesUnderForwards looks below the directories of the forwards only: the
// comments of other tasks do not count.
func TestFilesUnderForwards(t *testing.T) {
	c := newCluster(t, 1)
	x := c.addTask(0, "x")
	y := c.addTask(0, "y")
	c.comment(0, y.ULID, "on y\n", model.NewULID())
	if _, err := c.stores[0].Update(c.op(0, "move-out", x.ULID), func(tx *store.Tx) error {
		return tx.MoveOut(store.MoveOut{To: model.NewULID(), Tasks: []model.ULID{x.ULID}})
	}); err != nil {
		t.Fatal(err)
	}
	under := func() bool {
		t.Helper()
		ix, err := index.Open(c.stores[0])
		if err != nil {
			t.Fatal(err)
		}
		if err := ix.Refresh(); err != nil {
			t.Fatal(err)
		}
		got, err := filesUnderForwards(ix, c.stores[0].Repo(), c.headTree(0))
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	if under() {
		t.Error("a comment on another task counts as under a forward")
	}
	c.comment(0, x.ULID, "after the move\n", model.NewULID())
	if !under() {
		t.Error("a comment under the forward is not found")
	}
}
