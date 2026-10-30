package schema

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
	"go.yaml.in/yaml/v4"
)

const migrateSrc = `statuses:
- {name: todo, category: todo, default: true}
- name: ready # 準備
  category: todo
  x-note: keep
- {name: doing, category: doing, default: true}
- {name: done, category: done, default: true}
- {name: canceled, category: canceled, default: true}
kinds:
- {name: project, role: container}
- {name: epic, role: container, parents: [root, project]}
- {name: task, role: work, parents: [root, project, epic], default: true}
- {name: bug, role: work, parents: [root, epic, project]}
props:
  priority: {type: enum, values: [low, mid, high]}
  points: {type: number}
  size: {type: number}
  area: {type: multi_enum, values: [a, b]}
  refs: {type: task_ref, multi: true}
  ref: {type: task_ref}
workflow:
  transitions:
    todo: [doing, ready]
    ready: [doing, todo]
    doing: [done, todo]
  required:
    todo: [points]
    doing: [points, size]
scoped:
- where: project:01JBZ8K3QW7M2X9V4T6R5N8P0B
  statuses: [todo, ready, doing, done, canceled]
  workflow:
    transitions:
      todo: [done]
    required:
      todo: [points]
      ready: [size]
  templates:
    default:
      epic: e
      bug: b
      project: p
`

func decodeMigrate(t *testing.T) *Schema {
	t.Helper()
	s, err := Decode([]byte(migrateSrc))
	if err != nil {
		t.Fatal(err)
	}
	if ps := s.Check(); len(ps) > 0 {
		t.Fatalf("the fixture has problems: %v", ps)
	}
	return s
}

func listsString(l Lists) string {
	parts := make([]string, len(l))
	for i, x := range l {
		parts[i] = fmt.Sprintf("%s:%v", x.Key, x.Values)
	}
	return strings.Join(parts, " ")
}

func names[T any](xs []T, name func(T) string) string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = name(x)
	}
	return strings.Join(out, " ")
}

func statusNames(s *Schema) string {
	return names(s.Statuses, func(x Status) string {
		if x.Default {
			return x.Name + "*"
		}
		return x.Name
	})
}

func kindParents(s *Schema) string {
	return names(s.Kinds, func(k Kind) string { return fmt.Sprintf("%s:%v", k.Name, k.Parents) })
}

func propNames(s *Schema) string {
	return names(s.Props, func(p Prop) string { return p.Name })
}

func TestMigrate(t *testing.T) {
	for _, tc := range []struct {
		name string
		m    Migration
		want map[string]string
	}{
		{"rename a status", Migration{Target: TargetStatus, Old: "todo", New: "open"}, map[string]string{
			"statuses":           "open* ready doing* done* canceled*",
			"transitions":        "open:[doing ready] ready:[doing open] doing:[done open]",
			"required":           "open:[points] doing:[points size]",
			"scoped.statuses":    "[open ready doing done canceled]",
			"scoped.transitions": "open:[done]",
			"scoped.required":    "open:[points] ready:[size]",
			"kinds":              "project:[] epic:[root project] task:[root project epic] bug:[root epic project]",
			"scoped.templates":   "map[bug:b epic:e project:p]",
			"props":              "priority points size area refs ref",
			"props.priority":     "[low mid high]",
		}},
		{"merge a status", Migration{Target: TargetStatus, Old: "todo", New: "ready"}, map[string]string{
			"statuses":           "ready* doing* done* canceled*",
			"transitions":        "ready:[doing ready] doing:[done ready]",
			"required":           "ready:[points] doing:[points size]",
			"scoped.statuses":    "[ready doing done canceled]",
			"scoped.transitions": "ready:[done]",
			"scoped.required":    "ready:[points size]",
		}},
		{"rename a kind", Migration{Target: TargetKind, Old: "epic", New: "feature"}, map[string]string{
			"kinds":            "project:[] feature:[root project] task:[root project feature] bug:[root feature project]",
			"scoped.templates": "map[bug:b feature:e project:p]",
			"statuses":         "todo* ready doing* done* canceled*",
		}},
		{"merge a kind", Migration{Target: TargetKind, Old: "epic", New: "project"}, map[string]string{
			"kinds":            "project:[] task:[root project] bug:[root project]",
			"scoped.templates": "map[bug:b project:p]",
		}},
		{"rename a prop", Migration{Target: TargetProp, Old: "points", New: "pts"}, map[string]string{
			"props":           "priority pts size area refs ref",
			"required":        "todo:[pts] doing:[pts size]",
			"scoped.required": "todo:[pts] ready:[size]",
		}},
		{"merge a prop", Migration{Target: TargetProp, Old: "points", New: "size"}, map[string]string{
			"props":           "priority size area refs ref",
			"required":        "todo:[size] doing:[size]",
			"scoped.required": "todo:[size] ready:[size]",
		}},
		{"rename a value", Migration{Target: TargetValue, Prop: "priority", Old: "mid", New: "medium"}, map[string]string{
			"props.priority": "[low medium high]",
			"props":          "priority points size area refs ref",
		}},
		{"merge a value", Migration{Target: TargetValue, Prop: "priority", Old: "mid", New: "high"}, map[string]string{
			"props.priority": "[low high]",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := decodeMigrate(t)
			got, err := s.Migrate(tc.m)
			if err != nil {
				t.Fatal(err)
			}
			pr, _ := got.Prop("priority")
			have := map[string]string{
				"statuses":           statusNames(got),
				"transitions":        listsString(got.Workflow.Transitions),
				"required":           listsString(got.Workflow.Required),
				"scoped.statuses":    fmt.Sprint(got.Scoped[0].Statuses),
				"scoped.transitions": listsString(got.Scoped[0].Workflow.Transitions),
				"scoped.required":    listsString(got.Scoped[0].Workflow.Required),
				"kinds":              kindParents(got),
				"scoped.templates":   fmt.Sprint(got.Scoped[0].Templates.Default),
				"props":              propNames(got),
				"props.priority":     fmt.Sprint(pr.Values),
			}
			for k, want := range tc.want {
				if have[k] != want {
					t.Errorf("%s = %s, want %s", k, have[k], want)
				}
			}
			if ps := got.Check(); len(ps) > 0 {
				t.Errorf("Check() = %v", ps)
			}
		})
	}
}

