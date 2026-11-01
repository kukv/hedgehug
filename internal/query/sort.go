package query

import (
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/schema"
)

// field is something a query can sort or group by.
type field struct {
	values func(*index.Entry) []string // empty: no value
	multi  bool                        // group: puts a task in one group per value
	less   func(a, b string) bool      // the order of two values

	// how mount.Query compares tasks of different stores (SortKey)
	by    string                        // "place", "number" or "text"
	order []string                      // by place: the values in the schema's order
	parse func(string) (float64, error) // by number: how a value is read
	class func(*index.Entry) int        // status: the category's place; kind: the role's place; nil: 0
}

// sortSpec is the sort: of a query.
type sortSpec struct {
	f    field
	desc bool
}

// orderBy reads a sort: or group: term.
func (c *compiler) orderBy(t Term) error {
	switch {
	case t.Neg:
		return c.fail(t, "%s cannot be negated", t.Key)
	case t.Op != ":":
		return c.fail(t, "%s cannot be compared with %s", t.Key, t.Op)
	case len(t.Values) != 1:
		return c.fail(t, "%s takes one field", t.Key)
	}
	name := t.Values[0]
	if t.Key == "group" {
		if c.group != nil {
			return c.fail(t, "a query takes one group:")
		}
		f, err := c.fieldOf(t, name, true)
		if err != nil {
			return err
		}
		c.group = &f
		return nil
	}
	if c.order != nil {
		return c.fail(t, "a query takes one sort:")
	}
	desc := strings.HasPrefix(name, "-")
	f, err := c.fieldOf(t, strings.TrimPrefix(name, "-"), false)
	if err != nil {
		return err
	}
	c.order = &sortSpec{f: f, desc: desc}
	return nil
}

// fieldOf returns the field name. label and assignee can be grouped by
// only.
func (c *compiler) fieldOf(t Term, name string, group bool) (field, error) {
	text := func(get func(*index.Entry) string) func(*index.Entry) []string {
		return func(e *index.Entry) []string { return single(get(e)) }
	}
	when := func(get func(*index.Entry) time.Time) func(*index.Entry) []string {
		return func(e *index.Entry) []string {
			if tm := get(e); !tm.IsZero() {
				return []string{tm.Format(time.RFC3339)}
			}
			return nil
		}
	}
	datetime := parsers[schema.TypeDatetime]
	switch name {
	case "status":
		var names []string
		for _, st := range c.sc.Statuses {
			names = append(names, st.Name)
		}
		f := placed(field{values: text(func(e *index.Entry) string { return e.Status })}, names)
		f.class = func(e *index.Entry) int {
			st, _ := c.sc.Status(e.Status)
			if i := slices.Index(schema.Categories, st.Category); i >= 0 {
				return i
			}
			return len(schema.Categories)
		}
		return f, nil
	case "kind":
		var names []string
		for _, k := range c.sc.Kinds {
			names = append(names, k.Name)
		}
		f := placed(field{values: text(func(e *index.Entry) string { return e.Kind })}, names)
		f.class = func(e *index.Entry) int {
			k, _ := c.sc.Kind(e.Kind)
			return roleClass(k.Role)
		}
		return f, nil
	case "title":
		return texted(field{values: text(func(e *index.Entry) string { return e.Title })}), nil
	case "created":
		return numbered(field{values: when(func(e *index.Entry) time.Time { return e.Created })}, datetime), nil
	case "updated":
		return numbered(field{values: when(func(e *index.Entry) time.Time { return e.Updated })}, datetime), nil
	case "label", "assignee":
		if !group {
			return field{}, c.fail(t, "%s can be grouped by, not sorted by", name)
		}
		get := func(e *index.Entry) []string { return e.Labels }
		if name == "assignee" {
			get = func(e *index.Entry) []string { return e.Assignees }
		}
		return texted(field{values: get, multi: true}), nil
	}
	p, ok := c.sc.Prop(name)
	if !ok {
		return field{}, c.failUnknown(t, "cannot sort or group by %q", name)
	}
	f := field{
		values: func(e *index.Entry) []string { return e.Prop(p.Name) },
		multi:  p.Multi || p.Type == schema.TypeMultiEnum,
	}
	switch {
	case p.Type == schema.TypeEnum || p.Type == schema.TypeMultiEnum:
		return placed(f, p.Values), nil
	case parsers[p.Type] != nil:
		return numbered(f, parsers[p.Type]), nil
	}
	return texted(f), nil
}

