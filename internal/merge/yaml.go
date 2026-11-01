package merge

import (
	"bytes"
	"errors"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/model"
)

// yaml merges schema.yaml, automations.yaml, store.yaml or a view key by
// key. The result is the document of the side it is built on, with its
// comments.
func (m *merger) yaml(base []byte, ours, theirs Side) (Result, error) {
	if ours.Data == nil || theirs.Data == nil {
		return m.revive(ours, theirs), nil
	}
	o, err1 := loadYAML(ours.Data)
	t, err2 := loadYAML(theirs.Data)
	if err1 != nil || err2 != nil {
		return m.other(ours, theirs)
	}
	b, err := loadYAML(base)
	if err != nil {
		b = nil // a base that cannot be read is empty
	}
	s := &sides{merger: m, ours: ours, theirs: theirs, oursTop: top(ours, theirs)}
	doc, other := t, o
	if s.oursTop {
		doc, other = o, t
	}
	flush := flushLists(doc, other)
	v := s.yamlNode(nil, root(b), root(o), root(t))
	if v == nil {
		s.res.Data = []byte{}
		return s.res, nil
	}
	data, err := dump(doc, v, flush)
	if err != nil {
		return Result{}, err
	}
	top := theirs.Data
	if s.oursTop {
		top = ours.Data
	}
	if blank(top) {
		// The side the result is built on holds only comments: they come
		// first, line for line.
		data = append(comments(top), data...)
	}
	s.res.Data = data
	return s.res, nil
}

// loadYAML reads a document. Data with only blank lines and comments is a
// document with no value; see comments for keeping them.
func loadYAML(data []byte) (*yaml.Node, error) {
	if blank(data) {
		return &yaml.Node{Kind: yaml.DocumentNode}, nil
	}
	var n yaml.Node
	if err := yaml.Load(data, &n); err != nil {
		return nil, err
	}
	if n.Kind != yaml.DocumentNode || len(n.Content) != 1 {
		return nil, errors.New("want one YAML document")
	}
	return &n, nil
}

// blank reports whether data holds only blank lines and comments. Data
// that is not UTF-8 is left to the parser, which refuses it.
func blank(data []byte) bool {
	if !utf8.Valid(data) {
		return false
	}
	for l := range bytes.Lines(data) {
		if l = bytes.TrimSpace(l); len(l) > 0 && l[0] != '#' {
			return false
		}
	}
	return true
}

// comments returns the lines of data, which holds only blank lines and
// comments, as they are with LF line ends. The parser would give them as
// one head comment, which the encoder writes back changed.
func comments(data []byte) []byte {
	out := bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	return out
}

// root is the value of a document, or nil.
func root(doc *yaml.Node) *yaml.Node {
	if doc == nil || len(doc.Content) == 0 {
		return nil
	}
	return doc.Content[0]
}

// flushLists tells how the result writes its block lists: as the first
// document with one writes them, else flush, as the encoders of model do.
// yaml.v4 indents the lists of one Dump all alike (the setting is the
// encoder's, not a node's), so a file that mixes the two styles comes out
// in the style of its first block list, as config.yaml does.
func flushLists(docs ...*yaml.Node) bool {
	for _, d := range docs {
		if r := root(d); r != nil {
			if flush, ok := model.FirstListStyle(r); ok {
				return flush
			}
		}
	}
	return true
}

// dump writes doc with v as its value.
func dump(doc, v *yaml.Node, flush bool) ([]byte, error) {
	doc.Content = []*yaml.Node{v}
	return yaml.Dump(doc, yaml.WithLineWidth(-1), yaml.WithIndent(2), yaml.WithCompactSeqIndent(flush))
}

// how is how a value of a yaml file is merged.
type how int

const (
	whole  how = iota // one side's value, timed by When
	byKey             // a mapping, key by key
	byItem            // a list, item by item
)

// split tells how the value at path is merged, given the versions of it
// that are there: key by key when all are mappings; item by item when all
// are lists whose items elemKey tells apart, with the key; else whole.
func split(path []string, nodes ...*yaml.Node) (how, string) {
	if !slices.ContainsFunc(nodes, func(n *yaml.Node) bool { return n != nil && !isMapping(n) }) {
		return byKey, ""
	}
	if k := elemKey(path); k != "" && itemsHave(k, nodes...) {
		return byItem, k
	}
	return whole, ""
}

// YAMLValue is a value of a yaml file that the merge takes whole from one
// side. When both sides changed it, their Side.When is asked about Path.
type YAMLValue struct {
	Path []string
	Node *yaml.Node
}

