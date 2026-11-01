package query

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

// Options holds what a query needs from outside the store.
type Options struct {
	Me []string // the accounts @me stands for
	// Overwritten holds the tasks is:overwritten matches: those with an
	// unread overwritten item in the inbox.
	Overwritten map[model.ULID]bool
	// MissingIDs makes a value naming a task the store does not have (in
	// project:, parent:, ...) match no task, listed in Result.IDs, where it
	// is otherwise the error of the query. mount.Query sets it: another store
	// may have the task.
	MissingIDs bool
}

// Result is what a query found.
type Result struct {
	Groups []Group
	// SortKey gives a task's key when the query has sort:; nil without it.
	SortKey func(*index.Entry) SortKey
	// Desc is whether the sort: is descending.
	Desc bool
	// IDs are the values of the terms that name a task (project:, parent:,
	// blocked_by:, a task_ref property, ...), with whether this store has the
	// task. A value it does not have matches no task here; mount.Query makes
	// it an error when no store has it.
	IDs []IDLookup
}

// IDLookup is one value that names a task. Err is nil when the store has the
// task, and says it does not otherwise (store.ErrNotFound).
type IDLookup struct {
	Pos   int // the term
	Term  string
	Value string
	Err   error
}

// Group is the tasks with one value of the group: field, or all the tasks
// found when the query has no group:.
type Group struct {
	Key   string // the value; empty for tasks with no value, and with no group:
	Tasks []*index.Entry
}

type pred func(*index.Entry) bool

// compiler turns terms into predicates, checking keys and values against
// the schema.
type compiler struct {
	ix      *index.Index
	sc      *schema.Schema
	opt     Options
	ids     []IDLookup // the values naming a task
	deleted bool       // the query names is:deleted
	order   *sortSpec  // sort:, if any
	group   *field     // group:, if any
}

// Run finds the tasks of ix that match every term of q, sorted and grouped
// as its sort: and group: say. Deleted tasks are left out unless q names
// is:deleted.
func Run(ix *index.Index, q *Query, opt Options) (*Result, error) {
	c := &compiler{ix: ix, sc: ix.Schema(), opt: opt}
	var preds []pred
	// A term of a property the schema lacks fails the query, but the other
	// terms are still read, so that Result.IDs tells what the store has.
	var unknown error
	for _, t := range q.Terms {
		if t.Key == "sort" || t.Key == "group" {
			if err := c.orderBy(t); err != nil {
				if !isUnknown(err) {
					return nil, err
				}
				unknown = cmp.Or(unknown, err)
			}
			continue
		}
		p, err := c.term(t)
		if err != nil {
			if !isUnknown(err) {
				return nil, err
			}
			unknown = cmp.Or(unknown, err)
			continue
		}
		if t.Neg {
			p = not(p)
		}
		preds = append(preds, p)
	}
	if unknown != nil {
		return &Result{IDs: c.ids}, unknown
	}
	hits := []*index.Entry{}
	for _, e := range ix.Tasks() {
		if (e.DeletedAt == nil || c.deleted) && matchAll(preds, e) {
			hits = append(hits, e)
		}
	}
	sortEntries(hits, c.order)
	res := &Result{Groups: []Group{{Tasks: hits}}}
	if c.group != nil {
		res.Groups = groupEntries(hits, *c.group)
	}
	if c.order != nil {
		res.SortKey, res.Desc = c.order.f.sortKey, c.order.desc
	}
	res.IDs = c.ids
	return res, nil
}

func isUnknown(err error) bool {
	var qe *Error
	return errors.As(err, &qe) && qe.Unknown
}

func (c *compiler) fail(t Term, format string, args ...any) error {
	return &Error{Pos: t.Pos, Term: t.Raw, Msg: fmt.Sprintf(format, args...)}
}

// failUnknown is fail for a property the schema does not have.
func (c *compiler) failUnknown(t Term, format string, args ...any) error {
	return &Error{Pos: t.Pos, Term: t.Raw, Msg: fmt.Sprintf(format, args...), Unknown: true}
}

// failWith is fail for an underlying error, which callers can unwrap.
func (c *compiler) failWith(t Term, err error) error {
	return &Error{Pos: t.Pos, Term: t.Raw, Msg: err.Error(), Err: err}
}

