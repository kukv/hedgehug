package ops

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

// addProject adds a task of kind project and returns its ULID.
func (ts *testSession) addProject(title string) model.ULID {
	ts.t.Helper()
	r, err := ts.Add(AddInput{Title: title, Kind: "project"})
	if err != nil {
		ts.t.Fatal(err)
	}
	return r.ULID
}

// storeSchema is schema.yaml of the test store, read from the file.
func (ts *testSession) storeSchema() (*schema.Schema, string) {
	ts.t.Helper()
	_, e, err := ts.Entry()
	if err != nil {
		ts.t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(e.Path, "schema.yaml")) //nolint:gosec // a file of the test's own store
	if err != nil {
		ts.t.Fatal(err)
	}
	sc, err := schema.Decode(b)
	if err != nil {
		ts.t.Fatal(err)
	}
	return sc, string(b)
}

var fourStatuses = []string{"todo", "in_progress", "done", "canceled"}

func TestScopedSetReplacesOnlyWhatIsGiven(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	p := ts.addProject("Web")

	before := ts.head()
	statuses := slices.Clone(fourStatuses)
	r, err := ts.ScopedSet(p.String(), schema.ScopedChange{Statuses: &statuses})
	if err != nil || !r.Committed || r.Commit == "" || r.Scoped == nil {
		t.Fatalf("ScopedSet = %+v, %v", r, err)
	}
	if ts.head() == before || lastOp(t, ts) != "schema" {
		t.Errorf("want one commit of op schema, head %s -> %s, op %q", before, ts.head(), lastOp(t, ts))
	}
	if v := r.View(); v.Project.ULID != p.String() || v.Project.Title != "Web" || v.Scoped == nil || !slices.Equal(v.Scoped.Statuses, fourStatuses) || v.Commit == nil {
		t.Errorf("View = %+v", v)
	}

	transitions := schema.Lists{{Key: "todo", Values: []string{"in_progress"}}}
	if _, err := ts.ScopedSet(p.String(), schema.ScopedChange{Transitions: &transitions}); err != nil {
		t.Fatal(err)
	}
	required := schema.Lists{{Key: "done", Values: []string{"points"}}}
	if _, err := ts.ScopedSet(p.String(), schema.ScopedChange{Required: &required}); err != nil {
		t.Fatal(err)
	}
	sc, _ := ts.storeSchema()
	got, ok := sc.ScopedFor(p)
	if !ok || !slices.Equal(got.Statuses, fourStatuses) || got.Workflow == nil ||
		len(got.Workflow.Transitions) != 1 || len(got.Workflow.Required) != 1 {
		t.Fatalf("override = %+v", got)
	}

	// the same change again writes nothing
	head := ts.head()
	r, err = ts.ScopedSet(p.String(), schema.ScopedChange{Required: &required})
	if err != nil || r.Committed || r.Commit != "" || ts.head() != head {
		t.Errorf("same change = %+v, %v", r, err)
	}
	if r.View().Commit != nil {
		t.Errorf("commit = %v, want null", r.View().Commit)
	}
}

func TestScopedSetEmptyGoesBackToTheStore(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	p := ts.addProject("Web")
	statuses := slices.Clone(fourStatuses)
	transitions := schema.Lists{{Key: "todo", Values: []string{"in_progress"}}}
	required := schema.Lists{{Key: "done", Values: []string{"points"}}}
	if _, err := ts.ScopedSet(p.String(), schema.ScopedChange{Statuses: &statuses, Transitions: &transitions, Required: &required}); err != nil {
		t.Fatal(err)
	}

	none := []string{}
	if _, err := ts.ScopedSet(p.String(), schema.ScopedChange{Statuses: &none}); err != nil {
		t.Fatal(err)
	}
	sc, _ := ts.storeSchema()
	got, _ := sc.ScopedFor(p)
	if got.Statuses != nil || got.Workflow == nil || len(got.Workflow.Transitions) != 1 {
		t.Fatalf("after clearing statuses: %+v", got)
	}

	noLists := schema.Lists{}
	if _, err := ts.ScopedSet(p.String(), schema.ScopedChange{Required: &noLists}); err != nil {
		t.Fatal(err)
	}
	r, err := ts.ScopedSet(p.String(), schema.ScopedChange{Transitions: &noLists})
	if err != nil || !r.Committed || r.Scoped != nil {
		t.Fatalf("last part: %+v, %v", r, err)
	}
	sc, _ = ts.storeSchema()
	if len(sc.Scoped) != 0 {
		t.Errorf("an override with nothing left is still there: %+v", sc.Scoped)
	}
	if v := r.View(); v.Scoped != nil {
		t.Errorf("View.Scoped = %+v, want null", v.Scoped)
	}
}

