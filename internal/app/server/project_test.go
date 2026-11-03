package server

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase"
)

func TestTaskProject(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	var p, c taskWriteBody
	ts.call("POST", "/api/tasks", map[string]any{"title": "P", "kind": "project"}, 201, &p)
	ts.call("POST", "/api/tasks", map[string]any{"title": "c", "parent": p.Task.ULID}, 201, &c)
	var show usecase.ShowView
	ts.call("GET", "/api/tasks/"+c.Task.ULID, nil, 200, &show)
	if show.Project == nil || show.Project.ULID != p.Task.ULID || show.Project.Title != "P" {
		t.Errorf("project = %+v", show.Project)
	}
	// a fresh value: decoding into the one above would keep its project
	var pshow usecase.ShowView
	ts.call("GET", "/api/tasks/"+p.Task.ULID, nil, 200, &pshow)
	if pshow.Project != nil {
		t.Errorf("the project has project %+v", pshow.Project)
	}
}
