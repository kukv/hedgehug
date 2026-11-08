package schema

import (
	"errors"
	"flag"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"go.yaml.in/yaml/v4"
)

var update = flag.Bool("update", false, "write the golden files")

func golden(t *testing.T, path string, got []byte) {
	t.Helper()
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRemoveStatusDropsReferences(t *testing.T) {
	t.Parallel()
	s := scopedBase(t, `workflow:
  transitions:
    backlog: [todo, on_hold]
    on_hold: [todo]
    waiting: [on_hold]
  required:
    on_hold: [assignees]
scoped:
- where: project:`+scopedULID+`
  statuses: [todo, on_hold, in_progress, done, canceled]
`)
	got, err := s.RemoveStatus("on_hold")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Status("on_hold"); ok {
		t.Error("on_hold is still defined")
	}
	tr := got.Workflow.Transitions
	if v, _ := tr.Get("backlog"); !slices.Equal(v, []string{"todo"}) {
		t.Errorf("backlog = %v", v)
	}
	if _, ok := tr.Get("on_hold"); ok {
		t.Error("the key on_hold stays")
	}
	if _, ok := tr.Get("waiting"); ok {
		t.Error("waiting, left with no target, stays")
	}
	if _, ok := got.Workflow.Required.Get("on_hold"); ok {
		t.Error("required on_hold stays")
	}
	if !slices.Equal(got.Scoped[0].Statuses, []string{"todo", "in_progress", "done", "canceled"}) {
		t.Errorf("scoped statuses = %v", got.Scoped[0].Statuses)
	}
	if _, ok := s.Status("on_hold"); !ok {
		t.Error("s was changed")
	}
}

func TestRemoveKindRefusesAKindLeftWithNoParent(t *testing.T) {
	t.Parallel()
	s := scopedBase(t, "")
	s.Kinds = append(s.Kinds, Kind{Name: "sub", Role: RoleWork, Parents: []string{"epic"}})
	_, err := s.RemoveKind("epic")
	var pe *ParentsEmptyError
	if !errors.As(err, &pe) || !slices.Equal(pe.Kinds, []string{"sub"}) {
		t.Fatalf("err = %v", err)
	}
}

func TestRemoveKindDropsParentsAndTemplateDefaults(t *testing.T) {
	t.Parallel()
	s := scopedBase(t, `scoped:
- where: project:`+scopedULID+`
  templates:
    default: {epic: plan, task: work}
`)
	got, err := s.RemoveKind("epic")
	if err != nil {
		t.Fatal(err)
	}
	story, _ := got.Kind("story")
	if !slices.Equal(story.Parents, []string{Root, "project"}) {
		t.Errorf("story parents = %v", story.Parents)
	}
	if _, ok := got.Scoped[0].Templates.Default["epic"]; ok {
		t.Error("the default template of epic stays")
	}
}

func TestRemovePropDropsRequired(t *testing.T) {
	t.Parallel()
	s := scopedBase(t, "workflow:\n  required:\n    done: [points]\n    in_progress: [points, assignees]\n")
	got, err := s.RemoveProp("points")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Workflow.Required.Get("done"); ok {
		t.Error("done, left with nothing, stays")
	}
	if v, _ := got.Workflow.Required.Get("in_progress"); !slices.Equal(v, []string{"assignees"}) {
		t.Errorf("in_progress = %v", v)
	}
}

func TestChangeStatusDefaultMovesIt(t *testing.T) {
	t.Parallel()
	got, _, err := Default().ChangeStatus("backlog", StatusChange{Default: true})
	if err != nil {
		t.Fatal(err)
	}
	if d, _ := got.DefaultStatus(CategoryTodo); d.Name != "backlog" {
		t.Errorf("default of todo = %s", d.Name)
	}
	if ps := got.Check(); len(ps) > 0 {
		t.Errorf("Check = %v", ps)
	}
}

func TestChangeKindDefaultRefusesAContainer(t *testing.T) {
	t.Parallel()
	if _, _, err := Default().ChangeKind("epic", KindChange{Default: true}); !errors.Is(err, ErrDefaultContainer) {
		t.Errorf("err = %v", err)
	}
}

func TestChangeKindParentsAnywhere(t *testing.T) {
	t.Parallel()
	var anywhere []string
	got, _, err := Default().ChangeKind("bug", KindChange{Parents: &anywhere})
	if err != nil {
		t.Fatal(err)
	}
	if k, _ := got.Kind("bug"); k.Parents != nil {
		t.Errorf("parents = %v", k.Parents)
	}
}

func TestChangePropRefusesTaskRefFieldsOnOthers(t *testing.T) {
	t.Parallel()
	multi := true
	if _, _, _, err := Default().ChangeProp("priority", PropChange{Multi: &multi}); !errors.Is(err, ErrNotTaskRef) {
		t.Errorf("err = %v", err)
	}
}

func TestOrderProps(t *testing.T) {
	t.Parallel()
	s := Default() // priority start due estimate points | duplicates causes relates
	got, err := s.OrderProps([]string{"relates", "duplicates", "causes"})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, p := range got.Props {
		names = append(names, p.Name)
	}
	want := []string{"priority", "start", "due", "estimate", "points", "relates", "duplicates", "causes"}
	if !slices.Equal(names, want) {
		t.Errorf("props = %v", names)
	}
	if _, err := s.OrderProps([]string{"relates", "duplicates"}); !errors.Is(err, ErrOrderStale) {
		t.Errorf("a missing name: err = %v", err)
	}
	if _, err := s.OrderProps([]string{"priority", "relates", "duplicates", "causes"}); !errors.Is(err, ErrOrderStale) {
		t.Errorf("a mix of the two groups: err = %v", err)
	}
}

func TestOrderStatusesStale(t *testing.T) {
	t.Parallel()
	if _, err := Default().OrderStatuses([]string{"todo", "done"}); !errors.Is(err, ErrOrderStale) {
		t.Errorf("err = %v", err)
	}
}

func TestNotDefined(t *testing.T) {
	t.Parallel()
	s := Default()
	for name, err := range map[string]error{
		"status": func() error { _, err := s.RemoveStatus("x"); return err }(),
		"kind":   func() error { _, _, err := s.ChangeKind("x", KindChange{}); return err }(),
		"prop":   func() error { _, err := s.RemoveProp("x"); return err }(),
	} {
		if !errors.Is(err, ErrNotDefined) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestAddedProblemsIgnoresThePath(t *testing.T) {
	t.Parallel()
	s := Default()
	s.Statuses = append(s.Statuses, Status{Name: "todo", Category: CategoryTodo}) // status_twice at statuses[7]
	before := s.Check()
	got, err := s.RemoveStatus("backlog") // the second todo moves to statuses[6]
	if err != nil {
		t.Fatal(err)
	}
	if ps := got.AddedProblems(before); len(ps) > 0 {
		t.Errorf("AddedProblems = %v", ps)
	}
	if ps := got.NewProblems(before); len(ps) == 0 {
		t.Error("NewProblems, which compares the paths, finds nothing: the test does not shift the index")
	}
	if _, err := s.OrderStatuses([]string{"backlog", "todo", "on_hold", "in_progress", "waiting", "done", "canceled", "todo"}); !errors.Is(err, ErrOrderStale) {
		t.Errorf("a schema with a name twice is ordered: err = %v", err)
	}
}

func TestTaskUses(t *testing.T) {
	t.Parallel()
	var n yaml.Node
	if err := n.Encode([]string{"backend", "infra"}); err != nil {
		t.Fatal(err)
	}
	tk := &model.Task{Status: "todo", Kind: "task", Props: model.Fields{{Key: "area", Value: &n}}}
	cases := []struct {
		target     Target
		prop, name string
		want       bool
	}{
		{TargetStatus, "", "todo", true},
		{TargetStatus, "", "done", false},
		{TargetKind, "", "task", true},
		{TargetProp, "", "area", true},
		{TargetProp, "", "points", false},
		{TargetValue, "area", "infra", true},
		{TargetValue, "area", "ops", false},
	}
	for _, c := range cases {
		if got := TaskUses(tk, c.target, c.prop, c.name); got != c.want {
			t.Errorf("TaskUses(%s %s %s) = %v", c.target, c.prop, c.name, got)
		}
	}
}

func TestEditKeepsComments(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("testdata/notes.yaml")
	if err != nil {
		t.Fatal(err)
	}
	s, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	s, err = s.OrderStatuses([]string{"done", "todo", "doing"})
	if err != nil {
		t.Fatal(err)
	}
	s = s.AddStatus(Status{Name: "review", Category: CategoryDoing, Color: "teal"})
	if s, _, err = s.ChangeStatus("done", StatusChange{Label: ptr("完了")}); err != nil {
		t.Fatal(err)
	}
	if s, err = s.OrderProps([]string{"points", "priority"}); err != nil {
		t.Fatal(err)
	}
	out, err := s.Encode()
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "testdata/edit-notes.golden.yaml", out)
}

func ptr[T any](v T) *T { return &v }

func encoded(t *testing.T, s *Schema) string {
	t.Helper()
	b, err := s.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const scopedWorkflow = `workflow:
  transitions:
    todo: [on_hold, done]
    on_hold: [todo]
    waiting: [on_hold]
  required:
    on_hold: [points]
    done: [points]
scoped:
- where: project:` + scopedULID + `
  statuses: [todo, on_hold, done]
  workflow:
    transitions:
      todo: [on_hold, done]
      on_hold: [todo]
      waiting: [on_hold]
    required:
      on_hold: [points]
      done: [points, assignees]
`

func TestRemoveStatusOnAScopedWorkflow(t *testing.T) {
	t.Parallel()
	s := scopedBase(t, scopedWorkflow)
	before := encoded(t, s)
	got, err := s.RemoveStatus("on_hold")
	if err != nil {
		t.Fatal(err)
	}
	for name, w := range map[string]*Workflow{"store": &got.Workflow, "scoped": got.Scoped[0].Workflow} {
		if v, _ := w.Transitions.Get("todo"); !slices.Equal(v, []string{"done"}) {
			t.Errorf("%s: todo targets = %v", name, v)
		}
		if _, ok := w.Transitions.Get("on_hold"); ok {
			t.Errorf("%s: the key on_hold stays", name)
		}
		if _, ok := w.Transitions.Get("waiting"); ok {
			t.Errorf("%s: waiting, left with no target, stays", name)
		}
		if _, ok := w.Required.Get("on_hold"); ok {
			t.Errorf("%s: required on_hold stays", name)
		}
		if _, ok := w.Required.Get("done"); !ok {
			t.Errorf("%s: required done went", name)
		}
	}
	if encoded(t, s) != before {
		t.Error("s was changed")
	}
}

func TestRemoveStatusEmptiesTheKeys(t *testing.T) {
	t.Parallel()
	s := scopedBase(t, "workflow:\n  transitions:\n    waiting: [on_hold]\n  required:\n    on_hold: [points]\n")
	got, err := s.RemoveStatus("on_hold")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Workflow.Transitions) != 0 || len(got.Workflow.Required) != 0 {
		t.Errorf("workflow = %+v", got.Workflow)
	}
}

func TestRemovePropOnAScopedWorkflow(t *testing.T) {
	t.Parallel()
	s := scopedBase(t, scopedWorkflow)
	before := encoded(t, s)
	got, err := s.RemoveProp("points")
	if err != nil {
		t.Fatal(err)
	}
	for name, w := range map[string]*Workflow{"store": &got.Workflow, "scoped": got.Scoped[0].Workflow} {
		if _, ok := w.Required.Get("on_hold"); ok {
			t.Errorf("%s: on_hold, left with nothing, stays", name)
		}
	}
	if v, _ := got.Scoped[0].Workflow.Required.Get("done"); !slices.Equal(v, []string{"assignees"}) {
		t.Errorf("scoped done = %v", v)
	}
	if _, ok := got.Workflow.Required.Get("done"); ok {
		t.Error("store done, left with nothing, stays")
	}
	if encoded(t, s) != before {
		t.Error("s was changed")
	}
}

func TestRemoveKindLeavesSUnchanged(t *testing.T) {
	t.Parallel()
	s := scopedBase(t, `scoped:
- where: project:`+scopedULID+`
  templates:
    default: {epic: plan, task: work}
`)
	before := encoded(t, s)
	if _, err := s.RemoveKind("epic"); err != nil {
		t.Fatal(err)
	}
	if encoded(t, s) != before {
		t.Error("s was changed")
	}
}

func TestRemoveKindLeavesNoEmptyTemplates(t *testing.T) {
	t.Parallel()
	s := scopedBase(t, `scoped:
- where: project:`+scopedULID+`
  statuses: [todo, done]
  templates:
    default: {epic: plan}
`)
	got, err := s.RemoveKind("epic")
	if err != nil {
		t.Fatal(err)
	}
	if got.Scoped[0].Templates != nil {
		t.Errorf("templates = %+v", got.Scoped[0].Templates)
	}
	if out := encoded(t, got); strings.Contains(out, "templates") {
		t.Errorf("templates stay:\n%s", out)
	}

	// only keeps the templates of the override alive.
	s = scopedBase(t, `scoped:
- where: project:`+scopedULID+`
  templates:
    only: [work]
    default: {epic: plan}
`)
	if got, err = s.RemoveKind("epic"); err != nil {
		t.Fatal(err)
	}
	if tm := got.Scoped[0].Templates; tm == nil || !slices.Equal(tm.Only, []string{"work"}) || len(tm.Default) != 0 {
		t.Errorf("templates = %+v", tm)
	}
	if out := encoded(t, got); strings.Contains(out, "default: {") {
		t.Errorf("an empty default stays:\n%s", out)
	}
}

func TestRemoveStatusLeavesSUnchanged(t *testing.T) {
	t.Parallel()
	s := scopedBase(t, scopedWorkflow)
	before := encoded(t, s)
	if _, err := s.RemoveStatus("done"); err != nil {
		t.Fatal(err)
	}
	if encoded(t, s) != before {
		t.Error("RemoveStatus changed s")
	}
}

func TestChangeStatusFields(t *testing.T) {
	t.Parallel()
	s := Default()
	cat := CategoryDoing
	got, _, err := s.ChangeStatus("backlog", StatusChange{Category: &cat})
	if err != nil {
		t.Fatal(err)
	}
	if st, _ := got.Status("backlog"); st.Category != CategoryDoing {
		t.Errorf("category = %s", st.Category)
	}
	if st, _ := s.Status("backlog"); st.Category == CategoryDoing {
		t.Error("s was changed")
	}
	// "" clears the label and the color; nil leaves them.
	s, _, err = s.ChangeStatus("done", StatusChange{Label: ptr("完了"), Color: ptr("teal")})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name         string
		ch           StatusChange
		label, color string
	}{
		{"nil leaves", StatusChange{}, "完了", "teal"},
		{"clear label", StatusChange{Label: ptr("")}, "", "teal"},
		{"clear color", StatusChange{Color: ptr("")}, "完了", ""},
	} {
		g, _, err := s.ChangeStatus("done", c.ch)
		if err != nil {
			t.Fatal(err)
		}
		if st, _ := g.Status("done"); st.Label != c.label || st.Color != c.color {
			t.Errorf("%s: label %q color %q", c.name, st.Label, st.Color)
		}
	}
}

func TestChangeKindFields(t *testing.T) {
	t.Parallel()
	s := Default()
	role := RoleContainer
	got, _, err := s.ChangeKind("task", KindChange{Role: &role})
	if err != nil {
		t.Fatal(err)
	}
	if k, _ := got.Kind("task"); k.Role != RoleContainer {
		t.Errorf("role = %s", k.Role)
	}
	if k, _ := s.Kind("task"); k.Role != RoleWork {
		t.Error("s was changed")
	}
	ps := []string{"epic", "story"}
	if got, _, err = s.ChangeKind("bug", KindChange{Parents: &ps}); err != nil {
		t.Fatal(err)
	}
	if k, _ := got.Kind("bug"); !slices.Equal(k.Parents, ps) {
		t.Errorf("parents = %v", k.Parents)
	}
	ps[0] = "x" // the schema has its own copy
	if k, _ := got.Kind("bug"); k.Parents[0] != "epic" {
		t.Error("parents share the slice of the caller")
	}
}

func TestOrderKinds(t *testing.T) {
	t.Parallel()
	s := Default()
	var names []string
	for _, k := range s.Kinds {
		names = append(names, k.Name)
	}
	slices.Reverse(names)
	got, err := s.OrderKinds(names)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, k := range got.Kinds {
		out = append(out, k.Name)
	}
	if !slices.Equal(out, names) {
		t.Errorf("kinds = %v", out)
	}
	if _, err := s.OrderKinds(names[1:]); !errors.Is(err, ErrOrderStale) {
		t.Errorf("a missing name: err = %v", err)
	}
	if _, err := s.OrderKinds(append(slices.Clone(names[1:]), names[1])); !errors.Is(err, ErrOrderStale) {
		t.Errorf("a name twice: err = %v", err)
	}
}

func TestAddPropRefusesBadValues(t *testing.T) {
	t.Parallel()
	for name, vs := range map[string][]string{"repeated": {"a", "b", "a"}, "empty": {"a", ""}} {
		if _, err := Default().AddProp(Prop{Name: "area", Type: TypeEnum, Values: vs}); !errors.Is(err, ErrValueInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	got, err := Default().AddProp(Prop{Name: "area", Type: TypeEnum, Values: []string{"a", "b"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Prop("area"); !ok {
		t.Error("area is not added")
	}
}

func TestRemovalLeavesNoEmptyWorkflow(t *testing.T) {
	t.Parallel()
	store := scopedBase(t, "workflow:\n  transitions:\n    waiting: [on_hold]\n  required:\n    on_hold: [points]\n")
	got, err := store.RemoveStatus("on_hold")
	if err != nil {
		t.Fatal(err)
	}
	if out := encoded(t, got); strings.Contains(out, "workflow") {
		t.Errorf("an empty store workflow stays:\n%s", out)
	}

	scoped := scopedBase(t, `scoped:
- where: project:`+scopedULID+`
  statuses: [todo, on_hold, done]
  workflow:
    transitions:
      waiting: [on_hold]
    required:
      on_hold: [points]
`)
	got, err = scoped.RemoveStatus("on_hold")
	if err != nil {
		t.Fatal(err)
	}
	if got.Scoped[0].Workflow != nil {
		t.Errorf("workflow = %+v", got.Scoped[0].Workflow)
	}
	if out := encoded(t, got); strings.Contains(out, "workflow") {
		t.Errorf("an empty override workflow stays:\n%s", out)
	}

	prop := scopedBase(t, "workflow:\n  required:\n    done: [points]\nscoped:\n- where: project:"+scopedULID+"\n  workflow:\n    required:\n      done: [points]\n")
	got, err = prop.RemoveProp("points")
	if err != nil {
		t.Fatal(err)
	}
	if out := encoded(t, got); strings.Contains(out, "workflow") {
		t.Errorf("RemoveProp leaves a workflow:\n%s", out)
	}
}

func TestUntouchedWorkflowRoundTrips(t *testing.T) {
	t.Parallel()
	s := scopedBase(t, "workflow:\n  # keep me\n  transitions:\n    todo: [done]\n")
	b, err := s.Encode()
	if err != nil {
		t.Fatal(err)
	}
	s2, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	if got := encoded(t, s2); got != string(b) || !strings.Contains(got, "# keep me") {
		t.Errorf("round trip changed:\n%s", got)
	}
}
