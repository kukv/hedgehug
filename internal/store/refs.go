package store

import (
	"errors"
	"fmt"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// ref is one reference from a task to another task of the store.
type ref struct {
	field string // parent, blocked_by or the name of a task_ref property
	id    model.ID
}

// refsOf returns the references of t: its parent, its blockers and the
// values of its task_ref properties. Property values that are not IDs are
// left out; ValidateTask reports them.
func refsOf(t *model.Task, sc *schema.Schema) []ref {
	var out []ref
	if t.Parent != nil {
		out = append(out, ref{"parent", *t.Parent})
	}
	for _, id := range t.BlockedBy {
		out = append(out, ref{"blocked_by", id})
	}
	for _, f := range t.Props {
		p, ok := sc.Prop(f.Key)
		if !ok || p.Type != schema.TypeTaskRef || f.Value == nil {
			continue
		}
		items := []*yaml.Node{f.Value}
		if f.Value.Kind == yaml.SequenceNode {
			items = f.Value.Content
		}
		for _, n := range items {
			if n.Kind != yaml.ScalarNode {
				continue
			}
			if id, err := model.ParseID(n.Value); err == nil {
				out = append(out, ref{f.Key, id})
			}
		}
	}
	return out
}

// checkRefs returns the violations of the references that a write of t over
// old adds; old is nil for a new task. A reference old already had is not
// checked, so a task left with a missing reference by a purge or a sync can
// still be edited.
func (tx *Tx) checkRefs(old, t *model.Task, sc *schema.Schema) ([]schema.Violation, error) {
	had := map[ref]bool{}
	if old != nil {
		for _, r := range refsOf(old, sc) {
			had[r] = true
		}
	}
	var vs []schema.Violation
	for _, r := range refsOf(t, sc) {
		if had[r] {
			continue
		}
		had[r] = true // a reference written twice is checked once
		ok, err := tx.exists(r.id)
		if err != nil {
			return nil, err
		}
		if !ok {
			vs = append(vs, schema.Violation{Code: schema.CodeMissingRef, Field: r.field, Msg: fmt.Sprintf("task %s is not in this store", r.id)})
		}
	}
	return vs, nil
}

// exists reports whether the task id is in the store as the operation
// leaves it. A deleted task counts, and so does a file that does not
// decode: the task is there, only broken.
func (tx *Tx) exists(id model.ID) (bool, error) {
	_, err := tx.task(id)
	switch {
	case err == nil, errors.As(err, new(*model.DecodeError)):
		return true, nil
	case errors.Is(err, ErrNotFound):
		return false, nil
	}
	return false, err
}
