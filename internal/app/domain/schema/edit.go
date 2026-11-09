package schema

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"go.yaml.in/yaml/v4"
)

var (
	ErrNotDefined   = errors.New("not defined")
	ErrOrderStale   = errors.New("the names differ from the schema")
	ErrNotEnum      = errors.New("not an enum or multi_enum property")
	ErrValueInvalid = errors.New("invalid value")
	ErrNameTaken    = errors.New("the name is already defined")
	// ErrPropMerge is a migration of a property into one that is defined:
	// properties are not merged.
	ErrPropMerge = errors.New("properties cannot be merged")
)

func nameTaken(what, name string) error { return fmt.Errorf("%s %q: %w", what, name, ErrNameTaken) }

func notDefined(what, name string) error { return fmt.Errorf("%s %q: %w", what, name, ErrNotDefined) }

// StatusChange and PropChange say what changes. A Name is the
// new name of the definition; the tasks that use it are rewritten by the
// Migrations the change returns.
type StatusChange struct {
	Name, Label, Color *string
	Category           *Category
	Default            bool
}

type PropChange struct {
	Name, Label *string
	Multi       *bool
	Values      *[]ValueEdit
}

// ValueEdit is one value of an enum or multi_enum property after an edit.
// From lists the current values it comes from: none for a new value, one for
// a value kept or renamed, several for a merge.
type ValueEdit struct {
	Name string
	From []string
}

func set[T any](dst *T, v *T) {
	if v != nil {
		*dst = *v
	}
}

// AddStatus returns s with st at the end. Check finds a name that is
// empty or already defined.
func (s *Schema) AddStatus(st Status) *Schema {
	out := s.clone()
	out.Statuses = append(out.Statuses, st)
	return out
}

// ChangeStatus returns s with the status name changed as ch says, and the
// migration that a new name needs for the tasks. A default takes it from the
// other default of its category.
func (s *Schema) ChangeStatus(name string, ch StatusChange) (*Schema, []Migration, error) {
	out := s.clone()
	i := slices.IndexFunc(out.Statuses, func(x Status) bool { return x.Name == name })
	if i < 0 {
		return nil, nil, notDefined("status", name)
	}
	if ch.Name != nil && *ch.Name != name {
		if _, ok := s.Status(*ch.Name); ok {
			return nil, nil, nameTaken("status", *ch.Name)
		}
	}
	st := &out.Statuses[i]
	set(&st.Label, ch.Label)
	set(&st.Color, ch.Color)
	set(&st.Category, ch.Category)
	if ch.Default {
		for j := range out.Statuses {
			if out.Statuses[j].Category == st.Category {
				out.Statuses[j].Default = j == i
			}
		}
	}
	if ch.Name == nil || *ch.Name == name {
		return out, nil, nil
	}
	m := Migration{Target: TargetStatus, Old: name, New: *ch.Name}
	out, err := out.Migrate(m)
	return out, []Migration{m}, err
}

// RemoveStatus returns s without the status name and without the references
// to it in the workflows and the overrides. A transition left with no
// target, and a required list left with nothing, go too, as in Carry.
func (s *Schema) RemoveStatus(name string) (*Schema, error) {
	out := s.clone()
	i := slices.IndexFunc(out.Statuses, func(x Status) bool { return x.Name == name })
	if i < 0 {
		return nil, notDefined("status", name)
	}
	out.Statuses = slices.Delete(out.Statuses, i, i+1)
	for _, w := range out.workflows() {
		w.Transitions = dropKey(w.Transitions, name)
		w.Transitions = dropValue(w.Transitions, name)
		w.Required = dropKey(w.Required, name)
	}
	out.dropEmptyWorkflows()
	for j := range out.Scoped {
		if out.Scoped[j].Statuses != nil {
			out.Scoped[j].Statuses = slices.DeleteFunc(slices.Clone(out.Scoped[j].Statuses), func(x string) bool { return x == name })
		}
	}
	return out, nil
}

// dropEmptyWorkflows takes an override's workflow that is left with no
// transition and no required list, so that no `workflow: {}` is written.
func (s *Schema) dropEmptyWorkflows() {
	for i := range s.Scoped {
		if w := s.Scoped[i].Workflow; w != nil && w.IsZero() {
			s.Scoped[i].Workflow = nil
		}
	}
}

// dropKey is l without the key name.
func dropKey(l Lists, name string) Lists {
	return nilIfEmpty(slices.DeleteFunc(slices.Clone(l), func(x List) bool { return x.Key == name }))
}

// dropValue is l without the value name; a key that had values and is left
// with none goes.
func dropValue(l Lists, name string) Lists {
	var out Lists
	for _, x := range l {
		vs := slices.DeleteFunc(slices.Clone(x.Values), func(v string) bool { return v == name })
		if len(vs) == 0 && len(x.Values) > 0 {
			continue
		}
		out = append(out, List{x.Key, vs})
	}
	return out
}

