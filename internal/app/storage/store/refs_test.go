package store

import (
	"errors"
	"testing"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
)

// wantViolation checks that err is a ValidationError on task with a
// violation of code on field.
func wantViolation(t *testing.T, err error, task model.ULID, code, field string) {
	t.Helper()
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Task != task {
		t.Errorf("err = %v, want a ValidationError on %s", err, task)
		return
	}
	for _, v := range ve.Violations {
		if v.Code == code && v.Field == field {
			return
		}
	}
	t.Errorf("violations = %v, want %s on %s", ve.Violations, code, field)
}

// ulidList is a property value listing ULIDs.
func ulidList(ulids ...model.ULID) *yaml.Node {
	n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, ulid := range ulids {
		n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: ulid.String()})
	}
	return n
}

// settleKinds gives the tasks that have a parent, and the default kind of
// newTask (project), the kind a task has under that parent. A test that wants
// another kind sets it.
func settleKinds(s *Store, tasks []*model.Task) {
	by := map[model.ULID]*model.Task{}
	for _, t := range tasks {
		by[t.ULID] = t
	}
	seen := map[model.ULID]bool{} // a cycle of parents ends the walk
	var kindOf func(t *model.Task) string
	kindOf = func(t *model.Task) string {
		if seen[t.ULID] {
			return t.Kind
		}
		seen[t.ULID] = true
		if t.Parent != nil && t.Kind == "project" {
			parentKind := ""
			if p, ok := by[*t.Parent]; ok {
				parentKind = kindOf(p)
			} else if p, err := s.Task(*t.Parent); err == nil {
				parentKind = p.Kind
			}
			t.Kind = schema.DefaultKindUnder(parentKind)
		}
		return t.Kind
	}
	for _, t := range tasks {
		kindOf(t)
	}
}

