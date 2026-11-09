package convert

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/merge"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
)

// mapSource is a Source of a map.
type mapSource map[string][]byte

func (m mapSource) Read(p string) []byte { return m[p] }
func (m mapSource) Paths(prefix string) []string {
	var out []string
	for p := range m {
		if strings.HasPrefix(p, prefix) {
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return out
}

var created = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// oldStore builds the files of a store in format 1.
type oldStore struct {
	t     *testing.T
	files mapSource
	ws    *model.Workspace
	n     int
}

const oldSchema = `statuses:
  - {name: todo, category: todo, default: true}
  - {name: in_progress, category: doing, default: true}
  - {name: done, category: done, default: true}
  - {name: canceled, category: canceled, default: true}
kinds:
  - {name: project, role: container, parents: [root]}
  - {name: epic, role: container, parents: [root, project]}
  - {name: story, role: work, parents: [root, project, epic]}
  - {name: task, role: work, parents: [root, project, epic, story, task], default: true, template: tmpl-task}
  - {name: bug, role: work, template: tmpl-bug}
  - {name: area, role: container, template: tmpl-area}
  - {name: chore, role: work, template: tmpl-chore}
`

func newOldStore(t *testing.T) *oldStore {
	t.Helper()
	s := &oldStore{t: t, files: mapSource{}}
	s.ws = &model.Workspace{ULID: model.DerivedULID(created, "ws"), Name: "web", IDPrefix: "WEB", Created: created, Updated: created, PrefixSet: created}
	b, err := s.ws.Encode()
	if err != nil {
		t.Fatal(err)
	}
	s.files[model.WorkspacePath(s.ws.ULID)] = b
	s.files["schema.yaml"] = []byte(oldSchema)
	s.files[model.StoreMetaPath] = []byte("ulid: 01JBZ8K3QW7M2X9V4T6R5N8P0A\nname: x\nformat: 1\n")
	return s
}

// task adds a task of kind under parent (nil for the root) and returns it.
func (s *oldStore) task(kind string, parent *model.Task, title string) *model.Task {
	s.t.Helper()
	s.n++
	t := &model.Task{
		ULID: model.DerivedULID(created, "task "+title), ID: model.FormatTaskID("WEB", s.n), Workspace: s.ws.ULID,
		Kind: kind, Title: title, Status: "todo", Created: created, Updated: created,
	}
	if parent != nil {
		t.Parent = &parent.ULID
	}
	// Ranks ascend in the order the tasks are made, among all of them.
	var rank model.Rank
	for _, b := range s.files.Paths("tasks/") {
		if o, err := model.DecodeTask(s.files[b]); err == nil && o.Rank > rank {
			rank = o.Rank
		}
	}
	t.Rank, _ = model.Between(rank, "")
	b, err := t.Encode()
	if err != nil {
		s.t.Fatal(err)
	}
	s.files[model.TaskPath(t.ULID)] = b
	return t
}

func (s *oldStore) template(name, kind string, childKinds ...string) {
	s.t.Helper()
	tp := &model.Template{Name: name, Item: model.TemplateItem{Kind: kind, Title: name}}
	for _, k := range childKinds {
		tp.Item.Children = append(tp.Item.Children, model.TemplateItem{Kind: k, Title: "c"})
	}
	b, err := tp.Encode()
	if err != nil {
		s.t.Fatal(err)
	}
	s.files[model.TemplatePath(name)] = b
}

var testNow = time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)

// apply runs step 1→2 and returns the tree and what it reported.
func (s *oldStore) apply(opt Options) (*Tree, Report) {
	s.t.Helper()
	tr := NewTree(s.files)
	opt.Now = testNow
	rep, err := step2(tr, opt)
	if err != nil {
		s.t.Fatal(err)
	}
	return tr, rep
}

func taskIn(t *testing.T, tr *Tree, orig *model.Task) *model.Task {
	t.Helper()
	b := tr.Read(model.TaskPath(orig.ULID))
	got, err := model.DecodeTask(b)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func want(t *testing.T, tr *Tree, orig *model.Task, kind string, parent *model.Task) {
	t.Helper()
	got := taskIn(t, tr, orig)
	if got.Kind != kind {
		t.Errorf("%s: kind = %q, want %q", orig.Title, got.Kind, kind)
	}
	switch {
	case parent == nil && got.Parent != nil:
		t.Errorf("%s: parent = %v, want the root", orig.Title, got.Parent)
	case parent != nil && (got.Parent == nil || *got.Parent != parent.ULID):
		t.Errorf("%s: parent = %v, want %s", orig.Title, got.Parent, parent.Title)
	}
}

func TestStep2CustomKinds(t *testing.T) {
	t.Parallel()
	s := newOldStore(t)
	top := s.task("area", nil, "top-area")  // group at the root: project
	sub := s.task("area", top, "sub-area")  // group below: epic
	item := s.task("chore", sub, "a chore") // item: task
	prj := s.task("project", nil, "plain")  // same name as a fixed kind: kept
	st := s.task("story", prj, "a story")   //
	tr, rep := s.apply(Options{})
	want(t, tr, top, "project", nil)
	want(t, tr, sub, "epic", top)
	want(t, tr, item, "task", sub)
	want(t, tr, prj, "project", nil)
	want(t, tr, st, "story", prj)
	if len(rep.Lines) == 0 {
		t.Error("the report is empty")
	}
}

func TestStep2KindFlag(t *testing.T) {
	t.Parallel()
	s := newOldStore(t)
	prj := s.task("project", nil, "p")
	a := s.task("chore", prj, "chore")
	b := s.task("area", prj, "area")
	tr, _ := s.apply(Options{Kinds: map[string]string{"chore": "bug", "area": "epic"}})
	want(t, tr, a, "bug", prj)
	want(t, tr, b, "epic", prj)

	// the target must be a fixed kind, the source one the store has
	for _, kinds := range []map[string]string{{"chore": "nope"}, {"missing": "task"}} {
		if _, err := step2(NewTree(s.files), Options{Kinds: kinds}); err == nil {
			t.Errorf("--kind %v: no error", kinds)
		}
	}
}

func TestStep2PlacementUnderItems(t *testing.T) {
	t.Parallel()
	s := newOldStore(t)
	prj := s.task("project", nil, "p")
	story := s.task("story", prj, "story")
	underStory := s.task("task", story, "task under story") // step
	deep := s.task("task", underStory, "task under that")   // lifted to the story, a step
	deeper := s.task("bug", deep, "bug under that")         // lifted to the story, a step
	steps := s.task("step", prj, "a step under a project")  // a task: the step has no place there
	tr, rep := s.apply(Options{})
	want(t, tr, story, "story", prj)
	want(t, tr, underStory, "step", story)
	want(t, tr, deep, "step", story)
	want(t, tr, deeper, "step", story)
	want(t, tr, steps, "task", prj)
	joined := strings.Join(rep.Lines, "\n")
	for _, w := range []string{"step にした", "上げた"} {
		if !strings.Contains(joined, w) {
			t.Errorf("the report lacks %q:\n%s", w, joined)
		}
	}
}

func TestStep2PlacementOfGroups(t *testing.T) {
	t.Parallel()
	s := newOldStore(t)
	p1 := s.task("project", nil, "p1")
	p2 := s.task("project", p1, "p2 under p1")        // up to the root
	e1 := s.task("epic", p2, "e1")                    //
	e2 := s.task("epic", e1, "e2 under e1")           // up to p2
	pe := s.task("project", e1, "project under epic") // up to p2, then the root
	tr, _ := s.apply(Options{})
	want(t, tr, p2, "project", nil)
	want(t, tr, e1, "epic", p2)
	want(t, tr, e2, "epic", p2)
	want(t, tr, pe, "project", nil)
}

func TestStep2RootTasksGoUnderUncategorized(t *testing.T) {
	t.Parallel()
	s := newOldStore(t)
	prj := s.task("project", nil, "p")
	a := s.task("task", nil, "root task")
	b := s.task("epic", nil, "root epic")
	c := s.task("step", nil, "root step")
	tr, rep := s.apply(Options{})
	u := model.DerivedULID(created, "uncategorized "+s.ws.ULID.String())
	got, err := model.DecodeTask(tr.Read(model.TaskPath(u)))
	if err != nil {
		t.Fatalf("the 未分類 project is not there: %v", err)
	}
	if got.Title != merge.Uncategorized || got.Kind != "project" || got.Parent != nil || got.Workspace != s.ws.ULID {
		t.Errorf("project = %+v", got)
	}
	if got.Status != "todo" || !got.Created.Equal(created) {
		t.Errorf("status %q, created %v", got.Status, got.Created)
	}
	if got.ID != "WEB-5" { // the next number: the four tasks above are WEB-1 to WEB-4
		t.Errorf("id = %q, want WEB-5", got.ID)
	}
	want(t, tr, prj, "project", nil)
	want(t, tr, a, "task", got)
	want(t, tr, b, "epic", got)
	want(t, tr, c, "task", got)
	if !strings.Contains(strings.Join(rep.Lines, "\n"), "未分類") {
		t.Errorf("the report does not name 未分類: %v", rep.Lines)
	}
	// the ranks under it keep the order the tasks had
	ra, rb, rc := taskIn(t, tr, a).Rank, taskIn(t, tr, b).Rank, taskIn(t, tr, c).Rank
	if ra >= rb || rb >= rc {
		t.Errorf("ranks %q %q %q do not keep the order", ra, rb, rc)
	}
}

func TestStep2KeepsAnUncategorizedProjectThatIsThere(t *testing.T) {
	t.Parallel()
	s := newOldStore(t)
	have := s.task("project", nil, merge.Uncategorized)
	a := s.task("task", nil, "root task")
	tr, _ := s.apply(Options{})
	want(t, tr, a, "task", have)
	for _, p := range tr.Paths("tasks/") {
		if u, _ := model.ParseTaskPath(p); u != have.ULID && u != a.ULID {
			t.Errorf("another task %s was made", p)
		}
	}
}

func TestStep2NothingToMoveMakesNoProject(t *testing.T) {
	t.Parallel()
	s := newOldStore(t)
	prj := s.task("project", nil, "p")
	s.task("task", prj, "t")
	tr, _ := s.apply(Options{})
	if n := len(tr.Paths("tasks/")); n != 2 {
		t.Errorf("%d task files, want 2", n)
	}
	if ch := tr.Changes(); len(ch) != 1 || ch["schema.yaml"] == nil {
		t.Errorf("changes = %v, want only schema.yaml", slices.Sorted(keysOf(ch)))
	}
}

func keysOf(m map[string][]byte) func(func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

func TestStep2Schema(t *testing.T) {
	t.Parallel()
	s := newOldStore(t)
	s.task("project", nil, "p")
	s.files["schema.yaml"] = append(s.files["schema.yaml"], []byte(`scoped:
  - where: project:`+model.DerivedULID(created, "task p").String()+`
    templates:
      default: {area: scoped-area, chore: scoped-chore}
`)...)
	tr, rep := s.apply(Options{Kinds: map[string]string{"chore": "bug"}})
	sc, err := schema.Decode(tr.Read("schema.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if sc.Extra.Get("kinds") != nil {
		t.Error("kinds is still in schema.yaml")
	}
	if sc.Templates == nil {
		t.Fatal("no templates.default")
	}
	// task and bug come from their own kinds; area is a group below the
	// root as far as a kind-wide default goes: epic. chore went to bug by
	// --kind, and bug has a template of its own, which wins.
	got := sc.Templates.Default
	if got["task"] != "tmpl-task" || got["bug"] != "tmpl-bug" || got["epic"] != "tmpl-area" {
		t.Errorf("templates.default = %v", got)
	}
	if d := sc.Scoped[0].Templates.Default; d["epic"] != "scoped-area" || d["bug"] != "scoped-chore" || len(d) != 2 {
		t.Errorf("scoped templates.default = %v", d)
	}
	if ps := sc.Check(); len(ps) > 0 {
		t.Errorf("the schema does not check: %v", ps)
	}
	if !strings.Contains(strings.Join(rep.Lines, "\n"), "templates.default") {
		t.Errorf("the report does not tell of templates.default: %v", rep.Lines)
	}
}

// A templates.only of the store was checked and never in effect; it is taken
// out, and the report says so. The one of a project's override stays.
func TestStep2DropsTemplatesOnlyOfTheStore(t *testing.T) {
	t.Parallel()
	s := newOldStore(t)
	s.task("project", nil, "p")
	s.files["schema.yaml"] = append(s.files["schema.yaml"], []byte(`templates:
  only: [tmpl-task, tmpl-bug]
scoped:
  - where: project:`+model.DerivedULID(created, "task p").String()+`
    templates:
      only: [tmpl-bug]
`)...)
	tr, rep := s.apply(Options{})
	sc, err := schema.Decode(tr.Read("schema.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if sc.Templates != nil && len(sc.Templates.Only) > 0 {
		t.Errorf("templates.only is still there: %v", sc.Templates.Only)
	}
	if got := sc.Scoped[0].Templates.Only; len(got) != 1 || got[0] != "tmpl-bug" {
		t.Errorf("the override's only = %v", got)
	}
	if ps := sc.Check(); len(ps) > 0 {
		t.Errorf("the schema does not check: %v", ps)
	}
	if !strings.Contains(strings.Join(rep.Lines, "\n"), "templates.only") {
		t.Errorf("the report does not tell of templates.only: %v", rep.Lines)
	}
}

func TestStep2Templates(t *testing.T) {
	t.Parallel()
	s := newOldStore(t)
	s.task("project", nil, "p")
	s.template("w", "area", "chore", "story", "")
	s.template("untouched", "task")
	tr, rep := s.apply(Options{})
	tp, err := model.DecodeTemplate("w", tr.Read(model.TemplatePath("w")))
	if err != nil {
		t.Fatal(err)
	}
	if tp.Item.Kind != "epic" {
		t.Errorf("root kind = %q, want epic", tp.Item.Kind)
	}
	var kinds []string
	for _, c := range tp.Item.Children {
		kinds = append(kinds, c.Kind)
	}
	if !slices.Equal(kinds, []string{"task", "story", ""}) {
		t.Errorf("children kinds = %v", kinds)
	}
	if string(tr.Read(model.TemplatePath("untouched"))) != string(s.files[model.TemplatePath("untouched")]) {
		t.Error("a template with fixed kinds was rewritten")
	}
	if !strings.Contains(strings.Join(rep.Lines, "\n"), "テンプレート w") {
		t.Errorf("the report does not name the template: %v", rep.Lines)
	}
}

func TestStep2StampsWhatItChanges(t *testing.T) {
	t.Parallel()
	s := newOldStore(t)
	prj := s.task("project", nil, "p")
	moved := s.task("task", nil, "moved")
	kept := s.task("task", prj, "kept")
	tr, _ := s.apply(Options{})
	if got := taskIn(t, tr, moved).Updated; !got.Equal(testNow) {
		t.Errorf("a changed task: updated = %v", got)
	}
	if string(tr.Read(model.TaskPath(kept.ULID))) != string(s.files[model.TaskPath(kept.ULID)]) {
		t.Error("a task that did not change was rewritten")
	}
}

func TestStep2LeavesWhatItCannotReadAlone(t *testing.T) {
	t.Parallel()
	s := newOldStore(t)
	prj := s.task("project", nil, "p")
	orphan := s.task("task", prj, "orphan")
	orphan.Parent = ptr(model.DerivedULID(created, "gone")) // its parent is purged
	b, _ := orphan.Encode()
	s.files[model.TaskPath(orphan.ULID)] = b
	broken := model.TaskPath(model.DerivedULID(created, "broken"))
	s.files[broken] = []byte("not a task")
	tr, rep := s.apply(Options{})
	if string(tr.Read(broken)) != "not a task" {
		t.Error("a broken file was changed")
	}
	if got := taskIn(t, tr, orphan); got.Parent == nil || *got.Parent != *orphan.Parent {
		t.Errorf("a task whose parent is gone moved: %v", got.Parent)
	}
	if !strings.Contains(strings.Join(rep.Lines, "\n"), broken) {
		t.Errorf("the report does not name the broken file: %v", rep.Lines)
	}
}

// A deleted task is placed as a live one is, so that restoring it breaks no
// rule: a workspace with nothing at its root but a deleted task still gets
// the 未分類 project.
func TestStep2MovesDeletedTasksToo(t *testing.T) {
	t.Parallel()
	s := newOldStore(t)
	gone := s.task("task", nil, "deleted at the root")
	gone.DeletedAt = &created
	b, err := gone.Encode()
	if err != nil {
		t.Fatal(err)
	}
	s.files[model.TaskPath(gone.ULID)] = b
	tr, _ := s.apply(Options{})
	u := model.DerivedULID(created, "uncategorized "+s.ws.ULID.String())
	unc, err := model.DecodeTask(tr.Read(model.TaskPath(u)))
	if err != nil || unc.DeletedAt != nil {
		t.Fatalf("the 未分類 project is not there and live: %v, %v", unc, err)
	}
	want(t, tr, gone, "task", unc)
	if taskIn(t, tr, gone).DeletedAt == nil {
		t.Error("the task is not deleted any more")
	}
}

func ptr[T any](v T) *T { return &v }
