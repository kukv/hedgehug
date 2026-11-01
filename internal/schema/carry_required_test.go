package schema

import (
	"slices"
	"testing"
)

// A required list whose fields are all gone is dropped with its key, as an
// empty transitions list is, and told of as "required <status>".
func TestCarryDropsAnEmptiedRequiredList(t *testing.T) {
	t.Parallel()
	dest := Default()
	sc := Scoped{
		Where: "project:01JBZ8K3QW7M2X9V4T6R5N8P0A",
		Workflow: &Workflow{
			Required: Lists{{Key: "todo", Values: []string{"gone", "also_gone"}}, {Key: "done", Values: []string{"assignees"}}, {Key: "canceled"}},
		},
	}
	got, dropped := dest.Carry(sc, func(string) bool { return true })
	if _, ok := got.Workflow.Required.Get("todo"); ok {
		t.Errorf("an emptied list is kept: %+v", got.Workflow.Required)
	}
	if v, ok := got.Workflow.Required.Get("done"); !ok || !slices.Equal(v, []string{"assignees"}) {
		t.Errorf("done required = %v, %v", v, ok)
	}
	if _, ok := got.Workflow.Required.Get("canceled"); !ok {
		t.Error("a list that was empty from the start is dropped")
	}
	want := []string{"prop gone", "prop also_gone", "required todo"}
	if !slices.Equal(dropped, want) {
		t.Errorf("dropped = %v, want %v", dropped, want)
	}
}
