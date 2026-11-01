package schema

import (
	"slices"
	"testing"
)

func TestCarry(t *testing.T) {
	dest := Default() // no review status
	sc := Scoped{
		Where:    "project:01JBZ8K3QW7M2X9V4T6R5N8P0A",
		Statuses: []string{"todo", "review", "in_progress", "done", "canceled", "backlog"},
		Workflow: &Workflow{
			Transitions: Lists{{Key: "todo", Values: []string{"review", "in_progress"}}, {Key: "review", Values: []string{"done"}}},
			Required:    Lists{{Key: "review", Values: []string{"assignees"}}, {Key: "done", Values: []string{"gone", "assignees"}}},
		},
		Templates: &Templates{Only: []string{"bug", "nope"}, Default: map[string]string{"task": "nope", "alien": "bug"}},
	}
	have := func(name string) bool { return name == "bug" }
	got, dropped := dest.Carry(sc, have)

	for _, name := range got.Statuses {
		if name == "review" {
			t.Errorf("review is still in statuses: %v", got.Statuses)
		}
	}
	if v, _ := got.Workflow.Transitions.Get("todo"); !slices.Equal(v, []string{"in_progress"}) {
		t.Errorf("todo transitions = %v", v)
	}
	if _, ok := got.Workflow.Transitions.Get("review"); ok {
		t.Error("the transitions of review are kept")
	}
	if v, _ := got.Workflow.Required.Get("done"); !slices.Equal(v, []string{"assignees"}) {
		t.Errorf("done required = %v", v)
	}
	if !slices.Equal(got.Templates.Only, []string{"bug"}) || len(got.Templates.Default) != 0 {
		t.Errorf("templates = %+v", got.Templates)
	}
	want := []string{"status review", "prop gone", "template nope", "kind alien"}
	for _, w := range want {
		if !slices.Contains(dropped, w) {
			t.Errorf("dropped %v lacks %q", dropped, w)
		}
	}
	if len(dropped) != len(want) {
		t.Errorf("dropped = %v, want %v", dropped, want)
	}
	if sc.Statuses[1] != "review" {
		t.Error("Carry changed its argument")
	}
	// What is left passes Check once it is in the schema.
	dest.Scoped = []Scoped{got}
	if ps := dest.Check(); len(ps) != 0 {
		t.Errorf("Check: %v", ps)
	}
}
