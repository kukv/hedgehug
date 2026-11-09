package upgrade

import (
	"bytes"
	"slices"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/domain/model"
)

// schema3 is a schema.yaml of format 3: priority, due and points are
// properties of the store, of types of its choosing.
const schema3 = `props:
  priority: {type: enum, values: [minor, major, blocker]}
  points: {type: text}
  due: {type: datetime}
  note: {type: text}
labels: [bug, ui]
`

// task3 is the file of a task of format 3 with the YAML props, and fn applied to it.
func task3(t *testing.T, props string, fn func(*model.Task)) []byte {
	t.Helper()
	tk := &model.Task{
		ULID: model.NewULID(), ID: "ME-1", Workspace: model.NewULID(), Kind: "task", Title: "x", Status: "todo",
		Created: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), Updated: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
	}
	if err := yaml.Unmarshal([]byte(props), &tk.Props); err != nil {
		t.Fatal(err)
	}
	if fn != nil {
		fn(tk)
	}
	b, err := tk.Encode()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func up3(t *testing.T, raw []byte, sc string) *model.Task {
	t.Helper()
	got, err := To(3, 4, raw, Cx{Schema: []byte(sc)})
	if err != nil {
		t.Fatal(err)
	}
	tk, err := model.DecodeTask(got)
	if err != nil {
		t.Fatal(err)
	}
	return tk
}

func TestTask3to4MovesTheFiveToTheFields(t *testing.T) {
	t.Parallel()
	got := up3(t, task3(t, "priority: major\nstart: 2026-11-03\ndue: 2026-11-08\nestimate: 1h30m\npoints: 5\nnote: keep\n", nil), schema3)
	if got.Priority != "major" || got.Start == nil || got.Start.String() != "2026-11-03" || got.Due == nil || got.Due.String() != "2026-11-08" ||
		got.Estimate == nil || got.Estimate.String() != "1h30m" || got.Points == nil || *got.Points != 5 {
		t.Errorf("fields = %q %v %v %v %v", got.Priority, got.Start, got.Due, got.Estimate, got.Points)
	}
	if len(got.Props) != 1 || got.Props[0].Key != "note" {
		t.Errorf("props = %v; want note alone", got.Props)
	}
	if !got.Updated.Equal(time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("updated = %v; the step does not move it", got.Updated)
	}
}

func TestTask3to4LeavesATaskWithNothingToDoAsItIs(t *testing.T) {
	t.Parallel()
	raw := task3(t, "note: keep\n", func(tk *model.Task) { tk.Labels = []string{"bug"} })
	got, err := To(3, 4, raw, Cx{Schema: []byte(schema3)})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, raw) {
		t.Errorf("rewritten:\n%s\nwas\n%s", got, raw)
	}
}

func TestTask3to4PrioritiesComeFromTheListTheDefinitionOrTheDefault(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, schema, value string
		want                string // "" for a value that stays in props
	}{
		{"the list wins over the definition", "priorities: [Hot, Cold]\nprops:\n  priority: {type: enum, values: [minor, major]}\n", "hot", "Hot"},
		{"the definition's values", "props:\n  priority: {type: enum, values: [minor, major]}\n", "MAJOR", "major"},
		{"a definition that is not an enum gives the default", "props:\n  priority: {type: text}\n", "Urgent", "urgent"},
		{"no definition gives the default", "note: x\n", "mid", "mid"},
		{"no schema.yaml gives the default", "-", "low", "low"},
		{"a value the list lacks stays in props", "priorities: [Hot]\n", "cold", ""},
	} {
		var cx Cx
		if c.schema != "-" {
			cx.Schema = []byte(c.schema)
		}
		got, err := To(3, 4, task3(t, "priority: "+c.value+"\n", nil), cx)
		if err != nil {
			t.Fatal(err)
		}
		tk, _ := model.DecodeTask(got)
		if tk.Priority != c.want || (c.want == "") != (tk.Props.Get("priority") != nil) {
			t.Errorf("%s: priority %q, props %v; want %q", c.name, tk.Priority, tk.Props, c.want)
		}
	}
}

