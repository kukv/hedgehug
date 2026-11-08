package schema

import (
	"errors"
	"slices"
	"testing"
)

func labelsOf[T any](xs []T, f func(T) string) []string {
	var out []string
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}

// run follows a task value through the migrations as ApplyTask does.
func run(ms []Migration, v string) string {
	for _, m := range ms {
		if m.Old == v {
			v = m.New
		}
	}
	return v
}

func TestChangeStatusNameRewritesReferences(t *testing.T) {
	t.Parallel()
	s := scopedBase(t, `workflow:
  transitions:
    todo: [in_progress]
    backlog: [todo]
  required:
    todo: [assignees]
scoped:
- where: project:`+scopedULID+`
  statuses: [todo, done]
`)
	got, ms, err := s.ChangeStatus("todo", StatusChange{Name: ptr("open"), Label: ptr("Open")})
	if err != nil {
		t.Fatal(err)
	}
	if st, ok := got.Status("open"); !ok || st.Label != "Open" || !st.Default {
		t.Errorf("open = %+v, %v", st, ok)
	}
	if _, ok := got.Status("todo"); ok {
		t.Error("todo is still defined")
	}
	tr := got.Workflow.Transitions
	if v, _ := tr.Get("open"); !slices.Equal(v, []string{"in_progress"}) {
		t.Errorf("open transitions = %v", v)
	}
	if v, _ := tr.Get("backlog"); !slices.Equal(v, []string{"open"}) {
		t.Errorf("backlog transitions = %v", v)
	}
	if _, ok := got.Workflow.Required.Get("open"); !ok {
		t.Error("required was not renamed")
	}
	if !slices.Equal(got.Scoped[0].Statuses, []string{"open", "done"}) {
		t.Errorf("scoped statuses = %v", got.Scoped[0].Statuses)
	}
	if want := []Migration{{Target: TargetStatus, Old: "todo", New: "open"}}; !slices.Equal(ms, want) {
		t.Errorf("migrations = %v", ms)
	}
	if st, _ := s.Status("todo"); st.Name != "todo" {
		t.Error("s was changed")
	}
}

func TestChangeNameTaken(t *testing.T) {
	t.Parallel()
	s := Default()
	if _, _, err := s.ChangeStatus("todo", StatusChange{Name: ptr("done")}); !errors.Is(err, ErrNameTaken) {
		t.Errorf("status: %v", err)
	}
	if _, _, err := s.ChangeKind("bug", KindChange{Name: ptr("task")}); !errors.Is(err, ErrNameTaken) {
		t.Errorf("kind: %v", err)
	}
	if _, _, _, err := s.ChangeProp("due", PropChange{Name: ptr("start")}); !errors.Is(err, ErrNameTaken) {
		t.Errorf("prop: %v", err)
	}
}

func TestChangeNameToItselfIsNoMigration(t *testing.T) {
	t.Parallel()
	_, ms, err := Default().ChangeStatus("todo", StatusChange{Name: ptr("todo"), Label: ptr("T")})
	if err != nil || len(ms) != 0 {
		t.Errorf("ms = %v, err = %v", ms, err)
	}
}

func TestChangeKindNameRewritesReferences(t *testing.T) {
	t.Parallel()
	s := scopedBase(t, `scoped:
- where: project:`+scopedULID+`
  templates:
    default: {task: bug-report}
`)
	got, ms, err := s.ChangeKind("task", KindChange{Name: ptr("item")})
	if err != nil {
		t.Fatal(err)
	}
	if k, ok := got.Kind("item"); !ok || !k.Default || !slices.Contains(k.Parents, "item") {
		t.Errorf("item = %+v, %v", k, ok)
	}
	if _, ok := got.Kind("task"); ok {
		t.Error("task is still defined")
	}
	if d := got.Scoped[0].Templates.Default; d["item"] != "bug-report" || d["task"] != "" {
		t.Errorf("templates.default = %v", d)
	}
	if want := []Migration{{Target: TargetKind, Old: "task", New: "item"}}; !slices.Equal(ms, want) {
		t.Errorf("migrations = %v", ms)
	}
}

func TestChangePropNameRewritesRequired(t *testing.T) {
	t.Parallel()
	s := scopedBase(t, `workflow:
  required:
    done: [priority]
`)
	got, ms, _, err := s.ChangeProp("priority", PropChange{Name: ptr("importance")})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Prop("importance"); !ok {
		t.Error("importance is not defined")
	}
	if v, _ := got.Workflow.Required.Get("done"); !slices.Equal(v, []string{"importance"}) {
		t.Errorf("required = %v", v)
	}
	if want := []Migration{{Target: TargetProp, Old: "priority", New: "importance"}}; !slices.Equal(ms, want) {
		t.Errorf("migrations = %v", ms)
	}
}

