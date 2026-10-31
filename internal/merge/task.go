package merge

import (
	"bytes"
	"slices"
	"strings"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

// taskMerge is the merge of one task file. The result is dst, the decoded
// file of the side the result is built on, with the merged values put in.
type taskMerge struct {
	*merger
	ours, theirs Side
	b, o, t, dst *model.Task
	oursTop      bool   // dst is ours
	lostID       string // the id that lost to the other side's
	res          Result
}

func (m *merger) task(base []byte, ours, theirs Side) (Result, error) {
	if ours.Data == nil || theirs.Data == nil {
		return m.revive(ours, theirs), nil
	}
	o, err1 := model.DecodeTask(ours.Data)
	t, err2 := model.DecodeTask(theirs.Data)
	if err1 != nil || err2 != nil {
		return m.other(ours, theirs)
	}
	b, err := model.DecodeTask(base)
	if err != nil {
		// No base, or one that cannot be read: both sides made the file.
		b = &model.Task{}
	}
	oursTop := top(ours, theirs)
	from := theirs.Data
	if oursTop {
		from = ours.Data
	}
	dst, err := model.DecodeTask(from)
	if err != nil {
		return Result{}, err
	}
	tm := &taskMerge{merger: m, ours: ours, theirs: theirs, b: b, o: o, t: t, dst: dst, oursTop: oursTop}
	oEdited, tEdited := edited(b, o), edited(b, t)
	for _, f := range taskFields {
		f.merge(tm)
	}
	tm.updated()
	tm.deletedAt(oEdited, tEdited)
	data, err := dst.Encode()
	if err != nil {
		return Result{}, err
	}
	tm.res.Data = data
	return tm.res, nil
}

// taskField is a field of a task file other than ulid, updated and
// deleted_at, which have rules of their own.
type taskField struct {
	same  func(a, b *model.Task) bool
	merge func(m *taskMerge)
}

var taskFields = []taskField{
	idField(),
	formerIDsField(),
	scalarField("workspace", func(x *model.Task) *model.ULID { return &x.Workspace }),
	scalarField("kind", func(x *model.Task) *string { return &x.Kind }),
	scalarField("title", func(x *model.Task) *string { return &x.Title }),
	scalarField("parent", func(x *model.Task) **model.ULID { return &x.Parent }),
	scalarField("rank", func(x *model.Task) *model.Rank { return &x.Rank }),
	scalarField("status", func(x *model.Task) *string { return &x.Status }),
	setField("assignees", func(x *model.Task) *[]string { return &x.Assignees }, ident),
	setField("watchers", func(x *model.Task) *[]string { return &x.Watchers }, ident),
	setField("labels", func(x *model.Task) *[]string { return &x.Labels }, ident),
	setField("repos", func(x *model.Task) *[]string { return &x.Repos }, ident),
	setField("blocked_by", func(x *model.Task) *[]model.ULID { return &x.BlockedBy }, model.ULID.String),
	scalarField("sprint", func(x *model.Task) **model.ULID { return &x.Sprint }),
	setField("releases", func(x *model.Task) *[]model.ULID { return &x.Releases }, model.ULID.String),
	setField("links", func(x *model.Task) *[]model.Link { return &x.Links }, func(l model.Link) string { return l.Ref }),
	{
		same: func(a, b *model.Task) bool { return sameFields(a.Props, b.Props) },
		merge: func(m *taskMerge) {
			m.dst.Props = m.fields("props.", m.b.Props, m.o.Props, m.t.Props, m.dst.Props, m.multi)
		},
	},
	{
		same:  func(a, b *model.Task) bool { return model.NodeEqual(a.GH, b.GH) },
		merge: (*taskMerge).gh,
	},
	scalarField("created", func(x *model.Task) *time.Time { return &x.Created }),
	scalarField("created_by", func(x *model.Task) *string { return &x.CreatedBy }),
	{
		same: func(a, b *model.Task) bool { return sameFields(a.Extra, b.Extra) },
		merge: func(m *taskMerge) {
			m.dst.Extra = m.fields("", m.b.Extra, m.o.Extra, m.t.Extra, m.dst.Extra, nil)
		},
	},
	{
		same:  func(a, b *model.Task) bool { return a.Body == b.Body },
		merge: (*taskMerge).body,
	},
}

func ident(s string) string { return s }

// edited reports whether x changed a field of b other than updated and
// deleted_at.
func edited(b, x *model.Task) bool {
	return slices.ContainsFunc(taskFields, func(f taskField) bool { return !f.same(b, x) })
}

func jsonEqual(a, b any) bool { return bytes.Equal(jsonOf(a), jsonOf(b)) }

// scalarField is a field that takes one side's value whole.
func scalarField[T any](name string, get func(*model.Task) *T) taskField {
	return taskField{
		same: func(a, b *model.Task) bool { return jsonEqual(*get(a), *get(b)) },
		merge: func(m *taskMerge) {
			v := pick(m, name, *get(m.b), *get(m.o), *get(m.t), jsonEqual)
			// An equal value keeps the form it has in the file.
			if !jsonEqual(v, *get(m.dst)) {
				*get(m.dst) = v
			}
		},
	}
}

// idField is a scalar whose losing value goes into former_ids, so that the
// number is not given again. Under OursWins the caller keeps id and
// former_ids as they are.
func idField() taskField {
	f := scalarField("id", func(x *model.Task) *string { return &x.ID })
	inner := f.merge
	f.merge = func(m *taskMerge) {
		inner(m)
		b, o, t := m.b.ID, m.o.ID, m.t.ID
		if m.p == LWW && o != b && t != b && o != t {
			m.lostID = o
			if m.dst.ID == o {
				m.lostID = t
			}
		}
	}
	return f
}

// formerIDsField is a set that also takes the id that lost.
func formerIDsField() taskField {
	f := setField("former_ids", func(x *model.Task) *[]string { return &x.FormerIDs }, ident)
	inner := f.merge
	f.merge = func(m *taskMerge) {
		inner(m)
		if m.lostID != "" && !slices.Contains(m.dst.FormerIDs, m.lostID) {
			m.dst.FormerIDs = append(slices.Clone(m.dst.FormerIDs), m.lostID)
		}
	}
	return f
}

// setField is a list merged as a set. Items are told apart by key; links
// whose item both sides changed keep the later one.
func setField[T any](name string, get func(*model.Task) *[]T, key func(T) string) taskField {
	same := func(a, b []T) bool {
		return slices.EqualFunc(a, b, func(x, y T) bool { return key(x) == key(y) && jsonEqual(x, y) })
	}
	return taskField{
		same: func(a, b *model.Task) bool { return same(*get(a), *get(b)) },
		merge: func(m *taskMerge) {
			b, o, t := *get(m.b), *get(m.o), *get(m.t)
			var v []T
			switch {
			case same(b, o):
				v = t
			case same(b, t) || same(o, t):
				v = o
			default:
				v = mergeSet(b, o, t, key)
				for i, x := range v {
					// An item on both sides takes the value by the scalar rule.
					bi, oi, ti := find(b, key(x), key), find(o, key(x), key), find(t, key(x), key)
					if oi >= 0 && ti >= 0 {
						var bx T
						if bi >= 0 {
							bx = b[bi]
						}
						v[i] = pick(m, name, bx, o[oi], t[ti], jsonEqual)
					}
				}
			}
			*get(m.dst) = v
		},
	}
}

func find[T any](s []T, k string, key func(T) string) int {
	return slices.IndexFunc(s, func(x T) bool { return key(x) == k })
}

// mergeSet applies the additions and removals of both sides to base: the
// items of base that both sides kept, in base's order, then the items either
// side added, ordered by key so that the result does not depend on which
// side is ours. An item one side removed and the other changed keeps the
// change.
func mergeSet[T any](base, ours, theirs []T, key func(T) string) []T {
	var out, added []T
	for _, x := range base {
		k := key(x)
		oi, ti := find(ours, k, key), find(theirs, k, key)
		switch {
		case find(out, k, key) >= 0:
		case oi >= 0 && ti >= 0:
			out = append(out, x)
		case oi >= 0 && !jsonEqual(x, ours[oi]):
			out = append(out, ours[oi])
		case ti >= 0 && !jsonEqual(x, theirs[ti]):
			out = append(out, theirs[ti])
		}
	}
	for _, x := range slices.Concat(ours, theirs) {
		k := key(x)
		if find(base, k, key) < 0 && find(added, k, key) < 0 {
			added = append(added, x)
		}
	}
	slices.SortFunc(added, func(x, y T) int { return strings.Compare(key(x), key(y)) })
	return append(out, added...)
}

// pick merges one value. When both sides changed it to different values,
// the later side wins and the loser is recorded; under OursWins ours wins
// and nothing is recorded.
func pick[T any](m *taskMerge, name string, b, o, t T, eq func(a, b any) bool) T {
	switch {
	case eq(b, o):
		return t
	case eq(b, t) || eq(o, t):
		return o
	case m.p == OursWins:
		return o
	}
	oj, tj := jsonOf(o), jsonOf(t)
	// Values that encode alike lose nothing, though one side must still win.
	record := !bytes.Equal(oj, tj)
	if m.oursLater(oj, tj) {
		if record {
			m.lost(name, tj, m.theirs.Author)
		}
		return o
	}
	if record {
		m.lost(name, oj, m.ours.Author)
	}
	return t
}

// oursLater reports whether ours wins a value both sides changed, given
// the values as JSON. Values that encode alike are told apart by the files.
func (m *taskMerge) oursLater(oj, tj []byte) bool {
	if bytes.Equal(oj, tj) {
		oj, tj = m.ours.Data, m.theirs.Data
	}
	return later(m.o.Updated, m.t.Updated, m.ours.Author, m.theirs.Author, oj, tj)
}

func (m *taskMerge) lost(field string, value []byte, author string) {
	m.res.Overwritten = append(m.res.Overwritten, Overwritten{Path: m.path, Field: field, Value: string(value), Author: author})
}

// multi reports whether the prop key holds a list of values, merged as a set.
func (m *taskMerge) multi(key string) bool {
	if m.sc == nil {
		return false
	}
	p, ok := m.sc.Prop(key)
	return ok && (p.Type == schema.TypeMultiEnum || p.Type == schema.TypeTaskRef && p.Multi)
}

func nodeEqual(a, b any) bool { return model.NodeEqual(a.(*yaml.Node), b.(*yaml.Node)) }

// fields merges a mapping key by key, naming each key with prefix. The keys
// keep dst's order, and the keys only the other side has follow in its
// order.
func (m *taskMerge) fields(prefix string, b, o, t, dst model.Fields, multi func(string) bool) model.Fields {
	other := o
	if m.oursTop {
		other = t
	}
	var keys []string
	for _, f := range slices.Concat(dst, other) {
		if !slices.Contains(keys, f.Key) {
			keys = append(keys, f.Key)
		}
	}
	var out model.Fields
	for _, k := range keys {
		v := m.node(prefix+k, b.Get(k), o.Get(k), t.Get(k), dst.Get(k), multi != nil && multi(k))
		if v != nil {
			out = append(out, model.Field{Key: k, Value: v})
		}
	}
	return out
}

// sameFields reports whether a and b hold the same keys and values, in any
// order.
func sameFields(a, b model.Fields) bool {
	if len(a) != len(b) {
		return false
	}
	for _, f := range a {
		v := b.Get(f.Key)
		if v == nil || !model.NodeEqual(f.Value, v) {
			return false
		}
	}
	return true
}

// node merges the value of one key; nil is a key that is not there. A key
// one side removed and the other changed keeps the change. d is dst's value,
// kept when the result equals it, so that its form survives.
func (m *taskMerge) node(name string, b, o, t, d *yaml.Node, multi bool) *yaml.Node {
	var v *yaml.Node
	oc, tc := !model.NodeEqual(b, o), !model.NodeEqual(b, t)
	switch {
	case !oc:
		v = t
	case !tc || model.NodeEqual(o, t):
		v = o
	case o == nil:
		v = t
	case t == nil:
		v = o
	default:
		if bs, os, ts, ok := scalarLists(b, o, t); multi && ok {
			top, other := t, o
			if m.oursTop {
				top, other = o, t
			}
			v = listNode(d, mergeSet(bs, os, ts, ident), b, top, other)
		} else {
			v = pick(m, name, b, o, t, nodeEqual)
		}
	}
	if v != nil && model.NodeEqual(v, d) {
		return d
	}
	return v
}

// scalarLists returns the values of nodes that are each a list of scalars
// or absent.
func scalarLists(nodes ...*yaml.Node) (b, o, t []string, ok bool) {
	out := make([][]string, len(nodes))
	for i, n := range nodes {
		if n == nil {
			continue
		}
		if n.Kind != yaml.SequenceNode {
			return nil, nil, nil, false
		}
		for _, it := range n.Content {
			if it.Kind != yaml.ScalarNode {
				return nil, nil, nil, false
			}
			out[i] = append(out[i], it.Value)
		}
	}
	return out[0], out[1], out[2], true
}

// listNode builds a list of values in the style of like, reusing the item
// nodes of from that hold the same value, the first found. from must not
// depend on which side is ours.
func listNode(like *yaml.Node, values []string, from ...*yaml.Node) *yaml.Node {
	n := *like
	n.Content = nil
	for _, s := range values {
		it := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}
	search:
		for _, f := range from {
			if f == nil {
				continue
			}
			for _, c := range f.Content {
				if c.Value == s {
					it = c
					break search
				}
			}
		}
		n.Content = append(n.Content, it)
	}
	return &n
}

