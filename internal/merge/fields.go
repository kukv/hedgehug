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

// sides is what the merge of one file field by field needs besides the
// decoded values.
type sides struct {
	*merger
	ours, theirs Side
	oTime, tTime time.Time // when each side changed the file: its updated
	oursTop      bool      // the result is built on ours
	res          Result
}

// fileMerge is the merge of one file decoded as an X. The result is dst, the
// decoded file of the side the result is built on, with the merged values
// put in.
type fileMerge[X any] struct {
	*sides
	b, o, t, dst *X
}

// field is one field of a file of type X.
type field[X any] struct {
	same  func(a, b *X) bool
	merge func(m *fileMerge[X])
	// names lists, as TaskChanges names them, what differs between a and b
	// when same says they differ; nil for a kind that does not report it.
	names func(a, b *X) []string
}

// one names a field that is a single value.
func one[X any](name string) func(a, b *X) []string {
	return func(_, _ *X) []string { return []string{name} }
}

// fileKind is how a kind of file read field by field is merged.
type fileKind[X any] struct {
	decode  func([]byte) (*X, error)
	updated func(*X) *time.Time
	fields  []field[X]
	encode  func(*X) ([]byte, error)
	after   func(m *fileMerge[X]) // when set, runs after the fields and updated
}

// load decodes the versions of a file. It reports false when a side cannot
// be read. A base that is not there or cannot be read is empty: both sides
// made the file.
func load[X any](m *merger, base []byte, ours, theirs Side, k fileKind[X]) (*fileMerge[X], bool) {
	o, err1 := k.decode(ours.Data)
	t, err2 := k.decode(theirs.Data)
	if err1 != nil || err2 != nil {
		return nil, false
	}
	b, err := k.decode(base)
	if err != nil {
		b = new(X)
	}
	oursTop := top(ours, theirs)
	from := theirs.Data
	if oursTop {
		from = ours.Data
	}
	dst, err := k.decode(from)
	if err != nil {
		return nil, false
	}
	s := &sides{merger: m, ours: ours, theirs: theirs, oTime: *k.updated(o), tTime: *k.updated(t), oursTop: oursTop}
	return &fileMerge[X]{sides: s, b: b, o: o, t: t, dst: dst}, true
}

// mergeFile merges a file field by field, then its updated. A side that
// removed the file while the other changed it loses the removal, and a
// side that cannot be read makes it an other file.
func mergeFile[X any](m *merger, base []byte, ours, theirs Side, k fileKind[X]) (Result, error) {
	if ours.Data == nil || theirs.Data == nil {
		return m.revive(ours, theirs), nil
	}
	fm, ok := load(m, base, ours, theirs, k)
	if !ok {
		return m.other(ours, theirs)
	}
	for _, f := range k.fields {
		f.merge(fm)
	}
	fm.updated(k.updated)
	if k.after != nil {
		k.after(fm)
	}
	data, err := k.encode(fm.dst)
	if err != nil {
		return Result{}, err
	}
	fm.res.Data = data
	return fm.res, nil
}

// updated is the later of the two when both sides changed it.
func (m *fileMerge[X]) updated(get func(*X) *time.Time) {
	b, o, t := *get(m.b), *get(m.o), *get(m.t)
	v := t
	if b.Equal(t) || !b.Equal(o) && o.After(t) {
		v = o
	}
	// The same time keeps the form it has in the file.
	if !v.Equal(*get(m.dst)) {
		*get(m.dst) = v
	}
}

func ident(s string) string { return s }

func jsonEqual(a, b any) bool { return bytes.Equal(jsonOf(a), jsonOf(b)) }

// scalarField is a field that takes one side's value whole.
func scalarField[X, T any](name string, get func(*X) *T) field[X] {
	return field[X]{
		same:  func(a, b *X) bool { return jsonEqual(*get(a), *get(b)) },
		names: one[X](name),
		merge: func(m *fileMerge[X]) {
			v := pick(m.sides, name, *get(m.b), *get(m.o), *get(m.t), jsonEqual)
			// An equal value keeps the form it has in the file.
			if !jsonEqual(v, *get(m.dst)) {
				*get(m.dst) = v
			}
		},
	}
}

// setField is a list merged as a set. Items are told apart by key; links
// whose item both sides changed keep the later one.
func setField[X, T any](name string, get func(*X) *[]T, key func(T) string) field[X] {
	same := func(a, b []T) bool {
		return slices.EqualFunc(a, b, func(x, y T) bool { return key(x) == key(y) && jsonEqual(x, y) })
	}
	return field[X]{
		same:  func(a, b *X) bool { return same(*get(a), *get(b)) },
		names: one[X](name),
		merge: func(m *fileMerge[X]) {
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
						v[i] = pick(m.sides, name, bx, o[oi], t[ti], jsonEqual)
					}
				}
			}
			*get(m.dst) = v
		},
	}
}