func (c *compiler) term(t Term) (pred, error) {
	if t.Key == "" {
		text := strings.ToLower(t.Values[0])
		return func(e *index.Entry) bool { return strings.Contains(e.Text, text) }, nil
	}
	if t.Op != ":" {
		p, ok := c.sc.Prop(t.Key)
		if !ok {
			return nil, c.failUnknown(t, "%s cannot be compared with %s", t.Key, t.Op)
		}
		if parsers[p.Type] == nil {
			return nil, c.fail(t, "%s cannot be compared with %s", t.Key, t.Op)
		}
		return c.compare(t, p)
	}
	switch t.Key {
	case "status":
		return c.status(t)
	case "kind":
		return c.matchText(t, func(e *index.Entry) []string { return single(e.Kind) }, func(v string) error {
			if _, ok := c.sc.Kind(v); !ok {
				return fmt.Errorf("unknown kind %q", v)
			}
			return nil
		})
	case "label":
		return c.matchText(t, func(e *index.Entry) []string { return e.Labels }, nil)
	case "assignee":
		return c.assignee(t)
	case "watching":
		return c.watching(t)
	case "mentions":
		return c.mentions(t)
	case "parent":
		return c.ref(t, func(e *index.Entry) []model.ULID {
			if e.Parent == nil {
				return nil
			}
			return []model.ULID{*e.Parent}
		})
	case "project":
		return c.project(t)
	case "workspace":
		return c.workspace(t)
	case "blocked_by":
		return c.ref(t, func(e *index.Entry) []model.ULID { return e.BlockedBy })
	case "blocks":
		return c.inverse(t, "blocked_by")
	case "is":
		return c.is(t)
	}
	if p, ok := c.sc.Prop(t.Key); ok {
		return c.prop(t, p)
	}
	for _, p := range c.sc.Props {
		if p.Type == schema.TypeTaskRef && p.Inverse == t.Key {
			return c.inverse(t, p.Name)
		}
	}
	return nil, c.failUnknown(t, "unknown key %q", t.Key)
}

// matchText matches when a value of the task equals a value of the term,
// ignoring case; none matches a task with no value. check, when set, refuses
// values the term cannot take.
func (c *compiler) matchText(t Term, get func(*index.Entry) []string, check func(string) error) (pred, error) {
	return c.matchBy(t, get, check, strings.EqualFold)
}

// matchBy is matchText with match telling whether a value of the task
// matches a value of the term.
func (c *compiler) matchBy(t Term, get func(*index.Entry) []string, check func(string) error, match func(got, want string) bool) (pred, error) {
	none := false
	var want []string
	for _, v := range t.Values {
		if v == "none" {
			none = true
			continue
		}
		if check != nil {
			if err := check(v); err != nil {
				return nil, c.failWith(t, err)
			}
		}
		want = append(want, v)
	}
	return func(e *index.Entry) bool {
		got := get(e)
		if none && len(got) == 0 {
			return true
		}
		for _, g := range got {
			for _, w := range want {
				if match(g, w) {
					return true
				}
			}
		}
		return false
	}, nil
}

// status matches a status or a category, which stands for every status in
// it. A name that is both is taken for the category.
func (c *compiler) status(t Term) (pred, error) {
	var names []string
	for _, v := range t.Values {
		switch {
		case v == "none":
			names = append(names, v)
		case slices.Contains(schema.Categories, schema.Category(v)):
			for _, st := range c.sc.Statuses {
				if st.Category == schema.Category(v) {
					names = append(names, st.Name)
				}
			}
		default:
			if _, ok := c.sc.Status(v); !ok {
				return nil, c.fail(t, "unknown status or category %q", v)
			}
			names = append(names, v)
		}
	}
	t.Values = names
	return c.matchText(t, func(e *index.Entry) []string { return single(e.Status) }, nil)
}

// assignee matches assignees.
func (c *compiler) assignee(t Term) (pred, error) {
	t, err := c.me(t)
	if err != nil {
		return nil, err
	}
	return c.matchText(t, func(e *index.Entry) []string { return e.Assignees }, nil)
}

// watching matches watchers.
func (c *compiler) watching(t Term) (pred, error) {
	t, err := c.me(t)
	if err != nil {
		return nil, err
	}
	return c.matchText(t, func(e *index.Entry) []string { return e.Watchers }, nil)
}

// mentions matches the mentions of the body and of the comments that are
// not deleted, as the inbox does: @login names any account with that login.
// A value with no host matches that login on any host.
func (c *compiler) mentions(t Term) (pred, error) {
	t, err := c.me(t)
	if err != nil {
		return nil, err
	}
	return c.matchBy(t, func(e *index.Entry) []string { return e.Mentions }, nil, index.MentionNames)
}

// me replaces @me among the values of t with the accounts in Options.Me.
// The accounts are then compared with strings.EqualFold, like
// config.SameAccount (spec: 共通の決まり, 大文字小文字).
func (c *compiler) me(t Term) (Term, error) {
	var vs []string
	for _, v := range t.Values {
		if v != "@me" {
			vs = append(vs, v)
			continue
		}
		if len(c.opt.Me) == 0 {
			return t, c.fail(t, "@me needs an account to be set up")
		}
		vs = append(vs, c.opt.Me...)
	}
	t.Values = vs
	return t, nil
}