// gh merges the values from GitHub key by key, or whole when one side's is
// not a mapping.
func (m *taskMerge) gh() {
	b, o, t := m.b.GH, m.o.GH, m.t.GH
	if model.NodeEqual(o, t) || !isMapping(o) || !isMapping(t) || b != nil && !isMapping(b) {
		m.dst.GH = m.node("gh", b, o, t, m.dst.GH, false)
		return
	}
	f := m.fields("gh.", fieldsOf(b), fieldsOf(o), fieldsOf(t), fieldsOf(m.dst.GH), nil)
	n := *m.dst.GH
	n.Content = nil
	for _, x := range f {
		k := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: x.Key}
		for i := 0; i+1 < len(m.dst.GH.Content); i += 2 {
			if m.dst.GH.Content[i].Value == x.Key {
				k = m.dst.GH.Content[i] // keeps the key's comments
			}
		}
		n.Content = append(n.Content, k, x.Value)
	}
	m.dst.GH = &n
}

func isMapping(n *yaml.Node) bool { return n != nil && n.Kind == yaml.MappingNode }

func fieldsOf(n *yaml.Node) model.Fields {
	var f model.Fields
	if isMapping(n) {
		_ = f.UnmarshalYAML(n) // cannot fail on a mapping
	}
	return f
}

