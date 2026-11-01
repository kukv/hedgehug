package schema

import (
	"maps"
	"slices"
)

// Carry returns sc cut down to what the schema s knows, for a scoped entry
// that comes from another store, and what it left out, one line each: "status
// <name>", "template <name>", "kind <name>" and "prop <name>". It drops the
// statuses s does not have, the workflow transitions and required lists that
// name them (a list left with nothing is dropped as "required <status>"), the required fields that are neither a property of s nor a field
// that can be required, the default templates of kinds s does not have, and
// the templates that templateExists denies. The result may still fail Check
// (a status list that hides a default status of s); the caller checks.
func (s *Schema) Carry(sc Scoped, templateExists func(name string) bool) (Scoped, []string) {
	var dropped []string
	drop := func(what, name string) { dropped = append(dropped, what+" "+name) }
	hasStatus := func(name string) bool { _, ok := s.Status(name); return ok }
	hasKind := func(name string) bool { _, ok := s.Kind(name); return ok }
	hasField := func(name string) bool {
		_, ok := s.Prop(name)
		return ok || slices.Contains(requirableFields, name)
	}

	out := sc
	if sc.Statuses != nil {
		out.Statuses = []string{}
		for _, name := range sc.Statuses {
			if hasStatus(name) {
				out.Statuses = append(out.Statuses, name)
			} else {
				drop("status", name)
			}
		}
	}
	if sc.Workflow != nil {
		w := Workflow{
			Transitions: carryTransitions(sc.Workflow.Transitions, hasStatus, drop),
			Required: carryLists(sc.Workflow.Required, hasStatus, hasField,
				func(name string) { drop("status", name) }, func(name string) { drop("prop", name) },
				func(name string) { drop("required", name) }),
		}
		out.Workflow = &w
	}
	if sc.Templates != nil {
		t := Templates{}
		if sc.Templates.Only != nil {
			t.Only = []string{}
			for _, name := range sc.Templates.Only {
				if templateExists(name) {
					t.Only = append(t.Only, name)
				} else {
					drop("template", name)
				}
			}
		}
		if sc.Templates.Default != nil {
			t.Default = map[string]string{}
			for _, kind := range slices.Sorted(maps.Keys(sc.Templates.Default)) {
				name := sc.Templates.Default[kind]
				switch {
				case !hasKind(kind):
					drop("kind", kind)
				case !templateExists(name):
					drop("template", name)
				default:
					t.Default[kind] = name
				}
			}
		}
		out.Templates = &t
	}
	return out, unique(dropped)
}

// unique is xs without repeats, in the order of first appearance.
func unique(xs []string) []string {
	var out []string
	for _, x := range xs {
		if !slices.Contains(out, x) {
			out = append(out, x)
		}
	}
	return out
}

// carryLists keeps the entries of l whose key keepKey allows, and in them the
// values keepValue allows; dropKey and dropValue are told of what is left out.
// A key that keepKey denies goes with its values, which are not told apart.
// A key whose values are all gone goes too (dropEmpty is told of it, as
// "required <status>"), as in carryTransitions: it asked for something, and
// nothing of it is left.
func carryLists(l Lists, keepKey, keepValue func(string) bool, dropKey, dropValue, dropEmpty func(string)) Lists {
	var out Lists
	for _, x := range l {
		if !keepKey(x.Key) {
			dropKey(x.Key)
			continue
		}
		kept := []string{}
		for _, v := range x.Values {
			if keepValue(v) {
				kept = append(kept, v)
			} else {
				dropValue(v)
			}
		}
		if len(kept) == 0 && len(x.Values) > 0 {
			dropEmpty(x.Key)
			continue
		}
		out = append(out, List{Key: x.Key, Values: kept})
	}
	return out
}

// carryTransitions is carryLists for workflow.transitions. A key whose targets
// are all gone is dropped too, so that status follows the workflow of the
// store; an empty list would forbid every move from it. It is told as
// "transitions <status>".
func carryTransitions(l Lists, has func(string) bool, drop func(what, name string)) Lists {
	var out Lists
	for _, x := range l {
		if !has(x.Key) {
			drop("status", x.Key)
			continue
		}
		kept := []string{}
		for _, v := range x.Values {
			if has(v) {
				kept = append(kept, v)
			} else {
				drop("status", v)
			}
		}
		if len(kept) == 0 && len(x.Values) > 0 {
			drop("transitions", x.Key)
			continue
		}
		out = append(out, List{Key: x.Key, Values: kept})
	}
	return out
}