func TestTask3to4FixesTheTypes(t *testing.T) {
	t.Parallel()
	got := up3(t, task3(t, "points: \"3\"\nestimate: [45m]\ndue: 2026-11-08T15:30:00+09:00\nstart: 2026-11-04T00:00:00-05:00\npriority: \"\"\n", nil), schema3)
	if got.Points == nil || *got.Points != 3 {
		t.Errorf("points = %v; want 3", got.Points)
	}
	if got.Estimate == nil || got.Estimate.String() != "45m" {
		t.Errorf("estimate = %v; want 45m (a list of one)", got.Estimate)
	}
	if got.Due == nil || got.Due.String() != "2026-11-08" { // the day as written, not in UTC (06:30)
		t.Errorf("due = %v; want 2026-11-08", got.Due)
	}
	if got.Start == nil || got.Start.String() != "2026-11-04" {
		t.Errorf("start = %v; want 2026-11-04", got.Start)
	}
	if got.Priority != "" || len(got.Props) != 0 {
		t.Errorf("priority %q, props %v; an empty value is left out", got.Priority, got.Props)
	}
}

func TestBuiltinsTellTheTimeThatWasLeftOut(t *testing.T) {
	t.Parallel()
	tk, err := model.DecodeTask(task3(t, "due: 2026-11-08T15:30:00+09:00\nstart: 2026-11-04T00:00:00-05:00\n", nil))
	if err != nil {
		t.Fatal(err)
	}
	d, _ := BuiltinDefsOf([]byte(schema3))
	r := TaskBuiltins(tk, d)
	if len(r.Time) != 1 || r.Time[0].Prop != "due" || r.Time[0].Value != "2026-11-08T15:30:00+09:00" || len(r.Bad) != 0 {
		t.Errorf("time = %+v, bad = %+v; want the due alone, and nothing stopped", r.Time, r.Bad)
	}
}

func TestTask3to4KeepsWhatCannotBeMovedInProps(t *testing.T) {
	t.Parallel()
	got := up3(t, task3(t, "estimate: 3\npoints: lots\ndue: next week\nstart: [2026-11-01, 2026-11-02]\nnote: keep\n", nil), schema3)
	if got.Estimate != nil || got.Points != nil || got.Due != nil || got.Start != nil {
		t.Errorf("fields = %v %v %v %v; none was readable", got.Estimate, got.Points, got.Due, got.Start)
	}
	for _, k := range []string{"estimate", "points", "due", "start", "note"} {
		if got.Props.Get(k) == nil {
			t.Errorf("props.%s was dropped from a past version", k)
		}
	}
}

func TestTask3to4KeepsBothWhenTheFieldHasAValueToo(t *testing.T) {
	t.Parallel()
	raw := task3(t, "points: 8\npriority: major\n", func(tk *model.Task) { p := 5.0; tk.Points = &p })
	got := up3(t, raw, schema3)
	if got.Points == nil || *got.Points != 5 || got.Props.Get("points") == nil {
		t.Errorf("points %v, props %v; the field and the property both stay", got.Points, got.Props)
	}
	if got.Priority != "major" {
		t.Errorf("priority = %q; the other name moves", got.Priority)
	}
}

func TestBuiltinsOfAConversionTakeTheValueOfPropsOverTheField(t *testing.T) {
	t.Parallel()
	raw := task3(t, "points: 8\nestimate: 3\n", func(tk *model.Task) { p := 5.0; tk.Points = &p })
	tk, _ := model.DecodeTask(raw)
	d, _ := BuiltinDefsOf([]byte(schema3))
	r := TaskBuiltins(tk, d) // not KeepingBad: this is hh store convert
	if tk.Points == nil || *tk.Points != 8 || len(tk.Props) != 0 {
		t.Errorf("points %v, props %v; the value of props replaces the field", tk.Points, tk.Props)
	}
	if len(r.Bad) != 2 || r.Bad[0].Prop != "estimate" || r.Bad[0].Twice || r.Bad[1].Prop != "points" || !r.Bad[1].Twice {
		t.Errorf("bad = %+v; want estimate and points (twice)", r.Bad)
	}
}