func TestChangePropValues(t *testing.T) {
	t.Parallel()
	got, ms, removed, err := Default().ChangeProp("priority", PropChange{Values: &[]ValueEdit{
		{Name: "none"},
		{Name: "low", From: []string{"low"}},
		{Name: "high", From: []string{"high", "urgent"}},
		{Name: "medium", From: []string{"mid"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if p, _ := got.Prop("priority"); !slices.Equal(p.Values, []string{"none", "low", "high", "medium"}) {
		t.Errorf("values = %v", p.Values)
	}
	want := []Migration{
		{Target: TargetValue, Prop: "priority", Old: "urgent", New: "high"},
		{Target: TargetValue, Prop: "priority", Old: "mid", New: "medium"},
	}
	if !slices.Equal(ms, want) {
		t.Errorf("migrations = %v", ms)
	}
	if len(removed) != 0 {
		t.Errorf("removed = %v", removed)
	}
	if p, _ := Default().Prop("priority"); !slices.Equal(p.Values, []string{"low", "mid", "high", "urgent"}) {
		t.Error("the schema was changed")
	}
}

func TestChangePropValuesRemoved(t *testing.T) {
	t.Parallel()
	got, ms, removed, err := Default().ChangeProp("priority", PropChange{Values: &[]ValueEdit{
		{Name: "high", From: []string{"high"}}, {Name: "low", From: []string{"low"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if p, _ := got.Prop("priority"); !slices.Equal(p.Values, []string{"high", "low"}) {
		t.Errorf("values = %v", p.Values)
	}
	if len(ms) != 0 || !slices.Equal(removed, []string{"mid", "urgent"}) {
		t.Errorf("migrations = %v, removed = %v", ms, removed)
	}
}

func TestChangePropValuesSwapAndChain(t *testing.T) {
	t.Parallel()
	// low and high trade names: the order of the moves, and a parking
	// place, keep each task value from being moved twice
	_, ms, _, err := Default().ChangeProp("priority", PropChange{Values: &[]ValueEdit{
		{Name: "high", From: []string{"low"}},
		{Name: "low", From: []string{"high"}},
		{Name: "mid", From: []string{"mid"}},
		{Name: "urgent", From: []string{"urgent"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for start, want := range map[string]string{"low": "high", "high": "low", "mid": "mid", "urgent": "urgent"} {
		if got := run(ms, start); got != want {
			t.Errorf("swap: %s ended as %s, want %s (migrations %v)", start, got, want, ms)
		}
	}

	// each takes the name the one before held
	_, ms, _, err = Default().ChangeProp("priority", PropChange{Values: &[]ValueEdit{
		{Name: "x", From: []string{"low"}},
		{Name: "low", From: []string{"mid"}},
		{Name: "mid", From: []string{"high"}},
		{Name: "urgent", From: []string{"urgent"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for start, want := range map[string]string{"low": "x", "mid": "low", "high": "mid", "urgent": "urgent"} {
		if got := run(ms, start); got != want {
			t.Errorf("chain: %s ended as %s, want %s (migrations %v)", start, got, want, ms)
		}
	}
}

func TestChangePropValuesInvalid(t *testing.T) {
	t.Parallel()
	for name, vs := range map[string][]ValueEdit{
		"empty name":      {{Name: ""}},
		"repeated name":   {{Name: "a"}, {Name: "a"}},
		"from twice":      {{Name: "a", From: []string{"low"}}, {Name: "b", From: []string{"low"}}},
		"unknown from":    {{Name: "a", From: []string{"nope"}}},
		"new name taken":  {{Name: "low"}, {Name: "mid", From: []string{"mid"}}},
		"from repeated":   {{Name: "a", From: []string{"low", "low"}}},
		"name of a goner": {{Name: "urgent", From: []string{"low"}}},
	} {
		if _, _, _, err := Default().ChangeProp("priority", PropChange{Values: &vs}); !errors.Is(err, ErrValueInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if _, _, _, err := Default().ChangeProp("start", PropChange{Values: &[]ValueEdit{{Name: "x"}}}); !errors.Is(err, ErrNotEnum) {
		t.Errorf("not enum: err = %v", err)
	}
}

func TestChangeKindColor(t *testing.T) {
	t.Parallel()
	got, _, err := Default().ChangeKind("bug", KindChange{Color: ptr("teal")})
	if err != nil {
		t.Fatal(err)
	}
	if k, _ := got.Kind("bug"); k.Color != "teal" {
		t.Errorf("color = %q", k.Color)
	}
	if got, _, err = got.ChangeKind("bug", KindChange{Color: ptr("")}); err != nil {
		t.Fatal(err)
	}
	if k, _ := got.Kind("bug"); k.Color != "" {
		t.Errorf("color = %q", k.Color)
	}
}

func TestDefaultLabels(t *testing.T) {
	t.Parallel()
	s := Dev()
	if got := labelsOf(s.Statuses, func(x Status) string { return x.Label }); !slices.Equal(got,
		[]string{"Backlog", "Todo", "On hold", "In progress", "Review", "Waiting", "Done", "Canceled"}) {
		t.Errorf("statuses = %v", got)
	}
	if got := labelsOf(s.Kinds, func(x Kind) string { return x.Label }); !slices.Equal(got, []string{"Project", "Epic", "Story", "Task", "Bug"}) {
		t.Errorf("kinds = %v", got)
	}
	for name, want := range map[string][2]string{"duplicates": {"Duplicates", "Duplicated by"}, "causes": {"Causes", "Caused by"}, "relates": {"Relates to", ""}} {
		if p, _ := s.Prop(name); p.Label != want[0] || p.Inverse != want[1] {
			t.Errorf("%s = %q / %q", name, p.Label, p.Inverse)
		}
	}
	if p, _ := s.Prop("priority"); p.Label != "優先度" {
		t.Errorf("priority label = %q", p.Label)
	}
}
