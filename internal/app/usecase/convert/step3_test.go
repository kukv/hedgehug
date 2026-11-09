package convert

import (
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/view"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// schema2 is a schema.yaml of format 2: the relations are task_ref properties,
// the store has one of its own (fixes) and a text property.
const schema2 = `statuses:
  - {name: todo, category: todo, default: true}
  - {name: done, category: done, default: true}
props:
  duplicates: {type: task_ref, multi: true, label: Duplicates, inverse: Duplicated by}
  causes: {type: task_ref, multi: true, label: Causes, inverse: Caused by}
  relates: {type: task_ref, multi: true, label: Relates to, symmetric: true}
  fixes: {type: task_ref, multi: true, label: Fixes, inverse: Fixed by}
  note: {type: text}
workflow:
  required:
    done: [fixes, note, causes]
`

// store2 is the files of a store in format 2.
type store2 struct {
	t     *testing.T
	files mapSource
	prj   *model.Task
	n     int
}

func newStore2(t *testing.T) *store2 {
	t.Helper()
	o := newOldStore(t)
	o.files["schema.yaml"] = []byte(schema2)
	o.files[model.StoreMetaPath] = []byte("ulid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nname: x\nformat: 2\n")
	s := &store2{t: t, files: o.files}
	s.prj = o.task("project", nil, "web")
	s.n = o.n
	return s
}

// task adds a task under the project, with the YAML props.
func (s *store2) task(title, props string) *model.Task {
	s.t.Helper()
	s.n++
	tk := &model.Task{
		ULID: model.DerivedULID(created, "task "+title), ID: model.FormatTaskID("WEB", s.n), Workspace: model.DerivedULID(created, "ws"),
		Kind: "task", Title: title, Status: "todo", Parent: &s.prj.ULID, Created: created, Updated: created,
	}
	tk.Rank, _ = model.Between("", "")
	if err := yaml.Unmarshal([]byte(props), &tk.Props); err != nil {
		s.t.Fatal(err)
	}
	b, err := tk.Encode()
	if err != nil {
		s.t.Fatal(err)
	}
	s.files[model.TaskPath(tk.ULID)] = b
	return tk
}

func (s *store2) put(path, content string) { s.files[path] = []byte(content) }

func (s *store2) run(opt Options) (*Tree, Report, error) {
	s.t.Helper()
	tr := NewTree(s.files)
	rep, err := step3(tr, opt)
	return tr, rep, err
}

func (s *store2) ok(opt Options) (*Tree, Report) {
	s.t.Helper()
	tr, rep, err := s.run(opt)
	if err != nil {
		s.t.Fatal(err)
	}
	return tr, rep
}

func lines(rep Report) string { return strings.Join(rep.Lines, "\n") }

func hasLine(t *testing.T, rep Report, want string) {
	t.Helper()
	if !strings.Contains(lines(rep), want) {
		t.Errorf("the report lacks %q:\n%s", want, lines(rep))
	}
}

func ulids(us ...*model.Task) []model.ULID {
	var out []model.ULID
	for _, u := range us {
		out = append(out, u.ULID)
	}
	return out
}

func TestStep3MovesTheValuesAndTheSchema(t *testing.T) {
	t.Parallel()
	s := newStore2(t)
	b := s.task("b", "")
	c := s.task("c", "")
	a := s.task("a", "duplicates: ["+b.ULID.String()+"]\ncauses: "+c.ULID.String()+"\nfixes: ["+b.ULID.String()+", "+c.ULID.String()+"]\nnote: keep\n")
	tr, rep := s.ok(Options{})
	got := taskIn(t, tr, a)
	if !slices.Equal(got.Duplicates, ulids(b)) || !slices.Equal(got.Causes, ulids(c)) || !slices.Equal(got.Relates, ulids(b, c)) {
		t.Errorf("duplicates %v, causes %v, relates %v", got.Duplicates, got.Causes, got.Relates)
	}
	if len(got.Props) != 1 || got.Props[0].Key != "note" {
		t.Errorf("props = %v", got.Props)
	}
	if !got.Updated.Equal(created) {
		t.Errorf("updated = %v; the step does not move it", got.Updated)
	}
	hasLine(t, rep, "fixes: 2 件を relates へ")
	// the other tasks are not rewritten
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
	hasLine(t, rep, "schema.yaml の task_ref のプロパティを消した")
}

func TestStep3WithoutTheDefinitionsMovesTheULIDsAndStopsAtTheOthers(t *testing.T) {
	t.Parallel()
	s := newStore2(t)
	s.put("schema.yaml", "statuses:\n  - {name: todo, category: todo, default: true}\n")
	b := s.task("b", "")
	a := s.task("a", "clones: ["+b.ULID.String()+"]\nrelates: ["+b.ULID.String()+", not-an-id]\n")
	// the names the schema no longer defines are checked as the others are
	tr, _, err := s.run(Options{})
	wantBad := usecase.AsError(err)
	if wantBad == nil || wantBad.Code != "convert.bad_value" || !strings.Contains(err.Error(), "props.relates: not-an-id") {
		t.Fatalf("error = %v", err)
	}
	if len(tr.Changes()) != 0 {
		t.Errorf("a step that stops wrote %v", tr.Changes())
	}
	tr, rep := s.ok(Options{DropBadValues: true})
	got := taskIn(t, tr, a)
	if !slices.Equal(got.Clones, ulids(b)) || !slices.Equal(got.Relates, ulids(b)) || len(got.Props) != 0 {
		t.Errorf("clones %v, relates %v, props %v; the ULID of a mixed value moves", got.Clones, got.Relates, got.Props)
	}
	hasLine(t, rep, "props.relates: not-an-id")
}

func TestStep3ATaskRefNamedClonesGoesToClones(t *testing.T) {
	t.Parallel()
	s := newStore2(t)
	s.put("schema.yaml", "props:\n  clones: {type: task_ref}\n")
	b := s.task("b", "")
	a := s.task("a", "clones: "+b.ULID.String()+"\n")
	tr, rep := s.ok(Options{})
	if got := taskIn(t, tr, a); !slices.Equal(got.Clones, ulids(b)) || len(got.Relates) != 0 {
		t.Errorf("clones %v, relates %v", got.Clones, got.Relates)
	}
	if strings.Contains(lines(rep), "を relates へ") {
		t.Errorf("clones is reported as a property of your own:\n%s", lines(rep))
	}
}

func TestStep3MovesASelfReferenceAndRewritesScopedRequired(t *testing.T) {
	t.Parallel()
	s := newStore2(t)
	s.put("schema.yaml", schema2+"scoped:\n  - where: project:"+model.NewULID().String()+"\n    workflow:\n      required:\n        done: [fixes, note, relates]\n")
	a := s.task("a", "")
	a.Props = nil
	if err := yaml.Unmarshal([]byte("causes: ["+a.ULID.String()+"]\n"), &a.Props); err != nil {
		t.Fatal(err)
	}
	enc, err := a.Encode()
	if err != nil {
		t.Fatal(err)
	}
	s.files[model.TaskPath(a.ULID)] = enc
	tr, _ := s.ok(Options{})
	if got := taskIn(t, tr, a).Causes; !slices.Equal(got, ulids(a)) {
		t.Errorf("causes = %v; a value that points at its own task moves as it is", got)
	}
	sc, err := schema.Decode(tr.Read("schema.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := sc.Scoped[0].Workflow.Required.Get("done"); !slices.Equal(got, []string{"relates", "note"}) {
		t.Errorf("scoped required of done = %v", got)
	}
}

func TestStep3CountsOnlyThePairsOfStoredRelates(t *testing.T) {
	t.Parallel()
	s := newStore2(t)
	b := s.task("b", "")
	a := s.task("a", "fixes: "+b.ULID.String()+"\n")
	// b holds a in relates, and a holds b through fixes of its own: that is no pair that was stored
	if err := yaml.Unmarshal([]byte("relates: ["+a.ULID.String()+"]\n"), &b.Props); err != nil {
		t.Fatal(err)
	}
	enc, err := b.Encode()
	if err != nil {
		t.Fatal(err)
	}
	s.files[model.TaskPath(b.ULID)] = enc
	_, rep := s.ok(Options{})
	if strings.Contains(lines(rep), "相互の組") {
		t.Errorf("a pair that came from a property of your own is counted:\n%s", lines(rep))
	}
}

func TestStep3TakesASortOrAGroupOfAGoneNameOutOfTheQuery(t *testing.T) {
	t.Parallel()
	s := newStore2(t)
	s.put(model.ViewPath("v"), "layout: table\nquery: 'sort:-fixes status:todo group:causes'\ntable:\n  group_by: relates\n")
	s.put(model.ViewPath("w"), "layout: table\nquery: '  sort:note group:duplicates  status:todo '\n")
	s.put(model.ViewPath("k"), "layout: table\nquery: 'sort:fixes'\n")
	tr, rep := s.ok(Options{})
	if got := viewOf(t, tr, "v"); got.Query != "status:todo" || got.Table.GroupBy != "" {
		t.Errorf("v: query %q, group_by %q", got.Query, got.Table.GroupBy)
	}
	if got := viewOf(t, tr, "w").Query; got != "  sort:note   status:todo " && got != "  sort:note  status:todo " {
		t.Errorf("w: query %q", got)
	}
	if got := viewOf(t, tr, "k").Query; got != "" {
		t.Errorf("k: query %q", got)
	}
	for _, w := range []string{"ビュー v の query から sort:-fixes を外した", "ビュー v の query から group:causes を外した", "ビュー v の table.group_by から", "ビュー w の query から group:duplicates を外した"} {
		hasLine(t, rep, w)
	}
}

func TestStep3RewritesTheDisplayNameOfTheOtherDirectionAndTheInverseOfYourOwn(t *testing.T) {
	t.Parallel()
	s := newStore2(t)
	s.put("schema.yaml", strings.Replace(schema2, "inverse: Fixed by", "inverse: fixed_by", 1))
	s.put(model.ViewPath("v"), "layout: tree\nquery: 'fixed_by:WEB-1 fixes.inverse:WEB-2 fixes:WEB-3 caused_by:WEB-4'\n")
	tr, _ := s.ok(Options{})
	if got := viewOf(t, tr, "v").Query; got != "related_by:WEB-1 related_by:WEB-2 relates:WEB-3 caused_by:WEB-4" {
		t.Errorf("query = %q", got)
	}
}

func TestStep3StopsWhereAPropertyOfAnotherTypeHasTheNameOfARelation(t *testing.T) {
	t.Parallel()
	s := newStore2(t)
	s.put("schema.yaml", "props:\n  clones: {type: text}\n  relates: {type: number}\n  fixes: {type: task_ref}\n")
	s.task("a", "clones: x\n")
	tr, _, err := s.run(Options{})
	if err == nil {
		t.Fatal("no error")
	}
	e := usecase.AsError(err)
	if e.Kind != usecase.KindInvalid || e.Code != "convert.prop_name_taken" {
		t.Fatalf("error = %v (kind %v, code %q)", err, e.Kind, e.Code)
	}
	for _, w := range []string{"clones (text)", "relates (number)"} {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("the error does not list %q: %v", w, err)
		}
	}
	if len(tr.Changes()) != 0 {
		t.Errorf("a step that stops wrote %v", tr.Changes())
	}
	// the names of the other direction are not stopped: doctor warns of them
	s2 := newStore2(t)
	s2.put("schema.yaml", schema2+"  blocks: {type: text}\n")
	if _, _, err := s2.run(Options{}); err != nil {
		t.Errorf("a property named like the other direction stopped the step: %v", err)
	}
}

func TestStep3KeepsBothSidesOfAMutualRelatesAndCountsThePairs(t *testing.T) {
	t.Parallel()
	s := newStore2(t)
	a := s.task("a", "")
	b := s.task("b", "")
	c := s.task("c", "")
	set := func(tk *model.Task, props string) {
		t.Helper()
		if err := yaml.Unmarshal([]byte(props), &tk.Props); err != nil {
			t.Fatal(err)
		}
		enc, err := tk.Encode()
		if err != nil {
			t.Fatal(err)
		}
		s.files[model.TaskPath(tk.ULID)] = enc
	}
	set(a, "relates: ["+b.ULID.String()+", "+c.ULID.String()+"]\n")
	set(b, "relates: ["+a.ULID.String()+"]\n")
	tr, rep := s.ok(Options{})
	if got := taskIn(t, tr, a).Relates; !slices.Equal(got, ulids(b, c)) {
		t.Errorf("a relates %v", got)
	}
	if got := taskIn(t, tr, b).Relates; !slices.Equal(got, ulids(a)) {
		t.Errorf("b relates %v", got)
	}
	hasLine(t, rep, "相互の組 1 件")
}

func TestStep3StopsAtAValueThatIsNoULIDAndListsIt(t *testing.T) {
	t.Parallel()
	s := newStore2(t)
	good := s.task("good", "")
	s.task("bad", "causes: ["+good.ULID.String()+", WEB-3]\nfixes: nope\n")
	s.put(model.TemplatePath("tpl"), "---\ntitle: tpl\nprops:\n  duplicates: [oops]\n---\n")
	tr, _, err := s.run(Options{})
	e := usecase.AsError(err)
	if e == nil || e.Kind != usecase.KindInvalid || e.Code != "convert.bad_value" {
		t.Fatalf("error = %v", err)
	}
	for _, w := range []string{"bad", "props.causes: WEB-3", "props.fixes: nope", "template tpl", "--drop-bad-values"} {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("the error lacks %q:\n%v", w, err)
		}
	}
	if len(tr.Changes()) != 0 {
		t.Errorf("a step that stops wrote %v", tr.Changes())
	}
}

