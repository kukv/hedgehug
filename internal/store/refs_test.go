package store

import (
	"errors"
	"testing"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// wantViolation checks that err is a ValidationError on task with a
// violation of code on field.
func wantViolation(t *testing.T, err error, task model.ID, code, field string) {
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

// idList is a property value listing ids.
func idList(ids ...model.ID) *yaml.Node {
	n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, id := range ids {
		n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: id.String()})
	}
	return n
}

func put(s *Store, tasks ...*model.Task) error {
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
	s := newTestStore(t)
	task := newTask("x")
	task.BlockedBy = []model.ID{model.NewID()}
	wantViolation(t, put(s, task), task.ID, schema.CodeMissingRef, "blocked_by")
}

func TestUpdateMissingTaskRefProp(t *testing.T) {
	s := newTestStore(t)
	task := newTask("x")
	task.Props.Set("duplicates", idList(model.NewID()))
	wantViolation(t, put(s, task), task.ID, schema.CodeMissingRef, "duplicates")
}

func TestUpdateKeepsOldMissingRef(t *testing.T) {
	s := newTestStore(t)
	blocker, task := newTask("blocker"), newTask("task")
	task.BlockedBy = []model.ID{blocker.ID}
	addTask(t, s, blocker)
	addTask(t, s, task)
	if _, err := s.Update(Op{Name: "purge", Author: testAuthor}, func(tx *Tx) error { return tx.RemoveTask(blocker.ID) }); err != nil {
		t.Fatal(err)
	}
	got, err := s.Task(task.ID)
	if err != nil {
		t.Fatal(err)
	}
	got.Title = "renamed"
	if err := put(s, got); err != nil {
		t.Errorf("editing a task with an old missing blocker failed: %v", err)
	}
}

func TestUpdateRefToDeletedTask(t *testing.T) {
	s := newTestStore(t)
	gone := newTask("gone")
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	gone.DeletedAt = &at
	addTask(t, s, gone)
	task := newTask("task")
	task.BlockedBy = []model.ID{gone.ID}
	if err := put(s, task); err != nil {
		t.Errorf("a reference to a deleted task was refused: %v", err)
	}
}

func TestUpdateRefToTaskRemovedInSameOp(t *testing.T) {
	s := newTestStore(t)
	blocker := newTask("blocker")
	addTask(t, s, blocker)
	task := newTask("task")
	task.BlockedBy = []model.ID{blocker.ID}
	_, err := s.Update(Op{Name: "update", Author: testAuthor}, func(tx *Tx) error {
		return errors.Join(tx.RemoveTask(blocker.ID), tx.PutTask(task))
	})
	wantViolation(t, err, task.ID, schema.CodeMissingRef, "blocked_by")
}

func TestUpdateRefToTaskMadeInSameOp(t *testing.T) {
	s := newTestStore(t)
	blocker, task := newTask("blocker"), newTask("task")
	task.BlockedBy = []model.ID{blocker.ID}
	if err := put(s, task, blocker); err != nil {
		t.Errorf("a reference to a task made in the same operation was refused: %v", err)
	}
}

func TestUpdateUnderBrokenParent(t *testing.T) {
	s := newTestStore(t)
	parent, child := newTask("parent"), newTask("child")
	child.Parent = &parent.ID
	addTask(t, s, parent)
	addTask(t, s, child)
	mustWrite(t, s, model.TaskPath(parent.ID), "not a task file\n")
	got, err := s.Task(child.ID)
	if err != nil {
		t.Fatal(err)
	}
	got.Title = "renamed"
	if err := put(s, got); err != nil {
		t.Errorf("editing a task under a broken parent failed: %v", err)
	}
}

func TestUpdateRefusesNewCycles(t *testing.T) {
	tests := []struct {
		name  string
		field string
		build func(a, b, c *model.Task) (setup []*model.Task, write *model.Task)
	}{
		{"parent self", "parent", func(a, _, _ *model.Task) ([]*model.Task, *model.Task) {
			a.Parent = &a.ID
			return nil, a
		}},
		{"parent move under own child", "parent", func(a, b, _ *model.Task) ([]*model.Task, *model.Task) {
			b.Parent = &a.ID
			w := *a
			w.Parent = &b.ID
			return []*model.Task{a, b}, &w
		}},
		{"blocked_by self", "blocked_by", func(a, _, _ *model.Task) ([]*model.Task, *model.Task) {
			a.BlockedBy = []model.ID{a.ID}
			return nil, a
		}},
		{"blocked_by two", "blocked_by", func(a, b, _ *model.Task) ([]*model.Task, *model.Task) {
			b.BlockedBy = []model.ID{a.ID}
			w := *a
			w.BlockedBy = []model.ID{b.ID}
			return []*model.Task{a, b}, &w
		}},
		{"blocked_by three", "blocked_by", func(a, b, c *model.Task) ([]*model.Task, *model.Task) {
			b.BlockedBy = []model.ID{c.ID}
			c.BlockedBy = []model.ID{a.ID}
			w := *a
			w.BlockedBy = []model.ID{b.ID}
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
			wantViolation(t, put(s, write), write.ID, schema.CodeCycle, tt.field)
		})
	}
}

func TestUpdateAllowsEditInOldCycle(t *testing.T) {
	s := newTestStore(t)
	a, b := newTask("a"), newTask("b")
	// A sync or a hand edit may leave cycles; write them past Update.
	mustWrite(t, s, model.TaskPath(a.ID), "---\nid: "+a.ID.String()+"\nworkspace: "+testWorkspaceID.String()+"\nkind: task\ntitle: a\nstatus: todo\nparent: "+b.ID.String()+"\nblocked_by: ["+b.ID.String()+"]\n---\n")
	mustWrite(t, s, model.TaskPath(b.ID), "---\nid: "+b.ID.String()+"\nworkspace: "+testWorkspaceID.String()+"\nkind: task\ntitle: b\nstatus: todo\nparent: "+a.ID.String()+"\nblocked_by: ["+a.ID.String()+"]\n---\n")
	got, err := s.Task(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	got.Title = "renamed"
	if err := put(s, got); err != nil {
		t.Errorf("editing a task in an old cycle failed: %v", err)
	}
}

func TestUpdateAllowsBlockerChain(t *testing.T) {
	s := newTestStore(t)
	a, b, c := newTask("a"), newTask("b"), newTask("c")
	b.BlockedBy = []model.ID{c.ID}
	addTask(t, s, c)
	addTask(t, s, b)
	a.BlockedBy = []model.ID{b.ID, c.ID}
	if err := put(s, a); err != nil {
		t.Errorf("a chain without a cycle was refused: %v", err)
	}
}
