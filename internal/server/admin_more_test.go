package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/ops"
)

type migrated struct {
	Changed []*ops.TaskView `json:"changed"`
	Commit  *string         `json:"commit"`
}

func TestMigrateProps(t *testing.T) {
	ts := newTestServer(t)
	if _, err := ts.session().Add(ops.AddInput{Title: "p", Assigns: []ops.Assign{{Key: "priority", Op: "=", Value: "high"}}}); err != nil {
		t.Fatal(err)
	}
	var m migrated
	body := map[string]any{"target": "value", "prop": "priority", "old": "high", "new": "urgent", "dry_run": true}
	m = migrated{}
	ts.call("POST", "/api/schema/migrate", body, 200, &m)
	if len(m.Changed) != 1 || m.Commit != nil || m.Changed[0].Props["priority"] != "urgent" {
		t.Fatalf("value dry run = %+v", m)
	}
	delete(body, "dry_run")
	m = migrated{}
	ts.call("POST", "/api/schema/migrate", body, 200, &m)
	if len(m.Changed) != 1 || m.Commit == nil || m.Changed[0].Props["priority"] != "urgent" {
		t.Fatalf("value = %+v", m)
	}
	body = map[string]any{"target": "prop", "old": "priority", "new": "importance", "dry_run": true}
	m = migrated{}
	ts.call("POST", "/api/schema/migrate", body, 200, &m)
	if len(m.Changed) != 1 || m.Changed[0].Props["importance"] != "urgent" || m.Changed[0].Props["priority"] != nil {
		t.Fatalf("prop dry run = %+v", m)
	}
	delete(body, "dry_run")
	m = migrated{}
	ts.call("POST", "/api/schema/migrate", body, 200, &m)
	if len(m.Changed) != 1 || m.Changed[0].Props["importance"] != "urgent" || m.Changed[0].Props["priority"] != nil {
		t.Fatalf("prop = %+v", m)
	}
}

// commitRaw writes body to the file rel of the store "me" and commits it, as
// a broken sync would leave it.
func (ts *testServer) commitRaw(rel, body string) {
	ts.t.Helper()
	ss := ts.session()
	_, e, err := ss.Entry()
	if err != nil {
		ts.t.Fatal(err)
	}
	s, err := ss.OpenStore()
	if err != nil {
		ts.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.Path, filepath.FromSlash(rel)), []byte(body), 0o600); err != nil {
		ts.t.Fatal(err)
	}
	author, err := config.CommitAuthor(config.Account{}, false, ts.getenv)
	if err != nil {
		ts.t.Fatal(err)
	}
	if _, err := s.Repo().Commit(gitrepo.Commit{Paths: []string{rel}, Subject: "raw", Author: author, When: time.Now()}); err != nil {
		ts.t.Fatal(err)
	}
}

func TestDoctorFindsAndRepairs(t *testing.T) {
	ts := newTestServer(t)
	ts.add("a", "")
	idB := ts.add("b", "")
	var show ops.ShowView
	ts.call("GET", "/api/tasks/"+idB, nil, 200, &show)
	rel := model.TaskPath(mustULID(t, show.ULID))
	_, e, err := ts.session().Entry()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(e.Path, filepath.FromSlash(rel))) //nolint:gosec // the test's own store
	if err != nil {
		t.Fatal(err)
	}
	ts.commitRaw(rel, strings.Replace(string(b), "id: "+idB, "id: ME-1", 1))
	var rep struct {
		Findings []map[string]any `json:"findings"`
		Repaired []map[string]any `json:"repaired"`
	}
	ts.call("GET", "/api/doctor", nil, 200, &rep)
	if len(rep.Findings) == 0 || len(rep.Repaired) != 0 {
		t.Fatalf("doctor = %+v; want the clash found", rep)
	}
	ts.call("POST", "/api/doctor/fix", nil, 200, &rep)
	if len(rep.Repaired) != 1 || rep.Repaired[0]["kind"] != "id_collision" {
		t.Fatalf("doctor fix = %+v; want one repair", rep)
	}

	ts.commitRaw(rel, "not a task\n")
	ts.call("GET", "/api/doctor", nil, 200, &rep)
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
	ts := newTestServer(t)
	ts.add("one", "")
	if p := ts.problem("DELETE", "/api/workspaces/me", nil, 409); p.Type != "workspace.not_empty" {
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
		Workspace ops.WorkspaceView `json:"workspace"`
	}
	ts.call("POST", "/api/stores", map[string]any{"name": "team", "prefix": "TM"}, 201, &made)
	if made.Workspace.PrefixSet.IsZero() {
		t.Errorf("store made without prefix_set: %+v", made)
	}
}

func TestSecondStore(t *testing.T) {
	ts := newTestServer(t)
	ts.add("one", "")
	ts.call("POST", "/api/stores", map[string]any{"name": "team", "prefix": "TM"}, 201, nil)
	var list []ops.WorkspaceView
	ts.call("GET", "/api/workspaces?store=team", nil, 200, &list)
	if len(list) != 1 || list[0].IDPrefix != "TM" || list[0].Tasks != 0 {
		t.Errorf("workspaces of team = %+v", list)
	}
	ts.call("GET", "/api/workspaces?store=me", nil, 200, &list)
	if len(list) != 1 || list[0].IDPrefix != "ME" {
		t.Errorf("workspaces of me = %+v", list)
	}
	var sc ops.SchemaView
	ts.call("GET", "/api/schema?store=team", nil, 200, &sc)
	ts.call("PUT", "/api/views/w?store=team", map[string]any{"layout": "kanban"}, 200, nil)
	var views []ops.SavedView
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
	if _, err := ts.session().StoreDefault("team"); err != nil {
		t.Fatal(err)
	}
	list = nil
	ts.call("GET", "/api/stores", nil, 200, &list)
	if len(list) != 2 || list[0].Default || list[1].Default {
		t.Errorf("two stores: %+v", list)
	}
}
