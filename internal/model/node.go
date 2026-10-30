package model

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"

	"go.yaml.in/yaml/v4"
)

// Field is one key and value of a YAML mapping.
type Field struct {
	Key   string
	Value *yaml.Node
}

// Fields is an ordered list of YAML keys and values. Props and unknown keys
// use it, so that their order and comments survive a round trip.
type Fields []Field

// Get returns the value of key, or nil when the key is absent.
func (f Fields) Get(key string) *yaml.Node {
	for _, x := range f {
		if x.Key == key {
			return x.Value
		}
	}
	return nil
}

// Set replaces the value of key, or appends the key when it is absent.
func (f *Fields) Set(key string, v *yaml.Node) {
	for i := range *f {
		if (*f)[i].Key == key {
			(*f)[i].Value = v
			return
		}
	}
	*f = append(*f, Field{key, v})
}

// Delete removes key.
func (f *Fields) Delete(key string) {
	*f = slices.DeleteFunc(*f, func(x Field) bool { return x.Key == key })
}

// UnmarshalYAML reads a mapping and keeps each value node as written.
func (f *Fields) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: want a mapping", n.Line)
	}
	*f = fieldsOf(n)
	return nil
}

func (f Fields) MarshalYAML() (any, error) { return mappingOf(nil, f), nil }

func fieldsOf(m *yaml.Node) Fields {
	f := make(Fields, 0, len(m.Content)/2)
	for i := 0; i+1 < len(m.Content); i += 2 {
		f = append(f, Field{m.Content[i].Value, m.Content[i+1]})
	}
	return f
}

// mappingOf builds a mapping node that holds f. The key nodes and the look
// of old are reused, so that comments survive.
func mappingOf(old *yaml.Node, f Fields) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	keys := map[string]*yaml.Node{}
	if old != nil && old.Kind == yaml.MappingNode {
		m.Style = old.Style
		m.HeadComment, m.LineComment, m.FootComment = old.HeadComment, old.LineComment, old.FootComment
		for i := 0; i+1 < len(old.Content); i += 2 {
			keys[old.Content[i].Value] = old.Content[i]
		}
	}
	for _, x := range f {
		k, ok := keys[x.Key]
		if !ok {
			k = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: x.Key}
		}
		if k.LineComment != "" && x.Value != nil && !keyCommentFits(x.Value) {
			// "key: # c" puts the comment on the key. Written before a value
			// that is not a block collection it would break the YAML, so
			// move it to the end of the value's line.
			kc := *k
			kc.LineComment = ""
			if x.Value.LineComment == "" {
				x.Value.LineComment = k.LineComment
			}
			k = &kc
		}
		m.Content = append(m.Content, k, x.Value)
	}
	return m
}

// keyCommentFits reports whether a line comment on a key can be written in
// front of the value v: v is a block collection with items, or an empty null.
func keyCommentFits(v *yaml.Node) bool {
	switch v.Kind {
	case yaml.MappingNode, yaml.SequenceNode:
		return v.Style&yaml.FlowStyle == 0 && len(v.Content) > 0
	case yaml.ScalarNode:
		return v.ShortTag() == "!!null" && v.Value == ""
	}
	return false
}

// NodeEqual reports whether a and b hold the same YAML value. Styles,
// comments and positions are ignored.
func NodeEqual(a, b *yaml.Node) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Kind == yaml.AliasNode || b.Kind == yaml.AliasNode {
		return a.Kind == b.Kind && NodeEqual(a.Alias, b.Alias)
	}
	if a.Kind != b.Kind || a.ShortTag() != b.ShortTag() || len(a.Content) != len(b.Content) {
		return false
	}
	if a.Kind == yaml.ScalarNode && a.Value != b.Value {
		return false
	}
	for i := range a.Content {
		if !NodeEqual(a.Content[i], b.Content[i]) {
			return false
		}
	}
	return true
}

// Doc is a YAML mapping kept from decode to encode, so that comments, key
// order and unknown keys survive.
type Doc struct {
	root *yaml.Node // document node holding one mapping
	line int        // added to node lines to give lines in the file
}

// ParseDoc parses src, which must be a YAML mapping or blank. lineOffset is
// the number of file lines before src, used in DecodeError.
func ParseDoc(src []byte, lineOffset int) (*Doc, error) {
	if len(bytes.TrimSpace(src)) == 0 {
		return newDocAt(lineOffset), nil
	}
	var root yaml.Node
	if err := yaml.Load(src, &root); err != nil {
		if strings.Contains(err.Error(), "no documents") {
			return newDocAt(lineOffset), nil
		}
		return nil, fmt.Errorf("YAML starting at line %d: %w", lineOffset+1, err)
	}
	if root.Kind == 0 || (root.Kind == yaml.DocumentNode && len(root.Content) == 0) {
		return newDocAt(lineOffset), nil
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		line := root.Line
		if len(root.Content) > 0 {
			line = root.Content[0].Line
		}
		return nil, &DecodeError{Line: line + lineOffset, Err: errors.New("want a YAML mapping")}
	}
	if err := checkNode(&root, lineOffset); err != nil {
		return nil, err
	}
	return &Doc{root: &root, line: lineOffset}, nil
}

func newDocAt(lineOffset int) *Doc {
	d := NewDoc()
	d.line = lineOffset
	return d
}

