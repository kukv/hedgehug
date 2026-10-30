package schema

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
	"go.yaml.in/yaml/v4"
)

func taskWith(t *testing.T, props string) *model.Task {
	t.Helper()
	src := "---\nid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nkind: task\ntitle: x\nstatus: todo\n"
	if props != "" {
		src += "props:\n" + props
	}
	task, err := model.DecodeTask([]byte(src + "---\n"))
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func codes(vs []Violation) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = v.Code + ":" + v.Field
	}
	return out
}

func TestValidateTaskOK(t *testing.T) {
	s := Default()
	s.Props = append(s.Props,
		Prop{Name: "area", Type: TypeMultiEnum, Values: []string{"backend", "frontend"}},
		Prop{Name: "owner", Type: TypeUser},
		Prop{Name: "spec", Type: TypeURL},
		Prop{Name: "flag", Type: TypeBool},
		Prop{Name: "at", Type: TypeDatetime},
		Prop{Name: "note", Type: TypeText},
		Prop{Name: "blocker", Type: TypeTaskRef},
	)
	task := taskWith(t, `  priority: high
  start: 2026-11-03
  estimate: 1h30m
  points: 2.5
  duplicates: [01JBZ8K3QW7M2X9V4T6R5N8P0B]
  area: [backend]
  owner: github.com/yourname
  spec: https://example.com/spec
  flag: true
  at: 2026-11-03T10:00:00+09:00
  note: anything
  blocker: 01JBZ8K3QW7M2X9V4T6R5N8P0C
  custom: 1
`)
	vs, undefined := ValidateTask(task, s.For(nil))
	if len(vs) != 0 {
		t.Errorf("violations: %v", codes(vs))
	}
	if len(undefined) != 1 || undefined[0] != "custom" {
		t.Errorf("undefined = %v, want [custom]", undefined)
	}
}

func TestValidateTaskViolations(t *testing.T) {
	s := Default()
	s.Props = append(s.Props,
		Prop{Name: "area", Type: TypeMultiEnum, Values: []string{"backend"}},
		Prop{Name: "owner", Type: TypeUser},
		Prop{Name: "spec", Type: TypeURL},
		Prop{Name: "flag", Type: TypeBool},
		Prop{Name: "blocker", Type: TypeTaskRef},
	)
	for _, tc := range []struct{ props, want string }{
		{"  priority: none\n", "bad_value:priority"},
		{"  start: 2026-13-01\n", "bad_value:start"},
		{"  estimate: 1d\n", "bad_value:estimate"},
		{"  points: many\n", "bad_value:points"},
		{"  points: NaN\n", "bad_value:points"},
		{"  points: .nan\n", "bad_value:points"},
		{"  points: Inf\n", "bad_value:points"},
		{"  points: -Inf\n", "bad_value:points"},
		{"  points: +Inf\n", "bad_value:points"},
		{"  points: .inf\n", "bad_value:points"},
		{"  duplicates: 01JBZ8K3QW7M2X9V4T6R5N8P0B\n", "bad_value:duplicates"}, // multi needs a list
		{"  duplicates: [nope]\n", "bad_value:duplicates"},
		{"  area: [ops]\n", "bad_value:area"},
		{"  owner: yourname\n", "bad_value:owner"},
		{"  spec: not a url\n", "bad_value:spec"},
		{"  flag: maybe\n", "bad_value:flag"},
		{"  blocker: [01JBZ8K3QW7M2X9V4T6R5N8P0C]\n", "bad_value:blocker"}, // single needs a scalar
	} {
		vs, _ := ValidateTask(taskWith(t, tc.props), s.For(nil))
		if got := codes(vs); len(got) != 1 || got[0] != tc.want {
			t.Errorf("%q: violations = %v, want [%s]", tc.props, got, tc.want)
		}
	}

	task := taskWith(t, "")
	task.Kind, task.Status, task.Title = "nope", "nope", ""
	vs, _ := ValidateTask(task, s.For(nil))
	want := map[string]bool{"unknown_kind:kind": true, "unknown_status:status": true, "missing_title:title": true}
	if got := codes(vs); len(got) != 3 || !want[got[0]] || !want[got[1]] || !want[got[2]] {
		t.Errorf("violations = %v", got)
	}
}

func TestValidateTaskNumberOK(t *testing.T) {
	for _, v := range []string{"1.5", "-3", "1e3"} {
		vs, _ := ValidateTask(taskWith(t, "  points: "+v+"\n"), Default().For(nil))
		if len(vs) != 0 {
			t.Errorf("points: %s: violations = %v, want none", v, codes(vs))
		}
	}
}

func TestValidateTaskRank(t *testing.T) {
	task := taskWith(t, "")
	task.Rank = "a0"
	if vs, _ := ValidateTask(task, Default().For(nil)); len(vs) != 0 {
		t.Errorf("valid rank: violations %v", codes(vs))
	}
	task.Rank = "!"
	vs, _ := ValidateTask(task, Default().For(nil))
	if got := codes(vs); len(got) != 1 || got[0] != "bad_value:rank" {
		t.Errorf("bad rank: violations %v, want [bad_value:rank]", got)
	}
}