func TestScopedSetKeepsTemplatesAndUnknownKeys(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	p := ts.addProject("Web")
	_, text := ts.storeSchema()
	ts.commitRawFile("schema.yaml", text+"scoped:\n  - where: project:"+p.String()+
		"\n    statuses: [todo, in_progress, done, canceled]\n    note: keep me\n    templates:\n      only: [bug]\n")

	required := schema.Lists{{Key: "done", Values: []string{"points"}}}
	if _, err := ts.ScopedSet(p.String(), schema.ScopedChange{Required: &required}); err != nil {
		t.Fatal(err)
	}
	none := []string{}
	if _, err := ts.ScopedSet(p.String(), schema.ScopedChange{Statuses: &none}); err != nil {
		t.Fatal(err)
	}
	noLists := schema.Lists{}
	r, err := ts.ScopedSet(p.String(), schema.ScopedChange{Required: &noLists})
	if err != nil || r.Scoped == nil || r.Scoped.Templates == nil {
		t.Fatalf("the templates keep the override: %+v, %v", r, err)
	}
	_, text = ts.storeSchema()
	if !strings.Contains(text, "note: keep me") || !strings.Contains(text, "only: [bug]") || strings.Contains(text, "required") {
		t.Errorf("schema.yaml:\n%s", text)
	}
}

func TestScopedSetLeavesEmptyScopedListBehind(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	p := ts.addProject("Web")
	statuses := slices.Clone(fourStatuses)
	if _, err := ts.ScopedSet(p.String(), schema.ScopedChange{Statuses: &statuses}); err != nil {
		t.Fatal(err)
	}
	none := []string{}
	if _, err := ts.ScopedSet(p.String(), schema.ScopedChange{Statuses: &none}); err != nil {
		t.Fatal(err)
	}
	if _, text := ts.storeSchema(); !strings.Contains(text, "scoped: []") {
		t.Errorf("schema.yaml:\n%s", text)
	}
}

func TestScopedSetOnABrokenSchemaIsScopedInvalid(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	p := ts.addProject("Web")
	_, text := ts.storeSchema()
	// a transition to a status that does not exist: the schema is broken before the change
	ts.commitRawFile("schema.yaml", text+"scoped:\n  - where: project:"+p.String()+"\n    workflow:\n      transitions:\n        todo: [nope]\n")
	head := ts.head()
	statuses := slices.Clone(fourStatuses)
	_, err := ts.ScopedSet(p.String(), schema.ScopedChange{Statuses: &statuses})
	e := AsError(err)
	if e == nil || e.Kind != KindInvalid || e.Code != "scoped.invalid" {
		t.Fatalf("err = %v", err)
	}
	if ps, _ := e.Args["problems"].([]string); len(ps) == 0 {
		t.Errorf("Args = %+v, want the problems", e.Args)
	}
	if ts.head() != head {
		t.Error("a refused change wrote a commit")
	}
}

func TestScopedSetRefusesWhatSchemaCheckRefuses(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	p := ts.addProject("Web")
	head := ts.head()
	for name, ch := range map[string]schema.ScopedChange{
		"a default status hidden": {Statuses: &[]string{"todo", "in_progress", "canceled"}},
		"an unknown status":       {Statuses: &[]string{"todo", "in_progress", "done", "canceled", "nope"}},
		"an unknown transition":   {Transitions: &schema.Lists{{Key: "todo", Values: []string{"nope"}}}},
		"an unknown field":        {Required: &schema.Lists{{Key: "done", Values: []string{"nope"}}}},
	} {
		_, err := ts.ScopedSet(p.String(), ch)
		e := AsError(err)
		if e == nil || e.Kind != KindInvalid || e.Code != "scoped.invalid" {
			t.Errorf("%s: err = %v", name, err)
			continue
		}
		if ps, _ := e.Args["problems"].([]string); len(ps) == 0 {
			t.Errorf("%s: Args = %+v, want the problems", name, e.Args)
		}
	}
	if ts.head() != head {
		t.Error("a refused change wrote a commit")
	}
}

func TestScopedSetNeedsAContainer(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	r, err := ts.Add(AddInput{Title: "plain"}) // the default kind is task
	if err != nil {
		t.Fatal(err)
	}
	none := []string{}
	_, err = ts.ScopedSet(r.ULID.String(), schema.ScopedChange{Statuses: &none})
	if e := AsError(err); e == nil || e.Kind != KindInvalid || e.Code != "scoped.not_container" || e.Args["kind"] != "task" {
		t.Errorf("task: %v", err)
	}
	// an epic is a container too
	ep, err := ts.Add(AddInput{Title: "epic", Kind: "epic"})
	if err != nil {
		t.Fatal(err)
	}
	statuses := slices.Clone(fourStatuses)
	if _, err := ts.ScopedSet(ep.ULID.String(), schema.ScopedChange{Statuses: &statuses}); err != nil {
		t.Errorf("epic: %v", err)
	}
	_, err = ts.ScopedSet("NOPE-9", schema.ScopedChange{Statuses: &statuses})
	if KindOf(err) != KindNotFound {
		t.Errorf("unknown: %v", err)
	}
	p := ts.addProject("Gone")
	if _, err := ts.Remove(p.String()); err != nil {
		t.Fatal(err)
	}
	_, err = ts.ScopedSet(p.String(), schema.ScopedChange{Statuses: &statuses})
	if e := AsError(err); e == nil || e.Code != "task.deleted" {
		t.Errorf("deleted: %v", err)
	}
}