// resolve turns the values of t into tasks; none is reported apart.
func (c *compiler) resolve(t Term) (none bool, ulids map[model.ULID]bool, err error) {
	ulids = map[model.ULID]bool{}
	for _, v := range t.Values {
		if v == "none" {
			none = true
			continue
		}
		// "<store>:<id>" is looked up in that store only; ids have no ":".
		id := v
		var lerr error
		if name, rest, ok := strings.Cut(v, ":"); ok {
			if name != c.ix.StoreName() {
				lerr = fmt.Errorf("%q: %w", v, store.ErrNotFound)
			}
			id = rest
		}
		var ulid model.ULID
		if lerr == nil {
			ulid, lerr = c.ix.Resolve(id)
		}
		if errors.Is(lerr, store.ErrNotFound) && c.opt.MissingIDs {
			// no task here: the value matches none
			c.ids = append(c.ids, IDLookup{Pos: t.Pos, Term: t.Raw, Value: v, Err: lerr})
			continue
		}
		if lerr != nil {
			return false, nil, c.failWith(t, lerr)
		}
		c.ids = append(c.ids, IDLookup{Pos: t.Pos, Term: t.Raw, Value: v})
		ulids[ulid] = true
	}
	return none, ulids, nil
}

// ref matches when the task points at one of the term's tasks through get;
// none matches a task that points at none.
func (c *compiler) ref(t Term, get func(*index.Entry) []model.ULID) (pred, error) {
	none, ulids, err := c.resolve(t)
	if err != nil {
		return nil, err
	}
	return func(e *index.Entry) bool {
		got := get(e)
		if none && len(got) == 0 {
			return true
		}
		for _, g := range got {
			if ulids[g] {
				return true
			}
		}
		return false
	}, nil
}

// project matches tasks that have one of the term's tasks among their
// ancestors.
func (c *compiler) project(t Term) (pred, error) {
	none, ulids, err := c.resolve(t)
	if err != nil {
		return nil, err
	}
	if none {
		return nil, c.fail(t, "project does not take none")
	}
	return func(e *index.Entry) bool {
		seen := map[model.ULID]bool{e.ULID: true}
		for p := e.Parent; p != nil && !seen[*p]; {
			if ulids[*p] {
				return true
			}
			seen[*p] = true
			a, ok := c.ix.Task(*p)
			if !ok {
				break
			}
			p = a.Parent
		}
		return false
	}, nil
}

// workspace matches tasks that sit in one of the term's workspaces.
func (c *compiler) workspace(t Term) (pred, error) {
	ulids := map[model.ULID]bool{}
	for _, v := range t.Values {
		w, err := c.ix.ResolveWorkspace(v)
		if err != nil {
			return nil, c.failWith(t, err)
		}
		ulids[w.ULID] = true
	}
	return func(e *index.Entry) bool { return ulids[e.Workspace] }, nil
}

// inverse matches tasks that one of the term's tasks points at through rel,
// as blocks: does for blocked_by; none matches a task no task points at.
func (c *compiler) inverse(t Term, rel string) (pred, error) {
	none, ulids, err := c.resolve(t)
	if err != nil {
		return nil, err
	}
	pointed := c.targeted(rel)
	return func(e *index.Entry) bool {
		if none && !pointed[e.ULID] {
			return true
		}
		for ulid := range ulids {
			if x, ok := c.ix.Task(ulid); ok && slices.Contains(x.Targets(rel), e.ULID) {
				return true
			}
		}
		return false
	}, nil
}

// targeted returns the tasks some task points at through rel.
func (c *compiler) targeted(rel string) map[model.ULID]bool {
	out := map[model.ULID]bool{}
	for _, e := range c.ix.Tasks() {
		for _, ulid := range e.Targets(rel) {
			out[ulid] = true
		}
	}
	return out
}

// taskRef matches tasks whose property p points at one of the term's
// tasks. For a symmetric property the tasks those point back at match too,
// and none matches a task with no such relation either way.
func (c *compiler) taskRef(t Term, p schema.Prop) (pred, error) {
	none, ulids, err := c.resolve(t)
	if err != nil {
		return nil, err
	}
	var back map[model.ULID]bool
	if p.Symmetric {
		back = c.targeted(p.Name)
	}
	return func(e *index.Entry) bool {
		got := e.Targets(p.Name)
		if none && len(got) == 0 && !back[e.ULID] {
			return true
		}
		for _, g := range got {
			if ulids[g] {
				return true
			}
		}
		if p.Symmetric {
			for ulid := range ulids {
				if x, ok := c.ix.Task(ulid); ok && slices.Contains(x.Targets(p.Name), e.ULID) {
					return true
				}
			}
		}
		return false
	}, nil
}