// reorder returns xs in the order of names, which has to name each item of
// xs once.
func reorder[T any](xs []T, nameOf func(T) string, names []string) ([]T, error) {
	if len(names) != len(xs) {
		return nil, ErrOrderStale
	}
	by := make(map[string]T, len(xs))
	for _, x := range xs {
		by[nameOf(x)] = x
	}
	if len(by) != len(xs) {
		return nil, ErrOrderStale // a name defined twice
	}
	out := make([]T, 0, len(xs))
	seen := map[string]bool{}
	for _, n := range names {
		x, ok := by[n]
		if !ok || seen[n] {
			return nil, ErrOrderStale
		}
		seen[n] = true
		out = append(out, x)
	}
	return out, nil
}

func (s *Schema) OrderStatuses(names []string) (*Schema, error) {
	out := s.clone()
	var err error
	if out.Statuses, err = reorder(out.Statuses, func(x Status) string { return x.Name }, names); err != nil {
		return nil, err
	}
	return out, nil
}

// SetTemplateDefaults returns s with templates.default replaced by def (kind to
// template name); an empty def takes it off. templates.only is kept. Check
// refuses a kind that does not exist and a name that is not a template name.
func (s *Schema) SetTemplateDefaults(def map[string]string) *Schema {
	out := s.clone()
	var tm Templates
	if out.Templates != nil {
		tm = *out.Templates
	}
	tm.Default = nil
	if len(def) > 0 {
		tm.Default = maps.Clone(def)
	}
	out.Templates = &tm
	if len(tm.Only) == 0 && len(tm.Default) == 0 {
		out.Templates = nil
	}
	return out
}

// AddProp returns s with p at the end. Check does not look at values twice,
// so a value that is empty or repeated is refused here.
func (s *Schema) AddProp(p Prop) (*Schema, error) {
	seen := map[string]bool{}
	for _, v := range p.Values {
		if v == "" || seen[v] {
			return nil, fmt.Errorf("value %q of %s is empty or repeated: %w", v, p.Name, ErrValueInvalid)
		}
		seen[v] = true
	}
	out := s.clone()
	out.Props = append(out.Props, p)
	return out, nil
}

// ChangeProp returns s with the property name changed as ch says, the
// migrations that a new name and the values need for the tasks, and the
// values that are removed. Values are for an enum or multi_enum property only.
func (s *Schema) ChangeProp(name string, ch PropChange) (*Schema, []Migration, []string, error) {
	out := s.clone()
	i := slices.IndexFunc(out.Props, func(x Prop) bool { return x.Name == name })
	if i < 0 {
		return nil, nil, nil, notDefined("property", name)
	}
	p := &out.Props[i]
	if ch.Name != nil && *ch.Name != name {
		if _, ok := s.Prop(*ch.Name); ok {
			return nil, nil, nil, nameTaken("property", *ch.Name)
		}
	}
	set(&p.Label, ch.Label)
	set(&p.Multi, ch.Multi)
	var ms []Migration
	var removed []string
	if ch.Values != nil {
		if p.Type != TypeEnum && p.Type != TypeMultiEnum {
			return nil, nil, nil, fmt.Errorf("%s: %w", name, ErrNotEnum)
		}
		var err error
		if p.Values, ms, removed, err = editValues(name, p.Values, *ch.Values); err != nil {
			return nil, nil, nil, err
		}
	}
	if ch.Name == nil || *ch.Name == name {
		return out, ms, removed, nil
	}
	m := Migration{Target: TargetProp, Old: name, New: *ch.Name}
	out, err := out.Migrate(m)
	return out, append(ms, m), removed, err
}

// ChangePriorities returns s with the values of priority after the edit,
// the migrations that carry the tasks to them, and the values that are
// removed. The rules are those of the values of an enum property, with two
// more: a value is told apart from another without regard to case, and the
// last value cannot go.
func (s *Schema) ChangePriorities(values []ValueEdit) (*Schema, []Migration, []string, error) {
	return s.changeList("priority", TargetPriority, s.Priorities, values, true, func(out *Schema, v []string) { out.Priorities = v })
}

// ChangeLabels is ChangePriorities for the values of labels, which may be
// all removed.
func (s *Schema) ChangeLabels(values []ValueEdit) (*Schema, []Migration, []string, error) {
	return s.changeList("label", TargetLabel, s.Labels, values, false, func(out *Schema, v []string) { out.Labels = v })
}

// changeList is the edit of one list of the schema: set puts the new values
// into the copy of s that is returned.
func (s *Schema) changeList(name string, target Target, cur []string, eds []ValueEdit, needOne bool, set func(*Schema, []string)) (*Schema, []Migration, []string, error) {
	values, ms, removed, err := editList(name, target, cur, eds, needOne)
	if err != nil {
		return nil, nil, nil, err
	}
	out := s.clone()
	set(out, values)
	return out, ms, removed, nil
}

