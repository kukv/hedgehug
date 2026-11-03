package historytest_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/history"
	"github.com/kukv/hedgehug/internal/history/historytest"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/testenv"
)

func TestMain(m *testing.M) { testenv.Main(m) }

func TestSynth(t *testing.T) { //nolint:paralleltest // sets the environment
	t.Setenv("HH_BENCH_CACHE", t.TempDir())
	const commits, tasks = 1000, 50
	dir := historytest.Synth(t, commits, tasks, 7)
	if again := historytest.Synth(t, commits, tasks, 7); again != dir {
		t.Errorf("a second call gave %s, want the store of the first, %s", again, dir)
	}

	s, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Repo().Close() }()
	st, err := s.Repo().Status()
	if err != nil || len(st) != 0 {
		t.Errorf("status = %v, %v; want a clean work tree", st, err)
	}
	ix, err := index.Open(s)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(ix.Tasks()); got != tasks {
		t.Errorf("the index has %d tasks, want %d", got, tasks)
	}
	for _, e := range ix.Tasks() {
		if p := ix.Problems(e.ULID); len(p) != 0 {
			t.Errorf("%s has problems: %v", e.ID, p)
		}
	}
	h, err := history.Open(s, history.Options{})
	if err != nil {
		t.Fatal(err)
	}
	all := h.All()
	// the commits made, the init commit, and nothing else
	if len(all) != commits+1 {
		t.Errorf("the history has %d commits, want %d", len(all), commits+1)
	}
	merges, mine := 0, 0
	for _, c := range all {
		if c.Op == "merge" {
			merges++
		}
	}
	for _, c := range all[:historytest.UndoWindow-1] {
		if c.Author == historytest.Accounts[0].ID() {
			mine++
		}
	}
	if merges == 0 {
		t.Error("no merge was made")
	}
	if mine != 0 {
		t.Errorf("%d of the newest %d commits are Alice's, want none", mine, historytest.UndoWindow-1)
	}
	if c := all[historytest.UndoWindow-1]; c.Author != historytest.Accounts[0].ID() || c.Op != "edit" {
		t.Errorf("commit %d back = %+v, want Alice's edit", historytest.UndoWindow, c)
	}
}
