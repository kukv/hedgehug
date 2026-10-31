package merge

import (
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/model"
)

// yaml merges schema.yaml, automations.yaml or store.yaml key by key. The
// result is the document of the side it is built on, with its comments.
func (m *merger) yaml(base []byte, ours, theirs Side) (Result, error) {
	if ours.Data == nil || theirs.Data == nil {
		return m.revive(ours, theirs), nil
	}
	o, ok1 := loadYAML(ours.Data)
	t, ok2 := loadYAML(theirs.Data)
	if !ok1 || !ok2 {
		return m.other(ours, theirs)
	}
	b, ok := loadYAML(base)
	if !ok {
		b = nil // a base that cannot be read is empty
	}
	s := &sides{merger: m, ours: ours, theirs: theirs, oursTop: top(ours, theirs)}
	doc := t
	if s.oursTop {
		doc = o
	}
	v := s.yamlNode(nil, root(b), root(o), root(t))
	if v == nil {
		s.res.Data = []byte{}
		return s.res, nil
	}
	doc.Content = []*yaml.Node{v}
	data, err := yaml.Dump(doc, yaml.WithLineWidth(-1))
	if err != nil {
		return Result{}, err
	}
	s.res.Data = data
	return s.res, nil
}

// loadYAML reads a document. Blank data is a document with no value.
func loadYAML(data []byte) (*yaml.Node, bool) {
	var n yaml.Node
	if err := yaml.Load(data, &n); err != nil {
		if strings.Contains(err.Error(), "no documents") {
			return &yaml.Node{Kind: yaml.DocumentNode}, true
		}
		return nil, false
	}
	return &n, n.Kind == yaml.DocumentNode && len(n.Content) == 1
}

// root is the value of a document, or nil.
func root(doc *yaml.Node) *yaml.Node {
	if doc == nil || len(doc.Content) == 0 {
		return nil
	}
	return doc.Content[0]
}

// yamlNode merges the value at path; nil is a value that is not there.
// Mappings are merged key by key, and the lists whose items elemKey tells
// apart item by item. Other values take one side's value whole, timed by
// When. A key or an item one side removed and the other changed keeps the
// change, and whoever removed it is told.
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
	case isMapping(o) && isMapping(t) && (b == nil || isMapping(b)):
		v = m.yamlMapping(path, b, o, t, d)
	default:
		if k := elemKey(path); k != "" && itemsHave(k, b, o, t) {
			v = m.yamlList(path, k, b, o, t, d)
		} else {
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
// mappings each holding a scalar at key.
func itemsHave(key string, nodes ...*yaml.Node) bool {
	for _, n := range nodes {
		if n == nil {
			continue
		}
		if n.Kind != yaml.SequenceNode {
			return false
		}
		for _, it := range n.Content {
			if v := fieldsOf(it).Get(key); v == nil || v.Kind != yaml.ScalarNode {
				return false
			}
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

// numbers merges numbers.yaml: each prefix takes the largest number of the
// three, so that no number is given again. Nothing is overwritten. The
// result is the document of the side it is built on, with its comments.
func (m *merger) numbers(base []byte, ours, theirs Side) (Result, error) {
	if ours.Data == nil || theirs.Data == nil {
		return m.revive(ours, theirs), nil
	}
	o, err1 := model.DecodeNumbers(ours.Data)
	t, err2 := model.DecodeNumbers(theirs.Data)
	if err1 != nil || err2 != nil {
		return m.other(ours, theirs)
	}
	n, err := model.DecodeNumbers(base)
	if err != nil {
		n = model.Numbers{}
	}
	for _, x := range []model.Numbers{o, t} {
		for k, v := range x {
			n[k] = max(n[k], v)
		}
	}
	from := theirs.Data
	if top(ours, theirs) {
		from = ours.Data
	}
	doc, ok := loadYAML(from)
	mp := root(doc)
	if !ok || !isMapping(mp) {
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
	data, err := yaml.Dump(doc, yaml.WithLineWidth(-1))
	if err != nil {
		return Result{}, err
	}
	return Result{Data: data}, nil
}
