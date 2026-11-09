package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/schemas"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/views"
)

type migrated struct {
	Changed []*usecase.TaskView `json:"changed"`
	Commit  *string             `json:"commit"`
}

func TestMigrateProps(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ts.labels("bug", "ui")
	if _, err := tasks.Add(ts.session(), tasks.AddInput{Title: "p", Assigns: []usecase.Assign{{Key: "priority", Op: "=", Value: "high"}, {Key: "labels", Op: "+=", Value: "bug"}}}); err != nil {
		t.Fatal(err)
	}
	var m migrated
	body := map[string]any{"target": "priority", "old": "high", "new": "urgent", "dry_run": true}
	ts.call("POST", "/api/schema/migrate", body, 200, &m)
	if len(m.Changed) != 1 || m.Commit != nil || m.Changed[0].Priority == nil || *m.Changed[0].Priority != "urgent" {
		t.Fatalf("priority dry run = %+v", m)
	}
	delete(body, "dry_run")
	m = migrated{}
	ts.call("POST", "/api/schema/migrate", body, 200, &m)
	if len(m.Changed) != 1 || m.Commit == nil || m.Changed[0].Priority == nil || *m.Changed[0].Priority != "urgent" {
		t.Fatalf("priority = %+v", m)
	}
	body = map[string]any{"target": "label", "old": "bug", "new": "ui"}
	m = migrated{}
	ts.call("POST", "/api/schema/migrate", body, 200, &m)
	if len(m.Changed) != 1 || m.Commit == nil || len(m.Changed[0].Labels) != 1 || m.Changed[0].Labels[0] != "ui" {
		t.Fatalf("label = %+v", m)
	}
	var sc schemas.SchemaView
	ts.call("GET", "/api/schema", nil, 200, &sc)
	if len(sc.Labels) != 1 || sc.Labels[0] != "ui" || sc.Priorities[len(sc.Priorities)-1] != "urgent" || len(sc.Priorities) != 3 {
		t.Errorf("schema after the migrations: priorities %v, labels %v", sc.Priorities, sc.Labels)
	}

	// A property is renamed; into another it is refused.
	ts.call("POST", "/api/schema/props", map[string]any{"name": "area", "type": "text"}, 200, nil)
	ts.call("POST", "/api/schema/props", map[string]any{"name": "zone", "type": "text"}, 200, nil)
	head := ts.uc.Head()
	for _, dry := range []bool{true, false} {
		p := ts.problem("POST", "/api/schema/migrate", map[string]any{"target": "prop", "old": "area", "new": "zone", "dry_run": dry}, 422)
		if p.Type != "migrate.prop_merge" {
			t.Errorf("dry run %v: %+v", dry, p)
		}
	}
	if ts.uc.Head() != head {
		t.Error("a refused merge committed")
	}
	ts.call("POST", "/api/schema/migrate", map[string]any{"target": "prop", "old": "area", "new": "region"}, 200, nil)
}

// The old and the new name of a priority or a label are found without regard
// to case; a name that is not defined is 404 schema.not_defined.
func TestMigrateIgnoresCaseAndNamesWhatIsNotDefined(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ts.labels("bug", "ui")
	if _, err := tasks.Add(ts.session(), tasks.AddInput{Title: "p", Assigns: []usecase.Assign{{Key: "priority", Op: "=", Value: "low"}, {Key: "labels", Op: "+=", Value: "bug"}}}); err != nil {
		t.Fatal(err)
	}
	var m migrated
	ts.call("POST", "/api/schema/migrate", map[string]any{"target": "label", "old": "BUG", "new": "UI"}, 200, &m)
	if len(m.Changed) != 1 || len(m.Changed[0].Labels) != 1 || m.Changed[0].Labels[0] != "ui" {
		t.Fatalf("label = %+v", m)
	}
	m = migrated{}
	ts.call("POST", "/api/schema/migrate", map[string]any{"target": "priority", "old": "LOW", "new": "High"}, 200, &m)
	if len(m.Changed) != 1 || m.Changed[0].Priority == nil || *m.Changed[0].Priority != "high" {
		t.Fatalf("priority = %+v", m)
	}
	for _, b := range []map[string]any{
		{"target": "label", "old": "nope", "new": "x"},
		{"target": "priority", "old": "nope", "new": "x"},
		{"target": "status", "old": "nope", "new": "x"},
		{"target": "prop", "old": "nope", "new": "x"},
		{"target": "value", "prop": "nope", "old": "a", "new": "b"},
	} {
		if p := ts.problem("POST", "/api/schema/migrate", b, 404); p.Type != "schema.not_defined" {
			t.Errorf("%v: %+v", b, p)
		}
	}
}