func TestTask3to4TakesTheLabelsTheListLacks(t *testing.T) {
	t.Parallel()
	raw := task3(t, "", func(tk *model.Task) { tk.Labels = []string{"UI", "wip", "bug", "Bug"} })
	got := up3(t, raw, schema3)
	if !slices.Equal(got.Labels, []string{"ui", "bug"}) {
		t.Errorf("labels = %v; want the list's spelling, once each, without wip", got.Labels)
	}
	// a schema without labels: gives none
	got = up3(t, raw, "note: x\n")
	if len(got.Labels) != 0 {
		t.Errorf("labels = %v; a store with no list has none left", got.Labels)
	}
}

func TestTask3to4TakesTheTracesOfTheLabelSync(t *testing.T) {
	t.Parallel()
	raw := task3(t, "", func(tk *model.Task) {
		tk.Links = []model.Link{
			{Type: "issue", Ref: "github.com/o/r#1", Sync: "push", Fields: []string{"title", "labels"}},
			{Type: "issue", Ref: "github.com/o/r#2", Sync: "push", Fields: []string{"labels"}},
		}
		var doc yaml.Node
		if err := yaml.Unmarshal([]byte("issues:\n- ref: github.com/o/r#1\n  state: open\n  synced: {title: x, labels: [bug], assignees: []}\n"), &doc); err != nil {
			t.Fatal(err)
		}
		tk.GH = doc.Content[0]
	})
	got := up3(t, raw, schema3)
	if !slices.Equal(got.Links[0].Fields, []string{"title"}) || len(got.Links[1].Fields) != 0 {
		t.Errorf("link fields = %v, %v", got.Links[0].Fields, got.Links[1].Fields)
	}
	b, _ := yaml.Marshal(got.GH)
	if strings.Contains(string(b), "labels") || !strings.Contains(string(b), "title: x") {
		t.Errorf("gh = %s; want synced without labels and with the rest", b)
	}
}

func TestTemplate3to4MovesTheItemsAndTheChildren(t *testing.T) {
	t.Parallel()
	src := "---\nkind: task\ntitle: t\nprops:\n  priority: Major\n  points: \"2\"\n  note: keep\nlabels: [bug, wip]\nchildren:\n- title: c\n  props:\n    due: 2026-11-09T10:00:00Z\n  labels: [UI]\n---\n"
	got, err := TemplateTo(3, 4, "tpl", []byte(src), Cx{Schema: []byte(schema3)})
	if err != nil {
		t.Fatal(err)
	}
	tp, err := model.DecodeTemplate("tpl", got)
	if err != nil {
		t.Fatal(err)
	}
	it := tp.Item
	if it.Priority != "major" || it.Points == nil || *it.Points != 2 || len(it.Props) != 1 || !slices.Equal(it.Labels, []string{"bug"}) {
		t.Errorf("item = %+v", it)
	}
	c := it.Children[0]
	if c.Due == nil || c.Due.String() != "2026-11-09" || len(c.Props) != 0 || !slices.Equal(c.Labels, []string{"ui"}) {
		t.Errorf("child = %+v", c)
	}
}

func TestMoveLegacyBuiltinsLeavesTheLabelsAlone(t *testing.T) {
	t.Parallel()
	raw := task3(t, "points: \"3\"\npriority: High\nnote: keep\n", func(tk *model.Task) { tk.Labels = []string{"wip"} })
	got, err := MoveLegacyBuiltins(raw, []string{"low", "high"})
	if err != nil {
		t.Fatal(err)
	}
	tk, _ := model.DecodeTask(got)
	if tk.Priority != "high" || tk.Points == nil || *tk.Points != 3 || len(tk.Props) != 1 || !slices.Equal(tk.Labels, []string{"wip"}) {
		t.Errorf("task = %+v", tk)
	}
}

func TestMoveLegacyBuiltinsMovesNothingOfATaskThatCannotBeMovedWhole(t *testing.T) {
	t.Parallel()
	for name, raw := range map[string][]byte{
		"the field has a value": task3(t, "points: 3\n", func(tk *model.Task) { p := 5.0; tk.Points = &p }),
		"a value is no number":  task3(t, "points: lots\npriority: high\n", nil),
		"a priority not listed": task3(t, "priority: nope\n", nil),
	} {
		got, err := MoveLegacyBuiltins(raw, []string{"low", "high"})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, raw) {
			t.Errorf("%s: rewritten:\n%s", name, got)
		}
	}
}
