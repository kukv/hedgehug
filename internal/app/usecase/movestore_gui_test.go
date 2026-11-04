package usecase

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/store"
)

// addSprint puts the sprint "Sprint 1" on C (API-3) of org-a and returns its
// ULID, as TestMoveStoreUnlinksSprintAndReleases does.
func (f *moveFx) addSprint(t *testing.T) model.ULID {
	t.Helper()
	a := f.ts.ForStore("org-a")
	as, err := a.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	var sprint model.ULID
	_, err = a.Update(as, store.Op{Name: "sprint"}, func(tx *store.Tx) error {
		c, err := tx.Task(f.c)
		if err != nil {
			return err
		}
		sp := &model.Sprint{ULID: model.NewULID(), Workspace: c.Workspace, Name: "Sprint 1"}
		if err := tx.PutSprint(sp); err != nil {
			return err
		}
		sprint = sp.ULID
		c.Sprint = &sp.ULID
		return tx.PutTask(c)
	})
	if err != nil {
		t.Fatal(err)
	}
	return sprint
}

func TestMovePlanGUIView(t *testing.T) {
	t.Parallel()
	f := newMoveFx(t)
	sprint := f.addSprint(t)
	v, err := f.ts.PrepareMovePlan(f.in())
	if err != nil {
		t.Fatal(err)
	}
	if v.From != "org-a" || v.To != "org-b" {
		t.Errorf("from/to = %s/%s", v.From, v.To)
	}
	if v.Workspace.ULID.IsZero() || v.Workspace.Name != "org-b" || v.Workspace.Prefix != "WEB" {
		t.Errorf("workspace = %+v", v.Workspace)
	}
	if v.Parent != nil {
		t.Errorf("parent = %+v; none was given", v.Parent)
	}
	wantTasks := []MoveTaskView{{ULID: f.p, ID: "API-2", Title: "P"}, {ULID: f.c, ID: "API-3", Title: "C"}, {ULID: f.g, ID: "API-4", Title: "G"}}
	if !slices.Equal(v.Tasks, wantTasks) {
		t.Errorf("tasks = %+v; want %+v", v.Tasks, wantTasks)
	}
	wantUnlinked := []MoveUnlinkView{
		{Task: MoveRefView{ULID: f.g, ID: "API-4"}, Field: "blocked_by", Target: MoveTargetView{ULID: f.first, Label: "API-1"}},
		{Task: MoveRefView{ULID: f.o, ID: "API-5"}, Field: "blocked_by", Target: MoveTargetView{ULID: f.c, Label: "API-3"}},
		{Task: MoveRefView{ULID: f.c, ID: "API-3"}, Field: "sprint", Target: MoveTargetView{ULID: sprint, Label: "Sprint 1"}},
	}
	if len(v.Unlinked) != len(wantUnlinked) {
		t.Fatalf("unlinked = %+v; want %d", v.Unlinked, len(wantUnlinked))
	}
	for _, w := range wantUnlinked {
		if !slices.Contains(v.Unlinked, w) {
			t.Errorf("unlinked = %+v; want %+v in it", v.Unlinked, w)
		}
	}
	if v.Hidden == nil || len(v.Hidden) != 0 {
		t.Errorf("hidden = %#v; want []", v.Hidden)
	}
	if v.Scoped != nil {
		t.Errorf("scoped = %+v; nothing applies", v.Scoped)
	}
}

func TestMovePlanGUIViewParentAndProps(t *testing.T) {
	t.Parallel()
	f := newMoveFx(t)
	in := f.in()
	in.Arg, in.Parent = "API-3", "WEB-1"
	v, err := f.ts.PrepareMovePlan(in)
	if err != nil {
		t.Fatal(err)
	}
	if v.Parent == nil || v.Parent.ID != "WEB-1" || v.Parent.Title == "" || v.Parent.ULID.IsZero() {
		t.Errorf("parent = %+v; want WEB-1", v.Parent)
	}

	// a property reference keeps "props." so a property named sprint or releases stays apart
	a, b := model.NewULID(), model.NewULID()
	p := MovePlan{
		From: "x", To: "y", Labels: map[model.ULID]string{a: "API-1"},
		Unlinked: []store.Unlink{{Task: a, Field: "props.reviewer", Target: b}},
	}
	got := p.GUIView().Unlinked
	want := []MoveUnlinkView{{Task: MoveRefView{ULID: a, ID: "API-1"}, Field: "props.reviewer", Target: MoveTargetView{ULID: b, Label: b.String()}}}
	if !slices.Equal(got, want) {
		t.Errorf("unlinked = %+v; want %+v", got, want)
	}
}

