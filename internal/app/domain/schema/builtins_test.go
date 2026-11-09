package schema

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
)

func mustTask(t *testing.T, front string) *model.Task {
	t.Helper()
	task, err := model.DecodeTask([]byte("---\nulid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nkind: task\ntitle: x\nstatus: todo\n" + front + "---\n"))
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func withLists(priorities, labels []string) *Schema {
	s := Default()
	s.Priorities, s.Labels = priorities, labels
	return s
}

func TestBuiltinsTable(t *testing.T) {
	t.Parallel()
	want := map[string]PropType{
		"priority": TypeEnum, "start": TypeDate, "due": TypeDate,
		"estimate": TypeDuration, "points": TypeNumber, "labels": TypeMultiEnum,
	}
	if len(Builtins) != len(want) {
		t.Fatalf("Builtins = %v", Builtins)
	}
	for _, b := range Builtins {
		if want[b.Name] != b.Type {
			t.Errorf("%s is %s", b.Name, b.Type)
		}
		if got, ok := BuiltinOf(b.Name); !ok || got != b {
			t.Errorf("BuiltinOf(%s) = %v, %v", b.Name, got, ok)
		}
		if !slices.Contains(model.TaskKeys(), b.Name) {
			t.Errorf("%s is not a task key", b.Name)
		}
	}
	if _, ok := BuiltinOf("area"); ok {
		t.Error("BuiltinOf(area) found")
	}
	if len(Default().Props) != 0 {
		t.Errorf("Default props = %v", Default().Props)
	}
}

func TestMatchValue(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		values []string
		v, got string
		ok     bool
	}{
		{[]string{"bug", "docs"}, "Bug", "bug", true},
		{[]string{"bug", "docs"}, "docs", "docs", true},
		{[]string{"Bug", "bug"}, "bug", "bug", true},
		{[]string{"bug"}, "feature", "", false},
		{nil, "bug", "", false},
	} {
		if got, ok := MatchValue(c.values, c.v); got != c.got || ok != c.ok {
			t.Errorf("MatchValue(%v, %q) = %q, %v", c.values, c.v, got, ok)
		}
	}
}