func TestMigrateKeepsReceiver(t *testing.T) {
	for _, m := range []Migration{
		{Target: TargetStatus, Old: "todo", New: "ready"},
		{Target: TargetKind, Old: "epic", New: "project"},
		{Target: TargetProp, Old: "points", New: "size"},
		{Target: TargetValue, Prop: "priority", Old: "mid", New: "high"},
	} {
		s := decodeMigrate(t)
		if _, err := s.Migrate(m); err != nil {
			t.Fatal(err)
		}
		if got := encodeString(t, s); got != migrateSrc {
			t.Errorf("%+v changed the receiver:\n%s", m, got)
		}
	}
}

func TestMigrateStatusMergeOtherCategory(t *testing.T) {
	s := decodeMigrate(t)
	got, err := s.Migrate(Migration{Target: TargetStatus, Old: "todo", New: "doing"})
	if err != nil {
		t.Fatal(err)
	}
	if st, _ := got.Status("doing"); !st.Default || st.Category != CategoryDoing {
		t.Errorf("doing = %+v", st)
	}
	ps := got.Check()
	found := false
	for _, p := range ps {
		if strings.Contains(p.Msg, "category todo needs exactly one default status, has 0") {
			found = true
		}
	}
	if !found {
		t.Errorf("Check() = %v, want no default of todo", ps)
	}
}

func TestMigrateRenameKeepsNotes(t *testing.T) {
	s := decodeMigrate(t)
	got, err := s.Migrate(Migration{Target: TargetStatus, Old: "ready", New: "prepared"})
	if err != nil {
		t.Fatal(err)
	}
	out := encodeString(t, got)
	if want := "- name: prepared # 準備\n  category: todo\n  x-note: keep\n"; !strings.Contains(out, want) {
		t.Errorf("output lacks %q:\n%s", want, out)
	}
	if want := "    todo: [doing, prepared]\n    prepared: [doing, todo]\n"; !strings.Contains(out, want) {
		t.Errorf("output lacks %q:\n%s", want, out)
	}

	// Encode writes into the file the result shares with s, so start again.
	got, err = decodeMigrate(t).Migrate(Migration{Target: TargetStatus, Old: "todo", New: "ready"})
	if err != nil {
		t.Fatal(err)
	}
	out = encodeString(t, got)
	if want := "statuses:\n- name: ready # 準備\n  category: todo\n  default: true\n  x-note: keep\n- {name: doing"; !strings.Contains(out, want) {
		t.Errorf("output lacks %q:\n%s", want, out)
	}
}

func TestMigrateErrors(t *testing.T) {
	for _, tc := range []struct {
		m    Migration
		want string
	}{
		{Migration{Target: TargetStatus, Old: "nope", New: "x"}, `status "nope" is not defined`},
		{Migration{Target: TargetStatus, Old: "todo", New: "todo"}, "the same"},
		{Migration{Target: TargetKind, Old: "nope", New: "x"}, `kind "nope" is not defined`},
		{Migration{Target: TargetKind, Old: "task", New: "task"}, "the same"},
		{Migration{Target: TargetProp, Old: "nope", New: "x"}, `property "nope" is not defined`},
		{Migration{Target: TargetProp, Old: "points", New: "points"}, "the same"},
		{Migration{Target: TargetProp, Old: "priority", New: "points"}, "types differ"},
		{Migration{Target: TargetProp, Old: "refs", New: "ref"}, "types differ"},
		{Migration{Target: TargetValue, Prop: "nope", Old: "a", New: "b"}, `property "nope" is not defined`},
		{Migration{Target: TargetValue, Prop: "points", Old: "a", New: "b"}, "not enum or multi_enum"},
		{Migration{Target: TargetValue, Prop: "priority", Old: "nope", New: "b"}, `value "nope" of priority is not defined`},
		{Migration{Target: TargetValue, Prop: "area", Old: "a", New: "a"}, "the same"},
		{Migration{Target: "nope", Old: "a", New: "b"}, "unknown target"},
	} {
		_, err := decodeMigrate(t).Migrate(tc.m)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("Migrate(%+v) = %v, want %q", tc.m, err, tc.want)
		}
	}
}

