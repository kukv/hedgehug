package index

import (
	"fmt"
	"maps"
	"slices"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// readSchema decodes schema.yaml as tree has it.
func (ix *Index) readSchema(tree plumbing.Hash) (*schema.Schema, error) {
	b, err := ix.repo.ReadFile(tree, schema.Path)
	if err != nil {
		return nil, err
	}
	sc, err := schema.Decode(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", schema.Path, err)
	}
	return sc, nil
}

// rebuild indexes every task file of tree.
func (ix *Index) rebuild(tree plumbing.Hash) error {
	sc, err := ix.readSchema(tree)
	if err != nil {
		return err
	}
	tasks := map[model.ID]*Entry{}
	decoded := map[model.ID]*model.Task{}
	for p, err := range ix.repo.Files(tree, "tasks/") {
		if err != nil {
			return err
		}
		id, err := model.ParseTaskPath(p)
		if err != nil {
			continue // not a task file
		}
		b, err := ix.repo.ReadFile(tree, p)
		if err != nil {
			return err
		}
		tasks[id], decoded[id] = newEntry(id, b)
	}
	ix.tree, ix.sc, ix.tasks = tree, sc, tasks
	ix.derive()
	return ix.revalidate(slices.Collect(maps.Keys(tasks)), decoded)
}

// revalidate checks the tasks ids against the schema and keeps the
// violations in their entries. decoded holds tasks already decoded; the
// others are read from the tree again. Broken and missing entries are
// skipped.
func (ix *Index) revalidate(ids []model.ID, decoded map[model.ID]*model.Task) error {
	for _, id := range ids {
		e := ix.tasks[id]
		if e == nil || e.Broken != "" {
			continue
		}
		t := decoded[id]
		if t == nil {
			b, err := ix.repo.ReadFile(ix.tree, model.TaskPath(id))
			if err != nil {
				return err
			}
			if t, err = model.DecodeTask(b); err != nil {
				return fmt.Errorf("%s: %w", model.TaskPath(id), err)
			}
		}
		e.Violations = ix.violations(t)
	}
	return nil
}

// violations checks t against the schema where it sits. Placement is not
// checked when the parent is missing or broken, since its kind is unknown.
func (ix *Index) violations(t *model.Task) []schema.Violation {
	e := ix.sc.For(ix.ancestors(t))
	vs, _ := schema.ValidateTask(t, e)
	if parent, known := ix.parentKind(t); known {
		for _, v := range schema.ValidatePlacement(t.Kind, parent, e) {
			if !slices.Contains(vs, v) { // ValidateTask reports an unknown kind too
				vs = append(vs, v)
			}
		}
	}
	return vs
}

// parentKind returns the kind of t's parent, empty at the root, and whether
// it is known: it is not when the parent is missing or broken.
func (ix *Index) parentKind(t *model.Task) (string, bool) {
	if t.Parent == nil {
		return "", true
	}
	p := ix.tasks[*t.Parent]
	if p == nil || p.Broken != "" {
		return "", false
	}
	return p.Kind, true
}

// ancestors returns what Schema.For needs for t: its ancestors nearest
// first, led by t itself when it is a container. The walk stops at a
// missing or broken task, and when it comes back to a task it has seen.
func (ix *Index) ancestors(t *model.Task) []model.ID {
	var out []model.ID
	if k, ok := ix.sc.Kind(t.Kind); ok && k.Role == schema.RoleContainer {
		out = append(out, t.ID)
	}
	seen := map[model.ID]bool{t.ID: true}
	for p := t.Parent; p != nil && !seen[*p]; {
		seen[*p] = true
		e := ix.tasks[*p]
		if e == nil || e.Broken != "" {
			break
		}
		out = append(out, e.ID)
		p = e.Parent
	}
	return out
}
