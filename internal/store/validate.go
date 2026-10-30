package store

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// SchemaError means a schema written with PutSchema has problems.
type SchemaError struct{ Problems []schema.Problem }

func (e *SchemaError) Error() string {
	msgs := make([]string, len(e.Problems))
	for i, p := range e.Problems {
		msgs[i] = p.String()
	}
	return "invalid schema: " + strings.Join(msgs, "; ")
}

// ValidationError means a write would bring violations into a task.
type ValidationError struct {
	Task       model.ID
	Violations []schema.Violation
}

func (e *ValidationError) Error() string {
	msgs := make([]string, len(e.Violations))
	for i, v := range e.Violations {
		msgs[i] = v.String()
	}
	return fmt.Sprintf("task %s: %s", e.Task, strings.Join(msgs, "; "))
}

// validate checks every task put in the operation against sc. The rules are
// schema.ValidateWrite's; this only gathers what it needs: the task as it is
// on disk, and for both it and the task being written the kind of the
// parent and the ancestors. The old task's are read from disk, so a move is
// checked in its new place.
func (tx *Tx) validate(sc *schema.Schema) error {
	for _, id := range tx.taskOrder {
		t := tx.tasks[id]
		if t == nil {
			continue // removed in the same operation
		}
		old, err := tx.s.Task(id)
		// A file that does not decode is checked like a new task, so a
		// write can repair it.
		if errors.Is(err, ErrNotFound) || errors.As(err, new(*model.DecodeError)) {
			old = nil
		} else if err != nil {
			return err
		}
		parentKind, ancestors, err := lineage(t, sc, tx.task)
		if err != nil {
			return err
		}
		var oldParentKind string
		var oldE *schema.Effective
		if old != nil {
			var oldAncestors []model.ID
			if oldParentKind, oldAncestors, err = lineage(old, sc, tx.s.Task); err != nil {
				return err
			}
			oldE = sc.For(oldAncestors)
		}
		vs := schema.ValidateWrite(old, oldParentKind, oldE, t, parentKind, sc.For(ancestors))
		rvs, err := tx.checkRefs(old, t, sc)
		if err != nil {
			return err
		}
		if vs = append(vs, rvs...); len(vs) > 0 {
			return &ValidationError{Task: id, Violations: vs}
		}
	}
	return nil
}

// lineage returns the kind of t's parent, empty at the root, and the IDs
// For needs: t's ancestors nearest first, led by t itself when t is a
// container. Parents are read with read: as the operation leaves them for
// the task being written, from disk for the task as it was.
//
// The walk stops at the first task that is missing or does not decode, and
// keeps the ancestors found so far, with an empty parentKind when the direct
// parent is the one missing. So a task whose parent was purged or broken can
// still be edited; checkRefs refuses a write that adds a missing parent.
// It also stops when it comes back to a task it has seen.
func lineage(t *model.Task, sc *schema.Schema, read func(model.ID) (*model.Task, error)) (parentKind string, ancestors []model.ID, err error) {
	if k, ok := sc.Kind(t.Kind); ok && k.Role == schema.RoleContainer {
		ancestors = append(ancestors, t.ID)
	}
	seen := map[model.ID]bool{t.ID: true}
	for cur := t; cur.Parent != nil; {
		id := *cur.Parent
		if seen[id] {
			break // a cycle the store already had; checkRefs refuses new ones
		}
		seen[id] = true
		p, err := read(id)
		if errors.Is(err, ErrNotFound) || errors.As(err, new(*model.DecodeError)) {
			break
		}
		if err != nil {
			return "", nil, fmt.Errorf("parent of %s: %w", cur.ID, err)
		}
		if cur == t {
			parentKind = p.Kind
		}
		ancestors = append(ancestors, id)
		cur = p
	}
	return parentKind, ancestors, nil
}