func migrateTask(t *testing.T, props string) (*Schema, *taskT) {
	t.Helper()
	return decodeMigrate(t), &taskT{taskWith(t, props)}
}

// taskT wraps a task for reading its props as text.
type taskT struct{ *model.Task }

func (x *taskT) prop(key string) string {
	n := x.Props.Get(key)
	switch {
	case n == nil:
		return "<none>"
	case n.Kind == yaml.SequenceNode:
		vs := make([]string, len(n.Content))
		for i, c := range n.Content {
			vs[i] = c.Value
		}
		return fmt.Sprint(vs)
	}
	return n.Value
}

func (x *taskT) keys() string {
	ks := make([]string, len(x.Props))
	for i, f := range x.Props {
		ks[i] = f.Key
	}
	return strings.Join(ks, " ")
}

func TestApplyTask(t *testing.T) {
	const props = "  priority: mid\n  points: 3\n  area: [a, b]\n"
	for _, tc := range []struct {
		name    string
		m       Migration
		changed bool
		check   func(x *taskT) string
		want    string
	}{
		{"status", Migration{Target: TargetStatus, Old: "todo", New: "ready"}, true, func(x *taskT) string { return x.Status }, "ready"},
		{"other status", Migration{Target: TargetStatus, Old: "doing", New: "ready"}, false, func(x *taskT) string { return x.Status }, "todo"},
		{"kind", Migration{Target: TargetKind, Old: "task", New: "bug"}, true, func(x *taskT) string { return x.Kind }, "bug"},
		{"other kind", Migration{Target: TargetKind, Old: "epic", New: "bug"}, false, func(x *taskT) string { return x.Kind }, "task"},
		{"prop", Migration{Target: TargetProp, Old: "points", New: "size"}, true, func(x *taskT) string { return x.keys() + " " + x.prop("size") }, "priority size area 3"},
		{"absent prop", Migration{Target: TargetProp, Old: "ref", New: "refs"}, false, func(x *taskT) string { return x.keys() }, "priority points area"},
		{"value", Migration{Target: TargetValue, Prop: "priority", Old: "mid", New: "high"}, true, func(x *taskT) string { return x.prop("priority") }, "high"},
		{"other value", Migration{Target: TargetValue, Prop: "priority", Old: "low", New: "high"}, false, func(x *taskT) string { return x.prop("priority") }, "mid"},
		{"multi value", Migration{Target: TargetValue, Prop: "area", Old: "a", New: "c"}, true, func(x *taskT) string { return x.prop("area") }, "[c b]"},
		{"multi value merged", Migration{Target: TargetValue, Prop: "area", Old: "a", New: "b"}, true, func(x *taskT) string { return x.prop("area") }, "[b]"},
		{"value of an absent prop", Migration{Target: TargetValue, Prop: "priority", Old: "mid", New: "high"}, false, nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := props
			if tc.check == nil {
				p = "  points: 3\n"
			}
			s, x := migrateTask(t, p)
			before, err := x.Encode()
			if err != nil {
				t.Fatal(err)
			}
			changed, err := tc.m.ApplyTask(x.Task, s)
			if err != nil {
				t.Fatal(err)
			}
			if changed != tc.changed {
				t.Errorf("changed = %v, want %v", changed, tc.changed)
			}
			if tc.check != nil {
				if got := tc.check(x); got != tc.want {
					t.Errorf("got %s, want %s", got, tc.want)
				}
			}
			after, err := x.Encode()
			if err != nil {
				t.Fatal(err)
			}
			if (string(after) != string(before)) != tc.changed {
				t.Errorf("file changed = %v, want %v:\n%s", string(after) != string(before), tc.changed, after)
			}
		})
	}
}

func TestApplyTaskPropOnBoth(t *testing.T) {
	s, x := migrateTask(t, "  points: 3\n  size: 5\n")
	if _, err := (Migration{Target: TargetProp, Old: "points", New: "size"}).ApplyTask(x.Task, s); err == nil ||
		!strings.Contains(err.Error(), "has both points and size") {
		t.Errorf("err = %v, want both", err)
	}
}
