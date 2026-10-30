package index

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"maps"
	"slices"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/gitrepo"
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
	tasks := map[model.ULID]*Entry{}
	decoded := map[model.ULID]*model.Task{}
	for p, err := range ix.repo.Files(tree, "tasks/") {
		if err != nil {
			return err
		}
		ulid, err := model.ParseTaskPath(p)
		if err != nil {
			continue // not a task file
		}
		b, err := ix.repo.ReadFile(tree, p)
		if err != nil {
			return err
		}
		tasks[ulid], decoded[ulid] = newEntry(ulid, b)
	}
	ix.sc, ix.tasks = sc, tasks
	ix.derive()
	if err := ix.revalidateAndFindProblems(tree, slices.Collect(maps.Keys(tasks)), decoded); err != nil {
		return err
	}
	ix.tree = tree // only now: a failed rebuild must not look like a done one
	return nil
}

// revalidate checks the tasks ulids against the schema and keeps the
// violations in their entries. decoded holds tasks already decoded; the
// others are read from tree again. Broken and missing entries are skipped.
func (ix *Index) revalidate(tree plumbing.Hash, ulids []model.ULID, decoded map[model.ULID]*model.Task) error {
	for _, ulid := range ulids {
		e := ix.tasks[ulid]
		if e == nil || e.Broken != "" {
			continue
		}
		t := decoded[ulid]
		if t == nil {
			b, err := ix.repo.ReadFile(tree, model.TaskPath(ulid))
			if err != nil {
				return err
			}
			if t, err = model.DecodeTask(b); err != nil {
				return fmt.Errorf("%s: %w", model.TaskPath(ulid), err)
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
func (ix *Index) ancestors(t *model.Task) []model.ULID {
	var out []model.ULID
	if k, ok := ix.sc.Kind(t.Kind); ok && k.Role == schema.RoleContainer {
		out = append(out, t.ULID)
	}
	seen := map[model.ULID]bool{t.ULID: true}
	for p := t.Parent; p != nil && !seen[*p]; {
		seen[*p] = true
		e := ix.tasks[*p]
		if e == nil || e.Broken != "" {
			break
		}
		out = append(out, e.ULID)
		p = e.Parent
	}
	return out
}

const (
	cacheName   = "index" // in .hedgehug/local
	cacheFormat = 4       // bump when Entry or cache changes
)

// cache is the index as it is kept on disk, for one tree of HEAD.
type cache struct {
	Format int
	Tree   plumbing.Hash
	Tasks  []*Entry
}

// load reads the cache into tasks and returns the tree it is for. It
// reports false when there is no usable cache.
func (ix *Index) load() (plumbing.Hash, bool) {
	b, err := ix.s.ReadLocal(cacheName)
	if err != nil {
		return plumbing.ZeroHash, false
	}
	var c cache
	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&c); err != nil || c.Format != cacheFormat {
		return plumbing.ZeroHash, false
	}
	ix.tasks = make(map[model.ULID]*Entry, len(c.Tasks))
	for _, e := range c.Tasks {
		ix.tasks[e.ULID] = e
	}
	return c.Tree, true
}

// save writes the cache. A failure is ignored: the next Open rebuilds. It
// takes no lock, since a cache is right for its tree whoever writes last.
func (ix *Index) save() {
	c := cache{Format: cacheFormat, Tree: ix.tree, Tasks: slices.Collect(maps.Values(ix.tasks))}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(&c); err != nil {
		return
	}
	_ = ix.s.WriteLocal(cacheName, buf.Bytes())
}

// update brings the index from tree from, which tasks match, to tree to: it
// reads the task files that changed, and checks again the tasks whose
// violations may have changed. A change to schema.yaml checks every task.
func (ix *Index) update(from, to plumbing.Hash) error {
	sc, err := ix.readSchema(to)
	if err != nil {
		return err
	}
	var changes []gitrepo.Change
	if from != to {
		if changes, err = ix.repo.DiffTrees(from, to, ""); err != nil {
			return err
		}
	}
	ix.sc = sc
	decoded := map[model.ULID]*model.Task{}
	var touched []model.ULID
	all := false
	for _, c := range changes {
		if c.Path == schema.Path {
			all = true
			continue
		}
		ulid, err := model.ParseTaskPath(c.Path)
		if err != nil {
			continue
		}
		touched = append(touched, ulid)
		if c.Deleted {
			delete(ix.tasks, ulid)
			continue
		}
		b, err := ix.repo.ReadFile(to, c.Path)
		if err != nil {
			return err
		}
		ix.tasks[ulid], decoded[ulid] = newEntry(ulid, b)
	}
	ix.derive()
	ulids := ix.descendants(touched)
	if all {
		ulids = slices.Collect(maps.Keys(ix.tasks))
	}
	// A task's placement depends on its parent's kind, and its schema on its
	// ancestors, so the descendants of a changed task are checked again.
	if err := ix.revalidateAndFindProblems(to, ulids, decoded); err != nil {
		return err
	}
	ix.tree = to // only now: after an error, from stays the tree the tasks match
	return nil
}

// revalidateAndFindProblems revalidates ulids against tree, then finds the
// problems, which depend on the violations.
func (ix *Index) revalidateAndFindProblems(tree plumbing.Hash, ulids []model.ULID, decoded map[model.ULID]*model.Task) error {
	if err := ix.revalidate(tree, ulids, decoded); err != nil {
		return err
	}
	ix.problems = ix.findProblems()
	return nil
}

// descendants returns ulids and all their descendants, each once. A cycle of
// parents is walked once.
func (ix *Index) descendants(ulids []model.ULID) []model.ULID {
	seen := map[model.ULID]bool{}
	var out []model.ULID
	stack := slices.Clone(ulids)
	for len(stack) > 0 {
		ulid := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[ulid] {
			continue
		}
		seen[ulid] = true
		out = append(out, ulid)
		for _, c := range ix.children[ulid] {
			stack = append(stack, c.ULID)
		}
	}
	return out
}