func TestDoctorFindsAndRepairs(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ts.add("a", "")
	idB := ts.add("b", "")
	var show usecase.ShowView
	ts.call("GET", "/api/tasks/"+idB, nil, 200, &show)
	rel := model.TaskPath(mustULID(t, show.ULID))
	b, err := os.ReadFile(filepath.Join(ts.uc.StoreDir("me"), filepath.FromSlash(rel))) //nolint:gosec // the test's own store
	if err != nil {
		t.Fatal(err)
	}
	ts.uc.CommitRawFile(rel, strings.Replace(string(b), "id: "+idB, "id: ME-1", 1))
	var reps []doctorReport
	ts.call("GET", "/api/doctor", nil, 200, &reps)
	if len(reps) != 1 {
		t.Fatalf("doctor = %+v; want one report", reps)
	}
	rep := reps[0]
	if len(rep.Findings) == 0 || len(rep.Repaired) != 0 {
		t.Fatalf("doctor = %+v; want the clash found", rep)
	}
	ts.call("POST", "/api/doctor/fix", nil, 200, &reps)
	rep = reps[0]
	if len(rep.Repaired) != 1 || rep.Repaired[0]["kind"] != "id_collision" {
		t.Fatalf("doctor fix = %+v; want one repair", rep)
	}

	ts.uc.CommitRawFile(rel, "not a task\n")
	ts.call("GET", "/api/doctor", nil, 200, &reps)
	rep = reps[0]
	var named bool
	for _, f := range rep.Findings {
		w, _ := f["where"].(string)
		named = named || strings.Contains(w, rel)
	}
	if !named {
		t.Errorf("no finding names %s: %+v", rel, rep.Findings)
	}
}

func TestWorkspaceErrors(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ts.add("one", "")
	p := ts.problem("DELETE", "/api/workspaces/me", nil, 409)
	if p.Type != "workspace.not_empty" || p.Args["tasks"] != float64(1) {
		t.Errorf("not empty: %+v", p)
	}
	if p := ts.problem("GET", "/api/workspaces?store=nope", nil, 404); p.Type != "store.not_found" {
		t.Errorf("list of unknown store: %+v", p)
	}
	ts.problem("DELETE", "/api/workspaces/nope", nil, 404)
	ts.problem("PATCH", "/api/workspaces/nope", map[string]any{"name": "x"}, 404)
	if p := ts.problem("POST", "/api/workspaces", map[string]any{"name": "", "prefix": "WEB"}, 400); p.Type != "workspace.bad_name" {
		t.Errorf("empty name: %+v", p)
	}
	if p := ts.problem("POST", "/api/workspaces", map[string]any{"name": "web", "prefix": ""}, 400); p.Type != "workspace.bad_prefix" {
		t.Errorf("empty prefix: %+v", p)
	}
	if p := ts.problem("POST", "/api/stores", map[string]any{"name": "", "prefix": "TM"}, 400); p.Type != "store.bad_name" {
		t.Errorf("empty store name: %+v", p)
	}
	var out workspaceBody
	ts.call("POST", "/api/workspaces", map[string]any{"name": "web", "prefix": "WEB"}, 201, &out)
	if out.Workspace.PrefixSet.IsZero() {
		t.Errorf("added without prefix_set: %+v", out)
	}
	ts.call("DELETE", "/api/workspaces/web", nil, 200, &out)
	if out.Workspace.PrefixSet.IsZero() {
		t.Errorf("removed without prefix_set: %+v", out)
	}
	var made struct {
		Workspace usecase.WorkspaceView `json:"workspace"`
	}
	ts.call("POST", "/api/stores", map[string]any{"name": "team", "prefix": "TM"}, 201, &made)
	if made.Workspace.PrefixSet.IsZero() {
		t.Errorf("store made without prefix_set: %+v", made)
	}
}

func TestSecondStore(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ts.add("one", "")
	ts.call("POST", "/api/stores", map[string]any{"name": "team", "prefix": "TM"}, 201, nil)
	var list []usecase.WorkspaceView
	ts.call("GET", "/api/workspaces?store=team", nil, 200, &list)
	if len(list) != 1 || list[0].IDPrefix != "TM" || list[0].Tasks != 0 {
		t.Errorf("workspaces of team = %+v", list)
	}
	ts.call("GET", "/api/workspaces?store=me", nil, 200, &list)
	if len(list) != 1 || list[0].IDPrefix != "ME" {
		t.Errorf("workspaces of me = %+v", list)
	}
	var sc schemas.SchemaView
	ts.call("GET", "/api/schema?store=team", nil, 200, &sc)
	ts.call("PUT", "/api/views/w?store=team", map[string]any{"layout": "kanban"}, 200, nil)
	var views []views.SavedView
	ts.call("GET", "/api/views?store=team", nil, 200, &views)
	if len(views) != 1 {
		t.Errorf("views of team = %+v", views)
	}
	ts.call("GET", "/api/views?store=me", nil, 200, &views)
	if len(views) != 0 {
		t.Errorf("views of me = %+v", views)
	}
	ts.problem("GET", "/api/workspaces?store=nope", nil, 404)
}

func TestStoreDefault(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	type item struct {
		Name    string `json:"name"`
		Default bool   `json:"default"`
	}
	var list []item
	ts.call("GET", "/api/stores", nil, 200, &list)
	if len(list) != 1 || !list[0].Default {
		t.Fatalf("one store: %+v", list)
	}
	// With two stores the CLI picks none without --store, whatever
	// default_store says, so none is the default.
	ts.call("POST", "/api/stores", map[string]any{"name": "team", "prefix": "TM"}, 201, nil)
	if _, err := stores.StoreDefault(ts.session(), "team"); err != nil {
		t.Fatal(err)
	}
	list = nil
	ts.call("GET", "/api/stores", nil, 200, &list)
	if len(list) != 2 || list[0].Default || list[1].Default {
		t.Errorf("two stores: %+v", list)
	}
}
