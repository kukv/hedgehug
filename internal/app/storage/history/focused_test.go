package history_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/history"
	"github.com/kukv/hedgehug/internal/app/storage/store"
)

func removeIndex(t *testing.T, s *store.Store) {
	t.Helper()
	if err := os.Remove(filepath.Join(s.Dir(), ".hedgehug", "local", "history")); err != nil {
		t.Fatal(err)
	}
}

func TestIncrementalMergeEqualsFreshBuild(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	p := newPlumber(t, s)
	u := model.NewULID()
	tp := model.TaskPath(u)
	at := func(n int) time.Time { return t0.Add(time.Duration(n) * time.Minute) }
	file := func(content string) map[string]string { return map[string]string{tp: content} }
	r := p.commit(at(0), nil, file("0"))
	a := p.commit(at(1), []plumbing.Hash{r}, file("a"))
	setMain(t, s, a)
	if _, err := history.Open(s, history.Options{}); err != nil { // saved
		t.Fatal(err)
	}
	// A diamond on top: b beside a, and a merge of both.
	b := p.commit(at(2), []plumbing.Hash{r}, file("b"))
	m := p.commit(at(3), []plumbing.Hash{a, b}, file("m"))
	setMain(t, s, m)
	inc, err := history.Open(s, history.Options{})
	if err != nil {
		t.Fatal(err)
	}
	removeIndex(t, s)
	fresh, err := history.Open(s, history.Options{})
	if err != nil {
		t.Fatal(err)
	}
	gi, gf := inc.All(), fresh.All()
	if len(gi) != 4 || len(gf) != 4 {
		t.Fatalf("%d and %d commits, want 4", len(gi), len(gf))
	}
	for i := range gi {
		if gi[i].Hash != gf[i].Hash || gi[i].Gen != gf[i].Gen {
			t.Fatalf("commit %d: %v gen %d, fresh %v gen %d", i, gi[i].Hash, gi[i].Gen, gf[i].Hash, gf[i].Gen)
		}
	}
	if gi[0].Hash != m || gi[0].Gen != 3 {
		t.Fatalf("merge %v gen %d, want %v gen 3", gi[0].Hash, gi[0].Gen, m)
	}
}

func TestLastChangeSkipsCommitsOutsideFrom(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	p := newPlumber(t, s)
	at := func(n int) time.Time { return t0.Add(time.Duration(n) * time.Minute) }
	file := func(x, y string) map[string]string { return map[string]string{"x": x, "y": y} }
	r := p.commit(at(0), nil, file("0", "0"))
	s1 := p.commit(at(1), []plumbing.Hash{r}, file("1", "0"))
	s2 := p.commit(at(2), []plumbing.Hash{r}, file("2", "2")) // newer than s1, on the other branch
	m := p.commit(at(3), []plumbing.Hash{s1, s2}, file("3", "2"))
	setMain(t, s, m)
	h, err := history.Open(s, history.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		path string
		from plumbing.Hash
		want plumbing.Hash
	}{
		{"x from s1 skips the newer s2 and the merge", "x", s1, s1},
		{"x from s2 skips the merge", "x", s2, s2},
		{"y from s1 skips the newer s2", "y", s1, r},
		{"y from s2", "y", s2, s2},
		{"x from r", "x", r, r},
	} {
		got, ok, err := h.LastChange(c.path, c.from)
		if err != nil || !ok || got.Hash != c.want {
			t.Errorf("%s: %v, %v, %v; want %v", c.name, got.Hash, ok, err, c.want)
		}
	}
}

func TestSaveIsDeterministic(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	for range 20 { // many paths, so that a map order would show
		writeTask(t, s, model.NewULID(), "t")
	}
	if _, err := history.Open(s, history.Options{}); err != nil {
		t.Fatal(err)
	}
	first, err := s.ReadLocal("history")
	if err != nil {
		t.Fatal(err)
	}
	for i := range 5 {
		removeIndex(t, s)
		if _, err := history.Open(s, history.Options{}); err != nil {
			t.Fatal(err)
		}
		again, err := s.ReadLocal("history")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(first, again) {
			t.Fatalf("save %d differs from the first", i)
		}
	}
}

func TestTaskIncludesWorklogs(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	p := newPlumber(t, s)
	u, w := model.NewULID(), model.NewULID()
	at := func(n int) time.Time { return t0.Add(time.Duration(n) * time.Minute) }
	r := p.commit(at(0), nil, map[string]string{model.TaskPath(u): "t"})
	c1 := p.commit(at(1), []plumbing.Hash{r}, map[string]string{model.TaskPath(u): "t", model.WorklogPath(u, w): "w1"})
	c2 := p.commit(at(2), []plumbing.Hash{c1}, map[string]string{model.TaskPath(u): "t", model.WorklogPath(u, w): "w2"})
	setMain(t, s, c2)
	h, err := history.Open(s, history.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := hashes(h.Task(u)), []plumbing.Hash{c2, c1, r}; !slices.Equal(got, want) {
		t.Fatalf("Task: %v, want %v", got, want)
	}
	if got := hashes(h.Path(model.TaskPath(u))); !slices.Equal(got, []plumbing.Hash{r}) {
		t.Fatalf("Path of the task file: %v", got)
	}
}

// A zone offset beyond ±18h is not trusted: the file is rebuilt.
func TestZoneOffsetOutOfRangeIsRebuilt(t *testing.T) {
	t.Parallel()
	for _, hours := range []int{18, 19} {
		s := newStore(t)
		p := newPlumber(t, s)
		r := p.commit(t0.In(time.FixedZone("", hours*3600)), nil, map[string]string{"x": "0"})
		setMain(t, s, r)
		if _, err := history.Open(s, history.Options{}); err != nil {
			t.Fatal(err)
		}
		built := false
		h, err := history.Open(s, history.Options{Progress: func(int, int) { built = true }})
		if err != nil {
			t.Fatal(err)
		}
		if built != (hours > 18) {
			t.Errorf("+%dh: rebuilt %v", hours, built)
		}
		if len(h.All()) != 1 {
			t.Errorf("+%dh: %d commits", hours, len(h.All()))
		}
	}
}