// checkNode rejects anchors, aliases, merge keys and duplicate keys, which
// Hedgehug files do not use and which can make rewritten output invalid.
func checkNode(n *yaml.Node, off int) error {
	if n.Anchor != "" {
		return &DecodeError{Line: n.Line + off, Err: errors.New("YAML anchors are not supported")}
	}
	if n.Kind == yaml.AliasNode {
		return &DecodeError{Line: n.Line + off, Err: errors.New("YAML aliases are not supported")}
	}
	if n.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			k := n.Content[i]
			if k.Kind != yaml.ScalarNode {
				continue
			}
			if k.ShortTag() == "!!merge" || (k.Value == "<<" && k.Style == 0) {
				return &DecodeError{Line: k.Line + off, Err: errors.New("YAML merge keys are not supported")}
			}
			if seen[k.Value] {
				return &DecodeError{Field: k.Value, Line: k.Line + off, Err: fmt.Errorf("duplicate key %q", k.Value)}
			}
			seen[k.Value] = true
		}
	}
	for _, c := range n.Content {
		if err := checkNode(c, off); err != nil {
			return err
		}
	}
	return nil
}

// NewDoc returns an empty Doc, for values made in code.
func NewDoc() *Doc {
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	return &Doc{root: &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{m}}}
}

func (d *Doc) mapping() *yaml.Node { return d.root.Content[0] }

// Get returns the value of key, or nil when it is absent.
func (d *Doc) Get(key string) *yaml.Node { return fieldsOf(d.mapping()).Get(key) }

// Decode reads the value of key into out. An absent key or null leaves out
// unchanged. Strings and lists of strings are read as written, so a title
// such as 2026-11-03 stays a string. A **yaml.Node gets the node itself.
func (d *Doc) Decode(key string, out any) error {
	n := d.Get(key)
	if n == nil || n.ShortTag() == "!!null" {
		return nil
	}
	var err error
	switch o := out.(type) {
	case *string:
		*o, err = scalar(n)
	case *[]string:
		*o, err = scalars(n)
	case **yaml.Node:
		*o = n
	default:
		err = n.Decode(out)
	}
	if err != nil {
		return &DecodeError{Field: key, Line: n.Line + d.line, Err: err}
	}
	return nil
}

func scalar(n *yaml.Node) (string, error) {
	if n.Kind != yaml.ScalarNode {
		return "", errors.New("want a single value")
	}
	return n.Value, nil
}

func scalars(n *yaml.Node) ([]string, error) {
	if n.Kind != yaml.SequenceNode {
		return nil, errors.New("want a list")
	}
	s := make([]string, 0, len(n.Content))
	for _, it := range n.Content {
		v, err := scalar(it)
		if err != nil {
			return nil, err
		}
		s = append(s, v)
	}
	return s, nil
}

// Extra returns the keys not in known, in file order.
func (d *Doc) Extra(known []string) Fields {
	var f Fields
	for _, x := range fieldsOf(d.mapping()) {
		if !slices.Contains(known, x.Key) {
			f = append(f, x)
		}
	}
	return f
}

// Encoder writes values back into a Doc, known keys first in the order they
// are put, then the extra keys.
type Encoder struct {
	d      *Doc
	fields Fields
	err    error
}

func (d *Doc) Encoder() *Encoder { return &Encoder{d: d} }

// Put sets key to v. A *yaml.Node is used as is; a nil one writes null. When
// the key is absent from the file and zero is true, the key is left out.
// When v equals the value in the file, the file's node is kept as written.
func (e *Encoder) Put(key string, v any, zero bool) {
	if e.err != nil {
		return
	}
	old := e.d.Get(key)
	if old == nil && zero {
		return
	}
	nv, ok := v.(*yaml.Node)
	if !ok || nv == nil {
		if ok {
			v = nil
		}
		nv = new(yaml.Node)
		if err := nv.Encode(v); err != nil {
			e.err = fmt.Errorf("%s: %w", key, err)
			return
		}
	}
	if NodeEqual(old, nv) {
		nv = old
	} else {
		styleLike(nv, old)
	}
	e.fields = append(e.fields, Field{key, nv})
}

// Bytes returns the YAML, with extra after the known keys.
func (e *Encoder) Bytes(extra Fields) ([]byte, error) {
	if e.err != nil {
		return nil, e.err
	}
	e.d.root.Content[0] = mappingOf(e.d.mapping(), slices.Concat(e.fields, extra))
	return yaml.Dump(e.d.root, yaml.WithLineWidth(-1))
}

// styleLike gives a new value node the look of the node it replaces. With no
// node to follow, lists of scalars and the short mappings inside lists are
// written in flow style, as in [a, b] and {type: x, ref: y}.
func styleLike(nv, old *yaml.Node) {
	if nv.Kind == yaml.SequenceNode {
		var tmpl *yaml.Node
		if old != nil && old.Kind == yaml.SequenceNode {
			for _, it := range old.Content {
				if it.Kind == yaml.MappingNode {
					tmpl = it
					break
				}
			}
		}
		for _, it := range nv.Content {
			switch {
			case it.Kind != yaml.MappingNode:
			case tmpl != nil:
				it.Style = tmpl.Style
			case short(it):
				it.Style = yaml.FlowStyle
			}
		}
	}
	switch {
	case old != nil && old.Kind == nv.Kind && nv.Kind != yaml.ScalarNode:
		nv.Style = old.Style
	case nv.Kind == yaml.SequenceNode && flat(nv):
		nv.Style = yaml.FlowStyle
	}
	if old != nil {
		nv.HeadComment, nv.LineComment, nv.FootComment = old.HeadComment, old.LineComment, old.FootComment
	}
}

// flat reports whether n is a scalar or a collection of scalars.
func flat(n *yaml.Node) bool {
	for _, c := range n.Content {
		if c.Kind != yaml.ScalarNode {
			return false
		}
	}
	return true
}

// short reports whether every value of the mapping m is flat.
func short(m *yaml.Node) bool {
	for i := 1; i < len(m.Content); i += 2 {
		if !flat(m.Content[i]) {
			return false
		}
	}
	return true
}
