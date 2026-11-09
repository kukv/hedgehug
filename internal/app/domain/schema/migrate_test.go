package schema

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
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
props:
  level: {type: enum, values: [low, mid, high]}
  score: {type: number}
  size: {type: number}
  area: {type: multi_enum, values: [a, b]}
  tags: {type: text, multi: true}
  memo: {type: text}
workflow:
  transitions:
    todo: [doing, ready]
    ready: [doing, todo]
    doing: [done, todo]
  required:
    todo: [score]
    doing: [score, size]
scoped:
- where: project:01JBZ8K3QW7M2X9V4T6R5N8P0B
  statuses: [todo, ready, doing, done, canceled]
  workflow:
    transitions:
      todo: [done]
    required:
      todo: [score]
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

func propNames(s *Schema) string {
	return names(s.Props, func(p Prop) string { return p.Name })
}

func TestMigrate(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		m    Migration
		want map[string]string
	}{
		{"rename a status", Migration{Target: TargetStatus, Old: "todo", New: "open"}, map[string]string{
			"statuses":           "open* ready doing* done* canceled*",
			"transitions":        "open:[doing ready] ready:[doing open] doing:[done open]",
			"required":           "open:[score] doing:[score size]",
			"scoped.statuses":    "[open ready doing done canceled]",
			"scoped.transitions": "open:[done]",
			"scoped.required":    "open:[score] ready:[size]",
			"scoped.templates":   "map[bug:b epic:e project:p]",
			"props":              "level score size area tags memo",
			"props.level":        "[low mid high]",
		}},
		{"merge a status", Migration{Target: TargetStatus, Old: "todo", New: "ready"}, map[string]string{
			"statuses":           "ready* doing* done* canceled*",
			"transitions":        "ready:[doing ready] doing:[done ready]",
			"required":           "ready:[score] doing:[score size]",
			"scoped.statuses":    "[ready doing done canceled]",
			"scoped.transitions": "ready:[done]",
			"scoped.required":    "ready:[score size]",
		}},
		{"rename a prop", Migration{Target: TargetProp, Old: "score", New: "pts"}, map[string]string{
			"props":           "level pts size area tags memo",
			"required":        "todo:[pts] doing:[pts size]",
			"scoped.required": "todo:[pts] ready:[size]",
		}},
		{"rename a value", Migration{Target: TargetValue, Prop: "level", Old: "mid", New: "medium"}, map[string]string{
			"props.level": "[low medium high]",
			"props":       "level score size area tags memo",
		}},
		{"merge a value", Migration{Target: TargetValue, Prop: "level", Old: "mid", New: "high"}, map[string]string{
			"props.level": "[low high]",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := decodeMigrate(t)
			got, err := s.Migrate(tc.m)
			if err != nil {
				t.Fatal(err)
			}
			pr, _ := got.Prop("level")
			have := map[string]string{
				"statuses":           statusNames(got),
				"transitions":        listsString(got.Workflow.Transitions),
				"required":           listsString(got.Workflow.Required),
				"scoped.statuses":    fmt.Sprint(got.Scoped[0].Statuses),
				"scoped.transitions": listsString(got.Scoped[0].Workflow.Transitions),
				"scoped.required":    listsString(got.Scoped[0].Workflow.Required),
				"scoped.templates":   fmt.Sprint(got.Scoped[0].Templates.Default),
				"props":              propNames(got),
				"props.level":        fmt.Sprint(pr.Values),
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
	t.Parallel()
	for _, m := range []Migration{
		{Target: TargetStatus, Old: "todo", New: "ready"},
		{Target: TargetProp, Old: "score", New: "pts"},
		{Target: TargetValue, Prop: "level", Old: "mid", New: "high"},
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
	t.Parallel()
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
	t.Parallel()
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

	// A renamed key of a mapping keeps its node: the property's block
	// style, comment and unknown key, and the comment of a transition.
	for _, tc := range []struct {
		m       Migration
		from    string
		to      string
		unmoved string
	}{
		{Migration{Target: TargetProp, Old: "points", New: "pts"}, "  points:\n    type: number # 数値\n    min: 0\n", "  pts:\n    type: number # 数値\n    min: 0\n", "  priority: {type: enum, values: [low, high], order: desc} # 未知のキー\n"},
		{Migration{Target: TargetStatus, Old: "todo", New: "open"}, "    todo: [in_progress] # 開始\n", "    open: [in_progress] # 開始\n", "# 既定の未着手\n- name: open\n  label: 未着手 # ラベル\n"},
	} {
		src, err := os.ReadFile("testdata/extras.yaml")
		if err != nil {
			t.Fatal(err)
		}
		s, err := Decode(src)
		if err != nil {
			t.Fatal(err)
		}
		got, err := s.Migrate(tc.m)
		if err != nil {
			t.Fatal(err)
		}
		want := strings.Replace(string(src), tc.from, tc.to, 1)
		if tc.m.Target == TargetStatus {
			want = strings.Replace(want, "- name: todo\n", "- name: open\n", 1)
		}
		if out := encodeString(t, got); out != want || !strings.Contains(out, tc.unmoved) {
			t.Errorf("%+v:\n%s\nwant:\n%s", tc.m, out, want)
		}
	}
}

func TestMigrateErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		m    Migration
		want string
	}{
		{Migration{Target: TargetStatus, Old: "nope", New: "x"}, `status "nope" is not defined`},
		{Migration{Target: TargetStatus, Old: "todo", New: "todo"}, "the same"},
		{Migration{Target: TargetProp, Old: "nope", New: "x"}, `property "nope" is not defined`},
		{Migration{Target: TargetProp, Old: "score", New: "score"}, "the same"},
		{Migration{Target: TargetProp, Old: "level", New: "score"}, "cannot be merged"},
		{Migration{Target: TargetProp, Old: "score", New: "size"}, "cannot be merged"},
		{Migration{Target: TargetValue, Prop: "nope", Old: "a", New: "b"}, `property "nope" is not defined`},
		{Migration{Target: TargetValue, Prop: "score", Old: "a", New: "b"}, "not enum or multi_enum"},
		{Migration{Target: TargetValue, Prop: "level", Old: "nope", New: "b"}, `value "nope" of level is not defined`},
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
	t.Parallel()
	const props = "  level: mid\n  score: 3\n  area: [a, b]\n"
	for _, tc := range []struct {
		name    string
		m       Migration
		changed bool
		check   func(x *taskT) string
		want    string
	}{
		{"status", Migration{Target: TargetStatus, Old: "todo", New: "ready"}, true, func(x *taskT) string { return x.Status }, "ready"},
		{"other status", Migration{Target: TargetStatus, Old: "doing", New: "ready"}, false, func(x *taskT) string { return x.Status }, "todo"},
		{"prop", Migration{Target: TargetProp, Old: "score", New: "size"}, true, func(x *taskT) string { return x.keys() + " " + x.prop("size") }, "level size area 3"},
		{"absent prop", Migration{Target: TargetProp, Old: "memo", New: "tags"}, false, func(x *taskT) string { return x.keys() }, "level score area"},
		{"value", Migration{Target: TargetValue, Prop: "level", Old: "mid", New: "high"}, true, func(x *taskT) string { return x.prop("level") }, "high"},
		{"other value", Migration{Target: TargetValue, Prop: "level", Old: "low", New: "high"}, false, func(x *taskT) string { return x.prop("level") }, "mid"},
		{"multi value", Migration{Target: TargetValue, Prop: "area", Old: "a", New: "c"}, true, func(x *taskT) string { return x.prop("area") }, "[c b]"},
		{"multi value merged", Migration{Target: TargetValue, Prop: "area", Old: "a", New: "b"}, true, func(x *taskT) string { return x.prop("area") }, "[b]"},
		{"value of an absent prop", Migration{Target: TargetValue, Prop: "level", Old: "mid", New: "high"}, false, nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := props
			if tc.check == nil {
				p = "  score: 3\n"
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
	t.Parallel()
	s, x := migrateTask(t, "  score: 3\n  size: 5\n")
	if _, err := (Migration{Target: TargetProp, Old: "score", New: "size"}).ApplyTask(x.Task, s); err == nil ||
		!strings.Contains(err.Error(), "has both score and size") {
		t.Errorf("err = %v, want both", err)
	}
}
