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
	// CodeNoID is the problem of a task whose id is missing or not an id.
	CodeNoID = "no_id"
	// CodeDuplicateID is the problem of tasks that share an id.
	CodeDuplicateID = "duplicate_id"
	// CodeIDPrefix is the problem of an id whose prefix is not the one of
	// the task's workspace.
	CodeIDPrefix = "id_prefix"
)

// Problem is something wrong with a task: what is:invalid finds, and what is
// shown as a warning after a sync.
type Problem struct {
	Code  string // a schema violation code, missing_ref, cycle or broken
	Field string
	Msg   string
}

// Problems returns what is wrong with the task ulid.
func (ix *Index) Problems(ulid model.ULID) []Problem { return slices.Clone(ix.problems[ulid]) }

// findProblems finds what is wrong with each task: a file that does not
// decode, schema violations, references to tasks not in the store, and
// cycles of parents or blockers, and ids and workspaces that do not fit.
func (ix *Index) findProblems() map[model.ULID][]Problem {
	out := map[model.ULID][]Problem{}
	add := func(ulid model.ULID, p Problem) { out[ulid] = append(out[ulid], p) }
	byID := map[string][]model.ULID{}
	for _, e := range ix.tasks {
		if e.Broken != "" {
			add(e.ULID, Problem{CodeBroken, "", e.Broken})
			continue
		}
		for _, v := range e.Violations {
			add(e.ULID, Problem{v.Code, v.Field, v.Msg})
		}
		for _, r := range ix.refs(e) {
			if ix.tasks[r.ulid] == nil {
				add(e.ULID, Problem{schema.CodeMissingRef, r.field, fmt.Sprintf("task %s is not in this store", r.ulid)})
			}
		}
		prefix, _, idOK := model.ParseTaskID(e.ID)
		if !idOK {
			add(e.ULID, Problem{CodeNoID, "id", "missing or not an id such as WEB-42; the next write gives one"})
		} else {
			byID[e.ID] = append(byID[e.ID], e.ULID)
		}
		ws, wsOK := ix.workspaces[e.Workspace]
		switch {
		case e.Workspace.IsZero() || !wsOK:
			add(e.ULID, Problem{store.CodeNoWorkspace, "workspace", "missing or not a workspace of this store; set it with hedgehug edit"})
		case idOK && prefix != ws.IDPrefix:
			add(e.ULID, Problem{CodeIDPrefix, "id", fmt.Sprintf("the id prefix is not the one of workspace %s (%s); the next write renumbers it", ws.Name, ws.IDPrefix)})
		}
		if e.Parent != nil {
			if p := ix.tasks[*e.Parent]; p != nil && p.Broken == "" && p.Workspace != e.Workspace {
				add(e.ULID, Problem{store.CodeWorkspaceMismatch, "workspace", "differs from the workspace of the parent"})
			}
		}
	}
	for taskID, ulids := range byID {
		if len(ulids) < 2 {
			continue
		}
		for _, ulid := range ulids {
			others := slices.DeleteFunc(slices.Clone(ulids), func(o model.ULID) bool { return o == ulid })
			add(ulid, Problem{CodeDuplicateID, "id", fmt.Sprintf("id %s is also the id of %s", taskID, joinULIDs(others))})
		}
	}
	parents := func(e *Entry) []model.ULID {
		if e.Parent == nil {
			return nil
		}
		return []model.ULID{*e.Parent}
	}
	for _, ulid := range ix.cycles(parents) {
		add(ulid, Problem{schema.CodeCycle, "parent", "the parents form a cycle"})
	}
	for _, ulid := range ix.cycles(func(e *Entry) []model.ULID { return e.BlockedBy }) {
		add(ulid, Problem{schema.CodeCycle, "blocked_by", "the blockers form a cycle"})
	}
	return out
}

// joinULIDs lists ulids in order, separated by commas.
func joinULIDs(ulids []model.ULID) string {
	strs := make([]string, len(ulids))
	for i, ulid := range ulids {
		strs[i] = ulid.String()
	}
	slices.Sort(strs)
	return strings.Join(strs, ", ")
}

// ref is one reference from a task to another.
type ref struct {
	field string
	ulid  model.ULID
}

// refs returns the references of e: its parent, its blockers and its
// task_ref properties, by the store's schema.
func (ix *Index) refs(e *Entry) []ref {
	var out []ref
	if e.Parent != nil {
		out = append(out, ref{"parent", *e.Parent})
	}
	for _, ulid := range e.BlockedBy {
		out = append(out, ref{"blocked_by", ulid})
	}
	for _, p := range e.Props {
		if def, ok := ix.sc.Prop(p.Key); ok && def.Type == schema.TypeTaskRef {
			for _, ulid := range e.Targets(p.Key) {
				out = append(out, ref{p.Key, ulid})
			}
		}
	}
	return out
}

// cycles returns the tasks that lie on a cycle of next, by Tarjan's
// algorithm for strongly connected components: a component of more than
// one task, or a task that points at itself. Edges to tasks not in the
// index are ignored.
func (ix *Index) cycles(next func(*Entry) []model.ULID) []model.ULID {
	order := map[model.ULID]int{}
	low := map[model.ULID]int{}
	onStack := map[model.ULID]bool{}
	var stack, out []model.ULID
	var visit func(ulid model.ULID)
	visit = func(ulid model.ULID) {
		n := len(order)
		order[ulid], low[ulid] = n, n
		stack = append(stack, ulid)
		onStack[ulid] = true
		self := false
		for _, to := range next(ix.tasks[ulid]) {
			switch {
			case ix.tasks[to] == nil:
			case to == ulid:
				self = true
			case !hasKey(order, to):
				visit(to)
				low[ulid] = min(low[ulid], low[to])
			case onStack[to]:
				low[ulid] = min(low[ulid], order[to])
			}
		}
		if low[ulid] != order[ulid] {
			return
		}
		var comp []model.ULID
		for {
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[top] = false
			comp = append(comp, top)
			if top == ulid {
				break
			}
		}
		if len(comp) > 1 || self {
			out = append(out, comp...)
		}
	}
	for ulid := range ix.tasks {
		if !hasKey(order, ulid) {
			visit(ulid)
		}
	}
	return out
}

func hasKey[K comparable, V any](m map[K]V, k K) bool {
	_, ok := m[k]
	return ok
}

// Reverse returns the tasks that point at ulid through rel: blocked_by or a
// task_ref property. For a symmetric property it also returns the tasks ulid
// points at. Each task appears once, in ULID order.
func (ix *Index) Reverse(ulid model.ULID, rel string) []*Entry {
	seen := map[model.ULID]bool{}
	var out []*Entry
	add := func(e *Entry) {
		if e != nil && !seen[e.ULID] {
			seen[e.ULID] = true
			out = append(out, e)
		}
	}
	for _, e := range ix.tasks {
		if slices.Contains(e.Targets(rel), ulid) {
			add(e)
		}
	}
	if p, ok := ix.sc.Prop(rel); ok && p.Symmetric {
		if self := ix.tasks[ulid]; self != nil {
			for _, to := range self.Targets(rel) {
				add(ix.tasks[to])
			}
		}
	}
	slices.SortFunc(out, func(a, b *Entry) int { return cmp.Compare(a.ULID.String(), b.ULID.String()) })
	return out
}
