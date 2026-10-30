package schema

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/kukv/hedgehug/internal/model"
	"go.yaml.in/yaml/v4"
)

// Target is what a migration renames.
type Target string

const (
	TargetStatus Target = "status"
	TargetKind   Target = "kind"
	TargetProp   Target = "prop"
	TargetValue  Target = "value" // a value of an enum or multi_enum property
)

// Migration renames Old to New, or merges Old into New when New is
// defined. Prop is the property whose value is renamed, for TargetValue
// only.
type Migration struct {
	Target Target
	Prop   string
	Old    string
	New    string
}

// Migrate returns s with the migration applied: the definition renamed, or
// removed in a merge, and every reference to it in the schema rewritten.
// A list that a merge leaves with a name twice keeps the first. A merged
// status or kind that was a default hands it to the one it merges into,
// when that has the same category or role; otherwise Check reports the
// missing default. s is not
// changed, but the result shares s's file, so that Encode keeps its
// comments; encoding the result writes into that file, so s is not to be
// encoded after it.
func (s *Schema) Migrate(m Migration) (*Schema, error) {
	if m.Old == m.New {
		return nil, fmt.Errorf("%q and %q are the same", m.Old, m.New)
	}
	out := s.clone()
	switch m.Target {
	case TargetStatus:
		return out, out.migrateStatus(m.Old, m.New)
	case TargetKind:
		return out, out.migrateKind(m.Old, m.New)
	case TargetProp:
		return out, out.migrateProp(m.Old, m.New)
	case TargetValue:
		return out, out.migrateValue(m.Prop, m.Old, m.New)
	}
	return nil, fmt.Errorf("unknown target %q", m.Target)
}

func (s *Schema) clone() *Schema {
	out := *s
	out.Statuses = slices.Clone(s.Statuses)
	out.Kinds = slices.Clone(s.Kinds)
	for i := range out.Kinds {
		out.Kinds[i].Parents = slices.Clone(out.Kinds[i].Parents)
	}
	out.Props = slices.Clone(s.Props)
	for i := range out.Props {
		out.Props[i].Values = slices.Clone(out.Props[i].Values)
	}
	out.Workflow = s.Workflow.clone()
	out.Scoped = slices.Clone(s.Scoped)
	for i, sc := range out.Scoped {
		out.Scoped[i].Statuses = slices.Clone(sc.Statuses)
		if sc.Workflow != nil {
			w := sc.Workflow.clone()
			out.Scoped[i].Workflow = &w
		}
		if sc.Templates != nil {
			out.Scoped[i].Templates = &Templates{Only: slices.Clone(sc.Templates.Only), Default: maps.Clone(sc.Templates.Default)}
		}
	}
	return &out
}

func (w Workflow) clone() Workflow {
	return Workflow{Transitions: w.Transitions.clone(), Required: w.Required.clone()}
}

func (l Lists) clone() Lists {
	out := slices.Clone(l)
	for i := range out {
		out[i].Values = slices.Clone(out[i].Values)
	}
	return out
}

// workflows returns the workflow of the store and those of the scoped
// overrides.
func (s *Schema) workflows() []*Workflow {
	out := []*Workflow{&s.Workflow}
	for i := range s.Scoped {
		if s.Scoped[i].Workflow != nil {
			out = append(out, s.Scoped[i].Workflow)
		}
	}
	return out
}

func (s *Schema) migrateStatus(from, to string) error {
	i := slices.IndexFunc(s.Statuses, func(x Status) bool { return x.Name == from })
	if i < 0 {
		return fmt.Errorf("status %q is not defined", from)
	}
	if j := slices.IndexFunc(s.Statuses, func(x Status) bool { return x.Name == to }); j >= 0 {
		if s.Statuses[i].Default && s.Statuses[j].Category == s.Statuses[i].Category {
			s.Statuses[j].Default = true
		}
		s.Statuses = slices.Delete(s.Statuses, i, i+1)
	} else {
		s.Statuses[i].Name = to
	}
	for _, w := range s.workflows() {
		w.Transitions = renameKey(w.Transitions, from, to)
		for k := range w.Transitions {
			w.Transitions[k].Values = replace(w.Transitions[k].Values, from, to)
		}
		w.Required = renameKey(w.Required, from, to)
	}
	for i := range s.Scoped {
		s.Scoped[i].Statuses = replace(s.Scoped[i].Statuses, from, to)
	}
	return nil
}

func (s *Schema) migrateKind(from, to string) error {
	i := slices.IndexFunc(s.Kinds, func(x Kind) bool { return x.Name == from })
	if i < 0 {
		return fmt.Errorf("kind %q is not defined", from)
	}
	if j := slices.IndexFunc(s.Kinds, func(x Kind) bool { return x.Name == to }); j >= 0 {
		if s.Kinds[i].Default && s.Kinds[j].Role == s.Kinds[i].Role {
			s.Kinds[j].Default = true
		}
		s.Kinds = slices.Delete(s.Kinds, i, i+1)
	} else {
		s.Kinds[i].Name = to
	}
	for i := range s.Kinds {
		s.Kinds[i].Parents = replace(s.Kinds[i].Parents, from, to)
	}
	for _, sc := range s.Scoped {
		if sc.Templates == nil {
			continue
		}
		d := sc.Templates.Default
		if t, ok := d[from]; ok {
			// In a merge the template of to wins.
			if _, has := d[to]; !has {
				d[to] = t
			}
			delete(d, from)
		}
	}
	return nil
}

