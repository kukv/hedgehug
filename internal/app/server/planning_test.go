package server

import (
	"net/url"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/planning"
	"github.com/kukv/hedgehug/internal/app/usecase/worklogs"
)

func TestSprintAPI(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	id := ts.add("one", "")

	var w planning.SprintWrite
	ts.call("POST", "/api/sprints", map[string]any{"name": "Sprint 1"}, 201, &w)
	if w.Sprint.State != model.SprintPlanned || w.Commit == nil {
		t.Fatalf("added = %+v", w)
	}
	ts.problem("POST", "/api/sprints", map[string]any{"name": "sprint 1"}, 409)
	ts.problem("POST", "/api/sprints/nope/start", nil, 404)
	path := "/api/sprints/" + url.PathEscape("Sprint 1")
	if p := ts.problem("POST", path+"/start", nil, 422); p.Type == "" {
		t.Errorf("no dates: %+v", p)
	}
	ts.call("PATCH", path, map[string]any{"start": "2026-11-02", "end": "2026-11-13", "goal": "ship"}, 200, &w)
	if w.Sprint.Goal != "ship" || w.Sprint.End != "2026-11-13" {
		t.Fatalf("patched = %+v", w)
	}
	ts.call("POST", "/api/sprints/"+w.Sprint.ULID+"/start", nil, 200, &w)
	if w.Sprint.State != model.SprintActive {
		t.Fatalf("started = %+v", w)
	}

	ts.call("PATCH", "/api/tasks/"+id, map[string]any{"sprint": w.Sprint.ULID}, 200, nil)
	var show usecase.ShowView
	ts.call("GET", "/api/tasks/"+id, nil, 200, &show)
	if show.Sprint == nil || show.Sprint.Name != "Sprint 1" {
		t.Fatalf("task sprint = %+v", show.Sprint)
	}

	var vs []planning.SprintView
	ts.call("GET", "/api/sprints?state=active", nil, 200, &vs)
	if len(vs) != 1 || vs[0].Tasks != 1 {
		t.Fatalf("list = %+v", vs)
	}

	ts.problem("POST", path+"/close", map[string]any{"to": "next"}, 409)
	ts.call("POST", path+"/close", map[string]any{"to": "backlog"}, 200, &w)
	if w.Sprint.State != model.SprintClosed || w.Moved != 1 || w.To != "backlog" {
		t.Fatalf("closed = %+v", w)
	}
	ts.call("DELETE", path, nil, 200, &w)
	ts.call("GET", "/api/sprints", nil, 200, &vs)
	if len(vs) != 0 {
		t.Fatalf("list = %+v", vs)
	}
}

func TestReleaseAPI(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	id := ts.add("one", "")

	var w planning.ReleaseWrite
	ts.call("POST", "/api/releases", map[string]any{"name": "v1", "date": "2026-12-01"}, 201, &w)
	ts.call("PATCH", "/api/tasks/"+id, map[string]any{"releases": []string{"v1"}, "status": "done"}, 200, nil)
	var vs []planning.ReleaseView
	ts.call("GET", "/api/releases", nil, 200, &vs)
	if len(vs) != 1 || vs[0].Tasks != 1 || vs[0].Done != 1 {
		t.Fatalf("list = %+v", vs)
	}
	var notes struct {
		Notes string `json:"notes"`
	}
	ts.call("GET", "/api/releases/v1/notes", nil, 200, &notes)
	if notes.Notes == "" {
		t.Fatal("no notes")
	}
	ts.call("PATCH", "/api/releases/v1", map[string]any{"state": "released"}, 200, &w)
	if w.Release.State != model.ReleaseReleased {
		t.Fatalf("patched = %+v", w)
	}
	ts.call("DELETE", "/api/releases/v1", nil, 200, &w)
	ts.problem("GET", "/api/releases/v1/notes", nil, 404)
}

func TestWorklogAPI(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	id := ts.add("one", "")
	var w worklogs.WorklogWrite
	ts.call("POST", "/api/tasks/"+id+"/worklogs", map[string]any{"duration": "1h30m", "note": "fix"}, 201, &w)
	if w.Worklog.Duration != "1h30m" || w.Commit == nil {
		t.Fatalf("added = %+v", w)
	}
	var vs []worklogs.WorklogView
	ts.call("GET", "/api/tasks/"+id+"/worklogs", nil, 200, &vs)
	if len(vs) != 1 || vs[0].Body != "fix" {
		t.Fatalf("list = %+v", vs)
	}
	ts.problem("POST", "/api/tasks/"+id+"/worklogs", map[string]any{"duration": "soon"}, 400)
	ts.problem("DELETE", "/api/tasks/"+id+"/worklogs/"+model.NewULID().String(), nil, 404)
	ts.call("DELETE", "/api/tasks/"+id+"/worklogs/"+w.Worklog.ULID, nil, 200, &w)
	ts.call("GET", "/api/tasks/"+id+"/worklogs", nil, 200, &vs)
	if len(vs) != 0 {
		t.Fatalf("list after delete = %+v", vs)
	}
}

func TestWorkspaceCountsPlans(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ts.call("POST", "/api/sprints", map[string]any{"name": "s1"}, 201, nil)
	ts.call("POST", "/api/releases", map[string]any{"name": "v1"}, 201, nil)
	var wss []usecase.WorkspaceView
	ts.call("GET", "/api/workspaces", nil, 200, &wss)
	if len(wss) != 1 || wss[0].Sprints != 1 || wss[0].Releases != 1 {
		t.Fatalf("workspaces = %+v", wss)
	}
	if p := ts.problem("DELETE", "/api/workspaces/"+wss[0].ULID, nil, 409); p.Type == "" {
		t.Errorf("delete: %+v", p)
	}
}