// roleClass is the place of a role: container, then work, then anything else.
func roleClass(r schema.Role) int {
	switch r {
	case schema.RoleContainer:
		return 0
	case schema.RoleWork:
		return 1
	}
	return 2
}

// byPlace orders values by their place in order; values not in it come
// after, by string.
func byPlace(order []string) func(a, b string) bool {
	place := func(s string) int {
		if i := slices.Index(order, s); i >= 0 {
			return i
		}
		return len(order)
	}
	return func(a, b string) bool {
		if pa, pb := place(a), place(b); pa != pb {
			return pa < pb
		}
		return a < b
	}
}

// byNumber orders values by what parse makes of them; values that do not
// parse come after, by string.
func byNumber(parse func(string) (float64, error)) func(a, b string) bool {
	return func(a, b string) bool {
		x, errA := parse(a)
		y, errB := parse(b)
		switch {
		case errA == nil && errB == nil && x != y:
			return x < y
		case errA == nil && errB != nil:
			return true
		case errA != nil && errB == nil:
			return false
		}
		return a < b
	}
}

func byText(a, b string) bool { return strings.ToLower(a) < strings.ToLower(b) }

// compare orders two tasks by the sort field. No value comes last, in either
// direction.
func (o *sortSpec) compare(a, b *index.Entry) int {
	va, vb := first(o.f.values(a)), first(o.f.values(b))
	switch {
	case va == "" && vb == "":
		return 0
	case va == "":
		return 1
	case vb == "":
		return -1
	}
	n := 0
	switch {
	case o.f.less(va, vb):
		n = -1
	case o.f.less(vb, va):
		n = 1
	}
	if o.desc {
		n = -n
	}
	return n
}

func first(vs []string) string {
	if len(vs) == 0 {
		return ""
	}
	return vs[0]
}

// sortEntries sorts es by o, then by rank and ULID; by rank and ULID alone when
// o is nil.
func sortEntries(es []*index.Entry, o *sortSpec) {
	slices.SortFunc(es, func(a, b *index.Entry) int {
		if o != nil {
			if n := o.compare(a, b); n != 0 {
				return n
			}
		}
		return byRank(a, b)
	})
}

// groupEntries splits es, already sorted, by the values of f. A task with
// several values is in the group of each; the group of tasks with no value
// comes last.
func groupEntries(es []*index.Entry, f field) []Group {
	buckets := map[string][]*index.Entry{}
	for _, e := range es {
		vs := f.values(e)
		if !f.multi && len(vs) > 1 {
			vs = vs[:1]
		}
		if len(vs) == 0 {
			vs = []string{""}
		}
		seen := map[string]bool{}
		for _, v := range vs {
			if !seen[v] {
				seen[v] = true
				buckets[v] = append(buckets[v], e)
			}
		}
	}
	keys := slices.Collect(maps.Keys(buckets))
	slices.SortFunc(keys, func(a, b string) int {
		switch {
		case a == b:
			return 0
		case a == "":
			return 1
		case b == "":
			return -1
		case f.less(a, b):
			return -1
		case f.less(b, a):
			return 1
		}
		return strings.Compare(a, b)
	})
	out := make([]Group, len(keys))
	for i, k := range keys {
		out[i] = Group{Key: k, Tasks: buckets[k]}
	}
	return out
}
