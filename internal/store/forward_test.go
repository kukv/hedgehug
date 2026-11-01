package store

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
)

const forwardStoreULID = "01JC0M3R5W7M2X9V4T6R5N8P1B"

func forwardFile(ulid string) string {
	return "---\nulid: " + ulid + "\nmoved_to: " + forwardStoreULID + "\n---\n"
}

// Tasks skips a forward; Forwards yields it; Task names it with ErrForward.
func TestForwards(t *testing.T) {
	s := newTestStore(t)
	a, b := mustULID(t, taskA), mustULID(t, taskB)
	mustWrite(t, s, model.TaskPath(a), taskFile(taskA, "A"))
	mustWrite(t, s, model.TaskPath(b), forwardFile(taskB))
	bad := mustULID(t, cmt1)
	mustWrite(t, s, model.TaskPath(bad), "---\nulid: "+cmt1+"\nmoved_to: "+forwardStoreULID+"\ntitle: x\n---\n")

	for task, err := range s.Tasks() {
		if err != nil || task.ULID != a {
			t.Errorf("Tasks yielded %+v, %v; want only A", task, err)
		}
	}
	var got []model.ULID
	var broken *FileError
	for f, err := range s.Forwards() {
		if err != nil {
			if !errors.As(err, &broken) || broken.Path != model.TaskPath(bad) {
				t.Errorf("Forwards err = %v, want a *FileError for the broken forward", err)
			}
			continue
		}
		if f.MovedTo.String() != forwardStoreULID {
			t.Errorf("moved_to = %s", f.MovedTo)
		}
		got = append(got, f.ULID)
	}
	if !slices.Equal(got, []model.ULID{b}) || broken == nil {
		t.Errorf("forwards = %v, broken = %v", got, broken)
	}
	// Code that treats a task that is not here as absent treats a forward so.
	if _, err := s.Task(b); !errors.Is(err, model.ErrForward) || !errors.Is(err, ErrNotFound) {
		t.Errorf("Task(forward) err = %v, want ErrForward and ErrNotFound", err)
	}
}

// A forward can be removed (purge --moved).
func TestForwardIsRemoved(t *testing.T) {
	s := newTestStore(t)
	b := mustULID(t, taskB)
	mustWrite(t, s, model.TaskPath(b), forwardFile(taskB))
	if _, err := s.Update(Op{Name: "purge-moved", Author: testAuthor}, func(tx *Tx) error { return tx.RemoveTask(b) }); err != nil {
		t.Fatalf("RemoveTask: %v", err)
	}
	if _, err := s.Task(b); !errors.Is(err, ErrNotFound) || errors.Is(err, model.ErrForward) {
		t.Errorf("after removal: err = %v, want a plain ErrNotFound", err)
	}
}
