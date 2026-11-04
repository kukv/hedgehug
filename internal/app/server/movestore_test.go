package server

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/remote/replicate"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// moveFixture adds to the store "me" of twoStores: ME-2 with its child ME-3
// (sprint S1), and ME-4, which ME-3 blocks.
func moveFixture(t *testing.T, ts *testServer) {
	t.Helper()
	ss := ts.session()
	ss.StoreName = "me"
	add := func(title, parent string) {
		if _, err := ss.Add(usecase.AddInput{Title: title, Parent: parent}); err != nil {
			t.Fatal(err)
		}
	}
	add("P", "")
	add("C", "ME-2")
	add("O", "")
	if _, err := ss.Dep("ME-4", "ME-3", true); err != nil {
		t.Fatal(err)
	}
	s, err := ss.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	_, ix, err := ss.Open()
	if err != nil {
		t.Fatal(err)
	}
	e, err := ss.Resolve(ix, "ME-3")
	if err != nil {
		t.Fatal(err)
	}
	sp := &model.Sprint{ULID: model.NewULID(), Workspace: e.Workspace, Name: "S1"}
	if _, err := ss.Update(s, store.Op{Name: "update", Task: e.ULID}, func(tx *store.Tx) error {
		task, err := tx.Task(e.ULID)
		if err != nil {
			return err
		}
		task.Sprint = &sp.ULID
		return errors.Join(tx.PutSprint(sp), tx.PutTask(task))
	}); err != nil {
		t.Fatal(err)
	}
}

// tasksIn is the titles of the tasks of the store.
func tasksIn(t *testing.T, ts *testServer, storeName string) []string {
	t.Helper()
	var out listBody
	ts.call("GET", "/api/tasks?store="+storeName, nil, 200, &out)
	var titles []string
	for _, v := range out.Tasks {
		titles = append(titles, v.Title)
	}
	slices.Sort(titles)
	return titles
}

func TestMoveStorePlanAPI(t *testing.T) {
	t.Parallel()
	ts := twoStores(t)
	moveFixture(t, ts)

	var v usecase.MovePlanView
	ts.call("POST", "/api/tasks/ME-2/move-store/plan", map[string]any{"to": "team"}, 200, &v)
	if v.From != "me" || v.To != "team" || v.Workspace.Prefix != "TM" || v.Workspace.ULID.IsZero() || v.Parent != nil {
		t.Fatalf("plan = %+v", v)
	}
	if len(v.Tasks) != 2 || v.Tasks[0].ID != "ME-2" || v.Tasks[0].Title != "P" || v.Tasks[1].ID != "ME-3" {
		t.Errorf("tasks = %+v", v.Tasks)
	}
	var blocked, sprint bool
	for _, u := range v.Unlinked {
		switch {
		case u.Field == "blocked_by" && u.Task.ID == "ME-4" && u.Target.Label == "ME-3":
			blocked = true
		case u.Field == "sprint" && u.Task.ID == "ME-3" && u.Target.Label == "S1":
			sprint = true
		}
	}
	if !blocked || !sprint || len(v.Unlinked) != 2 {
		t.Errorf("unlinked = %+v; want ME-4 blocked_by ME-3 and the sprint S1 of ME-3", v.Unlinked)
	}
	if v.Hidden == nil || v.Scoped != nil {
		t.Errorf("hidden = %#v, scoped = %+v", v.Hidden, v.Scoped)
	}
	// a plan writes nothing
	if got := tasksIn(t, ts, "team"); !slices.Equal(got, []string{"two"}) {
		t.Errorf("team = %v after a plan", got)
	}

	// the destination's workspace, and a parent of it
	ts.call("POST", "/api/tasks/ME-2/move-store/plan", map[string]any{"to": "team", "workspace": "TM", "parent": "TM-1"}, 200, &v)
	if v.Parent == nil || v.Parent.ID != "TM-1" || v.Parent.Title != "two" {
		t.Errorf("parent = %+v", v.Parent)
	}

	// errors
	if p := ts.problem("POST", "/api/tasks/ME-2/move-store/plan", map[string]any{"to": "me"}, 400); p.Type != "move.same_store" {
		t.Errorf("same store: %+v", p)
	}
	if p := ts.problem("POST", "/api/tasks/ME-2/move-store/plan", map[string]any{"to": "nowhere"}, 404); p.Type != "store.not_found" {
		t.Errorf("no such store: %+v", p)
	}
	if p := ts.problem("POST", "/api/tasks/ME-2/move-store/plan", map[string]any{"to": "team", "scoped": "sideways"}, 400); p.Type != "usage" {
		t.Errorf("bad scoped: %+v", p)
	}
	if p := ts.problem("POST", "/api/tasks/ME-99/move-store/plan", map[string]any{"to": "team"}, 404); p.Type != "task.not_found" {
		t.Errorf("no such task: %+v", p)
	}
}

