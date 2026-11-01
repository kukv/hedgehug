package store

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
)

func newTaskIn(ws *model.Workspace, title string) *model.Task {
	return &model.Task{ULID: model.NewULID(), Workspace: ws.ULID, Kind: "task", Title: title, Status: "todo"}
}

func TestAssignIDs(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	a, b := newTaskIn(ws, "a"), newTaskIn(ws, "b")
	mustPut(t, s, a, b) // one operation: numbers given in it count
	c := newTaskIn(ws, "c")
	mustPut(t, s, c)
	if a.ID != "TST-1" || b.ID != "TST-2" || c.ID != "TST-3" {
		t.Fatalf("ids = %s %s %s", a.ID, b.ID, c.ID)
	}
	if got, _ := s.Task(c.ULID); got.ID != "TST-3" {
		t.Errorf("file id = %s", got.ID)
	}
}

func TestAssignIDsCountsIDsGivenInTheOperation(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	a, b := newTaskIn(ws, "a"), newTaskIn(ws, "b")
	b.ID = "TST-5"
	mustPut(t, s, a, b)
	if a.ID != "TST-6" {
		t.Errorf("id = %s, want TST-6", a.ID)
	}
}

func TestCommitSubjectUsesNewID(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	x := newTaskIn(testWorkspace(t, s), "x")
	if _, err := s.Update(Op{Name: "add", Task: x.ULID, Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(x) }); err != nil {
		t.Fatal(err)
	}
	if subj := strings.SplitN(headLog(t, s.dir)[0].Message, "\n", 2)[0]; subj != "hedgehug: add TST-1 x" {
		t.Errorf("subject = %q", subj)
	}
}

func TestAssignIDsCountsFormerAndNumbers(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	old := newTaskIn(ws, "old")
	old.ID, old.FormerIDs = "TST-1", []string{"TST-20"}
	mustPut(t, s, old)
	gone := newTaskIn(ws, "gone")
	gone.ID = "TST-30"
	mustPut(t, s, gone)
	if _, err := s.Update(Op{Name: "purge", Author: testAuthor}, func(tx *Tx) error { return tx.RemoveTask(gone.ULID) }); err != nil {
		t.Fatal(err)
	}
	n := newTaskIn(ws, "new")
	mustPut(t, s, n)
	if n.ID != "TST-31" {
		t.Errorf("id = %s, want TST-31 (after the purged TST-30)", n.ID)
	}
}

func TestRenumberOnPrefixMismatch(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	api := &model.Workspace{ULID: model.NewULID(), Name: "api", IDPrefix: "API"}
	if _, err := s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error { return tx.PutWorkspace(api) }); err != nil {
		t.Fatal(err)
	}
	x := newTaskIn(ws, "x")
	mustPut(t, s, x)
	x.Workspace = api.ULID
	mustPut(t, s, x)
	if x.ID != "API-1" || !slices.Equal(x.FormerIDs, []string{"TST-1"}) {
		t.Errorf("id = %s, former = %v", x.ID, x.FormerIDs)
	}
	y := newTaskIn(ws, "y")
	mustPut(t, s, y)
	if y.ID != "TST-2" {
		t.Errorf("TST-1 was reused: %s", y.ID)
	}
}

func TestAssignIDsRefusesABadPrefix(t *testing.T) {
	t.Parallel()
	for _, prefix := range []string{"me", ""} {
		s := newTestStore(t)
		ws := testWorkspace(t, s)
		ws.IDPrefix = prefix
		b, err := ws.Encode()
		if err != nil {
			t.Fatal(err)
		}
		mustWrite(t, s, model.WorkspacePath(ws.ULID), string(b)) // as if edited by hand
		err = put(s, newTaskIn(ws, "x"))
		if err == nil || !strings.Contains(err.Error(), model.WorkspacePath(ws.ULID)) || !strings.Contains(err.Error(), "hedgehug workspace prefix") {
			t.Errorf("prefix %q: err = %v", prefix, err)
		}
	}
}

func TestIDNumbersHook(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	calls := 0
	s.SetIDNumbers(func() (map[string]int, error) {
		calls++
		return map[string]int{"TST": 41}, nil
	})
	x := newTaskIn(ws, "x")
	mustPut(t, s, x)
	if x.ID != "TST-42" || calls != 1 {
		t.Errorf("id = %s, calls = %d", x.ID, calls)
	}
	mustPut(t, s, x) // has an id that fits: no count
	if calls != 1 {
		t.Errorf("counted for a task that keeps its id")
	}
}

func TestConcurrentAddsGetDistinctIDs(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	other, err := Open(s.dir) // a second handle: only the file lock serializes them
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	tasks := make([]*model.Task, 20)
	for i := range tasks {
		tasks[i] = newTaskIn(ws, fmt.Sprint(i))
		h := s
		if i%2 == 1 {
			h = other
		}
		wg.Add(1)
		go func(h *Store, task *model.Task) {
			defer wg.Done()
			if _, err := h.Update(Op{Name: "add", Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(task) }); err != nil {
				t.Error(err)
			}
		}(h, tasks[i])
	}
	wg.Wait()
	seen := map[string]bool{}
	for _, task := range tasks {
		if seen[task.ID] {
			t.Errorf("id %s given twice", task.ID)
		}
		seen[task.ID] = true
	}
}

func TestAssignIDsCountsBrokenFiles(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	mustWrite(t, s, model.TaskPath(model.NewULID()), "---\nid: TST-7\ntitle: [unclosed\n---\nsee OLD-3\n")
	x := newTaskIn(ws, "x")
	mustPut(t, s, x)
	if x.ID != "TST-8" {
		t.Errorf("id = %s, want TST-8 (after TST-7 of the broken file)", x.ID)
	}
}

func TestUpdateRestoresIDsOnFailure(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	x := newTaskIn(ws, "x")
	x.ID, x.FormerIDs = "OLD-3", []string{"OLD-1"}
	x.Status = "no-such-status"
	if err := put(s, x); err == nil {
		t.Fatal("the write was not refused")
	}
	if x.ID != "OLD-3" || !slices.Equal(x.FormerIDs, []string{"OLD-1"}) {
		t.Errorf("after a refused write: id = %s, former = %v", x.ID, x.FormerIDs)
	}

	y := newTaskIn(ws, "y")
	s.commit = func(gitrepo.Commit) (plumbing.Hash, error) { return plumbing.ZeroHash, errors.New("commit fails") }
	if err := put(s, y); err == nil {
		t.Fatal("the commit did not fail")
	}
	if y.ID != "" || y.FormerIDs != nil {
		t.Errorf("after a failed commit: id = %s, former = %v", y.ID, y.FormerIDs)
	}
}