func TestScopedRemove(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	p := ts.addProject("Web")
	other := ts.addProject("Docs")
	_, text := ts.storeSchema()
	ts.commitRawFile("schema.yaml", text+"scoped:\n  - where: project:"+p.String()+
		"\n    statuses: [todo, in_progress, done, canceled]\n    templates:\n      only: [bug]\n  - where: project:"+other.String()+
		"\n    statuses: [todo, in_progress, done, canceled]\n    note: keep me\n    templates:\n      only: [bug]\n")

	r, err := ts.ScopedRemove(p.String())
	if err != nil || !r.Committed || r.Scoped != nil || lastOp(t, ts) != "schema" {
		t.Fatalf("ScopedRemove = %+v, %v", r, err)
	}
	sc, _ := ts.storeSchema()
	if _, ok := sc.ScopedFor(p); ok {
		t.Error("the override is still there")
	}
	if _, ok := sc.ScopedFor(other); !ok {
		t.Error("the other project lost its override")
	}
	_, text = ts.storeSchema()
	if !strings.Contains(text, "note: keep me") || !strings.Contains(text, "only: [bug]") {
		t.Errorf("the unknown key or the templates of the other override are gone:\n%s", text)
	}

	head := ts.head()
	r, err = ts.ScopedRemove(p.String())
	if err != nil || r.Committed || ts.head() != head {
		t.Errorf("second remove = %+v, %v", r, err)
	}
}

func TestScopedRemoveOfAGoneProjectByULID(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	gone := model.NewULID()
	_, text := ts.storeSchema()
	ts.commitRawFile("schema.yaml", text+"scoped:\n  - where: project:"+gone.String()+"\n    statuses: [todo, in_progress, done, canceled]\n")

	g, err := ts.ScopedGet(gone.String())
	if err != nil || g.Scoped == nil || g.Entry != nil || g.View().Project.ID != "" {
		t.Fatalf("ScopedGet = %+v, %v", g, err)
	}
	r, err := ts.ScopedRemove(gone.String())
	if err != nil || !r.Committed {
		t.Fatalf("ScopedRemove = %+v, %v", r, err)
	}
	if _, err := ts.ScopedRemove(gone.String()); KindOf(err) != KindNotFound {
		t.Errorf("a ULID with no project and no override: %v", err)
	}
}

func TestScopedGet(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	p := ts.addProject("Web")
	r, err := ts.ScopedGet(p.String())
	if err != nil || r.Scoped != nil || r.View().Scoped != nil || r.Entry == nil {
		t.Fatalf("no override: %+v, %v", r, err)
	}
	statuses := slices.Clone(fourStatuses)
	if _, err := ts.ScopedSet(p.String(), schema.ScopedChange{Statuses: &statuses}); err != nil {
		t.Fatal(err)
	}
	r, err = ts.ScopedGet(p.String())
	if err != nil || r.Scoped == nil || !slices.Equal(r.View().Scoped.Statuses, fourStatuses) || r.View().Commit != nil {
		t.Fatalf("override: %+v, %v", r, err)
	}
}

func TestScopedSetCanBeUndone(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	p := ts.addProject("Web")
	statuses := slices.Clone(fourStatuses)
	if _, err := ts.ScopedSet(p.String(), schema.ScopedChange{Statuses: &statuses}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Undo(); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	sc, _ := ts.storeSchema()
	if _, ok := sc.ScopedFor(p); ok {
		t.Errorf("the override is still there after undo: %+v", sc.Scoped)
	}
}

// Under the lock, a project that is gone is task.not_found with the same arg
// as the others (input); a file that cannot be read is not "not found".
func TestScopedGuardErrors(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	p := ts.addProject("Web")
	s, _, err := ts.OpenForWrite()
	if err != nil {
		t.Fatal(err)
	}
	run := func(id string, ulid model.ULID) error {
		_, err := ts.UpdateCommit(s, store.Op{Name: "schema", Task: ulid}, func(tx *store.Tx) error {
			return scopedGuard(id, ulid)(tx, s.Schema())
		})
		return err
	}
	gone := model.NewULID()
	if e := AsError(run("WEB-9", gone)); e == nil || e.Kind != KindNotFound || e.Code != "task.not_found" || e.Args["input"] != "WEB-9" {
		t.Errorf("gone: %+v", e)
	}
	_, entry, err := ts.Entry()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(entry.Path, model.TaskPath(p)), []byte("not a task"), 0o600); err != nil {
		t.Fatal(err)
	}
	err = run("WEB-1", p)
	if err == nil || KindOf(err) == KindNotFound {
		t.Errorf("unreadable: kind %v (%v); want an error that is not not_found", KindOf(err), err)
	}
}
