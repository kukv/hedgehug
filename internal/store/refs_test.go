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
func wantViolation(t *testing.T, err error, task model.ID, code, field string) { //nolint:unparam // Task 5 passes other codes
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
	return s.Update(Op{Name: "update", Author: testAuthor}, func(tx *Tx) error {
		for _, t := range tasks {
			if err := tx.PutTask(t); err != nil {
				return err
			}
		}
		return nil
	})
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
	if err := s.Update(Op{Name: "purge", Author: testAuthor}, func(tx *Tx) error { return tx.RemoveTask(blocker.ID) }); err != nil {
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
	err := s.Update(Op{Name: "update", Author: testAuthor}, func(tx *Tx) error {
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