// editList is editValues for a list of the schema: its migrations are of the
// target, and the new names differ without regard to case.
func editList(name string, target Target, cur []string, eds []ValueEdit, needOne bool) ([]string, []Migration, []string, error) {
	if needOne && len(eds) == 0 {
		return nil, nil, nil, fmt.Errorf("%s: the last value cannot be removed: %w", name, ErrValueInvalid)
	}
	folded := map[string]bool{}
	for _, e := range eds {
		k := strings.ToLower(e.Name)
		if folded[k] {
			return nil, nil, nil, fmt.Errorf("%s: value %q is repeated, case is ignored: %w", name, e.Name, ErrValueInvalid)
		}
		folded[k] = true
	}
	values, ms, removed, err := editValues(name, cur, eds)
	if err != nil {
		return nil, nil, nil, err
	}
	for i := range ms {
		ms[i].Target, ms[i].Prop = target, ""
	}
	return values, ms, removed, nil
}

// tempValue stands between two names of a swap; it is only ever written to
// a task in memory.
const tempValue = "\x00hedgehug-swap"

// editValues is the values cur after the edit eds, the migrations that carry
// the tasks from the current values to the new ones, in an order that never
// moves a value onto one that is still to be moved, and the current values
// that no edit keeps.
func editValues(prop string, cur []string, eds []ValueEdit) (values []string, ms []Migration, removed []string, err error) {
	bad := func(format string, a ...any) error {
		return fmt.Errorf("%s: %s: %w", prop, fmt.Sprintf(format, a...), ErrValueInvalid)
	}
	taken := map[string]bool{}
	from := map[string]bool{}
	for _, e := range eds {
		if e.Name == "" || taken[e.Name] {
			return nil, nil, nil, bad("value %q is empty or repeated", e.Name)
		}
		taken[e.Name] = true
		for _, f := range e.From {
			if !slices.Contains(cur, f) {
				return nil, nil, nil, bad("%q is not a current value", f)
			}
			if from[f] {
				return nil, nil, nil, bad("%q comes from more than one value", f)
			}
			from[f] = true
		}
	}
	for _, v := range cur {
		if from[v] {
			continue
		}
		if taken[v] {
			return nil, nil, nil, bad("%q is a value that goes, and the name of another", v)
		}
		removed = append(removed, v)
	}
	var pending [][2]string
	for _, e := range eds {
		values = append(values, e.Name)
		for _, f := range e.From {
			if f != e.Name {
				pending = append(pending, [2]string{f, e.Name})
			}
		}
	}
	emit := func(old, to string) {
		ms = append(ms, Migration{Target: TargetValue, Prop: prop, Old: old, New: to})
	}
	for len(pending) > 0 {
		k := slices.IndexFunc(pending, func(m [2]string) bool {
			return !slices.ContainsFunc(pending, func(o [2]string) bool { return o[0] == m[1] })
		})
		if k >= 0 {
			emit(pending[k][0], pending[k][1])
			pending = slices.Delete(pending, k, k+1)
			continue
		}
		// only cycles are left: park one value
		emit(pending[0][0], tempValue)
		pending[0][0] = tempValue
	}
	return values, ms, removed, nil
}

// RemoveProp returns s without the property name and without it in the
// required lists; a list left with nothing goes.
func (s *Schema) RemoveProp(name string) (*Schema, error) {
	out := s.clone()
	i := slices.IndexFunc(out.Props, func(x Prop) bool { return x.Name == name })
	if i < 0 {
		return nil, notDefined("property", name)
	}
	out.Props = slices.Delete(out.Props, i, i+1)
	for _, w := range out.workflows() {
		w.Required = nilIfEmpty(dropValue(w.Required, name))
	}
	out.dropEmptyWorkflows()
	return out, nil
}

// OrderProps puts the properties in the order of names, which names all of
// them.
func (s *Schema) OrderProps(names []string) (*Schema, error) {
	out := s.clone()
	sorted, err := reorder(out.Props, func(x Prop) string { return x.Name }, names)
	if err != nil {
		return nil, err
	}
	out.Props = sorted
	return out, nil
}

// AddedProblems lists the problems of s that are not in before. Problems are
// the same when their Code and Args are: a removal or a move shifts the
// indexes in Path.
func (s *Schema) AddedProblems(before []Problem) []Problem {
	var out []Problem
	for _, p := range s.Check() {
		if !slices.ContainsFunc(before, func(b Problem) bool { return b.Code == p.Code && slices.Equal(b.Args, p.Args) }) {
			out = append(out, p)
		}
	}
	return out
}

// TaskUses reports whether t uses the definition name: its status,
// a value for the property, or the value name of the property prop.
func TaskUses(t *model.Task, target Target, prop, name string) bool {
	switch target {
	case TargetStatus:
		return t.Status == name
	case TargetProp:
		return t.Props.Get(name) != nil
	case TargetPriority:
		return strings.EqualFold(t.Priority, name)
	case TargetLabel:
		return slices.ContainsFunc(t.Labels, func(l string) bool { return strings.EqualFold(l, name) })
	case TargetValue:
		n := t.Props.Get(prop)
		if n == nil {
			return false
		}
		if n.Kind == yaml.ScalarNode {
			return n.Value == name
		}
		return slices.ContainsFunc(n.Content, func(x *yaml.Node) bool { return x.Kind == yaml.ScalarNode && x.Value == name })
	}
	return false
}
