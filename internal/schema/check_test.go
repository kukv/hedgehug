package schema

import (
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
)

func TestCheckDefaults(t *testing.T) {
	t.Parallel()
	for name, s := range map[string]*Schema{"Default": Default(), "Dev": Dev()} {
		if ps := s.Check(); len(ps) != 0 {
			t.Errorf("%s has problems: %v", name, ps)
		}
	}
	s, err := Decode(readFull(t))
	if err != nil {
		t.Fatal(err)
	}
	s.Props = append(s.Props, Prop{Name: "points", Type: TypeNumber}) // full.yaml requires points
	if ps := s.Check(); len(ps) != 0 {
		t.Errorf("full.yaml has problems: %v", ps)
	}
}

func TestCheckProblems(t *testing.T) {
	t.Parallel()
	where := "project:01JBZ8K3QW7M2X9V4T6R5N8P0B"
	for _, tc := range []struct {
		name   string
		edit   func(s *Schema)
		inPath string
	}{
		{"duplicate status", func(s *Schema) { s.Statuses = append(s.Statuses, s.Statuses[0]) }, "statuses[7]"},
		{"empty status name", func(s *Schema) { s.Statuses[0].Name = "" }, "statuses[0]"},
		{"unknown category", func(s *Schema) { s.Statuses[0].Category = "x" }, "statuses[0]"},
		{"two defaults", func(s *Schema) { s.Statuses[0].Default = true }, "statuses"},
		{"no default", func(s *Schema) { s.Statuses[6].Default = false }, "statuses"},
		{"duplicate kind", func(s *Schema) { s.Kinds = append(s.Kinds, s.Kinds[0]) }, "kinds[5]"},
		{"unknown role", func(s *Schema) { s.Kinds[0].Role = "x" }, "kinds[0]"},
		{"unknown parent", func(s *Schema) { s.Kinds[1].Parents = []string{"nope"} }, "kinds[1].parents"},
		{"no work default", func(s *Schema) { s.Kinds[3].Default = false }, "kinds"},
		{"built-in prop name", func(s *Schema) { s.Props = append(s.Props, Prop{Name: "status", Type: TypeText}) }, "props.status"},
		{"duplicate prop", func(s *Schema) { s.Props = append(s.Props, s.Props[0]) }, "props.priority"},
		{"unknown type", func(s *Schema) { s.Props[0].Type = "x" }, "props.priority"},
		{"enum without values", func(s *Schema) { s.Props[0].Values = nil }, "props.priority"},
		{"inverse and symmetric", func(s *Schema) { s.Props[7].Inverse = "x" }, "props.relates"},
		{"inverse on text", func(s *Schema) { s.Props = append(s.Props, Prop{Name: "t", Type: TypeText, Inverse: "x"}) }, "props.t"},
		{"transition from unknown", func(s *Schema) { s.Workflow.Transitions = Lists{{"nope", []string{"todo"}}} }, "workflow.transitions"},
		{"transition to unknown", func(s *Schema) { s.Workflow.Transitions = Lists{{"todo", []string{"nope"}}} }, "workflow.transitions"},
		{"required unknown field", func(s *Schema) { s.Workflow.Required = Lists{{"done", []string{"nope"}}} }, "workflow.required"},
		{"bad where", func(s *Schema) { s.Scoped = []Scoped{{Where: "project:nope"}} }, "scoped[0].where"},
		{"not a project where", func(s *Schema) { s.Scoped = []Scoped{{Where: "store:x"}} }, "scoped[0].where"},
		{"duplicate where", func(s *Schema) { s.Scoped = []Scoped{{Where: where}, {Where: where}} }, "scoped[1].where"},
		{"hidden default status", func(s *Schema) { s.Scoped = []Scoped{{Where: where, Statuses: []string{"todo", "done", "canceled"}}} }, "scoped[0].statuses"},
		{"unknown scoped status", func(s *Schema) {
			s.Scoped = []Scoped{{Where: where, Statuses: []string{"todo", "in_progress", "done", "canceled", "nope"}}}
		}, "scoped[0].statuses"},
		{"scoped workflow", func(s *Schema) {
			s.Scoped = []Scoped{{Where: where, Workflow: &Workflow{Required: Lists{{"done", []string{"nope"}}}}}}
		}, "scoped[0].workflow.required"},
		{"scoped template kind", func(s *Schema) {
			s.Scoped = []Scoped{{Where: where, Templates: &Templates{Default: map[string]string{"nope": "x"}}}}
		}, "scoped[0].templates.default"},
		{"bad kind template name", func(s *Schema) { s.Kinds[4].Template = "../x" }, "kinds[4].template"},
		{"bad only template name", func(s *Schema) {
			s.Scoped = []Scoped{{Where: where, Templates: &Templates{Only: []string{"ok", "a/b"}}}}
		}, "scoped[0].templates.only"},
		{"bad default template name", func(s *Schema) {
			s.Scoped = []Scoped{{Where: where, Templates: &Templates{Default: map[string]string{"task": ".hidden"}}}}
		}, "scoped[0].templates.default"},
	} {
		s := Default()
		tc.edit(s)
		ps := s.Check()
		found := false
		for _, p := range ps {
			if strings.HasPrefix(p.Path, tc.inPath) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: no problem at %q in %v", tc.name, tc.inPath, ps)
		}
	}
}

func TestRequiredAcceptsBuiltinsAndProps(t *testing.T) {
	t.Parallel()
	s := Default()
	s.Workflow.Required = Lists{{"in_progress", []string{"assignees"}}, {"done", []string{"points", "sprint"}}}
	if ps := s.Check(); len(ps) != 0 {
		t.Errorf("problems: %v", ps)
	}
}

func TestBuiltinFilled(t *testing.T) {
	t.Parallel()
	empty := &model.Task{}
	for _, name := range requirableFields {
		filled, ok := builtinFilled(empty, name)
		if !ok || filled {
			t.Errorf("%s on empty task: filled=%v ok=%v", name, filled, ok)
		}
	}
	if filled, ok := builtinFilled(&model.Task{Assignees: []string{"a"}}, "assignees"); !ok || !filled {
		t.Errorf("assignees: filled=%v ok=%v", filled, ok)
	}
	if _, ok := builtinFilled(empty, "status"); ok {
		t.Error("status is not requirable")
	}
}
