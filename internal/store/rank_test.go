package store

import (
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
)

func rankTask(ws *model.Workspace, title string, r model.Rank) *model.Task {
	return &model.Task{ULID: model.NewULID(), Workspace: ws.ULID, Kind: "task", Title: title, Status: "todo", Rank: r}
}

// writeShared puts tasks on disk as a merge could leave them, bypassing
// Update so that they keep the rank they share.
func writeShared(t *testing.T, s *Store, tasks ...*model.Task) {
	t.Helper()
	for _, task := range tasks {
		task.Created, task.Updated = testNow, testNow
		b, err := task.Encode()
		if err != nil {
			t.Fatal(err)
		}
		mustWrite(t, s, model.TaskPath(task.ULID), string(b))
	}
}

func TestRerankOnWrite(t *testing.T) {
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	before, a, b, c, after := rankTask(ws, "before", "a0"), rankTask(ws, "a", "a5"), rankTask(ws, "b", "a5"), rankTask(ws, "c", "a5"), rankTask(ws, "after", "a9")
	before.ID, a.ID, b.ID, c.ID, after.ID = "TST-1", "TST-2", "TST-3", "TST-4", "TST-5"
	writeShared(t, s, before, a, b, c, after) // a, b, c share a5: their order is a, b, c
	mustPut(t, s, a)                          // a is first of the a5 group: goes between a0 and a5
	if a.Rank <= "a0" || a.Rank >= "a5" {
		t.Errorf("a.Rank = %s", a.Rank)
	}
	mustPut(t, s, c) // c is not first (b is): goes between a5 and a9
	if c.Rank <= "a5" || c.Rank >= "a9" {
		t.Errorf("c.Rank = %s", c.Rank)
	}
	if got, _ := s.Task(b.ULID); got.Rank != "a5" {
		t.Errorf("b was rewritten: %s", got.Rank)
	}
}

func TestRerankKeepsUniqueRank(t *testing.T) {
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	x := rankTask(ws, "x", "a5")
	mustPut(t, s, x)
	x.Title = "y"
	mustPut(t, s, x)
	if x.Rank != "a5" {
		t.Errorf("rank changed without a duplicate: %s", x.Rank)
	}
}

// Tasks put together that share a rank keep their order by id.
func TestRerankTasksOfOneOperation(t *testing.T) {
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	a, b, c := rankTask(ws, "a", "a5"), rankTask(ws, "b", "a5"), rankTask(ws, "c", "a5")
	mustPut(t, s, a, b, c) // TST-2..4 in this order
	if a.Rank >= b.Rank || b.Rank >= c.Rank {
		t.Errorf("ranks = %s, %s, %s; want them to increase", a.Rank, b.Rank, c.Rank)
	}
}

func TestBetweenBeforeEverything(t *testing.T) {
	r, err := model.Between("", "a5")
	if err != nil || r >= "a5" {
		t.Errorf("Between(\"\", a5) = %q, %v", r, err)
	}
}

// Only live siblings of the same parent and workspace share a rank with a
// task, and a task put in the operation counts as the operation leaves it.
func TestRerankSiblings(t *testing.T) {
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	other := &model.Workspace{ULID: model.NewULID(), Name: "other", IDPrefix: "OTH"}
	if _, err := s.Update(Op{Name: "workspace", Author: testAuthor}, func(tx *Tx) error { return tx.PutWorkspace(other) }); err != nil {
		t.Fatal(err)
	}
	p := rankTask(ws, "p", "a1")
	child := rankTask(ws, "child", "a5")
	child.Parent = &p.ULID
	deleted := rankTask(ws, "deleted", "a5")
	now := time.Now()
	deleted.DeletedAt = &now
	elsewhere := rankTask(other, "elsewhere", "a5")
	x := rankTask(ws, "x", "a5")
	mustPut(t, s, p, child, deleted, elsewhere, x)
	if x.Rank != "a5" || child.Rank != "a5" || elsewhere.Rank != "a5" {
		t.Errorf("ranks = %s, %s, %s; none shares a rank with a sibling", x.Rank, child.Rank, elsewhere.Rank)
	}

	// x moves under p in the operation, so y at the top level is alone.
	y := rankTask(ws, "y", "a5")
	x.Parent = &p.ULID
	mustPut(t, s, x, y)
	if y.Rank != "a5" {
		t.Errorf("y.Rank = %s, want a5: x is no longer its sibling", y.Rank)
	}
	if x.Rank == "a5" {
		t.Error("x shares a rank with child, its sibling now")
	}
}

func TestRerankIsRestoredWhenTheOperationFails(t *testing.T) {
	s := newTestStore(t)
	ws := testWorkspace(t, s)
	a, b := rankTask(ws, "a", "a5"), rankTask(ws, "b", "a5")
	a.ID, b.ID = "TST-1", "TST-2"
	writeShared(t, s, a, b)
	s.commit = func(gitrepo.Commit) error { return errors.New("commit fails") }
	if err := put(s, b); err == nil {
		t.Fatal("the commit did not fail")
	}
	if b.Rank != "a5" {
		t.Errorf("b.Rank = %s after a failed write, want a5", b.Rank)
	}
}