func TestMoveStorePlanAPIReadOnly(t *testing.T) {
	t.Parallel()
	ts := twoStores(t)
	s, err := ts.session().ForStore("team").OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if err := replicate.WriteAccess(s, replicate.Access{ReadOnly: true, Reason: replicate.ReasonNotWritable, Checked: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if p := ts.problem("POST", "/api/tasks/ME-1/move-store/plan", map[string]any{"to": "team"}, 409); p.Type != "store.read_only" {
		t.Errorf("plan: %+v", p)
	}
	plan := usecase.MovePlanView{From: "me", To: "team", Tasks: []usecase.MoveTaskView{}, Unlinked: []usecase.MoveUnlinkView{}, Hidden: []string{}}
	if p := ts.problem("POST", "/api/tasks/ME-1/move-store", map[string]any{"to": "team", "confirmed": plan}, 409); p.Type != "store.read_only" {
		t.Errorf("move: %+v", p)
	}
}

func TestMoveStoreAPI(t *testing.T) {
	t.Parallel()
	ts := twoStores(t)
	moveFixture(t, ts)

	// no confirmed plan: confirm.required, and nothing moves
	if p := ts.problem("POST", "/api/tasks/ME-2/move-store", map[string]any{"to": "team"}, 400); p.Type != "confirm.required" {
		t.Errorf("without confirmed: %+v", p)
	}

	var plan usecase.MovePlanView
	ts.call("POST", "/api/tasks/ME-2/move-store/plan", map[string]any{"to": "team"}, 200, &plan)

	// a task added under the moved ones after the plan was shown
	if _, err := func() (usecase.TaskResult, error) {
		ss := ts.session()
		ss.StoreName = "me"
		return ss.Add(usecase.AddInput{Title: "late", Parent: "ME-3"})
	}(); err != nil {
		t.Fatal(err)
	}
	if p := ts.problem("POST", "/api/tasks/ME-2/move-store", map[string]any{"to": "team", "confirmed": plan}, 409); p.Type != "move.changed" {
		t.Errorf("stale plan: %+v", p)
	}
	if got := tasksIn(t, ts, "team"); !slices.Equal(got, []string{"two"}) {
		t.Fatalf("team = %v after a refused move", got)
	}

	// the plan as it is now goes through
	ts.call("POST", "/api/tasks/ME-2/move-store/plan", map[string]any{"to": "team"}, 200, &plan)
	if len(plan.Tasks) != 3 {
		t.Fatalf("plan = %+v; want the three tasks", plan.Tasks)
	}
	var res usecase.MoveStoreResultView
	ts.call("POST", "/api/tasks/ME-2/move-store", map[string]any{"to": "team", "confirmed": plan}, 200, &res)
	if res.From != "me" || res.To != "team" || len(res.Tasks) != 3 || res.Tasks[0].OldID != "ME-2" || res.Tasks[0].NewID != "TM-2" {
		t.Fatalf("result = %+v", res)
	}
	if len(res.Unlinked) != 2 || res.Scoped != nil || res.Resumed {
		t.Errorf("result = %+v", res)
	}
	if got := tasksIn(t, ts, "team"); !slices.Equal(got, []string{"C", "P", "late", "two"}) {
		t.Errorf("team = %v", got)
	}
	if got := tasksIn(t, ts, "me"); !slices.Equal(got, []string{"O", "one"}) {
		t.Errorf("me = %v", got)
	}
	var moved usecase.ShowView
	ts.call("GET", "/api/tasks/"+res.Tasks[0].ULID.String()+"?store=team", nil, 200, &moved)
	if moved.ID != "TM-2" {
		t.Errorf("moved task = %s", moved.ID)
	}
}

// A scoped schema that goes along needs a choice: the plan says so without
// failing, and the move itself needs it given.
func TestMoveStoreAPIScoped(t *testing.T) {
	t.Parallel()
	ts := twoStores(t)
	var proj taskWriteBody
	ts.call("POST", "/api/tasks?store=me", map[string]any{"title": "Web", "kind": "project"}, 201, &proj)
	four := []string{"todo", "in_progress", "done", "canceled"}
	ts.call("PUT", "/api/schema/scoped/"+proj.Task.ID+"?store=me", map[string]any{"statuses": four}, 200, nil)
	path := "/api/tasks/" + proj.Task.ID + "/move-store"

	var v usecase.MovePlanView
	ts.call("POST", path+"/plan", map[string]any{"to": "team"}, 200, &v)
	if v.Scoped == nil || !v.Scoped.Required || v.Scoped.Choice != "" || v.Scoped.Dropped == nil {
		t.Fatalf("scoped before the choice = %+v", v.Scoped)
	}
	// the move without the choice is choice.required, not a move
	if p := ts.problem("POST", path, map[string]any{"to": "team", "confirmed": v}, 400); p.Type != "choice.required" || p.Args["what"] != "--scoped" {
		t.Errorf("without scoped: %+v", p)
	}

	ts.call("POST", path+"/plan", map[string]any{"to": "team", "scoped": "carry"}, 200, &v)
	if v.Scoped == nil || v.Scoped.Required || v.Scoped.Choice != "carry" {
		t.Fatalf("scoped carried = %+v", v.Scoped)
	}
	// a plan made for another choice is not the plan of this one
	var other usecase.MovePlanView
	ts.call("POST", path+"/plan", map[string]any{"to": "team", "scoped": "drop"}, 200, &other)
	if p := ts.problem("POST", path, map[string]any{"to": "team", "scoped": "carry", "confirmed": other}, 409); p.Type != "move.changed" {
		t.Errorf("plan of drop, move with carry: %+v", p)
	}

	var res usecase.MoveStoreResultView
	ts.call("POST", path, map[string]any{"to": "team", "scoped": "carry", "confirmed": v}, 200, &res)
	if res.Scoped == nil || !res.Scoped.Carried {
		t.Errorf("result = %+v", res)
	}
	var sc usecase.SchemaView
	ts.call("GET", "/api/schema?store=team", nil, 200, &sc)
	if len(sc.Scoped) != 1 || sc.Scoped[0].Where != "project:"+proj.Task.ULID {
		t.Errorf("scoped in team = %+v", sc.Scoped)
	}
}