// body merges the text with diff3. Overlaps keep both sides, the older
// first; under OursWins they keep ours.
func (m *taskMerge) body() {
	b, o, t := m.b.Body, m.o.Body, m.t.Body
	switch {
	case b == o:
		m.dst.Body = t
	case b == t || o == t:
		m.dst.Body = o
	case m.p == OursWins:
		m.dst.Body = merge3(b, o, t, func(a, _ string) string { return a })
	default:
		oursFirst := later(m.t.Updated, m.o.Updated, m.theirs.Author, m.ours.Author, m.theirs.Data, m.ours.Data)
		m.dst.Body = Diff3(b, o, t, oursFirst)
	}
}

// updated is the later of the two when both sides changed it.
func (m *taskMerge) updated() {
	b, o, t := m.b.Updated, m.o.Updated, m.t.Updated
	v := t
	if b.Equal(t) || !b.Equal(o) && o.After(t) {
		v = o
	}
	// The same time keeps the form it has in the file.
	if !v.Equal(m.dst.Updated) {
		m.dst.Updated = v
	}
}

// deletedAt merges deleted_at. A side that deleted a live task while the
// other edited it loses the deletion, and is told; otherwise deleted_at is
// a scalar.
func (m *taskMerge) deletedAt(oEdited, tEdited bool) {
	b, o, t := m.b.DeletedAt, m.o.DeletedAt, m.t.DeletedAt
	oc, tc := !jsonEqual(b, o), !jsonEqual(b, t)
	var v *time.Time
	switch {
	case b == nil && oc && !tc && o != nil && tEdited:
		m.res.Notices = append(m.res.Notices, Notice{Kind: Revived, Path: m.path, Author: m.ours.Author})
	case b == nil && tc && !oc && t != nil && oEdited:
		m.res.Notices = append(m.res.Notices, Notice{Kind: Revived, Path: m.path, Author: m.theirs.Author})
	default:
		v = pick(m, "deleted_at", b, o, t, jsonEqual)
	}
	if !jsonEqual(v, m.dst.DeletedAt) {
		m.dst.DeletedAt = v
	}
}