func put(s *Store, tasks ...*model.Task) error {
	settleKinds(s, tasks)
	_, err := s.Update(Op{Name: "update", Author: testAuthor}, func(tx *Tx) error {
		for _, t := range tasks {
			if err := tx.PutTask(t); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

func TestUpdateMissingBlocker(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	task := newTask("x")
	task.BlockedBy = []model.ULID{model.NewULID()}
	wantViolation(t, put(s, task), task.ULID, schema.CodeMissingRef, "blocked_by")
}

func TestUpdateMissingRelationTarget(t *testing.T) {
	t.Parallel()
	for _, rel := range schema.Relations {
		s := newTestStore(t)
		task := newTask("x")
		rel.Set(task, []model.ULID{model.NewULID()})
		wantViolation(t, put(s, task), task.ULID, schema.CodeMissingRef, rel.Name)
	}
}

// A relation other than blocked_by that points at its own task is ref.self;
// blocked_by is a cycle.
func TestUpdateSelfRelation(t *testing.T) {
	t.Parallel()
	for _, rel := range schema.Relations {
		s := newTestStore(t)
		task := newTask("x")
		addTask(t, s, task)
		got, err := s.Task(task.ULID)
		if err != nil {
			t.Fatal(err)
		}
		rel.Set(got, []model.ULID{got.ULID})
		code := schema.CodeSelfRef
		if rel.Name == "blocked_by" {
			code = schema.CodeCycle
		}
		wantViolation(t, put(s, got), got.ULID, code, rel.Name)
	}
}

func TestUpdateKeepsOldMissingRef(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	blocker, task := newTask("blocker"), newTask("task")
	task.BlockedBy = []model.ULID{blocker.ULID}
	addTask(t, s, blocker)
	addTask(t, s, task)
	if _, err := s.Update(Op{Name: "purge", Author: testAuthor}, func(tx *Tx) error { return tx.RemoveTask(blocker.ULID) }); err != nil {
		t.Fatal(err)
	}
	got, err := s.Task(task.ULID)
	if err != nil {
		t.Fatal(err)
	}
	got.Title = "renamed"
	if err := put(s, got); err != nil {
		t.Errorf("editing a task with an old missing blocker failed: %v", err)
	}
}

func TestUpdateRefToDeletedTask(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	gone := newTask("gone")
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	gone.DeletedAt = &at
	addTask(t, s, gone)
	task := newTask("task")
	task.BlockedBy = []model.ULID{gone.ULID}
	if err := put(s, task); err != nil {
		t.Errorf("a reference to a deleted task was refused: %v", err)
	}
}

func TestUpdateRefToTaskRemovedInSameOp(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	blocker := newTask("blocker")
	addTask(t, s, blocker)
	task := newTask("task")
	task.BlockedBy = []model.ULID{blocker.ULID}
	_, err := s.Update(Op{Name: "update", Author: testAuthor}, func(tx *Tx) error {
		return errors.Join(tx.RemoveTask(blocker.ULID), tx.PutTask(task))
	})
	wantViolation(t, err, task.ULID, schema.CodeMissingRef, "blocked_by")
}

func TestUpdateRefToTaskMadeInSameOp(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	blocker, task := newTask("blocker"), newTask("task")
	task.BlockedBy = []model.ULID{blocker.ULID}
	if err := put(s, task, blocker); err != nil {
		t.Errorf("a reference to a task made in the same operation was refused: %v", err)
	}
}

func TestUpdateUnderBrokenParent(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	parent, child := newTask("parent"), newTask("child")
	child.Parent = &parent.ULID
	addTask(t, s, parent)
	addTask(t, s, child)
	mustWrite(t, s, model.TaskPath(parent.ULID), "not a task file\n")
	got, err := s.Task(child.ULID)
	if err != nil {
		t.Fatal(err)
	}
	got.Title = "renamed"
	if err := put(s, got); err != nil {
		t.Errorf("editing a task under a broken parent failed: %v", err)
	}
}

func TestUpdateRefusesNewCycles(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		field string
		build func(a, b, c *model.Task) (setup []*model.Task, write *model.Task)
	}{
		{"parent self", "parent", func(a, _, _ *model.Task) ([]*model.Task, *model.Task) {
			a.Parent = &a.ULID
			return nil, a
		}},
		{"parent move under own child", "parent", func(a, b, _ *model.Task) ([]*model.Task, *model.Task) {
			b.Parent = &a.ULID
			w := *a
			w.Parent = &b.ULID
			return []*model.Task{a, b}, &w
		}},
		{"blocked_by self", "blocked_by", func(a, _, _ *model.Task) ([]*model.Task, *model.Task) {
			a.BlockedBy = []model.ULID{a.ULID}
			return nil, a
		}},
		{"blocked_by two", "blocked_by", func(a, b, _ *model.Task) ([]*model.Task, *model.Task) {
			b.BlockedBy = []model.ULID{a.ULID}
			w := *a
			w.BlockedBy = []model.ULID{b.ULID}
			return []*model.Task{a, b}, &w
		}},
		{"blocked_by three", "blocked_by", func(a, b, c *model.Task) ([]*model.Task, *model.Task) {
			b.BlockedBy = []model.ULID{c.ULID}
			c.BlockedBy = []model.ULID{a.ULID}
			w := *a
			w.BlockedBy = []model.ULID{b.ULID}
			return []*model.Task{a, c, b}, &w // each after what it points at
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestStore(t)
			a, b, c := newTask("a"), newTask("b"), newTask("c")
			setup, write := tt.build(a, b, c)
			for _, task := range setup {
				addTask(t, s, task)
			}
			// The setup wrote a; write a fresh copy so the write is a change.
			write.Title = "written"
			wantViolation(t, put(s, write), write.ULID, schema.CodeCycle, tt.field)
		})
	}
}

func TestUpdateAllowsEditInOldCycle(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	a, b := newTask("a"), newTask("b")
	// A sync or a hand edit may leave cycles; write them past Update.
	mustWrite(t, s, model.TaskPath(a.ULID), "---\nulid: "+a.ULID.String()+"\nworkspace: "+testWorkspaceULID.String()+"\nkind: task\ntitle: a\nstatus: todo\nparent: "+b.ULID.String()+"\nblocked_by: ["+b.ULID.String()+"]\n---\n")
	mustWrite(t, s, model.TaskPath(b.ULID), "---\nulid: "+b.ULID.String()+"\nworkspace: "+testWorkspaceULID.String()+"\nkind: task\ntitle: b\nstatus: todo\nparent: "+a.ULID.String()+"\nblocked_by: ["+a.ULID.String()+"]\n---\n")
	got, err := s.Task(a.ULID)
	if err != nil {
		t.Fatal(err)
	}
	got.Title = "renamed"
	if err := put(s, got); err != nil {
		t.Errorf("editing a task in an old cycle failed: %v", err)
	}
}

func TestUpdateAllowsBlockerChain(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	a, b, c := newTask("a"), newTask("b"), newTask("c")
	b.BlockedBy = []model.ULID{c.ULID}
	addTask(t, s, c)
	addTask(t, s, b)
	a.BlockedBy = []model.ULID{b.ULID, c.ULID}
	if err := put(s, a); err != nil {
		t.Errorf("a chain without a cycle was refused: %v", err)
	}
}