func (s *Schema) migrateProp(from, to string) error {
	i := slices.IndexFunc(s.Props, func(x Prop) bool { return x.Name == from })
	if i < 0 {
		return fmt.Errorf("property %q is not defined", from)
	}
	if np, ok := s.Prop(to); ok {
		op := s.Props[i]
		if op.Type != np.Type || op.Multi != np.Multi {
			return fmt.Errorf("the types differ: %s is %s, %s is %s", from, propTypeName(op), to, propTypeName(np))
		}
		s.Props = slices.Delete(s.Props, i, i+1)
	} else {
		s.Props[i].Name = to
	}
	for _, w := range s.workflows() {
		for k := range w.Required {
			w.Required[k].Values = replace(w.Required[k].Values, from, to)
		}
	}
	return nil
}

func propTypeName(p Prop) string {
	if p.Multi {
		return string(p.Type) + " (multi)"
	}
	return string(p.Type)
}

func (s *Schema) migrateValue(prop, from, to string) error {
	i := slices.IndexFunc(s.Props, func(x Prop) bool { return x.Name == prop })
	if i < 0 {
		return fmt.Errorf("property %q is not defined", prop)
	}
	p := &s.Props[i]
	if p.Type != TypeEnum && p.Type != TypeMultiEnum {
		return fmt.Errorf("%s is %s, not enum or multi_enum", prop, p.Type)
	}
	if !slices.Contains(p.Values, from) {
		return fmt.Errorf("value %q of %s is not defined", from, prop)
	}
	p.Values = replace(p.Values, from, to)
	return nil
}

// replace returns xs with from replaced by to, keeping the first to when
// there are two. A nil xs stays nil.
func replace(xs []string, from, to string) []string {
	if !slices.Contains(xs, from) {
		return xs
	}
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		if x == from {
			x = to
		}
		if x != to || !slices.Contains(out, to) {
			out = append(out, x)
		}
	}
	return out
}

// renameKey renames the key from of l to to. When to is a key too, the
// two lists become one at the place of the first, with the values of the
// second after its own.
func renameKey(l Lists, from, to string) Lists {
	i := slices.IndexFunc(l, func(x List) bool { return x.Key == from })
	if i < 0 {
		return l
	}
	j := slices.IndexFunc(l, func(x List) bool { return x.Key == to })
	if j < 0 {
		l[i].Key = to
		return l
	}
	first, second := min(i, j), max(i, j)
	l[first].Key = to
	for _, v := range l[second].Values {
		if !slices.Contains(l[first].Values, v) {
			l[first].Values = append(l[first].Values, v)
		}
	}
	return slices.Delete(l, second, second+1)
}

// ApplyTask rewrites the field of t that the migration renames, and
// reports whether it changed. In a merge of properties, a task with values
// for both is an error. s is the schema before the migration.
func (m Migration) ApplyTask(t *model.Task, s *Schema) (changed bool, err error) {
	switch m.Target {
	case TargetStatus:
		return swap(&t.Status, m.Old, m.New), nil
	case TargetKind:
		return swap(&t.Kind, m.Old, m.New), nil
	case TargetProp:
		i := slices.IndexFunc(t.Props, func(f model.Field) bool { return f.Key == m.Old })
		if i < 0 {
			return false, nil
		}
		if t.Props.Get(m.New) != nil {
			return false, fmt.Errorf("the task has both %s and %s", m.Old, m.New)
		}
		t.Props[i].Key = m.New
		return true, nil
	case TargetValue:
		if p, ok := s.Prop(m.Prop); !ok || (p.Type != TypeEnum && p.Type != TypeMultiEnum) {
			return false, fmt.Errorf("%s is not an enum or multi_enum property", m.Prop)
		}
		n := t.Props.Get(m.Prop)
		if n == nil {
			return false, nil
		}
		nn, ok := replaceValue(n, m.Old, m.New)
		if ok {
			t.Props.Set(m.Prop, nn)
		}
		return ok, nil
	}
	return false, errors.New("unknown target " + string(m.Target))
}

func swap(dst *string, from, to string) bool {
	if *dst != from {
		return false
	}
	*dst = to
	return true
}

// replaceValue returns a copy of n with the value from replaced by to: n
// itself when a scalar, or each item when a list, keeping the first to
// when there are two. n is left as it is, since the file it was read from
// still holds it.
func replaceValue(n *yaml.Node, from, to string) (*yaml.Node, bool) {
	switch n.Kind {
	case yaml.ScalarNode:
		if n.Value != from {
			return nil, false
		}
		c := *n
		c.Value = to
		return &c, true
	case yaml.SequenceNode:
		if !slices.ContainsFunc(n.Content, func(x *yaml.Node) bool { return x.Value == from }) {
			return nil, false
		}
		c := *n
		c.Content = nil
		seen := false
		for _, x := range n.Content {
			if x.Value == from {
				y := *x
				y.Value = to
				x = &y
			}
			if x.Kind == yaml.ScalarNode && x.Value == to {
				if seen {
					continue
				}
				seen = true
			}
			c.Content = append(c.Content, x)
		}
		return &c, true
	}
	return nil, false
}
