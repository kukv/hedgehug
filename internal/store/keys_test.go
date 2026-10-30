package store

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
)

func newTaskIn(ws *model.Workspace, title string) *model.Task {
	return &model.Task{ID: model.NewID(), Workspace: ws.ID, Kind: "task", Title: title, Status: "todo"}
}

func mustPut(t *testing.T, s *Store, tasks ...*model.Task) {
	t.Helper()
	if err := put(s, tasks...); err != nil {
		t.Fatal(err)
	}
}

func TestAssignKeys(t *testing.T) {
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	a, b := newTaskIn(ws, "a"), newTaskIn(ws, "b")
	mustPut(t, s, a, b) // one operation: numbers given in it count
	c := newTaskIn(ws, "c")
	mustPut(t, s, c)
	if a.Key != "TST-1" || b.Key != "TST-2" || c.Key != "TST-3" {
		t.Fatalf("keys = %s %s %s", a.Key, b.Key, c.Key)
	}
	if got, _ := s.Task(c.ID); got.Key != "TST-3" {
		t.Errorf("file key = %s", got.Key)
	}
}

func TestAssignKeysCountsKeysGivenInTheOperation(t *testing.T) {
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	a, b := newTaskIn(ws, "a"), newTaskIn(ws, "b")
	b.Key = "TST-5"
	mustPut(t, s, a, b)
	if a.Key != "TST-6" {
		t.Errorf("key = %s, want TST-6", a.Key)
	}
}

func TestCommitSubjectUsesNewKey(t *testing.T) {
	s := newTestStore(t)
	x := newTaskIn(testWorkspace(t, s), "x")
	if _, err := s.Update(Op{Name: "add", Task: x.ID, Author: testAuthor}, func(tx *Tx) error { return tx.PutTask(x) }); err != nil {
		t.Fatal(err)
	}
	if subj := strings.SplitN(headLog(t, s.dir)[0].Message, "\n", 2)[0]; subj != "hedgehug: add TST-1 x" {
		t.Errorf("subject = %q", subj)
	}
}

func TestAssignKeysCountsFormerAndNumbers(t *testing.T) {
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	old := newTaskIn(ws, "old")
	old.Key, old.FormerKeys = "TST-1", []string{"TST-20"}
	mustPut(t, s, old)
	gone := newTaskIn(ws, "gone")
	gone.Key = "TST-30"
	mustPut(t, s, gone)
	if _, err := s.Update(Op{Name: "purge", Author: testAuthor}, func(tx *Tx) error { return tx.RemoveTask(gone.ID) }); err != nil {
		t.Fatal(err)
	}
	n := newTaskIn(ws, "new")
	mustPut(t, s, n)
	if n.Key != "TST-31" {
		t.Errorf("key = %s, want TST-31 (after the purged TST-30)", n.Key)
	}
}

func TestRenumberOnPrefixMismatch(t *testing.T) {
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	api := &model.Workspace{ID: model.NewID(), Name: "api", KeyPrefix: "API"}
	if _, err := s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error { return tx.PutWorkspace(api) }); err != nil {
		t.Fatal(err)
	}
	x := newTaskIn(ws, "x")
	mustPut(t, s, x)
	x.Workspace = api.ID
	mustPut(t, s, x)
	if x.Key != "API-1" || !slices.Equal(x.FormerKeys, []string{"TST-1"}) {
		t.Errorf("key = %s, former = %v", x.Key, x.FormerKeys)
	}
	y := newTaskIn(ws, "y")
	mustPut(t, s, y)
	if y.Key != "TST-2" {
		t.Errorf("TST-1 was reused: %s", y.Key)
	}
}

func TestAssignKeysRefusesABadPrefix(t *testing.T) {
	for _, prefix := range []string{"me", ""} {
		s := newTestStore(t)
		ws := testWorkspace(t, s)
		ws.KeyPrefix = prefix
		b, err := ws.Encode()
		if err != nil {
			t.Fatal(err)
		}
		mustWrite(t, s, model.WorkspacePath(ws.ID), string(b)) // as if edited by hand
		err = put(s, newTaskIn(ws, "x"))
		if err == nil || !strings.Contains(err.Error(), model.WorkspacePath(ws.ID)) || !strings.Contains(err.Error(), "hedgehug workspace prefix") {
			t.Errorf("prefix %q: err = %v", prefix, err)
		}
	}
}

func TestKeyNumbersHook(t *testing.T) {
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	calls := 0
	s.SetKeyNumbers(func() (map[string]int, error) {
		calls++
		return map[string]int{"TST": 41}, nil
	})
	x := newTaskIn(ws, "x")
	mustPut(t, s, x)
	if x.Key != "TST-42" || calls != 1 {
		t.Errorf("key = %s, calls = %d", x.Key, calls)
	}
	mustPut(t, s, x) // has a key that fits: no count
	if calls != 1 {
		t.Errorf("counted for a task that keeps its key")
	}
}

func TestConcurrentAddsGetDistinctKeys(t *testing.T) {
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
		if seen[task.Key] {
			t.Errorf("key %s given twice", task.Key)
		}
		seen[task.Key] = true
	}
}

func TestAssignKeysCountsBrokenFiles(t *testing.T) {
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	mustWrite(t, s, model.TaskPath(model.NewID()), "---\nkey: TST-7\ntitle: [unclosed\n---\nsee OLD-3\n")
	x := newTaskIn(ws, "x")
	mustPut(t, s, x)
	if x.Key != "TST-8" {
		t.Errorf("key = %s, want TST-8 (after TST-7 of the broken file)", x.Key)
	}
}

func TestUpdateRestoresKeysOnFailure(t *testing.T) {
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	x := newTaskIn(ws, "x")
	x.Key, x.FormerKeys = "OLD-3", []string{"OLD-1"}
	x.Status = "no-such-status"
	if err := put(s, x); err == nil {
		t.Fatal("the write was not refused")
	}
	if x.Key != "OLD-3" || !slices.Equal(x.FormerKeys, []string{"OLD-1"}) {
		t.Errorf("after a refused write: key = %s, former = %v", x.Key, x.FormerKeys)
	}

	y := newTaskIn(ws, "y")
	s.commit = func(gitrepo.Commit) error { return errors.New("commit fails") }
	if err := put(s, y); err == nil {
		t.Fatal("the commit did not fail")
	}
	if y.Key != "" || y.FormerKeys != nil {
		t.Errorf("after a failed commit: key = %s, former = %v", y.Key, y.FormerKeys)
	}
}
