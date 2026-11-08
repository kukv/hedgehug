package schema

import (
	"errors"
	"flag"
	"os"
	"slices"
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
	got, err := Default().ChangeStatus("backlog", StatusChange{Default: true})
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
	if _, err := Default().ChangeKind("epic", KindChange{Default: true}); !errors.Is(err, ErrDefaultContainer) {
		t.Errorf("err = %v", err)
	}
}

func TestChangeKindParentsAnywhere(t *testing.T) {
	t.Parallel()
	var anywhere []string
	got, err := Default().ChangeKind("bug", KindChange{Parents: &anywhere})
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
	if _, err := Default().ChangeProp("priority", PropChange{Multi: &multi}); !errors.Is(err, ErrNotTaskRef) {
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

func TestValues(t *testing.T) {
	t.Parallel()
	s, err := Default().AddValue("priority", "none")
	if err != nil {
		t.Fatal(err)
	}
	if s, err = s.OrderValues("priority", []string{"none", "low", "mid", "high", "urgent"}); err != nil {
		t.Fatal(err)
	}
	if s, err = s.RemoveValue("priority", "urgent"); err != nil {
		t.Fatal(err)
	}
	p, _ := s.Prop("priority")
	if !slices.Equal(p.Values, []string{"none", "low", "mid", "high"}) {
		t.Errorf("values = %v", p.Values)
	}
	if _, err := s.AddValue("start", "x"); !errors.Is(err, ErrNotEnum) {
		t.Errorf("err = %v", err)
	}
	if _, err := s.RemoveValue("priority", "nope"); !errors.Is(err, ErrNotDefined) {
		t.Errorf("err = %v", err)
	}
}

func TestNotDefined(t *testing.T) {
	t.Parallel()
	s := Default()
	for name, err := range map[string]error{
		"status": func() error { _, err := s.RemoveStatus("x"); return err }(),
		"kind":   func() error { _, err := s.ChangeKind("x", KindChange{}); return err }(),
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
	if s, err = s.ChangeStatus("done", StatusChange{Label: ptr("完了")}); err != nil {
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
