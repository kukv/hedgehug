// Package browse reads the stores: hh ls, tree, show, log and people.
package browse

import (
	"cmp"
	"errors"
	"slices"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/history"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/storage/mount"
	"github.com/kukv/hedgehug/internal/app/storage/query"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

// listSet opens the stores a listing runs on: every store of config.yaml, or
// the one --store names. The set is the caller's own, not the one Mount
// caches: what a listing returns belongs to the indexes of its stores, which
// another operation of the session must not refresh under it. A store that
// does not open is warned of; the one --store names, and the only one
// config.yaml lists, are an error.
func listSet(ss *usecase.Session) (*mount.Set, error) {
	c, _, err := ss.Config()
	if err != nil {
		return nil, err
	}
	if ss.StoreName != "" || len(c.Stores) == 0 {
		// not found, or none at all, as the other operations say
		if _, _, err := ss.Entry(); err != nil {
			return nil, err
		}
	}
	set, toWarn, err := ss.OpenSet(c, true)
	if err != nil {
		return nil, err
	}
	ss.Keep(set.Close)
	for _, sk := range toWarn {
		ss.WarnSkipped(sk)
	}
	return set, nil
}

// MountedCount is how many of the stores of config.yaml open, which is what
// "two or more stores are mounted" counts. Nothing is warned of.
func MountedCount(ss *usecase.Session) (int, error) {
	c, _, err := ss.Config()
	if err != nil {
		return 0, err
	}
	set := mount.Open(c, mount.Options{Now: func() time.Time { return ss.Now() }})
	defer set.Close()
	return len(set.Stores), nil
}

// queryMounted runs q on the stores of set that store: and --store leave. A
// query that does not run is a usage error.
func queryMounted(ss *usecase.Session, set *mount.Set, q *query.Query) (*Result, error) {
	res, err := Query(set, q, ss.StoreName, func(m *mount.Mounted) (query.Options, error) {
		return ss.QueryOptions(m.Store)
	})
	if err != nil {
		var qe *query.Error
		if errors.As(err, &qe) {
			return nil, usecase.NewErr(usecase.KindUsage, "query.invalid", err)
		}
		return nil, usecase.Classify(err)
	}
	return res, nil
}

// GroupView is a group of a list: the tasks of every store with the value Key.
type GroupView struct {
	Key   string              `json:"key" doc:"The value of the group; empty for the tasks with none"`
	ULIDs []string            `json:"ulids"`
	Tasks []*usecase.TaskView `json:"-"`
}

// TaskList is what GET /api/tasks and hh ls --json give.
type TaskList struct {
	Tasks  *[]*usecase.TaskView `json:"tasks,omitempty" doc:"The tasks found, without tree"`
	Groups []GroupView          `json:"groups,omitempty" doc:"The groups, when the query has group:"`
	Roots  *[]*usecase.TaskView `json:"roots,omitempty" doc:"The roots of the tree, with tree"`
	Cycle  []*usecase.TaskView  `json:"cycle,omitempty" doc:"The tasks in a parent cycle, with tree"`
}

// ListView is what a query found: Tasks in the order of the groups, and the
// Groups themselves (only when the query has group:). Many says two or more
// stores are mounted, so that a reader tells which store a task is in.
type ListView struct {
	Tasks  []*usecase.TaskView
	Groups []GroupView
	Many   bool
}

// List runs the query on every store (or --store, or the store: of it). A
// query that does not parse is a usage error.
func List(ss *usecase.Session, queryText string) (ListView, error) {
	q, err := usecase.ParseQuery(queryText)
	if err != nil {
		return ListView{}, err
	}
	set, err := listSet(ss)
	if err != nil {
		return ListView{}, err
	}
	res, err := queryMounted(ss, set, q)
	if err != nil {
		return ListView{}, err
	}
	views := map[*index.Entry]*usecase.TaskView{}
	view := func(e Entry) (*usecase.TaskView, error) {
		if v, ok := views[e.Entry]; ok {
			return v, nil
		}
		ix, err := e.Store.Index()
		if err != nil {
			return nil, usecase.Classify(err)
		}
		v := usecase.ListViewOf(ix, e.Entry)
		views[e.Entry] = v
		return v, nil
	}
	out := ListView{Tasks: make([]*usecase.TaskView, 0, len(res.Entries)), Many: len(set.Stores) > 1}
	for _, e := range res.Entries {
		v, err := view(e)
		if err != nil {
			return ListView{}, err
		}
		out.Tasks = append(out.Tasks, v)
	}
	if usecase.HasTerm(q, "group") {
		for _, g := range res.Groups {
			gv := GroupView{Key: g.Key, ULIDs: make([]string, len(g.Entries)), Tasks: make([]*usecase.TaskView, len(g.Entries))}
			for i, e := range g.Entries {
				v, err := view(e)
				if err != nil {
					return ListView{}, err
				}
				gv.ULIDs[i], gv.Tasks[i] = v.ULID, v
			}
			out.Groups = append(out.Groups, gv)
		}
	}
	return out, nil
}

// JSON is v as GET /api/tasks and hh ls --json give it.
func (v ListView) JSON() TaskList {
	tasks := v.Tasks
	if tasks == nil {
		tasks = []*usecase.TaskView{}
	}
	return TaskList{Tasks: &tasks, Groups: v.Groups}
}

// treeNode is one task in ls --tree.
type treeNode struct {
	Entry    *index.Entry
	Context  bool // shown only to connect the tasks found
	InCycle  bool // in a parent cycle, out of reach of the roots
	Children []*treeNode
}

// TreeStoreView is the tree of the tasks a query found in one store. Cycle
// holds the tasks in a parent cycle, which no root reaches. The children of a
// task are in Children.
type TreeStoreView struct {
	Store        string
	Roots, Cycle []*usecase.TaskView
}

// TreeView is the trees of the tasks a query found, one per store it ran on,
// in config order (a store with no match has empty ones). Many says there are
// two or more stores.
type TreeView struct {
	Stores []TreeStoreView
	Many   bool
}

// Tree runs the query and arranges what it found under its ancestors, in each
// store. A query that groups or sorts is a usage error: the tree is in the
// order of the ranks.
func Tree(ss *usecase.Session, queryText string) (TreeView, error) {
	q, err := usecase.ParseQuery(queryText)
	if err != nil {
		return TreeView{}, err
	}
	if usecase.HasTerm(q, "group") || usecase.HasTerm(q, "sort") {
		//nolint:revive,staticcheck // the message is printed as it always was
		return TreeView{}, usecase.NewErr(usecase.KindUsage, "query.tree_order", errors.New("--tree orders tasks by rank; it takes neither group: nor sort:"))
	}
	set, err := listSet(ss)
	if err != nil {
		return TreeView{}, err
	}
	res, err := queryMounted(ss, set, q)
	if err != nil {
		return TreeView{}, err
	}
	out := TreeView{Many: len(set.Stores) > 1}
	for _, m := range res.Stores {
		ix, err := m.Index()
		if err != nil {
			return TreeView{}, usecase.Classify(err)
		}
		var found []*index.Entry
		for _, e := range res.Entries {
			if e.Store == m {
				found = append(found, e.Entry)
			}
		}
		roots, cycle := treeOf(ix, found)
		out.Stores = append(out.Stores, TreeStoreView{Store: m.Name, Roots: treeViews(ix, roots), Cycle: treeViews(ix, cycle)})
	}
	return out, nil
}

func treeViews(ix *index.Index, nodes []*treeNode) []*usecase.TaskView {
	var out []*usecase.TaskView
	for _, n := range nodes {
		out = append(out, treeViewOf(ix, n))
	}
	return out
}

// Drawn lists the tasks of the roots and then of the cycle, with the tasks
// under each, in the order they are drawn.
func (s TreeStoreView) Drawn() []*usecase.TaskView {
	var out []*usecase.TaskView
	var walk func(v *usecase.TaskView)
	walk = func(v *usecase.TaskView) {
		out = append(out, v)
		for _, c := range v.Children {
			walk(c)
		}
	}
	for _, v := range slices.Concat(s.Roots, s.Cycle) {
		walk(v)
	}
	return out
}

// JSON is t as GET /api/tasks?tree and hh ls --tree --json give it: the roots
// of every store, and the tasks in a parent cycle of every store.
func (t TreeView) JSON() TaskList {
	roots := []*usecase.TaskView{}
	var cycle []*usecase.TaskView
	for _, s := range t.Stores {
		roots = append(roots, s.Roots...)
		cycle = append(cycle, s.Cycle...)
	}
	return TaskList{Roots: &roots, Cycle: cycle}
}

// treeOf builds the tree of the tasks found and of the ancestors that
// connect them to a root. Tasks in a parent cycle never reach a root; they
// are returned apart, by ULID, each as the root of the tasks under it that
// are not in the cycle.
func treeOf(ix *index.Index, found []*index.Entry) (roots, cycle []*treeNode) {
	match := map[model.ULID]bool{}
	show := map[model.ULID]bool{}
	for _, e := range found {
		match[e.ULID] = true
		for cur := e; cur != nil && !show[cur.ULID]; {
			show[cur.ULID] = true
			if cur.Parent == nil {
				break
			}
			cur, _ = ix.Task(*cur.Parent)
		}
	}
	seen := map[model.ULID]bool{}
	var build func(e *index.Entry) *treeNode
	build = func(e *index.Entry) *treeNode {
		seen[e.ULID] = true
		n := &treeNode{Entry: e, Context: !match[e.ULID]}
		for _, c := range ix.Children(e.ULID) {
			if show[c.ULID] && !seen[c.ULID] {
				n.Children = append(n.Children, build(c))
			}
		}
		return n
	}
	for _, r := range ix.Roots() {
		if show[r.ULID] {
			roots = append(roots, build(r))
		}
	}
	// What the roots do not reach lies in a cycle or under one. Mark the
	// tasks in the cycle seen first, so that none is drawn under another.
	var members []*index.Entry
	for ulid := range show {
		if !seen[ulid] && inParentCycle(ix, ulid) {
			e, _ := ix.Task(ulid)
			members = append(members, e)
		}
	}
	slices.SortFunc(members, func(x, y *index.Entry) int { return cmp.Compare(x.ULID.String(), y.ULID.String()) })
	for _, e := range members {
		seen[e.ULID] = true
	}
	for _, e := range members {
		n := build(e)
		n.InCycle = true
		cycle = append(cycle, n)
	}
	return roots, cycle
}

// inParentCycle reports whether the parents of ulid lead back to it.
func inParentCycle(ix *index.Index, ulid model.ULID) bool {
	for _, p := range ix.Problems(ulid) {
		if p.Code == schema.CodeCycle && p.Field == "parent" {
			return true
		}
	}
	return false
}

// treeViewOf is the task of n with its children, as --json prints it.
func treeViewOf(ix *index.Index, n *treeNode) *usecase.TaskView {
	j := usecase.ListViewOf(ix, n.Entry)
	j.Context, j.InCycle = n.Context, n.InCycle
	for _, c := range n.Children {
		j.Children = append(j.Children, treeViewOf(ix, c))
	}
	return j
}

// ShowHistory is how many commits show lists under History.
const ShowHistory = 5

// Show reads the task arg names. The Body, Watchers, Repos and Comments of a
// task whose file does not decode are empty.
func Show(ss *usecase.Session, arg string) (*usecase.ShowView, error) {
	ts, arg, err := ss.ForTask(arg)
	if err != nil {
		return nil, err
	}
	if ts != ss {
		return Show(ts, arg)
	}
	s, ix, err := ss.Open()
	if err != nil {
		return nil, err
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return nil, err
	}
	j := &usecase.ShowView{TaskView: usecase.TaskViewOf(ix, e), Watchers: []string{}, Repos: []string{}, Comments: []usecase.CommentView{}, Reverse: map[string][]usecase.Ref{}, ReferencedBy: []usecase.Ref{}}
	j.Checklist, j.Progress = usecase.ChecklistOf(e), usecase.ProgressOf(ix, e)
	if e.Broken == "" {
		t, err := s.Task(e.ULID)
		if err != nil {
			return nil, err
		}
		j.Body, j.Watchers, j.Repos, j.CreatedBy = t.Body, usecase.OrEmpty(t.Watchers), usecase.OrEmpty(t.Repos), t.CreatedBy
		j.Links = usecase.LinkViewsOf(t)
		cs, err := liveComments(s, e.ULID)
		if err != nil {
			return nil, err
		}
		for _, c := range cs {
			j.Comments = append(j.Comments, usecase.CommentView{ULID: c.ULID.String(), Author: c.Author, Created: c.Created, Body: c.Body})
		}
	}
	j.Relations = reverse(ix, e.ULID)
	for _, rel := range j.Relations {
		j.Reverse[rel.Key] = append(j.Reverse[rel.Key], rel.Tasks...)
	}
	for _, x := range ix.Backlinks(e.ULID) {
		j.ReferencedBy = append(j.ReferencedBy, usecase.RefOf(ix, x.ULID))
	}
	entries, err := TaskLog(ss, s, ix.StoreName(), e.ULID, ShowHistory)
	if err != nil {
		return nil, err
	}
	j.History = make([]usecase.LogView, len(entries))
	for i, le := range entries {
		j.History[i] = usecase.LogViewOf(ix, le)
	}
	return j, nil
}

func liveComments(s *store.Store, ulid model.ULID) ([]*model.Comment, error) {
	var out []*model.Comment
	for c, err := range s.Comments(ulid) {
		if err != nil {
			return nil, err
		}
		if c.DeletedAt == nil {
			out = append(out, c)
		}
	}
	return out, nil
}

// reverse finds the tasks that point at ulid, by relation.
func reverse(ix *index.Index, ulid model.ULID) []usecase.RelationView {
	var out []usecase.RelationView
	refs := func(es []*index.Entry) []usecase.Ref {
		rs := make([]usecase.Ref, len(es))
		for i, x := range es {
			rs[i] = usecase.RefOf(ix, x.ULID)
		}
		return rs
	}
	if es := ix.Reverse(ulid, "blocked_by"); len(es) > 0 {
		out = append(out, usecase.RelationView{Key: "blocks", Label: "blocks", Tasks: refs(es)})
	}
	for _, p := range ix.Schema().Props {
		if p.Type != schema.TypeTaskRef {
			continue
		}
		es := ix.Reverse(ulid, p.Name)
		if len(es) == 0 {
			continue
		}
		label := p.Name + " (from)"
		switch {
		case p.Inverse != "":
			label = p.Inverse
		case p.Symmetric && p.Label != "":
			label = p.Label
		case p.Symmetric:
			label = p.Name
		}
		out = append(out, usecase.RelationView{Key: p.Name, Label: label, Tasks: refs(es)})
	}
	return out
}

// commitWorkspace is the workspace of a commit of the history index.
func commitWorkspace(ix *index.Index, c history.Commit) (model.ULID, bool) {
	return usecase.CommitWorkspace(ix, c.Workspace, c.Task)
}

// LogResult is the commits Log found. Many says they were taken from two or
// more stores, so that a reader tells which store a commit is in.
type LogResult struct {
	Logs []usecase.LogView
	Many bool
}

// Log lists the commits of every store, newest first (a commit of the same
// time comes in the order of the stores), or of the task args[0] names, in
// its store; limit 0 lists all, and limit counts the commits of all the
// stores together.
func Log(ss *usecase.Session, args []string, limit int) (LogResult, error) {
	return LogIn(ss, args, nil, limit)
}

// LogIn is Log with only the commits of the workspaces named (by name, id
// prefix or ULID, as elsewhere); none named takes every commit. A name is
// looked up in each store, as workspace: in a query is: it is an error only
// when no store has it. limit counts the commits that are kept. Naming
// workspaces with a task is a usage error.
func LogIn(ss *usecase.Session, args, workspaces []string, limit int) (LogResult, error) {
	if len(args) > 0 && len(workspaces) > 0 {
		return LogResult{}, usecase.NewErr(usecase.KindUsage, "log.workspace_with_task", errors.New("a task's log takes no --workspace"))
	}
	if len(args) > 0 {
		ts, arg, err := ss.ForTask(args[0])
		if err != nil {
			return LogResult{}, err
		}
		if ts != ss {
			return Log(ts, append([]string{arg}, args[1:]...), limit)
		}
		s, ix, err := ss.Open()
		if err != nil {
			return LogResult{}, err
		}
		e, err := ss.Resolve(ix, arg)
		if err != nil {
			return LogResult{}, err
		}
		entries, err := TaskLog(ss, s, ix.StoreName(), e.ULID, limit)
		if err != nil {
			return LogResult{}, err
		}
		return LogResult{Logs: storeLogs(ix.StoreName(), ix, entries)}, nil
	}
	set, err := listSet(ss)
	if err != nil {
		return LogResult{}, err
	}
	stores := set.Stores
	if ss.StoreName != "" {
		m, _ := set.Get(ss.StoreName)
		stores = []*mount.Mounted{m}
	}
	found := make([]bool, len(workspaces))
	notFound := make([]error, len(workspaces))
	all := []usecase.LogView{}
	for _, m := range stores {
		var entries []gitrepo.LogEntry
		var logErr error
		if err := m.With(func(ix *index.Index) {
			var h *history.History
			if h, logErr = ss.OpenHistory(m.Store, m.Name); logErr != nil {
				return
			}
			cs := h.All()
			if len(workspaces) > 0 {
				keep := map[model.ULID]bool{}
				for i, arg := range workspaces {
					w, err := usecase.ResolveWorkspace(ix, arg)
					if errors.Is(err, store.ErrNotFound) {
						notFound[i] = err // another store may have it
						continue
					}
					if err != nil {
						logErr = err
						return
					}
					keep[w.ULID], found[i] = true, true
				}
				cs = slices.DeleteFunc(cs, func(c history.Commit) bool {
					ws, ok := commitWorkspace(ix, c)
					return !ok || !keep[ws]
				})
			}
			entries, logErr = h.Entries(cs, limit)
		}); err != nil {
			return LogResult{}, usecase.Classify(err)
		}
		if logErr != nil {
			return LogResult{}, logErr
		}
		ix, err := m.Index()
		if err != nil {
			return LogResult{}, usecase.Classify(err)
		}
		all = append(all, storeLogs(m.Name, ix, entries)...)
	}
	for i, ok := range found {
		if !ok {
			return LogResult{}, notFound[i]
		}
	}
	slices.SortStableFunc(all, func(a, b usecase.LogView) int { return b.Date.Compare(a.Date) })
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return LogResult{Logs: all, Many: len(set.Stores) > 1}, nil
}

// storeLogs is the commits entries of the store name, whose index is ix.
func storeLogs(name string, ix *index.Index, entries []gitrepo.LogEntry) []usecase.LogView {
	out := make([]usecase.LogView, len(entries))
	for i, e := range entries {
		out[i] = usecase.LogViewOf(ix, e)
		out[i].Store = name
	}
	return out
}

// TaskLog returns the commits that touched the task, its comments or its
// work logs, newest first. name is the name of the store s, for the progress
// of building its history index.
func TaskLog(ss *usecase.Session, s *store.Store, name string, ulid model.ULID, limit int) ([]gitrepo.LogEntry, error) {
	h, err := ss.OpenHistory(s, name)
	if err != nil {
		return nil, err
	}
	return h.Entries(h.Task(ulid), limit)
}
