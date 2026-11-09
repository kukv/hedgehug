package convert

import (
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/domain/automation"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// schema3 is a schema.yaml of format 3: the store defines priority, points and
// due as properties, and has a list of labels.
const schema3 = `statuses:
  - {name: todo, category: todo, default: true}
  - {name: done, category: done, default: true}
props:
  priority: {type: enum, values: [minor, major, blocker]}
  points: {type: text}
  due: {type: datetime}
  note: {type: text}
labels: [bug, ui]
`

// store3 is the files of a store in format 3.
type store3 struct{ *store2 }

func newStore3(t *testing.T) *store3 {
	t.Helper()
	s := newStore2(t)
	s.put("schema.yaml", schema3)
	s.put(model.StoreMetaPath, "ulid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nname: x\nformat: 3\n")
	return &store3{s}
}

func (s *store3) run(opt Options) (*Tree, Report, error) {
	s.t.Helper()
	tr := NewTree(s.files)
	rep, err := step4(tr, opt)
	return tr, rep, err
}

func (s *store3) ok(opt Options) (*Tree, Report) {
	s.t.Helper()
	tr, rep, err := s.run(opt)
	if err != nil {
		s.t.Fatal(err)
	}
	return tr, rep
}

// labelled is a task with the labels.
func (s *store3) labelled(title string, labels ...string) *model.Task {
	s.t.Helper()
	tk := s.task(title, "")
	tk.Labels = labels
	b, err := tk.Encode()
	if err != nil {
		s.t.Fatal(err)
	}
	s.files[model.TaskPath(tk.ULID)] = b
	return tk
}

func TestStep4MovesTheFiveAndTheSchema(t *testing.T) {
	t.Parallel()
	s := newStore3(t)
	b := s.task("b", "")
	a := s.task("a", "priority: Major\nstart: 2026-11-03\ndue: 2026-11-08\nestimate: 1h30m\npoints: 5\nnote: keep\n")
	tr, rep := s.ok(Options{})
	got := taskIn(t, tr, a)
	if got.Priority != "major" || got.Start.String() != "2026-11-03" || got.Due.String() != "2026-11-08" || got.Estimate.String() != "1h30m" || *got.Points != 5 {
		t.Errorf("fields = %q %v %v %v %v", got.Priority, got.Start, got.Due, got.Estimate, got.Points)
	}
	if len(got.Props) != 1 || got.Props[0].Key != "note" {
		t.Errorf("props = %v", got.Props)
	}
	if !got.Updated.Equal(created) {
		t.Errorf("updated = %v; the step does not move it", got.Updated)
	}
	hasLine(t, rep, "props の priority・start・due・estimate・points の値を、タスクの同名の項目へ移した: 5 件")
	if _, ok := tr.Changes()[model.TaskPath(b.ULID)]; ok {
		t.Error("a task with nothing to move was rewritten")
	}
	sc, err := schema.Decode(tr.Read("schema.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(sc.Props) != 1 || sc.Props[0].Name != "note" {
		t.Errorf("props of schema.yaml = %+v; want note alone", sc.Props)
	}
	if !slices.Equal(sc.Priorities, []string{"minor", "major", "blocker"}) || !slices.Equal(sc.Labels, []string{"bug", "ui"}) {
		t.Errorf("priorities %v, labels %v", sc.Priorities, sc.Labels)
	}
	hasLine(t, rep, "schema.yaml の priorities: を priority の定義の values から作った: minor, major, blocker")
	hasLine(t, rep, "schema.yaml の組み込みと同じ名前のプロパティを消した（値は同名の項目へ移した）: priority, points, due")
}

func TestStep4PrioritiesFromTheListTheDefinitionOrTheDefault(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, schema string
		want         []string
	}{
		{"the list wins", "priorities: [Hot, Cold]\nprops:\n  priority: {type: enum, values: [minor, major]}\n", []string{"Hot", "Cold"}},
		{"the definition", "props:\n  priority: {type: enum, values: [minor, major]}\n", []string{"minor", "major"}},
		{"a definition that is not an enum", "props:\n  priority: {type: text}\n", schema.DefaultPriorities},
		{"neither", "props:\n  note: {type: text}\n", schema.DefaultPriorities},
	} {
		s := newStore3(t)
		s.put("schema.yaml", c.schema)
		s.task("a", "note: x\n")
		tr, _ := s.ok(Options{})
		sc, err := schema.Decode(tr.Read("schema.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(sc.Priorities, c.want) {
			t.Errorf("%s: priorities = %v; want %v", c.name, sc.Priorities, c.want)
		}
	}
}

func TestStep4TakesTheLabelsTheListLacksAndSaysWhich(t *testing.T) {
	t.Parallel()
	s := newStore3(t)
	a := s.labelled("a", "UI", "wip", "bug", "Bug")
	b := s.labelled("b", "wip", "WIP")
	c := s.labelled("c", "bug")
	tr, rep := s.ok(Options{})
	if got := taskIn(t, tr, a).Labels; !slices.Equal(got, []string{"ui", "bug"}) {
		t.Errorf("a: labels = %v; want the spelling of the list, once each", got)
	}
	if got := taskIn(t, tr, b).Labels; len(got) != 0 {
		t.Errorf("b: labels = %v", got)
	}
	if _, ok := tr.Changes()[model.TaskPath(c.ULID)]; ok {
		t.Error("a task whose labels are all in the list was rewritten")
	}
	hasLine(t, rep, "ラベル wip を外した（schema.yaml の labels: にない）: タスク 2 件・テンプレート 0 件")
}

func TestStep4AStoreWithoutAListLosesEveryLabel(t *testing.T) {
	t.Parallel()
	s := newStore3(t)
	s.put("schema.yaml", "props:\n  note: {type: text}\n")
	a := s.labelled("a", "bug")
	tr, rep := s.ok(Options{})
	if got := taskIn(t, tr, a).Labels; len(got) != 0 {
		t.Errorf("labels = %v", got)
	}
	hasLine(t, rep, "ラベル bug を外した")
}

func TestStep4FixesTheTypesAndReportsTheTimeItLeftOut(t *testing.T) {
	t.Parallel()
	s := newStore3(t)
	a := s.task("a", "points: \"3\"\nestimate: [45m]\ndue: 2026-11-08T15:30:00+09:00\nstart: 2026-11-04T00:00:00-05:00\npriority: \"\"\n")
	tr, rep := s.ok(Options{})
	got := taskIn(t, tr, a)
	if *got.Points != 3 || got.Estimate.String() != "45m" || got.Due.String() != "2026-11-08" || got.Start.String() != "2026-11-04" || got.Priority != "" || len(got.Props) != 0 {
		t.Errorf("points %v, estimate %v, due %v, start %v, priority %q, props %v", got.Points, got.Estimate, got.Due, got.Start, got.Priority, got.Props)
	}
	hasLine(t, rep, "日付にして時刻を落とした: WEB-2 a: props.due: 2026-11-08T15:30:00+09:00")
	if strings.Contains(lines(rep), "props.start") {
		t.Errorf("a date at 00:00 is reported as one that lost a time:\n%s", lines(rep))
	}
}

func TestStep4StopsAtWhatCannotBeReadAndDropsItWithTheFlag(t *testing.T) {
	t.Parallel()
	s := newStore3(t)
	a := s.task("a", "estimate: 3\npoints: 2\npriority: nope\ndue: soon\nstart: [2026-11-01, 2026-11-02]\n")
	tr, _, err := s.run(Options{})
	e := usecase.AsError(err)
	if e == nil || e.Code != "convert.bad_value" || e.Kind != usecase.KindInvalid {
		t.Fatalf("error = %v", err)
	}
	for _, w := range []string{"props.estimate: 3", "props.priority: nope", "props.due: soon", "props.start:", "--drop-bad-values"} {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("the error lacks %q:\n%v", w, err)
		}
	}
	if len(tr.Changes()) != 0 {
		t.Errorf("a step that stops wrote %v", tr.Changes())
	}
	tr, rep := s.ok(Options{DropBadValues: true})
	got := taskIn(t, tr, a)
	if *got.Points != 2 || got.Estimate != nil || got.Priority != "" || got.Due != nil || got.Start != nil || len(got.Props) != 0 {
		t.Errorf("points %v, estimate %v, priority %q, due %v, start %v, props %v", got.Points, got.Estimate, got.Priority, got.Due, got.Start, got.Props)
	}
	hasLine(t, rep, "読めない値を外した: WEB-2 a: props.estimate: 3")
	hasLine(t, rep, "読めない値を外した: WEB-2 a: props.priority: nope")
}

func TestStep4StopsAtAKeyInPropsAndAtTheTopLevel(t *testing.T) {
	t.Parallel()
	s := newStore3(t)
	a := s.task("a", "points: 8\nnote: x\n")
	a.Points = new(float64)
	*a.Points = 5
	b, err := a.Encode()
	if err != nil {
		t.Fatal(err)
	}
	s.files[model.TaskPath(a.ULID)] = b
	_, _, err = s.run(Options{})
	if e := usecase.AsError(err); e == nil || e.Code != "convert.bad_value" || !strings.Contains(err.Error(), "props.points: 8（トップレベルの points にも値がある: 5）") {
		t.Fatalf("error = %v", err)
	}
	tr, rep := s.ok(Options{DropBadValues: true})
	got := taskIn(t, tr, a)
	if *got.Points != 8 || len(got.Props) != 1 {
		t.Errorf("points %v, props %v; the value of props is kept, and the one at the top level goes", *got.Points, got.Props)
	}
	hasLine(t, rep, "トップレベルの points: 5 を外し、props の値を移した: WEB-2 a: props.points: 8")
}

func TestStep4RewritesTheItemsAndChildrenOfATemplate(t *testing.T) {
	t.Parallel()
	s := newStore3(t)
	s.put("templates/tpl.md", "---\nkind: task\ntitle: t\nprops:\n  priority: Major\n  points: \"2\"\n  note: keep\nlabels: [bug, wip]\nchildren:\n- title: c\n  props:\n    due: 2026-11-09\n  labels: [UI, wip]\n---\n")
	tr, rep := s.ok(Options{})
	tp, err := model.DecodeTemplate("tpl", tr.Read("templates/tpl.md"))
	if err != nil {
		t.Fatal(err)
	}
	it := tp.Item
	if it.Priority != "major" || *it.Points != 2 || len(it.Props) != 1 || !slices.Equal(it.Labels, []string{"bug"}) {
		t.Errorf("item = %+v", it)
	}
	if c := it.Children[0]; c.Due.String() != "2026-11-09" || len(c.Props) != 0 || !slices.Equal(c.Labels, []string{"ui"}) {
		t.Errorf("child = %+v", c)
	}
	hasLine(t, rep, "テンプレート tpl の項目へ移した: 3 件")
	hasLine(t, rep, "ラベル wip を外した（schema.yaml の labels: にない）: タスク 0 件・テンプレート 1 件")
}

func TestStep4TakesTheTracesOfTheLabelSync(t *testing.T) {
	t.Parallel()
	s := newStore3(t)
	a := s.task("a", "")
	a.Links = []model.Link{{Type: "issue", Ref: "github.com/o/r#1", Sync: "push", Fields: []string{"title", "labels"}}}
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte("issues:\n- ref: github.com/o/r#1\n  state: open\n  synced: {title: x, labels: [bug]}\n"), &doc); err != nil {
		t.Fatal(err)
	}
	a.GH = doc.Content[0]
	enc, err := a.Encode()
	if err != nil {
		t.Fatal(err)
	}
	s.files[model.TaskPath(a.ULID)] = enc
	s.put("automations.yaml", "# mine\nissue_fields:\n  push: [title, labels]\n  mirror: [title, status] # keep\nfuture: 1\n")
	tr, rep := s.ok(Options{})
	got := taskIn(t, tr, a)
	if !slices.Equal(got.Links[0].Fields, []string{"title"}) {
		t.Errorf("link fields = %v", got.Links[0].Fields)
	}
	gh, _ := yaml.Marshal(got.GH)
	if strings.Contains(string(gh), "labels") || !strings.Contains(string(gh), "title: x") {
		t.Errorf("gh = %s", gh)
	}
	f, err := automation.Decode(tr.Read("automations.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(f.IssueFields.Push, []string{"title"}) || !slices.Equal(f.IssueFields.Mirror, []string{"title", "status"}) {
		t.Errorf("push %v, mirror %v", f.IssueFields.Push, f.IssueFields.Mirror)
	}
	if out := string(tr.Read("automations.yaml")); !strings.Contains(out, "# mine") || !strings.Contains(out, "# keep") || !strings.Contains(out, "future: 1") {
		t.Errorf("the comments and the keys it does not know were not kept:\n%s", out)
	}
	hasLine(t, rep, "automations.yaml の issue_fields の push・mirror から labels を外した")
	hasLine(t, rep, "GitHub のラベルの同期の跡")
}

func TestStep4LeavesAutomationsWithoutLabelsAlone(t *testing.T) {
	t.Parallel()
	s := newStore3(t)
	s.put("automations.yaml", "issue_fields:\n  push: [title]\n")
	tr, _ := s.ok(Options{})
	if _, ok := tr.Changes()["automations.yaml"]; ok {
		t.Error("automations.yaml was rewritten")
	}
}

func TestStep4FoldsTheListsThatDifferInCaseOnlyAndSkipsAnEmptyPriorities(t *testing.T) {
	t.Parallel()
	s := newStore3(t)
	s.put("schema.yaml", "priorities: [High, high, Low]\nlabels: [Bug, bug, ui]\n")
	a := s.task("a", "priority: HIGH\n")
	a.Labels = []string{"BUG", "ui"}
	b, err := a.Encode()
	if err != nil {
		t.Fatal(err)
	}
	s.files[model.TaskPath(a.ULID)] = b
	tr, rep := s.ok(Options{})
	got := taskIn(t, tr, a)
	if got.Priority != "High" || !slices.Equal(got.Labels, []string{"Bug", "ui"}) {
		t.Errorf("priority %q, labels %v", got.Priority, got.Labels)
	}
	sc, err := schema.Decode(tr.Read("schema.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(sc.Priorities, []string{"High", "Low"}) || !slices.Equal(sc.Labels, []string{"Bug", "ui"}) {
		t.Errorf("priorities %v, labels %v", sc.Priorities, sc.Labels)
	}
	hasLine(t, rep, "priorities: の大文字小文字だけ違う値を、先に書かれた綴りに畳んだ")
	hasLine(t, rep, "labels: の大文字小文字だけ違う値を、先に書かれた綴りに畳んだ")

	s = newStore3(t)
	s.put("schema.yaml", "priorities: []\nprops:\n  priority: {type: enum, values: [minor, major]}\n")
	a = s.task("a", "priority: Major\n")
	tr, rep = s.ok(Options{})
	if got := taskIn(t, tr, a); got.Priority != "major" {
		t.Errorf("priority = %q; an empty list falls through to the definition", got.Priority)
	}
	hasLine(t, rep, "priorities: が空だったので、次の出どころを使った: minor, major")
}

func TestStep4ListsTheLinksItStopsFromSyncing(t *testing.T) {
	t.Parallel()
	s := newStore3(t)
	a := s.task("a", "")
	a.Links = []model.Link{{Type: "issue", Ref: "github.com/o/r#1", Sync: "push", Fields: []string{"labels"}}}
	b, err := a.Encode()
	if err != nil {
		t.Fatal(err)
	}
	s.files[model.TaskPath(a.ULID)] = b
	s.put("templates/tpl.md", "---\nkind: task\ntitle: t\nlinks:\n- {type: issue, ref: 'github.com/o/r#9', sync: mirror, fields: [labels]}\n---\n")
	tr, rep := s.ok(Options{})
	if l := taskIn(t, tr, a).Links[0]; l.Sync != "link" || len(l.Fields) != 0 {
		t.Errorf("link = %+v", l)
	}
	hasLine(t, rep, "ラベルだけを同期していたリンクの sync を link（同期しない）にした: WEB-2 a: github.com/o/r#1")
	hasLine(t, rep, "ラベルだけを同期していたリンクの sync を link（同期しない）にした: template tpl: github.com/o/r#9")
}

func TestStep4CountsATemplateOnceForALabelOnItsItemAndItsChildren(t *testing.T) {
	t.Parallel()
	s := newStore3(t)
	s.put("templates/tpl.md", "---\nkind: task\ntitle: t\nlabels: [wip]\nchildren:\n- {title: a, labels: [wip]}\n- {title: b, labels: [wip]}\n---\n")
	_, rep := s.ok(Options{})
	hasLine(t, rep, "ラベル wip を外した（schema.yaml の labels: にない）: タスク 0 件・テンプレート 1 件")
}
