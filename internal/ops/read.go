package ops

import (
	"cmp"
	"errors"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/mount"
	"github.com/kukv/hedgehug/internal/query"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

// ParseQuery parses the words of a query. A query that does not parse is a
// usage error.
func ParseQuery(s string) (*query.Query, error) {
	q, err := query.Parse(s)
	if err != nil {
		return nil, newErr(KindUsage, "query.invalid", err)
	}
	return q, nil
}

// listSet opens the stores a listing runs on: every store of config.yaml, or
// the one --store names. The set is the caller's own, not the one Mount
// caches: what a listing returns belongs to the indexes of its stores, which
// another operation of the session must not refresh under it. A store that
// does not open is warned of; the one --store names, and the only one
// config.yaml lists, are an error.
func (ss *Session) listSet() (*mount.Set, error) {
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
	set, err := mount.Open(c, mount.Options{Now: func() time.Time { return ss.Now() }, Warn: ss.warn, Qualified: ss.qualified(c)})
	if err != nil {
		return nil, err
	}
	for _, sk := range set.Skipped {
		if ss.StoreName == sk.Name || len(c.Stores) == 1 {
			return nil, openError(sk.Name, sk.Err)
		}
		ss.warnSkipped(sk)
	}
	return set, nil
}

// queryMounted runs q on the stores of set that store: and --store leave. A
// query that does not run is a usage error.
func (ss *Session) queryMounted(set *mount.Set, q *query.Query) (*mount.Result, error) {
	res, err := mount.Query(set, q, ss.StoreName, func(m *mount.Mounted) (query.Options, error) {
		return ss.QueryOptions(m.Store)
	})
	if err != nil {
		var qe *query.Error
		if errors.As(err, &qe) {
			return nil, newErr(KindUsage, "query.invalid", err)
		}
		return nil, classify(err)
	}
	return res, nil
}

// ListResult is what a query found: Entries in the order of the groups, and
// the Groups themselves. Stores are the stores it ran on; Many says two or
// more stores are mounted, so that a reader tells which store a task is in.
// What Entries hold is read through their Store's Index.
type ListResult struct {
	Stores  []*mount.Mounted
	Entries []mount.Entry
	Groups  []mount.Group
	Many    bool
}

// List runs q on every store (or --store, or the store: of q).
func (ss *Session) List(q *query.Query) (ListResult, error) {
	set, err := ss.listSet()
	if err != nil {
		return ListResult{}, err
	}
	res, err := ss.queryMounted(set, q)
	if err != nil {
		return ListResult{}, err
	}
	return ListResult{Stores: res.Stores, Entries: res.Entries, Groups: res.Groups, Many: len(set.Stores) > 1}, nil
}

// HasTerm reports whether q has a term with the key.
func HasTerm(q *query.Query, key string) bool {
	for _, t := range q.Terms {
		if t.Key == key {
			return true
		}
	}
	return false
}

// TreeNode is one task in ls --tree.
type TreeNode struct {
	Entry    *index.Entry
	Context  bool // shown only to connect the tasks found
	InCycle  bool // in a parent cycle, out of reach of the roots
	Children []*TreeNode
}

// TreeStore is the tree of the tasks a query found in one store. Cycle holds
// the tasks in a parent cycle, which no root reaches.
type TreeStore struct {
	Store        *mount.Mounted
	Index        *index.Index
	Roots, Cycle []*TreeNode
}

// TreeResult is the trees of the tasks a query found, one per store it ran
// on, in config order (a store with no match has empty ones). Many says there
// are two or more stores.
type TreeResult struct {
	Stores []TreeStore
	Many   bool
}

// Tree runs q and arranges what it found under its ancestors, in each store.
// A query that groups or sorts is a usage error: the tree is in the order of
// the ranks.
func (ss *Session) Tree(q *query.Query) (TreeResult, error) {
	if HasTerm(q, "group") || HasTerm(q, "sort") {
		//nolint:revive,staticcheck // the message is printed as it always was
		return TreeResult{}, newErr(KindUsage, "query.tree_order", errors.New("--tree orders tasks by rank; it takes neither group: nor sort:"))
	}
	set, err := ss.listSet()
	if err != nil {
		return TreeResult{}, err
	}
	res, err := ss.queryMounted(set, q)
	if err != nil {
		return TreeResult{}, err
	}
	out := TreeResult{Many: len(set.Stores) > 1}
	for _, m := range res.Stores {
		ix, err := m.Index()
		if err != nil {
			return TreeResult{}, classify(err)
		}
		var found []*index.Entry
		for _, e := range res.Entries {
			if e.Store == m {
				found = append(found, e.Entry)
			}
		}
		roots, cycle := treeOf(ix, found)
		out.Stores = append(out.Stores, TreeStore{Store: m, Index: ix, Roots: roots, Cycle: cycle})
	}
	return out, nil
}

// treeOf builds the tree of the tasks found and of the ancestors that
// connect them to a root. Tasks in a parent cycle never reach a root; they
// are returned apart, by ULID, each as the root of the tasks under it that
// are not in the cycle.
func treeOf(ix *index.Index, found []*index.Entry) (roots, cycle []*TreeNode) {
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
	var build func(e *index.Entry) *TreeNode
	build = func(e *index.Entry) *TreeNode {
		seen[e.ULID] = true
		n := &TreeNode{Entry: e, Context: !match[e.ULID]}
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

// TreeViewOf is the task of n with its children, as --json prints it.
func TreeViewOf(ix *index.Index, n *TreeNode) *TaskView {
	j := TaskViewOf(ix, n.Entry)
	j.Context, j.InCycle = n.Context, n.InCycle
	j.Checklist, j.Progress = ChecklistOf(n.Entry), ProgressOf(ix, n.Entry)
	for _, c := range n.Children {
		j.Children = append(j.Children, TreeViewOf(ix, c))
	}
	return j
}

// Flatten lists the trees under roots in the order they are drawn.
func Flatten(roots []*TreeNode) []*TreeNode {
	var out []*TreeNode
	var walk func(n *TreeNode)
	walk = func(n *TreeNode) {
		out = append(out, n)
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, n := range roots {
		walk(n)
	}
	return out
}

// ShowHistory is how many commits show lists under History.
const ShowHistory = 5

// Relation is one kind of task pointing at the task shown.
type Relation struct {
	Key   string // the name in JSON: blocks, or the property
	Label string // the name in the text
	Tasks []*index.Entry
}

// ShowResult is a task with what points at it. Task and Comments are empty
// when the file of the task does not decode.
type ShowResult struct {
	Store     string // the name of the store the task is in
	Index     *index.Index
	Entry     *index.Entry
	Task      *model.Task
	Comments  []*model.Comment
	Relations []Relation
	Back      []*index.Entry
	History   []gitrepo.LogEntry
}

// Show reads the task arg names.
func (ss *Session) Show(arg string) (ShowResult, error) {
	ts, arg, err := ss.forTask(arg)
	if err != nil {
		return ShowResult{}, err
	}
	if ts != ss {
		return ts.Show(arg)
	}
	s, ix, err := ss.Open()
	if err != nil {
		return ShowResult{}, err
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return ShowResult{}, err
	}
	r := ShowResult{Store: ix.StoreName(), Index: ix, Entry: e}
	if e.Broken == "" {
		if r.Task, err = s.Task(e.ULID); err != nil {
			return ShowResult{}, err
		}
		if r.Comments, err = liveComments(s, e.ULID); err != nil {
			return ShowResult{}, err
		}
	}
	r.Relations = reverse(ix, e.ULID)
	r.Back = ix.Backlinks(e.ULID)
	if r.History, err = ss.TaskLog(s, e.ULID, ShowHistory); err != nil {
		return ShowResult{}, err
	}
	return r, nil
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
func reverse(ix *index.Index, ulid model.ULID) []Relation {
	var out []Relation
	if es := ix.Reverse(ulid, "blocked_by"); len(es) > 0 {
		out = append(out, Relation{"blocks", "blocks", es})
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
		out = append(out, Relation{p.Name, label, es})
	}
	return out
}

// CommentView is a comment as --json prints it.
type CommentView struct {
	ULID    string    `json:"ulid"`
	Author  string    `json:"author"`
	Created time.Time `json:"created"`
	Body    string    `json:"body"`
}

// ShowView is a task as show --json prints it.
type ShowView struct {
	*TaskView
	Body     string           `json:"body"`
	Watchers []string         `json:"watchers"`
	Repos    []string         `json:"repos"`
	Comments []CommentView    `json:"comments"`
	Reverse  map[string][]Ref `json:"reverse"`
	// ReferencedBy are the tasks whose body or comments write this one as #<ID>.
	ReferencedBy []Ref     `json:"referenced_by"`
	History      []LogView `json:"history"`
}

// ShowViewOf is r as show --json prints it.
func ShowViewOf(r ShowResult) *ShowView {
	j := &ShowView{TaskView: TaskViewOf(r.Index, r.Entry), Watchers: []string{}, Repos: []string{}, Comments: []CommentView{}, Reverse: map[string][]Ref{}, ReferencedBy: []Ref{}, History: LogViewsOf(r.Index, r.History)}
	j.Checklist, j.Progress = ChecklistOf(r.Entry), ProgressOf(r.Index, r.Entry)
	for _, x := range r.Back {
		j.ReferencedBy = append(j.ReferencedBy, Ref{ULID: x.ULID.String(), ID: x.ID, Title: x.Title})
	}
	if r.Task != nil {
		j.Body, j.Watchers, j.Repos = r.Task.Body, OrEmpty(r.Task.Watchers), OrEmpty(r.Task.Repos)
	}
	for _, c := range r.Comments {
		j.Comments = append(j.Comments, CommentView{c.ULID.String(), c.Author, c.Created, c.Body})
	}
	for _, rel := range r.Relations {
		for _, x := range rel.Tasks {
			j.Reverse[rel.Key] = append(j.Reverse[rel.Key], Ref{ULID: x.ULID.String(), ID: x.ID, Title: x.Title})
		}
	}
	return j
}

// LogView is a commit as --json prints it.
type LogView struct {
	Hash   string    `json:"hash"`
	Date   time.Time `json:"date"`
	Author struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"author"`
	Store   string  `json:"store"`
	Op      *string `json:"op"`
	Task    *string `json:"task"`
	ID      *string `json:"id"` // the current id of the task
	Subject string  `json:"subject"`
}

func logViewOf(ix *index.Index, e gitrepo.LogEntry) LogView {
	j := LogView{Hash: e.Hash.String(), Date: e.When, Subject: e.Subject, Store: ix.StoreName()}
	j.Author.Name, j.Author.Email = e.Author.Name, e.Author.Email
	if v, ok := e.Trailer("Hedgehug-Op"); ok {
		j.Op = &v
	}
	if v, ok := e.Trailer("Hedgehug-Task"); ok {
		j.Task = &v
		if ulid, err := model.ParseULID(v); err == nil {
			if t, ok := ix.Task(ulid); ok && t.ID != "" {
				j.ID = &t.ID
			}
		}
	}
	return j
}

// LogViewsOf is the commits as --json prints them.
func LogViewsOf(ix *index.Index, entries []gitrepo.LogEntry) []LogView {
	out := make([]LogView, len(entries))
	for i, e := range entries {
		out[i] = logViewOf(ix, e)
	}
	return out
}

// SubjectNow gives the subject of a Hedgehug commit with the task's current
// id and ULID in place of the label it was written with: the subject is
// "hedgehug: <op> <label> <title>". Other subjects, and tasks the index does
// not have, keep the subject as it is.
func SubjectNow(ix *index.Index, le gitrepo.LogEntry) string {
	op, ok1 := le.Trailer("Hedgehug-Op")
	taskULID, ok2 := le.Trailer("Hedgehug-Task")
	head := "hedgehug: " + op + " "
	rest, ok3 := strings.CutPrefix(le.Subject, head)
	if !ok1 || !ok2 || !ok3 {
		return le.Subject
	}
	ulid, err := model.ParseULID(taskULID)
	if err != nil {
		return le.Subject
	}
	e, ok := ix.Task(ulid)
	if !ok {
		return le.Subject
	}
	_, title, _ := strings.Cut(rest, " ")
	return strings.TrimRight(head+TaskLabel(ix, e)+" "+title, " ")
}

// StoreLog is a commit with the store it is in. Index is that store's.
type StoreLog struct {
	Store string
	Index *index.Index
	Entry gitrepo.LogEntry
}

// LogResult is the commits Log found. Many says they were taken from two or
// more stores, so that a reader tells which store a commit is in.
type LogResult struct {
	Logs []StoreLog
	Many bool
}

// StoreLogViews is the commits as --json prints them, each with its store.
func StoreLogViews(logs []StoreLog) []LogView {
	out := make([]LogView, len(logs))
	for i, l := range logs {
		out[i] = logViewOf(l.Index, l.Entry)
		out[i].Store = l.Store
	}
	return out
}

// Log lists the commits of every store, newest first (a commit of the same
// time comes in the order of the stores), or of the task args[0] names, in
// its store; limit 0 lists all, and limit counts the commits of all the
// stores together.
func (ss *Session) Log(args []string, limit int) (LogResult, error) {
	if len(args) > 0 {
		ts, arg, err := ss.forTask(args[0])
		if err != nil {
			return LogResult{}, err
		}
		if ts != ss {
			return ts.Log(append([]string{arg}, args[1:]...), limit)
		}
		s, ix, err := ss.Open()
		if err != nil {
			return LogResult{}, err
		}
		e, err := ss.Resolve(ix, arg)
		if err != nil {
			return LogResult{}, err
		}
		entries, err := ss.TaskLog(s, e.ULID, limit)
		if err != nil {
			return LogResult{}, err
		}
		return LogResult{Logs: storeLogs(ix.StoreName(), ix, entries)}, nil
	}
	set, err := ss.listSet()
	if err != nil {
		return LogResult{}, err
	}
	stores := set.Stores
	if ss.StoreName != "" {
		m, _ := set.Get(ss.StoreName)
		stores = []*mount.Mounted{m}
	}
	var all []StoreLog
	for _, m := range stores {
		var entries []gitrepo.LogEntry
		var logErr error
		if err := m.With(func(*index.Index) { entries, logErr = m.Store.Repo().Log(nil, limit) }); err != nil {
			return LogResult{}, classify(err)
		}
		if logErr != nil {
			return LogResult{}, logErr
		}
		ix, err := m.Index()
		if err != nil {
			return LogResult{}, classify(err)
		}
		all = append(all, storeLogs(m.Name, ix, entries)...)
	}
	slices.SortStableFunc(all, func(a, b StoreLog) int { return b.Entry.When.Compare(a.Entry.When) })
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return LogResult{Logs: all, Many: len(set.Stores) > 1}, nil
}

func storeLogs(name string, ix *index.Index, entries []gitrepo.LogEntry) []StoreLog {
	out := make([]StoreLog, len(entries))
	for i, e := range entries {
		out[i] = StoreLog{Store: name, Index: ix, Entry: e}
	}
	return out
}

// TaskLog returns the commits that touched the task, its comments or its
// work logs.
func (ss *Session) TaskLog(s *store.Store, ulid model.ULID, limit int) ([]gitrepo.LogEntry, error) {
	return s.Repo().Log([]string{
		model.TaskPath(ulid),
		path.Dir(model.CommentPath(ulid, model.ULID{})) + "/",
		path.Dir(model.WorklogPath(ulid, model.ULID{})) + "/",
	}, limit)
}
