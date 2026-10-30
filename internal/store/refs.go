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
// still be edited. A new parent or blocker that leads back to t through the
// same kind of reference is a cycle; cycles made only of references that were
// already there are let through.
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
			continue
		}
		if r.field != "parent" && r.field != "blocked_by" {
			continue
		}
		cycle, err := tx.reaches(r.id, t.ID, r.field)
		if err != nil {
			return nil, err
		}
		if cycle {
			vs = append(vs, schema.Violation{Code: schema.CodeCycle, Field: r.field, Msg: fmt.Sprintf("%s %s leads back to this task", r.field, r.id)})
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

// edges returns the tasks t points at through field: its parent or its
// blockers.
func edges(t *model.Task, field string) []model.ID {
	if field == "parent" {
		if t.Parent == nil {
			return nil
		}
		return []model.ID{*t.Parent}
	}
	return t.BlockedBy
}

// reaches reports whether following field from the task from leads to
// target, as the operation leaves the tasks. The walk stops at tasks that
// are missing or do not decode.
func (tx *Tx) reaches(from, target model.ID, field string) (bool, error) {
	seen := map[model.ID]bool{}
	stack := []model.ID{from}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if id == target {
			return true, nil
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		t, err := tx.task(id)
		if errors.Is(err, ErrNotFound) || errors.As(err, new(*model.DecodeError)) {
			continue
		}
		if err != nil {
			return false, err
		}
		stack = append(stack, edges(t, field)...)
	}
	return false, nil
}
