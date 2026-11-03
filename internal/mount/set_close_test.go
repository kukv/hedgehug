package mount

import (
	"testing"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/index"
)

// Close lets go of the repository of every store of the set, and the set is
// used again after it.
func TestSetCloseClosesEveryRepo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	stores := newStoresIn(t, root, "one", "two")
	tasks := putTasks(t, stores[0], 1, nil)
	c := &config.Config{Stores: stores}
	before := gitrepo.Live(root)
	set := Open(c, Options{})
	if got := gitrepo.Live(root); got != before+2 {
		t.Fatalf("live repos after Open = %d, want %d", got, before+2)
	}
	set.Close()
	if got := gitrepo.Live(root); got != before {
		t.Errorf("live repos after Close = %d, want %d", got, before)
	}
	m, _ := set.Get("one")
	if err := m.With(func(ix *index.Index) {
		if _, ok := ix.Task(tasks[0].ULID); !ok {
			t.Error("the task is not in the index of the set used after Close")
		}
	}); err != nil {
		t.Fatal(err)
	}
	set.Close()
}
