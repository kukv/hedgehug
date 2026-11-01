package merge

import (
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// Files is the set of files the repair looks at: HEAD with the merge laid
// over it.
type Files interface {
	Paths(prefix string) []string // sorted
	Read(path string) []byte      // nil when absent
}

type RepairInput struct {
	Files    Files
	Base     Files                                 // the merge base; nil for a repair commit
	OnOrigin func(ulid model.ULID, id string) bool // origin's main has this task with this id
	Changed  func(path string) bool                // the merge changed the file (parent_cycle picks among these)
	Author   func(path string) string              // who last changed the file
	Now      time.Time
}

// Repaired is one thing the repair fixed, or a duplicate it only tells of.
type Repaired struct {
	ULID   model.ULID // task or workspace
	Kind   string     // workspace_revived, id_prefix, parent_cycle, parent_deleted, parent_moved, ref_moved, workspace_mismatch, id_collision, id_renumbered, prefix_duplicate, name_duplicate
	Value  string     // the old value as one line of JSON (for id_collision: {"old":"WEB-43","new":"WEB-45"})
	Author string     // whom to tell; empty for id_renumbered
}

// Repair fixes what holds across files and a merge of single files can
// break: tasks in a removed workspace, bad id prefixes, parent cycles,
// children of deleted tasks, children in another workspace than their
// parent, and ids two tasks share or that do not fit the workspace. It
// tells of duplicate prefixes and names without changing them. It returns
// the files to write (nil data removes) and what it did, in the order of
// the kinds above. The same input gives the same output.
//
// A task that moved to another store leaves a forward (a task file with only
// moved_to). The children of a forward go to the root (parent_moved), the
// blockers and task_ref values that name a forward are removed (ref_moved),
// and the comment and worklog files under a forward are removed. The files
// removed are returned as dropped, each an Overwritten with Field "*" and
// the whole file as a JSON string.
func Repair(in RepairInput) (files map[string][]byte, repaired []Repaired, dropped []Overwritten, err error) {
	r := loadRepair(in)
	r.reviveWorkspaces()
	r.restorePrefixes()
	if err := r.breakCycles(); err != nil {
		return nil, nil, nil, err
	}
	if err := r.detachFromDeleted(); err != nil {
		return nil, nil, nil, err
	}
	r.removeMovedRefs()
	r.followParents()
	if err := r.renumber(); err != nil {
		return nil, nil, nil, err
	}
	r.duplicates()
	r.dropUnderForwards()
	files, err = r.files()
	return files, r.done, r.dropped, err
}

type repairer struct {
	in        RepairInput
	ws        map[model.ULID]*model.Workspace
	brokenWS  map[model.ULID]bool // workspace files that do not decode
	tasks     map[model.ULID]*model.Task
	order     []model.ULID // of tasks
	broken    map[string][]byte
	brokenIDs map[model.ULID]bool // task files that do not decode
	dirtyWS   map[model.ULID]bool
	dirty     map[model.ULID]bool // tasks
	raw       map[string][]byte   // revived workspace files that do not decode
	moved     map[model.ULID]bool // tasks moved to their parent's workspace
	forwards  map[model.ULID]bool // task files that are forwards
	dropped   []Overwritten
	drop      []string // files to remove
	done      []Repaired
}

func compareULIDs(a, b model.ULID) int { return strings.Compare(a.String(), b.String()) }

// loadRepair reads every task and workspace file. A file that does not
// decode, or whose ulid is not its path's, is kept as broken: it exists,
// but the repair neither changes it nor reads its fields.
func loadRepair(in RepairInput) *repairer {
	r := &repairer{
		in: in, ws: map[model.ULID]*model.Workspace{}, brokenWS: map[model.ULID]bool{},
		tasks: map[model.ULID]*model.Task{}, broken: map[string][]byte{}, brokenIDs: map[model.ULID]bool{},
		dirtyWS: map[model.ULID]bool{}, dirty: map[model.ULID]bool{}, raw: map[string][]byte{}, moved: map[model.ULID]bool{},
		forwards: map[model.ULID]bool{},
	}
	for _, p := range in.Files.Paths("workspaces/") {
		u, err := model.ParseWorkspacePath(p)
		if err != nil {
			continue
		}
		if w, err := model.DecodeWorkspace(in.Files.Read(p)); err == nil && w.ULID == u {
			r.ws[u] = w
		} else {
			r.brokenWS[u] = true
		}
	}
	for _, p := range in.Files.Paths("tasks/") {
		u, err := model.ParseTaskPath(p)
		if err != nil {
			continue
		}
		b := in.Files.Read(p)
		if model.IsForward(b) {
			// Not a task. A forward that does not decode is left alone.
			if f, err := model.DecodeForward(b); err == nil && f.ULID == u {
				r.forwards[u] = true
			}
			continue
		}
		if t, err := model.DecodeTask(b); err == nil && t.ULID == u {
			r.tasks[u] = t
			r.order = append(r.order, u)
		} else {
			r.broken[p] = b
			r.brokenIDs[u] = true
		}
	}
	slices.SortFunc(r.order, compareULIDs)
	return r
}

func (r *repairer) tell(u model.ULID, kind string, value []byte, path string) {
	r.done = append(r.done, Repaired{ULID: u, Kind: kind, Value: string(value), Author: r.in.Author(path)})
}

// parentMove is the value of a parent repair: where the task was and where
// the repair put it, each a ULID or null for the root. It is one line.
func parentMove(from, to []byte) []byte {
	return []byte(`{"from":` + string(from) + `,"to":` + string(to) + `}`)
}

// readBase returns the base's file at path, or nil.
func (r *repairer) readBase(path string) []byte {
	if r.in.Base == nil {
		return nil
	}
	return r.in.Base.Read(path)
}

// reviveWorkspaces puts back, from the base, the workspaces that tasks are
// in but that are gone.
func (r *repairer) reviveWorkspaces() {
	for _, u := range r.order {
		w := r.tasks[u].Workspace
		if w.IsZero() || r.ws[w] != nil || r.brokenWS[w] {
			continue
		}
		p := model.WorkspacePath(w)
		b := r.readBase(p)
		if b == nil {
			continue // left to doctor's no_workspace
		}
		if x, err := model.DecodeWorkspace(b); err == nil && x.ULID == w {
			r.ws[w] = x
			r.dirtyWS[w] = true
		} else {
			r.raw[p] = b
			r.brokenWS[w] = true
		}
		r.tell(w, "workspace_revived", []byte("null"), p)
	}
}

// restorePrefixes puts back the base's prefix, and when it was set, of a
// workspace whose prefix is not well formed.
func (r *repairer) restorePrefixes() {
	for _, u := range slices.SortedFunc(maps.Keys(r.ws), compareULIDs) {
		w := r.ws[u]
		if model.ValidIDPrefix(w.IDPrefix) == nil {
			continue
		}
		p := model.WorkspacePath(u)
		b := r.readBase(p)
		if b == nil {
			continue
		}
		x, err := model.DecodeWorkspace(b)
		if err != nil || model.ValidIDPrefix(x.IDPrefix) != nil {
			continue
		}
		old := w.IDPrefix
		w.IDPrefix, w.PrefixSet = x.IDPrefix, x.PrefixSet
		r.dirtyWS[u] = true
		r.tell(u, "id_prefix", jsonOf(old), p)
	}
}

// breakCycles breaks one parent cycle at a time until none is left. Of the
// tasks on the cycle whose parent differs from the base's, else whose file
// the merge changed, else all of them, the one updated last goes back to
// its parent in the base, or to the root when that does not break the
// cycle.
func (r *repairer) breakCycles() error {
	for {
		cycle := r.findCycle()
		if cycle == nil {
			return nil
		}
		cands := slices.DeleteFunc(slices.Clone(cycle), func(u model.ULID) bool { return !r.parentChanged(u) })
		if len(cands) == 0 {
			cands = slices.DeleteFunc(slices.Clone(cycle), func(u model.ULID) bool { return !r.in.Changed(model.TaskPath(u)) })
		}
		if len(cands) == 0 {
			cands = cycle
		}
		u := slices.MaxFunc(cands, func(a, b model.ULID) int {
			ta, tb := r.tasks[a], r.tasks[b]
			return cmp.Or(ta.Updated.Compare(tb.Updated),
				strings.Compare(r.in.Author(model.TaskPath(a)), r.in.Author(model.TaskPath(b))),
				compareULIDs(a, b))
		})
		t := r.tasks[u]
		old := jsonOf(t.Parent)
		t.Parent = r.baseParent(u)
		if t.Parent != nil && r.onCycle(u) {
			t.Parent = nil
		}
		if t.Parent == nil {
			if err := r.toRootEnd(t); err != nil {
				return err
			}
		}
		r.dirty[u] = true
		r.tell(u, "parent_cycle", parentMove(old, jsonOf(t.Parent)), model.TaskPath(u))
	}
}

// findCycle returns the tasks of a parent cycle, or nil.
func (r *repairer) findCycle() []model.ULID {
	const walking, walked = 1, 2
	state := map[model.ULID]int{}
	for _, start := range r.order {
		var path []model.ULID
		for u := start; state[u] != walked; {
			if state[u] == walking {
				return path[slices.Index(path, u):]
			}
			state[u] = walking
			path = append(path, u)
			p := r.tasks[u].Parent
			if p == nil || r.tasks[*p] == nil {
				break
			}
			u = *p
		}
		for _, u := range path {
			state[u] = walked
		}
	}
	return nil
}

// onCycle reports whether the parents of u lead back to u.
func (r *repairer) onCycle(u model.ULID) bool {
	seen := map[model.ULID]bool{}
	for p := r.tasks[u].Parent; p != nil && r.tasks[*p] != nil && !seen[*p]; p = r.tasks[*p].Parent {
		if *p == u {
			return true
		}
		seen[*p] = true
	}
	return false
}

// parentChanged reports whether task u has another parent than in the
// base. Without a base, no parent is known to have changed.
func (r *repairer) parentChanged(u model.ULID) bool {
	if r.in.Base == nil {
		return false
	}
	p, b := r.tasks[u].Parent, r.baseParent(u)
	return (p == nil) != (b == nil) || p != nil && *p != *b
}

// baseParent returns the parent of task u in the base, or nil.
func (r *repairer) baseParent(u model.ULID) *model.ULID {
	b := r.readBase(model.TaskPath(u))
	if b == nil {
		return nil
	}
	t, err := model.DecodeTask(b)
	if err != nil {
		return nil
	}
	return t.Parent
}

// toRootEnd gives t, which is now a root, a rank after the live roots of
// its workspace. Roots whose rank is not well formed are not counted.
func (r *repairer) toRootEnd(t *model.Task) error {
	var last model.Rank
	for _, u := range r.order {
		o := r.tasks[u]
		if o != t && o.Parent == nil && o.DeletedAt == nil && o.Workspace == t.Workspace && o.Rank.Validate() == nil && o.Rank > last {
			last = o.Rank
		}
	}
	rank, err := model.Between(last, "")
	if err != nil {
		return fmt.Errorf("%s: %w", model.TaskPath(t.ULID), err)
	}
	t.Rank = rank
	return nil
}

// detachFromDeleted moves to the root the live tasks whose parent is
// deleted, gone or moved to another store (a forward). A parent whose file
// does not decode is not known to be any of them.
func (r *repairer) detachFromDeleted() error {
	for _, u := range r.order {
		t := r.tasks[u]
		if t.DeletedAt != nil || t.Parent == nil || r.brokenIDs[*t.Parent] {
			continue
		}
		if p := r.tasks[*t.Parent]; p != nil && p.DeletedAt == nil {
			continue
		}
		kind := "parent_deleted"
		if r.forwards[*t.Parent] {
			kind = "parent_moved"
		}
		old := jsonOf(t.Parent)
		t.Parent = nil
		if err := r.toRootEnd(t); err != nil {
			return err
		}
		r.dirty[u] = true
		r.tell(u, kind, parentMove(old, jsonOf(t.Parent)), model.TaskPath(u))
	}
	return nil
}

// removeMovedRefs removes from each task the blockers and task_ref values
// that name a forward.
func (r *repairer) removeMovedRefs() {
	if len(r.forwards) == 0 {
		return
	}
	sc := schema.Default()
	if b := r.in.Files.Read(schema.Path); b != nil {
		if x, err := schema.Decode(b); err == nil {
			sc = x
		}
	}
	for _, u := range r.order {
		t := r.tasks[u]
		for _, target := range slices.Clone(t.BlockedBy) {
			if r.forwards[target] {
				t.BlockedBy = slices.DeleteFunc(t.BlockedBy, func(x model.ULID) bool { return x == target })
				r.movedRef(u, "blocked_by", target)
			}
		}
		for _, f := range slices.Clone(t.Props) {
			if p, ok := sc.Prop(f.Key); !ok || p.Type != schema.TypeTaskRef || f.Value == nil {
				continue
			}
			for _, target := range r.dropFromProp(t, f) {
				r.movedRef(u, "props."+f.Key, target)
			}
		}
	}
}

// dropFromProp removes the forwards from the task_ref property f of t, and
// the property itself when nothing is left. It returns the ULIDs removed.
func (r *repairer) dropFromProp(t *model.Task, f model.Field) []model.ULID {
	var gone []model.ULID
	isGone := func(n *yaml.Node) bool {
		if n.Kind != yaml.ScalarNode {
			return false
		}
		u, err := model.ParseULID(n.Value)
		if err != nil || !r.forwards[u] {
			return false
		}
		gone = append(gone, u)
		return true
	}
	if f.Value.Kind == yaml.SequenceNode {
		f.Value.Content = slices.DeleteFunc(f.Value.Content, isGone)
		if len(gone) > 0 && len(f.Value.Content) == 0 {
			t.Props.Delete(f.Key)
		}
	} else if isGone(f.Value) {
		t.Props.Delete(f.Key)
	}
	return gone
}

func (r *repairer) movedRef(u model.ULID, field string, target model.ULID) {
	r.dirty[u] = true
	v, _ := json.Marshal(struct {
		Field  string `json:"field"`
		Target string `json:"target"`
	}{field, target.String()}) // strings always encode
	r.tell(u, "ref_moved", v, model.TaskPath(u))
}

// dropUnderForwards removes the comment and worklog files under a forward,
// whole, and records each as lost.
func (r *repairer) dropUnderForwards() {
	if len(r.forwards) == 0 {
		return
	}
	for _, dir := range []struct {
		prefix string
		parse  func(string) (model.ULID, model.ULID, error)
	}{{"comments/", model.ParseCommentPath}, {"worklogs/", model.ParseWorklogPath}} {
		for _, p := range r.in.Files.Paths(dir.prefix) {
			if task, _, err := dir.parse(p); err != nil || !r.forwards[task] {
				continue
			}
			v, _ := json.Marshal(string(r.in.Files.Read(p))) // a string always encodes
			r.dropped = append(r.dropped, Overwritten{Path: p, Field: "*", Value: string(v), Author: r.in.Author(p)})
			r.drop = append(r.drop, p)
		}
	}
}

// followParents moves each task to the workspace of its parent, depth
// first from the roots, so a moved child takes its subtree along.
func (r *repairer) followParents() {
	children := map[model.ULID][]model.ULID{}
	var roots []model.ULID
	for _, u := range r.order {
		if p := r.tasks[u].Parent; p != nil && r.tasks[*p] != nil {
			children[*p] = append(children[*p], u)
		} else {
			roots = append(roots, u)
		}
	}
	var visit func(model.ULID)
	visit = func(u model.ULID) {
		w := r.tasks[u].Workspace
		for _, c := range children[u] {
			if t := r.tasks[c]; !w.IsZero() && t.Workspace != w {
				old := jsonOf(t.Workspace)
				t.Workspace = w
				r.dirty[c] = true
				r.moved[c] = true
				r.tell(c, "workspace_mismatch", old, model.TaskPath(c))
			}
			visit(c)
		}
	}
	for _, u := range roots {
		visit(u)
	}
}

// prefix returns the prefix of t's workspace, or false when t cannot be
// numbered: its workspace is not there or its prefix is not well formed.
func (r *repairer) prefix(t *model.Task) (string, bool) {
	w := r.ws[t.Workspace]
	if w == nil || model.ValidIDPrefix(w.IDPrefix) != nil {
		return "", false
	}
	return w.IDPrefix, true
}

// renumber gives a new number to each task whose id does not fit its
// workspace's prefix, and to all but one of the tasks that share an id.
// The task kept is the one origin has with that id when there is exactly
// one, else the one with the smallest ULID. Tasks renumbered for the
// prefix, and tasks that cannot be numbered (no workspace, or a prefix not
// well formed), are not counted among those that share an id. Numbers are
// given in one ULID order over all the tasks, whatever id they shared,
// after the largest number in use.
func (r *repairer) renumber() error {
	const collision, mismatch = 1, 2
	need := map[model.ULID]int{}
	for _, u := range r.order {
		t := r.tasks[u]
		p, ok := r.prefix(t)
		if !ok {
			continue
		}
		if q, _, ok := model.ParseTaskID(t.ID); !ok || q != p {
			need[u] = mismatch
		}
	}
	byID := map[string][]model.ULID{}
	for _, u := range r.order {
		if _, ok := r.prefix(r.tasks[u]); ok && r.tasks[u].ID != "" && need[u] == 0 {
			byID[r.tasks[u].ID] = append(byID[r.tasks[u].ID], u)
		}
	}
	for _, id := range slices.Sorted(maps.Keys(byID)) {
		g := byID[id]
		if len(g) < 2 {
			continue
		}
		keep := g[0]
		if on := slices.DeleteFunc(slices.Clone(g), func(u model.ULID) bool { return !r.in.OnOrigin(u, id) }); len(on) == 1 {
			keep = on[0]
		}
		for _, u := range g {
			if u != keep {
				need[u] = collision
			}
		}
	}
	if len(need) == 0 {
		return nil
	}
	used, err := r.usedNumbers()
	if err != nil {
		return err
	}
	var collided, renumbered []Repaired
	for _, u := range r.order {
		if need[u] == 0 {
			continue
		}
		t := r.tasks[u]
		p, _ := r.prefix(t)
		old := t.ID
		used[p]++
		t.ID = model.FormatTaskID(p, used[p])
		if old != "" && !slices.Contains(t.FormerIDs, old) {
			t.FormerIDs = append(t.FormerIDs, old)
		}
		r.dirty[u] = true
		switch {
		case need[u] == collision:
			v, _ := json.Marshal(struct {
				Old string `json:"old"`
				New string `json:"new"`
			}{old, t.ID}) // strings always encode
			collided = append(collided, Repaired{ULID: u, Kind: "id_collision", Value: string(v), Author: r.in.Author(model.TaskPath(u))})
		case !r.moved[u]:
			v := []byte("null")
			if old != "" {
				v = jsonOf(old)
			}
			renumbered = append(renumbered, Repaired{ULID: u, Kind: "id_renumbered", Value: string(v)})
		}
	}
	r.done = append(append(r.done, collided...), renumbered...)
	return nil
}

// usedNumbers counts the numbers in use as numbering does in the store:
// the ids and former ids of every task, the words of the form of an id in
// task files that do not decode, and numbers.yaml.
func (r *repairer) usedNumbers() (map[string]int, error) {
	used := map[string]int{}
	for _, t := range r.tasks {
		model.CountIDs(used, t)
	}
	for _, b := range r.broken {
		model.RaiseNumbers(used, model.IDNumbersIn(b))
	}
	if b := r.in.Files.Read(model.NumbersPath); b != nil {
		n, err := model.DecodeNumbers(b)
		if err != nil {
			return nil, err
		}
		model.RaiseNumbers(used, n)
	}
	return used, nil
}

// duplicates tells of workspaces that share a prefix, or a name in any
// case, with another. The later one of each pair is told: by when the
// prefix was set, or when the workspace was created, then by ULID.
func (r *repairer) duplicates() {
	type rule struct {
		kind  string
		key   func(*model.Workspace) string
		since func(*model.Workspace) time.Time
	}
	for _, x := range []rule{
		{"prefix_duplicate", func(w *model.Workspace) string { return w.IDPrefix }, func(w *model.Workspace) time.Time { return w.PrefixSet }},
		{"name_duplicate", func(w *model.Workspace) string { return strings.ToLower(w.Name) }, func(w *model.Workspace) time.Time { return w.Created }},
	} {
		groups := map[string][]*model.Workspace{}
		for _, u := range slices.SortedFunc(maps.Keys(r.ws), compareULIDs) {
			if w := r.ws[u]; x.key(w) != "" {
				groups[x.key(w)] = append(groups[x.key(w)], w)
			}
		}
		for _, k := range slices.Sorted(maps.Keys(groups)) {
			g := groups[k]
			slices.SortStableFunc(g, func(a, b *model.Workspace) int { return x.since(a).Compare(x.since(b)) })
			for _, w := range g[1:] {
				v := w.IDPrefix
				if x.kind == "name_duplicate" {
					v = w.Name
				}
				r.tell(w.ULID, x.kind, jsonOf(v), model.WorkspacePath(w.ULID))
			}
		}
	}
}

// files encodes what the repair changed, updated now.
func (r *repairer) files() (map[string][]byte, error) {
	if len(r.dirty)+len(r.dirtyWS)+len(r.raw)+len(r.drop) == 0 {
		return nil, nil
	}
	out := maps.Clone(r.raw)
	if out == nil {
		out = map[string][]byte{}
	}
	for _, p := range r.drop {
		out[p] = nil
	}
	for u := range r.dirtyWS {
		w := r.ws[u]
		w.Updated = r.in.Now
		b, err := w.Encode()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", model.WorkspacePath(u), err)
		}
		out[model.WorkspacePath(u)] = b
	}
	for u := range r.dirty {
		t := r.tasks[u]
		t.Updated = r.in.Now
		b, err := t.Encode()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", model.TaskPath(u), err)
		}
		out[model.TaskPath(u)] = b
	}
	return out, nil
}
