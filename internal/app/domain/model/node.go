package model

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
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
		if x.Value == nil {
			x.Value = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}
		}
		k, ok := keys[x.Key]
		if !ok {
			k = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: x.Key}
		}
		if k.LineComment != "" && !keyCommentFits(x.Value) {
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
		return nil, &DecodeError{Field: "frontmatter", Line: lineOffset + 1, Err: err}
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

// checkNode rejects anchors, aliases, tags, merge keys and duplicate keys, which
// Hedgehug files do not use and which can make rewritten output invalid.
func checkNode(n *yaml.Node, off int) error {
	if n.Anchor != "" {
		return &DecodeError{Line: n.Line + off, Err: errors.New("YAML anchors are not supported")}
	}
	// The decoder sets TaggedStyle only for a tag the writer typed; the tags
	// it resolves on its own (!!str, !!int, ...) leave it unset.
	if n.Style&yaml.TaggedStyle != 0 {
		return &DecodeError{Line: n.Line + off, Err: errors.New("YAML tags are not supported")}
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
// When v equals the value in the file, the file's node is kept as written,
// with its comments and the keys v does not know. Otherwise v is merged into
// the file's node (see mergeNode), so that the parts that did not change keep
// their nodes in the same way.
func (e *Encoder) Put(key string, v any, zero bool) {
	if e.err != nil {
		return
	}
	old := e.d.Get(key)
	if old == nil && zero {
		return
	}
	nv, ok := v.(*yaml.Node)
	if !ok && old != nil && decodesTo(old, reflect.ValueOf(v)) {
		e.fields = append(e.fields, Field{key, old})
		return
	}
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
	switch {
	case !ok:
		nv = mergeNode(old, nv, reflect.ValueOf(v))
	case NodeEqual(old, nv):
		nv = old
	default:
		styleLike(nv, old)
	}
	e.fields = append(e.fields, Field{key, nv})
}

// decodesTo reports whether n decodes to a value deeply equal to v. An
// invalid v matches nothing.
func decodesTo(n *yaml.Node, v reflect.Value) bool {
	if !v.IsValid() || !v.CanInterface() {
		return false
	}
	p := reflect.New(v.Type())
	if n.Decode(p.Interface()) != nil {
		return false
	}
	return reflect.DeepEqual(p.Elem().Interface(), v.Interface())
}

var (
	nodeType      = reflect.TypeFor[yaml.Node]()
	marshalerType = reflect.TypeFor[yaml.Marshaler]()
)

// mergeNode returns the node to write for nv, the encoding of the Go value v,
// in place of old, the node in the file. The parts of old that decode to the
// same value as the matching part of v are kept as written, with their
// comments, and so are the keys of a mapping that v's type does not know. An
// invalid v (a *yaml.Node, or no Go value) knows every key and matches only
// equal nodes.
func mergeNode(old, nv *yaml.Node, v reflect.Value) *yaml.Node {
	if old == nil {
		styleLike(nv, nil)
		return nv
	}
	v = deref(v)
	if NodeEqual(old, nv) || decodesTo(old, v) {
		return old
	}
	switch {
	case old.Kind == yaml.MappingNode && nv.Kind == yaml.MappingNode:
		if v.IsValid() && v.Kind() == reflect.Struct && !marshals(v.Type()) {
			return mergeStruct(old, nv, v)
		}
		// A Go map has no order of its own, so it keeps the file's.
		keepOrder := v.IsValid() && v.Kind() == reflect.Map
		return mergeMapping(old, nv, valuesOf(v), keepOrder)
	case old.Kind == yaml.SequenceNode && nv.Kind == yaml.SequenceNode:
		return mergeSeq(old, nv, v)
	}
	styleLike(nv, old)
	return nv
}

// deref follows pointers and interfaces. A nil one, or a yaml.Node, gives
// an invalid value.
func deref(v reflect.Value) reflect.Value {
	for v.IsValid() && (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	if v.IsValid() && v.Type() == nodeType {
		return reflect.Value{}
	}
	return v
}

func marshals(t reflect.Type) bool {
	return t.Implements(marshalerType) || reflect.PointerTo(t).Implements(marshalerType)
}

// structKeys returns the YAML keys of the struct t and the indexes of their
// fields.
func structKeys(t reflect.Type) map[string]int {
	keys := map[string]int{}
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() || f.Anonymous {
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("yaml"), ",")
		switch name {
		case "-":
			continue
		case "":
			name = strings.ToLower(f.Name)
		}
		keys[name] = i
	}
	return keys
}

// valuesOf returns a lookup of the Go values of a mapping that v is written
// as, where every key is data: a map, or a list of structs with its own
// MarshalYAML that is keyed by the string field tagged yaml:"-". Such a
// struct is the value, unless it has one field besides the key, which then
// is. The lookup gives an invalid value when it cannot tell.
func valuesOf(v reflect.Value) func(key string) reflect.Value {
	none := func(string) reflect.Value { return reflect.Value{} }
	switch {
	case !v.IsValid():
		return none
	case v.Kind() == reflect.Map && !marshals(v.Type()) && v.Type().Key().Kind() == reflect.String:
		return func(key string) reflect.Value {
			return v.MapIndex(reflect.ValueOf(key).Convert(v.Type().Key()))
		}
	case v.Kind() != reflect.Slice || !marshals(v.Type()) || v.Type().Elem().Kind() != reflect.Struct:
		return none
	}
	s := v.Type().Elem()
	keyField := -1
	var rest []int
	for i := range s.NumField() {
		f := s.Field(i)
		switch {
		case !f.IsExported() || f.Anonymous:
		case f.Tag.Get("yaml") == "-" && f.Type.Kind() == reflect.String:
			keyField = i
		default:
			rest = append(rest, i)
		}
	}
	if keyField < 0 {
		return none
	}
	return func(key string) reflect.Value {
		for i := range v.Len() {
			if it := v.Index(i); it.Field(keyField).String() == key {
				if len(rest) == 1 {
					return it.Field(rest[0])
				}
				return it
			}
		}
		return reflect.Value{}
	}
}

// mergeStruct merges the mapping of the struct v, whose YAML keys are known.
// The keys of nv come in its order; the keys of old that nv lacks are kept
// in their place when they are unknown, or decode to v's field, as one that
// omitempty dropped does.
func mergeStruct(old, nv *yaml.Node, v reflect.Value) *yaml.Node {
	of, nf := fieldsOf(old), fieldsOf(nv)
	known := structKeys(v.Type())
	field := func(key string) reflect.Value {
		if i, ok := known[key]; ok {
			return v.Field(i)
		}
		return reflect.Value{}
	}
	keep := func(x Field) bool {
		if nf.Get(x.Key) != nil {
			return false
		}
		_, ok := known[x.Key]
		return !ok || decodesTo(x.Value, field(x.Key))
	}
	var f Fields
	next := 0
	for _, x := range nf {
		j := slices.IndexFunc(of, func(o Field) bool { return o.Key == x.Key })
		if j >= next {
			for _, o := range of[next:j] {
				if keep(o) {
					f = append(f, o)
				}
			}
			next = j + 1
		}
		f = append(f, Field{x.Key, mergeNode(of.Get(x.Key), x.Value, field(x.Key))})
	}
	for _, o := range of[next:] {
		if keep(o) {
			f = append(f, o)
		}
	}
	return mappingOf(old, f)
}

// mergeMapping merges a mapping whose keys are all data, with the Go value
// of each key given by value. Each new key takes the old key of the same
// name; then the keys left over are paired in order, so a renamed key keeps
// its node and its key's comments, with the new name. Keys that nv lacks
// are dropped. With keepOrder, the keys in old stay in its order and the
// new keys follow.
func mergeMapping(old, nv *yaml.Node, value func(string) reflect.Value, keepOrder bool) *yaml.Node {
	of, nf := fieldsOf(old), fieldsOf(nv)
	pair := make([]int, len(nf)) // the index in of, or -1
	used := make([]bool, len(of))
	for i := range nf {
		pair[i] = slices.IndexFunc(of, func(o Field) bool { return o.Key == nf[i].Key })
		if pair[i] >= 0 {
			used[pair[i]] = true
		}
	}
	j := 0
	for i := range nf {
		for j < len(of) && used[j] {
			j++
		}
		if pair[i] < 0 && j < len(of) {
			pair[i], used[j] = j, true
		}
	}
	// The old mapping with the paired keys renamed, for mappingOf to take
	// the key nodes from.
	keys := *old
	keys.Content = slices.Clone(old.Content)
	for i := range nf {
		var ov *yaml.Node
		if p := pair[i]; p >= 0 {
			ov = of[p].Value
			if of[p].Key != nf[i].Key {
				k := *old.Content[2*p]
				k.Value = nf[i].Key
				keys.Content[2*p] = &k
			}
		}
		nf[i].Value = mergeNode(ov, nf[i].Value, value(nf[i].Key))
	}
	if keepOrder {
		pos := make(map[string]int, len(nf))
		for i, x := range nf {
			pos[x.Key] = len(of)
			if pair[i] >= 0 {
				pos[x.Key] = pair[i]
			}
		}
		slices.SortStableFunc(nf, func(a, b Field) int { return pos[a.Key] - pos[b.Key] })
	}
	return mappingOf(&keys, nf)
}

// mergeSeq merges the list v. Each new item takes the old item that decodes
// to the same value or, for structs, has the same name (or where); then the
// struct items left over are paired in order, so a renamed item keeps its
// node. Each old item is used once.
func mergeSeq(old, nv *yaml.Node, v reflect.Value) *yaml.Node {
	styleLike(nv, old)
	if !v.IsValid() || (v.Kind() != reflect.Slice && v.Kind() != reflect.Array) ||
		marshals(v.Type()) || v.Len() != len(nv.Content) {
		v = reflect.Value{}
	}
	item := func(i int) reflect.Value {
		if !v.IsValid() {
			return reflect.Value{}
		}
		return v.Index(i)
	}
	var elem reflect.Type
	if v.IsValid() {
		elem = v.Type().Elem()
		for elem.Kind() == reflect.Pointer {
			elem = elem.Elem()
		}
	}
	isStruct := elem != nil && elem.Kind() == reflect.Struct && !marshals(elem)
	id := ""
	if isStruct {
		id = idKey(elem)
	}
	match := make([]*yaml.Node, len(nv.Content))
	used := make([]bool, len(old.Content))
	pair := func(same func(o *yaml.Node, i int) bool) {
		for i := range nv.Content {
			for j, o := range old.Content {
				if match[i] == nil && !used[j] && same(o, i) {
					match[i], used[j] = o, true
				}
			}
		}
	}
	if id != "" {
		pair(func(o *yaml.Node, i int) bool {
			a, b := mapValue(o, id), mapValue(nv.Content[i], id)
			return a != nil && b != nil && a.Kind == yaml.ScalarNode && b.Kind == yaml.ScalarNode && a.Value == b.Value
		})
	} else {
		pair(func(o *yaml.Node, i int) bool { return NodeEqual(o, nv.Content[i]) || decodesTo(o, item(i)) })
	}
	if isStruct {
		pair(func(*yaml.Node, int) bool { return true })
	}
	for i, o := range match {
		if o != nil {
			nv.Content[i] = mergeNode(o, nv.Content[i], item(i))
		}
	}
	return nv
}

// idKey returns the YAML key that names an item of the struct t: name or
// where, or "" when it has neither.
func idKey(t reflect.Type) string {
	keys := structKeys(t)
	for _, k := range []string{"name", "where"} {
		if _, ok := keys[k]; ok {
			return k
		}
	}
	return ""
}

// mapValue returns the value of key in the mapping n, or nil.
func mapValue(n *yaml.Node, key string) *yaml.Node {
	if n.Kind != yaml.MappingNode {
		return nil
	}
	return fieldsOf(n).Get(key)
}

// Bytes returns the YAML, with extra after the known keys. A key written
// twice is an error, since the file could not be read back.
func (e *Encoder) Bytes(extra Fields) ([]byte, error) {
	if e.err != nil {
		return nil, e.err
	}
	all := slices.Concat(e.fields, extra)
	seen := map[string]bool{}
	for _, x := range all {
		if seen[x.Key] {
			return nil, fmt.Errorf("key %q is written twice", x.Key)
		}
		seen[x.Key] = true
	}
	e.d.root.Content[0] = mappingOf(e.d.mapping(), all)
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