func TestListsDecodeAndEncode(t *testing.T) {
	t.Parallel()
	s, err := Decode([]byte("statuses: []\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(s.Priorities, DefaultPriorities) || len(s.Labels) != 0 {
		t.Errorf("no keys: priorities %v, labels %v", s.Priorities, s.Labels)
	}
	out, err := s.Encode()
	if err != nil || strings.Contains(string(out), "priorities") || strings.Contains(string(out), "labels") {
		t.Errorf("the defaults are written: %q, %v", out, err)
	}

	src := "priorities: [p1, p2]\nlabels: [bug, docs]\n"
	s, err = Decode([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(s.Priorities, []string{"p1", "p2"}) || !slices.Equal(s.Labels, []string{"bug", "docs"}) {
		t.Errorf("priorities %v, labels %v", s.Priorities, s.Labels)
	}
	if out, err = s.Encode(); err != nil || string(out) != src {
		t.Errorf("round trip = %q, %v", out, err)
	}

	s, err = Decode([]byte("priorities: []\n"))
	if err != nil {
		t.Fatal(err)
	}
	if s.Priorities == nil || len(s.Priorities) != 0 {
		t.Errorf("priorities: [] = %#v", s.Priorities)
	}
	if ps := s.Check(); !slices.ContainsFunc(ps, func(p Problem) bool { return p.Code == "schema.values_empty" && p.Path == "priorities" }) {
		t.Errorf("Check = %v", ps)
	}
	if out, err = s.Encode(); err != nil || string(out) != "priorities: []\n" {
		t.Errorf("empty priorities = %q, %v", out, err)
	}

	s, err = Decode([]byte("priorities:\nlabels:\n"))
	if err != nil || !slices.Equal(s.Priorities, DefaultPriorities) {
		t.Errorf("null priorities = %v, %v", s.Priorities, err)
	}
}

func TestCheckLists(t *testing.T) {
	t.Parallel()
	has := func(s *Schema, code, path string) bool {
		return slices.ContainsFunc(s.Check(), func(p Problem) bool { return p.Code == code && p.Path == path })
	}
	if ps := withLists(DefaultPriorities, nil).Check(); len(ps) != 0 {
		t.Errorf("empty labels: %v", ps)
	}
	if !has(withLists([]string{}, nil), "schema.values_empty", "priorities") {
		t.Error("priorities: [] is not reported")
	}
	if !has(withLists([]string{"low", "LOW"}, nil), "schema.value_twice", "priorities") {
		t.Error("priorities that differ in case are not reported")
	}
	if !has(withLists(DefaultPriorities, []string{"Bug", "bug"}), "schema.value_twice", "labels") {
		t.Error("labels that differ in case are not reported")
	}
	if !has(withLists(DefaultPriorities, []string{"a", ""}), "schema.name_empty", "labels") {
		t.Error("an empty label is not reported")
	}
	s := Default()
	s.Props = Props{{Name: "labels", Type: TypeMultiEnum, Values: []string{"a"}}}
	if !has(s, "schema.prop_builtin", "props.labels") {
		t.Error("a property named labels is not refused")
	}
}

func TestRequirableBuiltins(t *testing.T) {
	t.Parallel()
	s := Default()
	s.Workflow.Required = Lists{{"done", []string{"priority", "start", "due", "estimate", "points", "labels"}}}
	if ps := s.Check(); len(ps) != 0 {
		t.Errorf("problems: %v", ps)
	}
	zero := 0.0
	d, _ := model.ParseDate("2026-11-10")
	h, _ := model.ParseDuration("1h")
	for _, c := range []struct {
		name string
		task *model.Task
	}{
		{"priority", &model.Task{Priority: "low"}},
		{"start", &model.Task{Start: &d}},
		{"due", &model.Task{Due: &d}},
		{"estimate", &model.Task{Estimate: &h}},
		{"points", &model.Task{Points: &zero}}, // 0 is a value
		{"labels", &model.Task{Labels: []string{"a"}}},
	} {
		if filled, ok := builtinFilled(c.task, c.name); !ok || !filled {
			t.Errorf("%s: filled=%v ok=%v", c.name, filled, ok)
		}
		if filled, ok := builtinFilled(&model.Task{}, c.name); !ok || filled {
			t.Errorf("%s on an empty task: filled=%v ok=%v", c.name, filled, ok)
		}
	}
	e := s.For(nil)
	vs := ValidateTransition("todo", "done", &model.Task{Points: &zero}, e)
	if want := []string{"required:priority", "required:start", "required:due", "required:estimate", "required:labels"}; !slices.Equal(codes(vs), want) {
		t.Errorf("violations = %v, want %v", codes(vs), want)
	}
}

func TestValidatePriorityAndLabels(t *testing.T) {
	t.Parallel()
	s := withLists([]string{"low", "High"}, []string{"bug", "docs"})
	e := s.For(nil)
	for _, c := range []struct {
		name, front string
		want        []string
	}{
		{"valid", "priority: low\nlabels: [bug, docs]\n", nil},
		{"case is ignored", "priority: HIGH\nlabels: [Bug]\n", nil},
		{"no values", "", nil},
		{"unknown priority", "priority: urgent\n", []string{"bad_value:priority"}},
		{"unknown label", "labels: [bug, feature]\n", []string{"bad_value:labels"}},
	} {
		vs, _ := ValidateTask(mustTask(t, c.front), e)
		if !slices.Equal(codes(vs), c.want) {
			t.Errorf("%s: violations = %v, want %v", c.name, codes(vs), c.want)
		}
	}

	vs, _ := ValidateTask(mustTask(t, "labels: [a, b]\n"), withLists(DefaultPriorities, nil).For(nil))
	if len(vs) != 1 || vs[0].Code != CodeBadValue || vs[0].Field != "labels" || vs[0].Msg != "設定の「ラベル」で足してください" {
		t.Errorf("empty list: violations = %v", vs)
	}
}

func TestValidateUnreadableTypedValues(t *testing.T) {
	t.Parallel()
	e := Default().For(nil)
	for _, c := range []struct{ front, want string }{
		{"due: next week\n", "bad_value:due"},
		{"start: 2026-13-01\n", "bad_value:start"},
		{"estimate: 1d\n", "bad_value:estimate"},
		{"points: many\n", "bad_value:points"},
		{"points: .nan\n", "bad_value:points"},
		{"priority: [a, b]\n", "bad_value:priority"},
	} {
		task := mustTask(t, c.front)
		vs, _ := ValidateTask(task, e)
		if !slices.Equal(codes(vs), []string{c.want}) {
			t.Errorf("%q: violations = %v, want [%s]", c.front, codes(vs), c.want)
		}
		// A write that does not touch the value is not refused for it.
		if got := ValidateWrite(task, "", e, task, "", e, false); len(got) != 0 {
			t.Errorf("%q: the write is refused: %v", c.front, codes(got))
		}
	}
	if vs, _ := ValidateTask(mustTask(t, "custom: 1\n"), e); len(vs) != 0 {
		t.Errorf("an unknown key is a violation: %v", codes(vs))
	}
}

func TestChangePriorities(t *testing.T) {
	t.Parallel()
	s := Default() // low mid high urgent
	got, ms, removed, err := s.ChangePriorities([]ValueEdit{
		{Name: "none"},
		{Name: "low", From: []string{"low"}},
		{Name: "high", From: []string{"high", "urgent"}},
		{Name: "medium", From: []string{"mid"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Priorities, []string{"none", "low", "high", "medium"}) {
		t.Errorf("priorities = %v", got.Priorities)
	}
	want := []Migration{
		{Target: TargetPriority, Old: "urgent", New: "high"},
		{Target: TargetPriority, Old: "mid", New: "medium"},
	}
	if !slices.Equal(ms, want) || len(removed) != 0 {
		t.Errorf("migrations = %v, removed = %v", ms, removed)
	}
	if !slices.Equal(s.Priorities, DefaultPriorities) {
		t.Errorf("s was changed: %v", s.Priorities)
	}

	got, ms, removed, err = s.ChangePriorities([]ValueEdit{{Name: "high", From: []string{"high"}}, {Name: "low", From: []string{"low"}}})
	if err != nil || !slices.Equal(got.Priorities, []string{"high", "low"}) || len(ms) != 0 || !slices.Equal(removed, []string{"mid", "urgent"}) {
		t.Errorf("reorder and remove: %v, %v, %v, %v", got, ms, removed, err)
	}
	if ps := got.Check(); len(ps) != 0 {
		t.Errorf("Check = %v", ps)
	}

	for name, eds := range map[string][]ValueEdit{
		"the last value":       {},
		"a name in two cases":  {{Name: "a"}, {Name: "A"}},
		"an empty name":        {{Name: ""}},
		"a name twice":         {{Name: "low", From: []string{"low"}}, {Name: "low"}},
		"a value that is not":  {{Name: "x", From: []string{"nope"}}},
		"a source used by two": {{Name: "a", From: []string{"low"}}, {Name: "b", From: []string{"low"}}},
		"a removed one reused": {{Name: "low", From: []string{"mid"}}, {Name: "mid"}},
	} {
		if _, _, _, err := s.ChangePriorities(eds); !errors.Is(err, ErrValueInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestChangeLabels(t *testing.T) {
	t.Parallel()
	s := withLists(DefaultPriorities, []string{"bug", "docs"})
	got, ms, removed, err := s.ChangeLabels([]ValueEdit{
		{Name: "defect", From: []string{"bug"}},
		{Name: "infra"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Labels, []string{"defect", "infra"}) || !slices.Equal(removed, []string{"docs"}) ||
		!slices.Equal(ms, []Migration{{Target: TargetLabel, Old: "bug", New: "defect"}}) {
		t.Errorf("labels %v, migrations %v, removed %v", got.Labels, ms, removed)
	}
	if got, _, removed, err = s.ChangeLabels(nil); err != nil || len(got.Labels) != 0 || len(removed) != 2 {
		t.Errorf("all labels removed: %v, %v, %v", got.Labels, removed, err)
	}
	if _, _, _, err = s.ChangeLabels([]ValueEdit{{Name: "Bug", From: []string{"bug"}}, {Name: "bug"}}); !errors.Is(err, ErrValueInvalid) {
		t.Errorf("names that differ in case: err = %v", err)
	}
	if got, ms, _, err = s.ChangeLabels([]ValueEdit{{Name: "Bug", From: []string{"bug"}}, {Name: "docs", From: []string{"docs"}}}); err != nil ||
		!slices.Equal(got.Labels, []string{"Bug", "docs"}) || len(ms) != 1 {
		t.Errorf("a change of case only: %v, %v, %v", got.Labels, ms, err)
	}
}

func TestMigratePriorityAndLabel(t *testing.T) {
	t.Parallel()
	s := withLists([]string{"low", "mid", "high"}, []string{"bug", "docs"})
	got, err := s.Migrate(Migration{Target: TargetPriority, Old: "mid", New: "medium"})
	if err != nil || !slices.Equal(got.Priorities, []string{"low", "medium", "high"}) {
		t.Errorf("rename a priority: %v, %v", got, err)
	}
	if got, err = s.Migrate(Migration{Target: TargetPriority, Old: "mid", New: "high"}); err != nil || !slices.Equal(got.Priorities, []string{"low", "high"}) {
		t.Errorf("merge a priority: %v, %v", got, err)
	}
	if got, err = s.Migrate(Migration{Target: TargetLabel, Old: "bug", New: "docs"}); err != nil || !slices.Equal(got.Labels, []string{"docs"}) {
		t.Errorf("merge a label: %v, %v", got, err)
	}
	if got, err = s.Migrate(Migration{Target: TargetLabel, Old: "bug", New: "defect"}); err != nil || !slices.Equal(got.Labels, []string{"defect", "docs"}) {
		t.Errorf("rename a label: %v, %v", got, err)
	}
	for _, m := range []Migration{
		{Target: TargetPriority, Old: "nope", New: "x"},
		{Target: TargetLabel, Old: "nope", New: "x"},
		{Target: TargetLabel, Old: "bug", New: "bug"},
	} {
		if _, err := s.Migrate(m); err == nil {
			t.Errorf("Migrate(%+v) succeeded", m)
		}
	}
	if !slices.Equal(s.Priorities, []string{"low", "mid", "high"}) || !slices.Equal(s.Labels, []string{"bug", "docs"}) {
		t.Error("the receiver was changed")
	}

	task := mustTask(t, "priority: mid\nlabels: [bug, docs]\n")
	for _, c := range []struct {
		m       Migration
		changed bool
		want    string
	}{
		{Migration{Target: TargetPriority, Old: "mid", New: "high"}, true, "high [bug docs]"},
		{Migration{Target: TargetPriority, Old: "low", New: "high"}, false, "mid [bug docs]"},
		{Migration{Target: TargetLabel, Old: "bug", New: "defect"}, true, "mid [defect docs]"},
		{Migration{Target: TargetLabel, Old: "bug", New: "docs"}, true, "mid [docs]"},
		{Migration{Target: TargetLabel, Old: "nope", New: "x"}, false, "mid [bug docs]"},
	} {
		x := *task
		x.Labels = slices.Clone(task.Labels)
		changed, err := c.m.ApplyTask(&x, s)
		if err != nil || changed != c.changed {
			t.Errorf("%+v: changed = %v, err = %v", c.m, changed, err)
		}
		if got := fmt.Sprint(x.Priority, " ", x.Labels); got != c.want {
			t.Errorf("%+v: got %s, want %s", c.m, got, c.want)
		}
	}
	if !slices.Equal(task.Labels, []string{"bug", "docs"}) {
		t.Errorf("the task was changed through a copy: %v", task.Labels)
	}
}

func TestTaskUsesPriorityAndLabel(t *testing.T) {
	t.Parallel()
	task := mustTask(t, "priority: mid\nlabels: [bug]\n")
	for _, c := range []struct {
		target Target
		name   string
		want   bool
	}{
		{TargetPriority, "mid", true},
		{TargetPriority, "low", false},
		{TargetLabel, "bug", true},
		{TargetLabel, "docs", false},
	} {
		if got := TaskUses(task, c.target, "", c.name); got != c.want {
			t.Errorf("TaskUses(%s %s) = %v", c.target, c.name, got)
		}
	}
}

// Tasks are found and rewritten without regard to case, as Validate finds
// them, and written as the list spells the value.
func TestMigrateAndUsesIgnoreCase(t *testing.T) {
	t.Parallel()
	s := withLists([]string{"low", "high"}, []string{"bug", "docs"})
	task := mustTask(t, "priority: HIGH\nlabels: [Bug, Docs]\n")
	if !TaskUses(task, TargetLabel, "", "bug") || !TaskUses(task, TargetPriority, "", "high") {
		t.Error("TaskUses misses a value that differs in case")
	}
	if changed, err := (Migration{Target: TargetLabel, Old: "bug", New: "defect"}).ApplyTask(task, s); err != nil || !changed || fmt.Sprint(task.Labels) != "[defect Docs]" {
		t.Errorf("rename: %v %v %v", changed, task.Labels, err)
	}
	if changed, err := (Migration{Target: TargetLabel, Old: "defect", New: "docs"}).ApplyTask(task, s); err != nil || !changed || fmt.Sprint(task.Labels) != "[docs]" {
		t.Errorf("merge: %v %v %v", changed, task.Labels, err)
	}
	if changed, err := (Migration{Target: TargetPriority, Old: "high", New: "urgent"}).ApplyTask(task, s); err != nil || !changed || task.Priority != "urgent" {
		t.Errorf("priority: %v %q %v", changed, task.Priority, err)
	}
}

// A value that is set replaces what was kept unread: the task is valid again
// in memory, and clearing it is the field at nil and the key out of Extra.
func TestSetValueIgnoresTheUnreadOne(t *testing.T) {
	t.Parallel()
	e := Default().For(nil)
	task := mustTask(t, "due: next week\n")
	d, _ := model.ParseDate("2026-11-10")
	task.Due = &d
	if vs, _ := ValidateTask(task, e); len(vs) != 0 {
		t.Errorf("violations = %v", codes(vs))
	}
	task.Due = nil
	task.Extra.Delete("due")
	if vs, _ := ValidateTask(task, e); len(vs) != 0 {
		t.Errorf("violations = %v", codes(vs))
	}
}

func TestPropsAreNotMerged(t *testing.T) {
	t.Parallel()
	s := withProps()
	_, err := s.Migrate(Migration{Target: TargetProp, Old: "begin", New: "finish"})
	if !errors.Is(err, ErrPropMerge) {
		t.Errorf("merge: err = %v", err)
	}
	if got, err := s.Migrate(Migration{Target: TargetProp, Old: "begin", New: "start_at"}); err != nil || got.Props[1].Name != "start_at" {
		t.Errorf("rename: %v, %v", got, err)
	}
	// A new name that is a built-in field is found by Check.
	got, _, _, err := s.ChangeProp("begin", PropChange{Name: ptr("due")})
	if err != nil {
		t.Fatal(err)
	}
	if ps := got.Check(); !slices.ContainsFunc(ps, func(p Problem) bool { return p.Code == "schema.prop_builtin" && p.Path == "props.due" }) {
		t.Errorf("Check = %v", ps)
	}
}