// YAMLValues lists the values of a file that ByKey reports, as the merge
// sees them: the keys of mappings and the items of the lists it tells apart
// are followed, by the key or the item's identity (which is not itself
// listed), and anything else is one value.
//
// The versions of a file need not agree on where a value starts. When one
// version holds one value where another has values under it (a mapping on
// one side and not on the other, a list whose items lose their identity
// key or share one, an empty mapping or list that gains items), When is
// asked about the higher path, and means the last change of any value at
// or under it. Likewise a value recorded at a higher path counts for the
// paths under it. WhenOf answers so.
func YAMLValues(data []byte) ([]YAMLValue, error) {
	doc, err := loadYAML(data)
	if err != nil {
		return nil, err
	}
	var out []YAMLValue
	var walk func(path []string, n *yaml.Node)
	walk = func(path []string, n *yaml.Node) {
		switch h, k := split(path, n); h {
		case byKey:
			for _, f := range fieldsOf(n) {
				walk(append(slices.Clone(path), f.Key), f.Value)
			}
		case byItem:
			for _, it := range n.Content {
				p := append(slices.Clone(path), fieldsOf(it).Get(k).Value)
				for _, f := range fieldsOf(it) {
					// The identity is the same on every side.
					if f.Key != k {
						walk(append(slices.Clone(p), f.Key), f.Value)
					}
				}
			}
		default:
			out = append(out, YAMLValue{Path: path, Node: n})
		}
	}
	if r := root(doc); r != nil {
		walk(nil, r)
	}
	return out, nil
}

// PathKey is the key of a yaml path in the map that WhenOf reads: each key
// of the path quoted, after a NUL. The root is "", and no two paths share
// a key, even with keys that are empty or hold a NUL.
func PathKey(path []string) string {
	var b strings.Builder
	for _, k := range path {
		b.WriteByte(0)
		b.WriteString(strconv.Quote(k))
	}
	return b.String()
}

// WhenOf builds a Side.When from when a side last changed each value,
// keyed by PathKey of the paths YAMLValues lists in its versions. A path is
// answered with the latest change at it, under it or above it.
func WhenOf(changed map[string]time.Time) func(path []string) time.Time {
	return func(path []string) time.Time {
		k := PathKey(path)
		var latest time.Time
		for p, at := range changed {
			if related(p, k) && at.After(latest) {
				latest = at
			}
		}
		return latest
	}
}

// related reports whether one of the path keys a and b is the other or a
// path under it.
func related(a, b string) bool {
	if len(a) > len(b) {
		a, b = b, a
	}
	return a == b || strings.HasPrefix(b, a) && b[len(a)] == 0
}

// yamlNode merges the value at path; nil is a value that is not there.
// split tells how. A key or an item one side removed and the other changed
// keeps the change, and whoever removed it is told.
func (m *sides) yamlNode(path []string, b, o, t *yaml.Node) *yaml.Node {
	d := t
	if m.oursTop {
		d = o
	}
	var v *yaml.Node
	oc, tc := !model.NodeEqual(b, o), !model.NodeEqual(b, t)
	switch {
	case !oc:
		v = t
	case !tc || model.NodeEqual(o, t):
		v = o
	case o == nil:
		v = t
		m.revived(m.ours.Author)
	case t == nil:
		v = o
		m.revived(m.theirs.Author)
	default:
		switch h, k := split(path, b, o, t); h {
		case byKey:
			v = m.yamlMapping(path, b, o, t, d)
		case byItem:
			v = m.yamlList(path, k, b, o, t, d)
		default:
			v = pickAt(m, strings.Join(path, "."), when(m.ours, path), when(m.theirs, path), b, o, t, nodeEqual)
		}
	}
	if v != nil && model.NodeEqual(v, d) {
		return d
	}
	return v
}

func (m *sides) revived(author string) {
	n := Notice{Kind: Revived, Path: m.path, Author: author}
	if !slices.Contains(m.res.Notices, n) {
		m.res.Notices = append(m.res.Notices, n)
	}
}

func when(s Side, path []string) time.Time {
	if s.When == nil {
		return time.Time{}
	}
	return s.When(path)
}

// yamlMapping merges mappings key by key. The keys keep d's order, and the
// keys only the other side has follow in its order.
func (m *sides) yamlMapping(path []string, b, o, t, d *yaml.Node) *yaml.Node {
	other := o
	if m.oursTop {
		other = t
	}
	var keys []string
	for _, f := range slices.Concat(fieldsOf(d), fieldsOf(other)) {
		if !slices.Contains(keys, f.Key) {
			keys = append(keys, f.Key)
		}
	}
	var f model.Fields
	for _, k := range keys {
		v := m.yamlNode(append(slices.Clone(path), k), fieldsOf(b).Get(k), fieldsOf(o).Get(k), fieldsOf(t).Get(k))
		if v != nil {
			f = append(f, model.Field{Key: k, Value: v})
		}
	}
	return mappingLike(d, f, other)
}

// mappingLike builds a mapping holding f in the style of like, with the
// key nodes of like, then of from, so that their comments survive.
func mappingLike(like *yaml.Node, f model.Fields, from ...*yaml.Node) *yaml.Node {
	n := *like
	n.Content = nil
	for _, x := range f {
		k := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: x.Key}
	search:
		for _, src := range append([]*yaml.Node{like}, from...) {
			for i := 0; src != nil && i+1 < len(src.Content); i += 2 {
				if src.Content[i].Value == x.Key {
					k = src.Content[i]
					break search
				}
			}
		}
		n.Content = append(n.Content, k, x.Value)
	}
	return &n
}

