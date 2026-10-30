package schema

import (
	"slices"

	"github.com/kukv/hedgehug/internal/model"
)

// Effective is the schema as it applies to the tasks of one project: the
// store's definitions plus that project's override, if any.
type Effective struct {
	s     *Schema
	scope *Scoped // nil when no override applies
}

// For returns the schema for a task whose ancestors are ancestors, nearest
// first (include the task itself first when it is a container). The override
// of the nearest ancestor that has one applies; overrides of farther
// ancestors are not combined with it.
func (s *Schema) For(ancestors []model.ID) *Effective {
	for _, a := range ancestors {
		for i := range s.Scoped {
			if w, err := parseWhere(s.Scoped[i].Where); err == nil && w == a {
				return &Effective{s: s, scope: &s.Scoped[i]}
			}
		}
	}
	return &Effective{s: s}
}

func (e *Effective) Schema() *Schema { return e.s }

// Statuses returns the statuses to show and offer, in store order.
func (e *Effective) Statuses() []Status {
	if e.scope == nil || e.scope.Statuses == nil {
		return slices.Clone(e.s.Statuses)
	}
	var out []Status
	for _, st := range e.s.Statuses {
		if slices.Contains(e.scope.Statuses, st.Name) {
			out = append(out, st)
		}
	}
	return out
}

// CanTransition reports whether a task may move from one status to another.
// Both the store and the project must allow it. Staying in the same status
// is always allowed.
func (e *Effective) CanTransition(from, to string) bool {
	if from == to {
		return true
	}
	if !allows(e.s.Workflow, from, to) {
		return false
	}
	return e.scope == nil || e.scope.Workflow == nil || allows(*e.scope.Workflow, from, to)
}

// allows applies one workflow: only the statuses it lists are limited.
func allows(w Workflow, from, to string) bool {
	targets, ok := w.Transitions.Get(from)
	return !ok || slices.Contains(targets, to)
}

// Required returns the fields a task needs to enter status: the union of the
// store's and the project's lists.
func (e *Effective) Required(status string) []string {
	out, _ := e.s.Workflow.Required.Get(status)
	out = slices.Clone(out)
	if e.scope != nil && e.scope.Workflow != nil {
		more, _ := e.scope.Workflow.Required.Get(status)
		for _, f := range more {
			if !slices.Contains(out, f) {
				out = append(out, f)
			}
		}
	}
	return out
}

// Templates returns the templates the project may use (nil: all) and the
// default template of each kind.
func (e *Effective) Templates() (only []string, defaults map[string]string) {
	defaults = map[string]string{}
	for _, k := range e.s.Kinds {
		if k.Template != "" {
			defaults[k.Name] = k.Template
		}
	}
	if e.scope == nil || e.scope.Templates == nil {
		return nil, defaults
	}
	for k, v := range e.scope.Templates.Default {
		defaults[k] = v
	}
	return slices.Clone(e.scope.Templates.Only), defaults
}
