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

// ErrCycle means following a task's parents led back to a task already seen.
var ErrCycle = errors.New("the parents of the task form a cycle")

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
// on disk, the kind of its parent and its ancestors.
func (tx *Tx) validate(sc *schema.Schema) error {
	for _, id := range tx.taskOrder {
		t := tx.tasks[id]
		if t == nil {
			continue // removed in the same operation
		}
		old, err := tx.s.Task(id)
		if errors.Is(err, ErrNotFound) {
			old = nil
		} else if err != nil {
			return err
		}
		parentKind, ancestors, err := tx.lineage(t, sc)
		if err != nil {
			return err
		}
		if vs := schema.ValidateWrite(old, t, parentKind, sc.For(ancestors)); len(vs) > 0 {
			return &ValidationError{Task: id, Violations: vs}
		}
	}
	return nil
}

// lineage returns the kind of t's parent, empty at the root, and the IDs
// For needs: t's ancestors nearest first, led by t itself when t is a
// container. Parents are read as the operation leaves them.
func (tx *Tx) lineage(t *model.Task, sc *schema.Schema) (parentKind string, ancestors []model.ID, err error) {
	if k, ok := sc.Kind(t.Kind); ok && k.Role == schema.RoleContainer {
		ancestors = append(ancestors, t.ID)
	}
	seen := map[model.ID]bool{t.ID: true}
	for cur := t; cur.Parent != nil; {
		id := *cur.Parent
		if seen[id] {
			return "", nil, fmt.Errorf("task %s: %w", t.ID, ErrCycle)
		}
		seen[id] = true
		p, err := tx.task(id)
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
