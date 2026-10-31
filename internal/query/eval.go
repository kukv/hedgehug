package query

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// Options holds what a query needs from outside the store.
type Options struct {
	Me []string // the accounts assignee:@me stands for
}

// Result is what a query found.
type Result struct{ Groups []Group }

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
	deleted bool      // the query names is:deleted
	order   *sortSpec // sort:, if any
	group   *field    // group:, if any
}

// Run finds the tasks of ix that match every term of q, sorted and grouped
// as its sort: and group: say. Deleted tasks are left out unless q names
// is:deleted.
func Run(ix *index.Index, q *Query, opt Options) (*Result, error) {
	c := &compiler{ix: ix, sc: ix.Schema(), opt: opt}
	var preds []pred
	for _, t := range q.Terms {
		if t.Key == "sort" || t.Key == "group" {
			if err := c.orderBy(t); err != nil {
				return nil, err
			}
			continue
		}
		p, err := c.term(t)
		if err != nil {
			return nil, err
		}
		if t.Neg {
			p = not(p)
		}
		preds = append(preds, p)
	}
	hits := []*index.Entry{}
	for _, e := range ix.Tasks() {
		if (e.DeletedAt == nil || c.deleted) && matchAll(preds, e) {
			hits = append(hits, e)
		}
	}
	sortEntries(hits, c.order)
	if c.group == nil {
		return &Result{Groups: []Group{{Tasks: hits}}}, nil
	}
	return &Result{Groups: groupEntries(hits, *c.group)}, nil
}

func (c *compiler) fail(t Term, format string, args ...any) error {
	return &Error{Pos: t.Pos, Term: t.Raw, Msg: fmt.Sprintf(format, args...)}
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
		if !ok || parsers[p.Type] == nil {
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
	return nil, c.fail(t, "unknown key %q", t.Key)
}

// matchText matches when a value of the task equals a value of the term,
// ignoring case; none matches a task with no value. check, when set, refuses
// values the term cannot take.
func (c *compiler) matchText(t Term, get func(*index.Entry) []string, check func(string) error) (pred, error) {
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
				if strings.EqualFold(g, w) {
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

// assignee matches assignees; @me stands for the accounts in Options.Me.
func (c *compiler) assignee(t Term) (pred, error) {
	var vs []string
	for _, v := range t.Values {
		if v != "@me" {
			vs = append(vs, v)
			continue
		}
		if len(c.opt.Me) == 0 {
			return nil, c.fail(t, "@me needs an account to be set up")
		}
		vs = append(vs, c.opt.Me...)
	}
	t.Values = vs
	return c.matchText(t, func(e *index.Entry) []string { return e.Assignees }, nil)
}

// resolve turns the values of t into tasks; none is reported apart.
func (c *compiler) resolve(t Term) (none bool, ulids map[model.ULID]bool, err error) {
	ulids = map[model.ULID]bool{}
	for _, v := range t.Values {
		if v == "none" {
			none = true
			continue
		}
		ulid, err := c.ix.Resolve(v)
		if err != nil {
			return false, nil, c.failWith(t, err)
		}
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
	return model.CompareRankKeys(model.RankKey{ID: a.ID, ULID: a.ULID, Rank: a.Rank}, model.RankKey{ID: b.ID, ULID: b.ULID, Rank: b.Rank})
}