// elemKey returns the key that tells apart the items of the list at path,
// or "" when its items are not told apart.
func elemKey(path []string) string {
	if len(path) != 1 {
		return ""
	}
	switch path[0] {
	case "rules":
		return "id"
	case "scoped":
		return "where"
	case "statuses", "kinds", "categories":
		return "name"
	}
	return ""
}

// itemsHave reports whether the nodes are lists, or absent, whose items are
// mappings each holding a scalar at key, no two the same.
func itemsHave(key string, nodes ...*yaml.Node) bool {
	for _, n := range nodes {
		if n == nil {
			continue
		}
		if n.Kind != yaml.SequenceNode {
			return false
		}
		seen := map[string]bool{}
		for _, it := range n.Content {
			v := fieldsOf(it).Get(key)
			if v == nil || v.Kind != yaml.ScalarNode || seen[v.Value] {
				return false
			}
			seen[v.Value] = true
		}
	}
	return true
}

// yamlList merges lists item by item, the items told apart by the value at
// key. The items are ordered as a set's.
func (m *sides) yamlList(path []string, key string, b, o, t, d *yaml.Node) *yaml.Node {
	id := func(n *yaml.Node) string { return fieldsOf(n).Get(key).Value }
	items := func(n *yaml.Node) []*yaml.Node {
		if n == nil {
			return nil
		}
		return n.Content
	}
	item := func(n *yaml.Node, k string) *yaml.Node {
		if i := find(items(n), k, id); i >= 0 {
			return n.Content[i]
		}
		return nil
	}
	out := *d
	out.Content = nil
	for _, x := range mergeSet(items(b), items(o), items(t), id) {
		k := id(x)
		if v := m.yamlNode(append(slices.Clone(path), k), item(b, k), item(o, k), item(t, k)); v != nil {
			out.Content = append(out.Content, v)
		}
	}
	return &out
}

// numbers merges numbers.yaml, a record that only grows so that no number
// is given again: each prefix that either side has takes the largest
// number of the three versions, even when a side lowered it or removed the
// file. A prefix that both sides removed stays removed: the merge does not
// undo what both did, as it keeps a file that both removed removed. Nothing
// is overwritten. The result is the file of a side that holds it already,
// or else the document of the side it is built on, with its comments.
//
// A side that cannot be read counts as one that kept base, so that no
// number goes down, and its file is recorded as overwritten.
func (m *merger) numbers(base []byte, ours, theirs Side) (Result, error) {
	b, err := model.DecodeNumbers(base)
	if err != nil {
		b = model.Numbers{}
	}
	o, err1 := model.DecodeNumbers(ours.Data)
	t, err2 := model.DecodeNumbers(theirs.Data)
	var lost []Overwritten
	switch {
	case err1 != nil && err2 != nil:
		return m.other(ours, theirs)
	case err1 != nil:
		o, lost = maps.Clone(b), m.blobLost(ours)
		ours.Data = nil
	case err2 != nil:
		t, lost = maps.Clone(b), m.blobLost(theirs)
		theirs.Data = nil
	}
	r, err := maxNumbers(b, o, t, ours, theirs)
	r.Overwritten = lost
	return r, err
}

// maxNumbers writes the largest numbers of b, o and t for the prefixes of
// o and t.
func maxNumbers(b, o, t model.Numbers, ours, theirs Side) (Result, error) {
	n := model.Numbers{}
	for _, x := range []model.Numbers{o, t} {
		for k := range x {
			n[k] = max(b[k], o[k], t[k])
		}
	}
	first, second := theirs, ours
	if top(ours, theirs) {
		first, second = ours, theirs
	}
	if first.Data == nil {
		first, second = second, first
	}
	for _, s := range []Side{first, second} {
		if x, _ := model.DecodeNumbers(s.Data); s.Data != nil && maps.Equal(x, n) {
			return Result{Data: s.Data}, nil
		}
	}
	doc, err := loadYAML(first.Data)
	mp := root(doc)
	if err != nil || !isMapping(mp) {
		data, err := n.Encode()
		return Result{Data: data}, err
	}
	seen := map[string]bool{}
	for i := 0; i+1 < len(mp.Content); i += 2 {
		k, v := mp.Content[i].Value, mp.Content[i+1]
		seen[k] = true
		var x int
		if v.Decode(&x) != nil || x != n[k] {
			v.Kind, v.Tag, v.Value, v.Style = yaml.ScalarNode, "!!int", strconv.Itoa(n[k]), 0
		}
	}
	for _, k := range slices.Sorted(maps.Keys(n)) {
		if !seen[k] {
			mp.Content = append(mp.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k},
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(n[k])})
		}
	}
	data, err := dump(doc, mp, flushLists(doc))
	if err != nil {
		return Result{}, err
	}
	return Result{Data: data}, nil
}