func TestStep3DropsTheValuesThatAreNoULIDWhenToldTo(t *testing.T) {
	t.Parallel()
	s := newStore2(t)
	good := s.task("good", "")
	bad := s.task("bad", "causes: ["+good.ULID.String()+", WEB-3]\nfixes: nope\n")
	tr, rep := s.ok(Options{DropBadValues: true})
	got := taskIn(t, tr, bad)
	if !slices.Equal(got.Causes, ulids(good)) || len(got.Relates) != 0 || len(got.Props) != 0 {
		t.Errorf("causes %v, relates %v, props %v", got.Causes, got.Relates, got.Props)
	}
	hasLine(t, rep, "ULID の形でない値を外した: ")
	hasLine(t, rep, "bad: props.causes: WEB-3")
	hasLine(t, rep, "bad: props.fixes: nope")
}

func TestStep3MovesAReferenceToATaskThatIsGone(t *testing.T) {
	t.Parallel()
	s := newStore2(t)
	gone := model.NewULID()
	a := s.task("a", "causes: ["+gone.String()+"]\n")
	tr, _ := s.ok(Options{}) // as blocked_by, a value that names no task is kept
	if got := taskIn(t, tr, a).Causes; !slices.Equal(got, []model.ULID{gone}) {
		t.Errorf("causes = %v", got)
	}
}

