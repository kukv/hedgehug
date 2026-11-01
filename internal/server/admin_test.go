package server

import (
	"testing"

	"github.com/kukv/hedgehug/internal/ops"
)

type workspaceBody struct {
	Workspace ops.WorkspaceView `json:"workspace"`
	Commit    *string           `json:"commit"`
	IDs       []struct {
		Old string `json:"old"`
		New string `json:"new"`
	} `json:"ids"`
}

func TestWorkspaces(t *testing.T) {
	ts := newTestServer(t)
	ts.add("one", "")
	var list []ops.WorkspaceView
	ts.call("GET", "/api/workspaces", nil, 200, &list)
	if len(list) != 1 || list[0].IDPrefix != "ME" || list[0].Tasks != 1 {
		t.Fatalf("workspaces = %+v", list)
	}
	var out workspaceBody
	ts.call("POST", "/api/workspaces", map[string]any{"name": "web", "prefix": "WEB"}, 201, &out)
	if out.Workspace.Name != "web" || out.Commit == nil {
		t.Fatalf("added = %+v", out)
	}
	ts.call("PATCH", "/api/workspaces/web", map[string]any{"name": "site"}, 200, &out)
	if out.Workspace.Name != "site" {
		t.Errorf("renamed = %+v", out)
	}
	ts.call("PATCH", "/api/workspaces/me", map[string]any{"prefix": "MY"}, 200, &out)
	if out.Workspace.IDPrefix != "MY" || len(out.IDs) != 1 || out.IDs[0].New != "MY-1" {
		t.Errorf("prefix = %+v", out)
	}
	if p := ts.problem("PATCH", "/api/workspaces/me", map[string]any{"name": "a", "prefix": "B"}, 400); p.Type != "workspace.one_change" {
		t.Errorf("both: %+v", p)
	}
	ts.problem("PATCH", "/api/workspaces/me", map[string]any{}, 400)
	ts.call("DELETE", "/api/workspaces/site", nil, 200, &out)
	ts.call("GET", "/api/workspaces", nil, 200, &list)
	if len(list) != 1 {
		t.Errorf("after delete = %+v", list)
	}
}

func TestSchema(t *testing.T) {
	ts := newTestServer(t)
	var sc ops.SchemaView
	ts.call("GET", "/api/schema", nil, 200, &sc)
	if len(sc.Statuses) == 0 {
		t.Fatalf("schema = %+v", sc)
	}
	var check struct {
		Findings []map[string]any `json:"findings"`
	}
	ts.call("GET", "/api/schema/check", nil, 200, &check)
	if check.Findings == nil || len(check.Findings) != 0 {
		t.Errorf("check = %+v", check)
	}
	id := ts.add("one", "")
	var show ops.ShowView
	ts.call("GET", "/api/tasks/"+id, nil, 200, &show)
	old := show.Status // the status the new task has
	var mig struct {
		Changed []*ops.TaskView `json:"changed"`
		Skipped []string        `json:"skipped"`
		Commit  *string         `json:"commit"`
	}
	ts.call("POST", "/api/schema/migrate", map[string]any{"target": "status", "old": old, "new": "renamed", "dry_run": true}, 200, &mig)
	if mig.Commit != nil || len(mig.Changed) != 1 || mig.Changed[0].Status != "renamed" {
		t.Errorf("dry run = %+v", mig)
	}
	ts.call("POST", "/api/schema/migrate", map[string]any{"target": "status", "old": old, "new": "renamed"}, 200, &mig)
	if mig.Commit == nil || mig.Changed[0].Status != "renamed" {
		t.Errorf("migrate = %+v", mig)
	}
	ts.problem("POST", "/api/schema/migrate", map[string]any{"target": "value", "old": "a", "new": "b"}, 400) // value needs prop
	ts.problem("POST", "/api/schema/migrate", map[string]any{"target": "nope", "old": "a", "new": "b"}, 400)
}

func TestStores(t *testing.T) {
	ts := newTestServer(t)
	var list []struct {
		ops.StoreView
		Default bool `json:"default"`
	}
	ts.call("GET", "/api/stores", nil, 200, &list)
	if len(list) != 1 || list[0].Name != "me" || !list[0].Default {
		t.Fatalf("stores = %+v", list)
	}
	var made struct {
		Name      string                `json:"name"`
		Path      string                `json:"path"`
		Workspace struct{ Name string } `json:"workspace"`
	}
	ts.call("POST", "/api/stores", map[string]any{"name": "team", "prefix": "TM"}, 201, &made)
	if made.Name != "team" || made.Path == "" {
		t.Fatalf("made = %+v", made)
	}
	var tasks listBody
	ts.call("GET", "/api/tasks?store=team", nil, 200, &tasks)
	if len(tasks.Tasks) != 0 {
		t.Errorf("tasks of team = %+v", tasks.Tasks)
	}
	ts.problem("POST", "/api/stores", map[string]any{"name": "team", "prefix": "TX"}, 409)
}

func TestDoctor(t *testing.T) {
	ts := newTestServer(t)
	var rep struct {
		Findings []map[string]any `json:"findings"`
		Repaired []map[string]any `json:"repaired"`
	}
	ts.call("GET", "/api/doctor", nil, 200, &rep)
	if rep.Findings == nil {
		t.Errorf("doctor = %+v", rep)
	}
	ts.call("POST", "/api/doctor/fix", nil, 200, &rep)
	if rep.Repaired == nil || rep.Findings == nil {
		t.Errorf("doctor fix = %+v", rep)
	}
}