func TestValidatePlacement(t *testing.T) {
	e := Default().For(nil)
	for _, tc := range []struct {
		kind, parent string
		ok           bool
	}{
		{"task", "", true}, // root
		{"task", "task", true},
		{"epic", "story", false},
		{"bug", "task", false},
		{"project", "epic", true}, // parents omitted: anywhere
		{"nope", "", false},
	} {
		vs := ValidatePlacement(tc.kind, tc.parent, e)
		if (len(vs) == 0) != tc.ok {
			t.Errorf("ValidatePlacement(%q, %q) = %v, want ok=%v", tc.kind, tc.parent, codes(vs), tc.ok)
		}
	}
}

func TestValidateTransition(t *testing.T) {
	s := Default()
	s.Workflow = Workflow{
		Transitions: Lists{{"backlog", []string{"todo"}}},
		Required:    Lists{{"in_progress", []string{"assignees"}}, {"done", []string{"points"}}},
	}
	e := s.For(nil)
	task := taskWith(t, "")

	if vs := ValidateTransition("backlog", "done", task, e); len(vs) == 0 || vs[0].Code != CodeTransition {
		t.Errorf("backlog→done = %v", codes(vs))
	}
	if vs := ValidateTransition("todo", "in_progress", task, e); len(vs) != 1 || vs[0].Field != "assignees" {
		t.Errorf("todo→in_progress without assignees = %v", codes(vs))
	}
	task.Assignees = []string{"github.com/me"}
	if vs := ValidateTransition("todo", "in_progress", task, e); len(vs) != 0 {
		t.Errorf("todo→in_progress with assignees = %v", codes(vs))
	}
	if vs := ValidateTransition("todo", "done", task, e); len(vs) != 1 || vs[0].Field != "points" {
		t.Errorf("todo→done without points = %v", codes(vs))
	}
	task.Props.Set("points", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: "3"})
	if vs := ValidateTransition("todo", "done", task, e); len(vs) != 0 {
		t.Errorf("todo→done with points = %v", codes(vs))
	}
	task.Assignees = nil
	if vs := ValidateTransition("in_progress", "in_progress", task, e); len(vs) != 0 {
		t.Errorf("staying must be allowed: %v", codes(vs))
	}
	if vs := ValidateTransition("todo", "nope", task, e); len(vs) != 1 || vs[0].Code != CodeUnknownStatus {
		t.Errorf("to an unknown status = %v", codes(vs))
	}
}

func TestValidateWrite(t *testing.T) {
	s := Default()
	s.Workflow = Workflow{
		Transitions: Lists{{"backlog", []string{"todo"}}},
		Required:    Lists{{"in_progress", []string{"assignees"}}},
	}
	e := s.For(nil)

	for _, tc := range []struct {
		name          string
		old           func(*model.Task) // nil: a new task
		cur           func(*model.Task)
		oldParentKind string
		parentKind    string
		want          []string
	}{
		{"new and valid", nil, func(*model.Task) {}, "", "", nil},
		{"new with a bad value", nil, func(t *model.Task) {
			t.Props.Set("priority", &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "none"})
		}, "", "", []string{"bad_value:priority"}},
		{"new needs required fields", nil, func(t *model.Task) { t.Status = "in_progress" }, "", "", []string{"required:assignees"}},
		{"new in a wrong place", nil, func(t *model.Task) { t.Kind = "bug" }, "", "task", []string{"placement:parent"}},
		{"unknown status is reported once", nil, func(t *model.Task) { t.Status = "nope" }, "", "", []string{"unknown_status:status"}},
		{"edit brings a violation", func(*model.Task) {}, func(t *model.Task) { t.Title = "" }, "", "", []string{"missing_title:title"}},
		{"old violation is left out", func(t *model.Task) { t.Kind = "nope" }, func(t *model.Task) {
			t.Kind = "nope"
			t.Title = "renamed"
		}, "", "", nil},
		{"old placement violation is subtracted when the parent did not change", func(t *model.Task) { t.Kind = "bug" }, func(t *model.Task) {
			t.Kind = "bug"
			t.Title = "renamed"
		}, "task", "task", nil},
		{"a move brings a placement violation", func(t *model.Task) { t.Kind = "bug" }, func(t *model.Task) { t.Kind = "bug" }, "", "task", []string{"placement:parent"}},
		{"transition not allowed", func(t *model.Task) { t.Status = "backlog" }, func(t *model.Task) { t.Status = "done" }, "", "", []string{"transition:status"}},
		{"same status skips the workflow", func(t *model.Task) { t.Status = "in_progress" }, func(t *model.Task) {
			t.Status = "in_progress"
			t.Title = "renamed"
		}, "", "", nil},
	} {
		var old *model.Task
		var oldE *Effective
		if tc.old != nil {
			old = taskWith(t, "")
			tc.old(old)
			oldE = e
		}
		cur := taskWith(t, "")
		tc.cur(cur)
		got := codes(ValidateWrite(old, tc.oldParentKind, oldE, cur, tc.parentKind, e, false))
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s: violations = %v, want %v", tc.name, got, tc.want)
		}
	}
}