func TestStep3ConvertsTheTemplates(t *testing.T) {
	t.Parallel()
	s := newStore2(t)
	x, y := model.NewULID(), model.NewULID()
	s.put(model.TemplatePath("tpl"), "---\ntitle: tpl\nprops:\n  causes: ["+x.String()+"]\n  note: keep\nchildren:\n  - title: child\n    props:\n      fixes: "+y.String()+"\n---\nbody\n")
	s.put(model.TemplatePath("plain"), "---\ntitle: plain\n---\n")
	tr, rep := s.ok(Options{})
	tp, err := model.DecodeTemplate("tpl", tr.Read(model.TemplatePath("tpl")))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(tp.Item.Causes, []model.ULID{x}) || tp.Item.Props.Get("causes") != nil || tp.Item.Props.Get("note") == nil {
		t.Errorf("item: causes %v, props %v", tp.Item.Causes, tp.Item.Props)
	}
	if kid := tp.Item.Children[0]; !slices.Equal(kid.Relates, []model.ULID{y}) || len(kid.Props) != 0 {
		t.Errorf("child: relates %v, props %v", kid.Relates, kid.Props)
	}
	if _, ok := tr.Changes()[model.TemplatePath("plain")]; ok {
		t.Error("a template with nothing to move was rewritten")
	}
	hasLine(t, rep, "テンプレート tpl")
}