// mapField is a mapping merged key by key, naming each key with prefix.
// With multi, the props that hold a list of values are merged as sets.
func mapField[X any](prefix string, get func(*X) *model.Fields, multi bool) field[X] {
	return field[X]{
		same: func(a, b *X) bool { return sameFields(*get(a), *get(b)) },
		names: func(a, b *X) []string {
			var out []string
			for _, f := range slices.Concat(*get(a), *get(b)) {
				if !model.NodeEqual(get(a).Get(f.Key), get(b).Get(f.Key)) && !slices.Contains(out, prefix+f.Key) {
					out = append(out, prefix+f.Key)
				}
			}
			return out
		},
		merge: func(m *fileMerge[X]) {
			*get(m.dst) = m.fields(prefix, *get(m.b), *get(m.o), *get(m.t), *get(m.dst), multi)
		},
	}
}

// bodyField is the Markdown body, merged with diff3.
func bodyField[X any](get func(*X) *string) field[X] {
	return field[X]{
		same:  func(a, b *X) bool { return *get(a) == *get(b) },
		names: one[X]("body"),
		merge: func(m *fileMerge[X]) { *get(m.dst) = m.text(*get(m.b), *get(m.o), *get(m.t)) },
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

// pick merges one value of a file whose sides are timed by updated.
func pick[T any](m *sides, name string, b, o, t T, eq func(a, b any) bool) T {
	return pickAt(m, name, m.oTime, m.tTime, b, o, t, eq)
}

// pickAt merges one value that ours changed at ot and theirs at tt. When
// both sides changed it to different values, the later side wins and the
// loser is recorded; under OursWins ours wins and nothing is recorded.
func pickAt[T any](m *sides, name string, ot, tt time.Time, b, o, t T, eq func(a, b any) bool) T {
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
	if m.oursLater(ot, tt, oj, tj) {
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
// the times and the values as JSON. Values that encode alike are told apart
// by the files.
func (m *sides) oursLater(ot, tt time.Time, oj, tj []byte) bool {
	if bytes.Equal(oj, tj) {
		oj, tj = m.ours.Data, m.theirs.Data
	}
	return later(ot, tt, m.ours.Author, m.theirs.Author, oj, tj)
}

func (m *sides) lost(field string, value []byte, author string) {
	m.res.Overwritten = append(m.res.Overwritten, Overwritten{Path: m.path, Field: field, Value: string(value), Author: author})
}

// multi reports whether the prop key holds a list of values, merged as a set.
func (m *sides) multi(key string) bool {
	if m.sc == nil {
		return false
	}
	p, ok := m.sc.Prop(key)
	return ok && (p.Type == schema.TypeMultiEnum || p.Type == schema.TypeTaskRef && p.Multi)
}

func nodeEqual(a, b any) bool { return model.NodeEqual(a.(*yaml.Node), b.(*yaml.Node)) }

// fields merges a mapping key by key, naming each key with prefix. The keys
// keep dst's order, and the keys only the other side has follow in its
// order. With multi, the props that hold a list of values are sets.
func (m *sides) fields(prefix string, b, o, t, dst model.Fields, multi bool) model.Fields {
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
		v := m.node(prefix+k, b.Get(k), o.Get(k), t.Get(k), dst.Get(k), multi && m.multi(k))
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
func (m *sides) node(name string, b, o, t, d *yaml.Node, multi bool) *yaml.Node {
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

func isMapping(n *yaml.Node) bool { return n != nil && n.Kind == yaml.MappingNode }

func fieldsOf(n *yaml.Node) model.Fields {
	var f model.Fields
	if isMapping(n) {
		_ = f.UnmarshalYAML(n) // cannot fail on a mapping
	}
	return f
}

// text merges a Markdown body with diff3. Overlaps keep both sides, the
// older first; under OursWins they keep ours.
func (m *sides) text(b, o, t string) string {
	switch {
	case b == o:
		return t
	case b == t || o == t:
		return o
	case m.p == OursWins:
		return merge3(b, o, t, func(a, _ string) string { return a })
	}
	oursFirst := later(m.tTime, m.oTime, m.theirs.Author, m.ours.Author, m.theirs.Data, m.ours.Data)
	return Diff3(b, o, t, oursFirst)
}