// prop matches a property by the type the schema gives it.
func (c *compiler) prop(t Term, p schema.Prop) (pred, error) {
	get := func(e *index.Entry) []string { return e.Prop(p.Name) }
	switch p.Type {
	case schema.TypeTaskRef:
		return c.taskRef(t, p)
	case schema.TypeEnum, schema.TypeMultiEnum:
		return c.matchText(t, get, func(v string) error {
			if !slices.ContainsFunc(p.Values, func(x string) bool { return strings.EqualFold(x, v) }) {
				return fmt.Errorf("%q is not one of %v", v, p.Values)
			}
			return nil
		})
	case schema.TypeBool:
		return c.matchText(t, get, func(v string) error {
			if v != "true" && v != "false" {
				return fmt.Errorf("%q is not true or false", v)
			}
			return nil
		})
	}
	if parsers[p.Type] != nil {
		return c.compare(t, p)
	}
	return c.matchText(t, get, nil)
}

// parsers turn values of the types that can be compared into numbers:
// dates and times into Unix seconds, durations into minutes.
var parsers = map[schema.PropType]func(string) (float64, error){
	schema.TypeNumber: func(s string) (float64, error) {
		x, err := strconv.ParseFloat(s, 64)
		if err == nil && (math.IsNaN(x) || math.IsInf(x, 0)) {
			return 0, fmt.Errorf("%q is not a finite number", s)
		}
		return x, err
	},
	schema.TypeDate: func(s string) (float64, error) {
		d, err := model.ParseDate(s)
		return float64(d.Time().Unix()), err
	},
	schema.TypeDatetime: func(s string) (float64, error) {
		tm, err := time.Parse(time.RFC3339, s)
		return float64(tm.Unix()), err
	},
	schema.TypeDuration: func(s string) (float64, error) {
		d, err := model.ParseDuration(s)
		return time.Duration(d).Minutes(), err
	},
}

// compare matches a property of a type in parsers with the term's operator;
// ":" is equality. Values in the file that do not parse never match.
func (c *compiler) compare(t Term, p schema.Prop) (pred, error) {
	parse := parsers[p.Type]
	none := false
	var want []float64
	for _, v := range t.Values {
		if v == "none" {
			if t.Op != ":" {
				return nil, c.fail(t, "none cannot be compared")
			}
			none = true
			continue
		}
		x, err := parse(v)
		if err != nil {
			return nil, c.failWith(t, err)
		}
		want = append(want, x)
	}
	op := t.Op
	return func(e *index.Entry) bool {
		got := e.Prop(p.Name)
		if none && len(got) == 0 {
			return true
		}
		for _, g := range got {
			x, err := parse(g)
			if err != nil {
				continue
			}
			for _, w := range want {
				if holds(op, x, w) {
					return true
				}
			}
		}
		return false
	}, nil
}

func holds(op string, a, b float64) bool {
	switch op {
	case "<":
		return a < b
	case "<=":
		return a <= b
	case ">":
		return a > b
	case ">=":
		return a >= b
	}
	return a == b
}

// is matches the built-in conditions.
func (c *compiler) is(t Term) (pred, error) {
	var ps []pred
	for _, v := range t.Values {
		switch v {
		case "blocked":
			ps = append(ps, c.blocked)
		case "invalid":
			ps = append(ps, func(e *index.Entry) bool { return len(c.ix.Problems(e.ULID)) > 0 })
		case "deleted":
			c.deleted = true
			ps = append(ps, func(e *index.Entry) bool { return e.DeletedAt != nil })
		case "overwritten":
			ps = append(ps, func(e *index.Entry) bool { return c.opt.Overwritten[e.ULID] })
		default:
			return nil, c.fail(t, "unknown condition is:%s", v)
		}
	}
	return anyOf(ps), nil
}

// blocked reports whether a blocker of e is open: in the index, readable,
// not deleted, and not in the done or canceled category.
func (c *compiler) blocked(e *index.Entry) bool {
	for _, ulid := range e.BlockedBy {
		b, ok := c.ix.Task(ulid)
		if !ok || b.Broken != "" || b.DeletedAt != nil {
			continue
		}
		st, _ := c.sc.Status(b.Status)
		if st.Category != schema.CategoryDone && st.Category != schema.CategoryCanceled {
			return true
		}
	}
	return false
}

func single(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}

func not(p pred) pred { return func(e *index.Entry) bool { return !p(e) } }

func anyOf(ps []pred) pred {
	return func(e *index.Entry) bool {
		for _, p := range ps {
			if p(e) {
				return true
			}
		}
		return false
	}
}

func matchAll(ps []pred, e *index.Entry) bool {
	for _, p := range ps {
		if !p(e) {
			return false
		}
	}
	return true
}

func byRank(a, b *index.Entry) int {
	return model.CompareRankKeys(a.RankKey(), b.RankKey())
}