func viewOf(t *testing.T, tr *Tree, name string) *view.View {
	t.Helper()
	v, err := view.Decode(name, tr.Read(model.ViewPath(name)))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestStep3RewritesTheViews(t *testing.T) {
	t.Parallel()
	s := newStore2(t)
	s.put(model.ViewPath("v"), "layout: table\nquery: 'causes.inverse:WEB-1  blocked_by.inverse:me -fixes:WEB-2 fixes.inverse:WEB-3 \"fixes.x\" status:todo relates.inverse:WEB-4'\ntable:\n  columns: [id, title, fixes, duplicates, causes, relates, due, note]\n")
	s.put(model.ViewPath("keep"), "layout: tree\nquery: status:todo\n")
	s.put(model.ViewPath("rel"), "layout: tree\nquery: relates:WEB-1 -relates:WEB-2 text\n")
	tr, rep := s.ok(Options{})
	v := viewOf(t, tr, "v")
	if want := `caused_by:WEB-1  blocks:me -relates:WEB-2 related_by:WEB-3 "fixes.x" status:todo related_by:WEB-4`; v.Query != want {
		t.Errorf("query = %q\n want    %q", v.Query, want)
	}
	if got := v.Table.Columns; !slices.Equal(got, []string{"id", "title", "due", "note"}) {
		t.Errorf("columns = %v", got)
	}
	if _, ok := tr.Changes()[model.ViewPath("keep")]; ok {
		t.Error("a view with nothing to rewrite was rewritten")
	}
	// relates:X meant both directions and does not now: it is listed, not rewritten
	if r := viewOf(t, tr, "rel"); r.Query != "relates:WEB-1 -relates:WEB-2 text" {
		t.Errorf("a relates: term was rewritten: %q", r.Query)
	}
	hasLine(t, rep, "ビュー rel の query の relates:WEB-1")
	hasLine(t, rep, "ビュー rel の query の -relates:WEB-2")
	hasLine(t, rep, "ビュー v の table.columns")
}

func TestStep3RewritesWorkflowRequired(t *testing.T) {
	t.Parallel()
	s := newStore2(t)
	tr, rep := s.ok(Options{})
	sc, err := schema.Decode(tr.Read("schema.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := sc.Workflow.Required.Get("done")
	if !slices.Equal(got, []string{"relates", "note", "causes"}) {
		t.Errorf("required of done = %v", got)
	}
	hasLine(t, rep, "workflow.required")
}

func TestStep3ReportsEachTaskRefOfTheStoreWithNothingToMove(t *testing.T) {
	t.Parallel()
	s := newStore2(t)
	_, rep := s.ok(Options{})
	hasLine(t, rep, "fixes: 0 件を relates へ")
}
