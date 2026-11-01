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

// runQuery runs q on the store. A query that does not run is a usage error.
func (ss *Session) runQuery(q *query.Query) (*index.Index, *query.Result, []*index.Entry, error) {
	s, ix, err := ss.Open()
	if err != nil {
		return nil, nil, nil, err
	}
	opt, err := ss.QueryOptions(s)
	if err != nil {
		return nil, nil, nil, err
	}
	res, err := query.Run(ix, q, opt)
	if err != nil {
		var qe *query.Error
		if errors.As(err, &qe) {
			return nil, nil, nil, newErr(KindUsage, "query.invalid", err)
		}
		return nil, nil, nil, err
	}
	var all []*index.Entry
	for _, g := range res.Groups {
		all = append(all, g.Tasks...)
	}
	return ix, res, all, nil
}

// ListResult is what a query found: Entries in the order of the groups, and
// the Groups themselves.
type ListResult struct {
	Index   *index.Index
	Entries []*index.Entry
	Groups  []query.Group
}

// List runs q.
func (ss *Session) List(q *query.Query) (ListResult, error) {
	ix, res, all, err := ss.runQuery(q)
	if err != nil {
		return ListResult{}, err
	}
	return ListResult{Index: ix, Entries: all, Groups: res.Groups}, nil
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

// TreeResult is the tree of the tasks a query found. Cycle holds the tasks
// in a parent cycle, which no root reaches.
type TreeResult struct {
	Index        *index.Index
	Roots, Cycle []*TreeNode
}

// Tree runs q and arranges what it found under its ancestors. A query that
// groups or sorts is a usage error: the tree is in the order of the ranks.
func (ss *Session) Tree(q *query.Query) (TreeResult, error) {
	if HasTerm(q, "group") || HasTerm(q, "sort") {
		//nolint:revive,staticcheck // the message is printed as it always was
		return TreeResult{}, newErr(KindUsage, "query.tree_order", errors.New("--tree orders tasks by rank; it takes neither group: nor sort:"))
	}
	ix, _, found, err := ss.runQuery(q)
	if err != nil {
		return TreeResult{}, err
	}
	roots, cycle := treeOf(ix, found)
	return TreeResult{Index: ix, Roots: roots, Cycle: cycle}, nil
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
	s, ix, err := ss.Open()
	if err != nil {
		return ShowResult{}, err
	}
	e, err := ss.Resolve(ix, arg)
	if err != nil {
		return ShowResult{}, err
	}
	r := ShowResult{Index: ix, Entry: e}
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
	Op      *string `json:"op"`
	Task    *string `json:"task"`
	ID      *string `json:"id"` // the current id of the task
	Subject string  `json:"subject"`
}

func logViewOf(ix *index.Index, e gitrepo.LogEntry) LogView {
	j := LogView{Hash: e.Hash.String(), Date: e.When, Subject: e.Subject}
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
	return strings.TrimRight(head+TaskLabel(e)+" "+title, " ")
}

// Log lists the commits of the store, or of the task args[0] names, newest
// first; limit 0 lists all.
func (ss *Session) Log(args []string, limit int) (*index.Index, []gitrepo.LogEntry, error) {
	s, ix, err := ss.Open()
	if err != nil {
		return nil, nil, err
	}
	var entries []gitrepo.LogEntry
	if len(args) == 0 {
		entries, err = s.Repo().Log(nil, limit)
	} else {
		e, rerr := ss.Resolve(ix, args[0])
		if rerr != nil {
			return nil, nil, rerr
		}
		entries, err = ss.TaskLog(s, e.ULID, limit)
	}
	if err != nil {
		return nil, nil, err
	}
	return ix, entries, nil
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
