package index

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

const (
	// CodeBroken is the problem of a task file that does not decode.
	CodeBroken = "broken"
	// CodeNoKey is the problem of a task whose key is missing or not a key.
	CodeNoKey = "no_key"
	// CodeDuplicateKey is the problem of tasks that share a key.
	CodeDuplicateKey = "duplicate_key"
	// CodeKeyPrefix is the problem of a key whose prefix is not the one of
	// the task's workspace.
	CodeKeyPrefix = "key_prefix"
)

// Problem is something wrong with a task: what is:invalid finds, and what is
// shown as a warning after a sync.
type Problem struct {
	Code  string // a schema violation code, missing_ref, cycle or broken
	Field string
	Msg   string
}

// Problems returns what is wrong with the task id.
func (ix *Index) Problems(id model.ID) []Problem { return slices.Clone(ix.problems[id]) }

// findProblems finds what is wrong with each task: a file that does not
// decode, schema violations, references to tasks not in the store, and
// cycles of parents or blockers, and keys and workspaces that do not fit.
func (ix *Index) findProblems() map[model.ID][]Problem {
	out := map[model.ID][]Problem{}
	add := func(id model.ID, p Problem) { out[id] = append(out[id], p) }
	byKey := map[string][]model.ID{}
	for _, e := range ix.tasks {
		if e.Broken != "" {
			add(e.ID, Problem{CodeBroken, "", e.Broken})
			continue
		}
		for _, v := range e.Violations {
			add(e.ID, Problem{v.Code, v.Field, v.Msg})
		}
		for _, r := range ix.refs(e) {
			if ix.tasks[r.id] == nil {
				add(e.ID, Problem{schema.CodeMissingRef, r.field, fmt.Sprintf("task %s is not in this store", r.id)})
			}
		}
		prefix, _, keyOK := model.ParseKey(e.Key)
		if !keyOK {
			add(e.ID, Problem{CodeNoKey, "key", "missing or not a key such as WEB-42; the next write gives one"})
		} else {
			byKey[e.Key] = append(byKey[e.Key], e.ID)
		}
		ws, wsOK := ix.workspaces[e.Workspace]
		switch {
		case e.Workspace.IsZero() || !wsOK:
			add(e.ID, Problem{store.CodeNoWorkspace, "workspace", "missing or not a workspace of this store; set it with hedgehug edit"})
		case keyOK && prefix != ws.KeyPrefix:
			add(e.ID, Problem{CodeKeyPrefix, "key", fmt.Sprintf("the key prefix is not the one of workspace %s (%s); the next write renumbers it", ws.Name, ws.KeyPrefix)})
		}
		if e.Parent != nil {
			if p := ix.tasks[*e.Parent]; p != nil && p.Broken == "" && p.Workspace != e.Workspace {
				add(e.ID, Problem{store.CodeWorkspaceMismatch, "workspace", "differs from the workspace of the parent"})
			}
		}
	}
	for key, ids := range byKey {
		if len(ids) < 2 {
			continue
		}
		for _, id := range ids {
			others := slices.DeleteFunc(slices.Clone(ids), func(o model.ID) bool { return o == id })
			add(id, Problem{CodeDuplicateKey, "key", fmt.Sprintf("key %s is also the key of %s", key, joinIDs(others))})
		}
	}
	parents := func(e *Entry) []model.ID {
		if e.Parent == nil {
			return nil
		}
		return []model.ID{*e.Parent}
	}
	for _, id := range ix.cycles(parents) {
		add(id, Problem{schema.CodeCycle, "parent", "the parents form a cycle"})
	}
	for _, id := range ix.cycles(func(e *Entry) []model.ID { return e.BlockedBy }) {
		add(id, Problem{schema.CodeCycle, "blocked_by", "the blockers form a cycle"})
	}
	return out
}

// joinIDs lists ids in order, separated by commas.
func joinIDs(ids []model.ID) string {
	strs := make([]string, len(ids))
	for i, id := range ids {
		strs[i] = id.String()
	}
	slices.Sort(strs)
	return strings.Join(strs, ", ")
}

// ref is one reference from a task to another.
type ref struct {
	field string
	id    model.ID
}

// refs returns the references of e: its parent, its blockers and its
// task_ref properties, by the store's schema.
func (ix *Index) refs(e *Entry) []ref {
	var out []ref
	if e.Parent != nil {
		out = append(out, ref{"parent", *e.Parent})
	}
	for _, id := range e.BlockedBy {
		out = append(out, ref{"blocked_by", id})
	}
	for _, p := range e.Props {
		if def, ok := ix.sc.Prop(p.Key); ok && def.Type == schema.TypeTaskRef {
			for _, id := range e.Targets(p.Key) {
				out = append(out, ref{p.Key, id})
			}
		}
	}
	return out
}

// cycles returns the tasks that lie on a cycle of next, by Tarjan's
// algorithm for strongly connected components: a component of more than
// one task, or a task that points at itself. Edges to tasks not in the
// index are ignored.
func (ix *Index) cycles(next func(*Entry) []model.ID) []model.ID {
	order := map[model.ID]int{}
	low := map[model.ID]int{}
	onStack := map[model.ID]bool{}
	var stack, out []model.ID
	var visit func(id model.ID)
	visit = func(id model.ID) {
		n := len(order)
		order[id], low[id] = n, n
		stack = append(stack, id)
		onStack[id] = true
		self := false
		for _, to := range next(ix.tasks[id]) {
			switch {
			case ix.tasks[to] == nil:
			case to == id:
				self = true
			case !hasKey(order, to):
				visit(to)
				low[id] = min(low[id], low[to])
			case onStack[to]:
				low[id] = min(low[id], order[to])
			}
		}
		if low[id] != order[id] {
			return
		}
		var comp []model.ID
		for {
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[top] = false
			comp = append(comp, top)
			if top == id {
				break
			}
		}
		if len(comp) > 1 || self {
			out = append(out, comp...)
		}
	}
	for id := range ix.tasks {
		if !hasKey(order, id) {
			visit(id)
		}
	}
	return out
}

func hasKey[K comparable, V any](m map[K]V, k K) bool {
	_, ok := m[k]
	return ok
}

// Reverse returns the tasks that point at id through rel: blocked_by or a
// task_ref property. For a symmetric property it also returns the tasks id
// points at. Each task appears once, in ID order.
func (ix *Index) Reverse(id model.ID, rel string) []*Entry {
	seen := map[model.ID]bool{}
	var out []*Entry
	add := func(e *Entry) {
		if e != nil && !seen[e.ID] {
			seen[e.ID] = true
			out = append(out, e)
		}
	}
	for _, e := range ix.tasks {
		if slices.Contains(e.Targets(rel), id) {
			add(e)
		}
	}
	if p, ok := ix.sc.Prop(rel); ok && p.Symmetric {
		if self := ix.tasks[id]; self != nil {
			for _, to := range self.Targets(rel) {
				add(ix.tasks[to])
			}
		}
	}
	slices.SortFunc(out, func(a, b *Entry) int { return cmp.Compare(a.ID.String(), b.ID.String()) })
	return out
}