// A scoped choice that is not given is no error for the plan the GUI shows
// first; PrepareMoveStore (the CLI's entry) still asks for it.
func TestMovePlanScopedChoice(t *testing.T) {
	t.Parallel()
	f := newMoveFx(t)
	f.setSchema(t, func(sc *schema.Schema) {
		sc.Statuses = append(sc.Statuses, schema.Status{Name: "review", Category: schema.CategoryDoing})
		sc.Scoped = []schema.Scoped{{
			Where:    "project:" + f.p.String(),
			Statuses: []string{"backlog", "todo", "review", "in_progress", "done", "canceled"},
		}}
	})
	_, err := f.ts.PrepareMoveStore(f.in())
	mustCode(t, err, KindUsage, "choice.required")

	v, err := f.ts.PrepareMovePlan(f.in())
	if err != nil {
		t.Fatal(err)
	}
	if v.Scoped == nil || !v.Scoped.Required || v.Scoped.Choice != "" || v.Scoped.Dropped == nil || len(v.Scoped.Dropped) != 0 {
		t.Errorf("scoped before the choice = %+v", v.Scoped)
	}

	in := f.in()
	in.Scoped = "carry"
	if v, err = f.ts.PrepareMovePlan(in); err != nil {
		t.Fatal(err)
	}
	if v.Scoped == nil || v.Scoped.Required || v.Scoped.Choice != "carry" || !slices.Contains(v.Scoped.Dropped, "status review") {
		t.Errorf("scoped carried = %+v", v.Scoped)
	}

	in.Scoped = "drop"
	if v, err = f.ts.PrepareMovePlan(in); err != nil {
		t.Fatal(err)
	}
	if v.Scoped == nil || v.Scoped.Required || v.Scoped.Choice != "drop" || len(v.Scoped.Dropped) != 0 {
		t.Errorf("scoped dropped = %+v", v.Scoped)
	}

	// the result of a carried move says what was left out
	in.Scoped = "carry"
	res, err := f.ts.MoveStore(in)
	if err != nil {
		t.Fatal(err)
	}
	if rv := res.View(); rv.Scoped == nil || !rv.Scoped.Carried || !slices.Contains(rv.Scoped.Dropped, "status review") {
		t.Errorf("result scoped = %+v", rv.Scoped)
	}
}

// The plan the user agreed to is compared, under the locks, with the plan
// made again.
func TestMoveStoreConfirmedView(t *testing.T) {
	t.Parallel()
	f := newMoveFx(t)
	plan, err := f.ts.PrepareMovePlan(f.in())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.ts.ForStore("org-a").Add(AddInput{Title: "late", Parent: "API-4"}); err != nil {
		t.Fatal(err)
	}
	hb := f.head(t, "org-b")
	in := f.in()
	in.ConfirmedView = &plan
	_, err = f.ts.MoveStore(in)
	mustCode(t, err, KindConflict, "move.changed")
	if f.head(t, "org-b") != hb {
		t.Error("the destination was written")
	}

	if plan, err = f.ts.PrepareMovePlan(f.in()); err != nil {
		t.Fatal(err)
	}
	in.ConfirmedView = &plan
	res, err := f.ts.MoveStore(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Moved) != 4 {
		t.Errorf("moved = %+v; want the four tasks", res.Moved)
	}
}

// A plan that went through JSON (null for an empty list) is the same plan.
func TestMoveStoreConfirmedViewAfterJSON(t *testing.T) {
	t.Parallel()
	f := newMoveFx(t)
	plan, err := f.ts.PrepareMovePlan(f.in())
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	var back MovePlanView
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	back.Hidden = nil
	in := f.in()
	in.ConfirmedView = &back
	if _, err := f.ts.MoveStore(in); err != nil {
		t.Fatalf("the same plan after JSON: %v", err)
	}
}

func TestMoveStoreResultView(t *testing.T) {
	t.Parallel()
	f := newMoveFx(t)
	res, err := f.ts.MoveStore(f.in())
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(res.View())
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"from", "to", "tasks", "unlinked"} {
		if _, ok := m[k]; !ok {
			t.Errorf("no %q in %s", k, b)
		}
	}
	for _, k := range []string{"scoped", "resumed"} {
		if _, ok := m[k]; ok {
			t.Errorf("%q in %s; nothing to say", k, b)
		}
	}
	task := m["tasks"].([]any)[0].(map[string]any)
	if task["ulid"] != f.p.String() || task["old_id"] != "API-2" || task["new_id"] == "" || task["new_id"] == "API-2" {
		t.Errorf("tasks[0] = %v", task)
	}
	u := m["unlinked"].([]any)[0].(map[string]any)
	for _, k := range []string{"task", "field", "target"} {
		if _, ok := u[k].(string); !ok {
			t.Errorf("unlinked[0] = %v; want a string %q", u, k)
		}
	}
}
