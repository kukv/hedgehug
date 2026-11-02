package server

import (
	"testing"

	"github.com/kukv/hedgehug/internal/ops"
)

func TestShowTaskAt(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	var created taskWriteBody
	ts.call("POST", "/api/tasks", map[string]any{"title": "orig", "body": "old body\n"}, 201, &created)
	id := created.Task.ID
	var out taskWriteBody
	ts.call("PATCH", "/api/tasks/"+id, map[string]any{"title": "changed", "labels": []string{"x"}}, 200, &out)

	var v ops.ShowView
	ts.call("GET", "/api/tasks/"+id+"/history/"+*created.Commit, nil, 200, &v)
	if v.ID != id || v.Title != "orig" || v.Body != "old body\n" || len(v.Labels) != 0 {
		t.Errorf("task at the first commit = %+v (body %q)", v.TaskView, v.Body)
	}
	ts.call("GET", "/api/tasks/"+id+"/history/"+*out.Commit, nil, 200, &v)
	if v.Title != "changed" || len(v.Labels) != 1 {
		t.Errorf("task at the second commit = %+v", v.TaskView)
	}

	// A task made after the commit was not there yet.
	other := ts.add("later", "")
	if p := ts.problem("GET", "/api/tasks/"+other+"/history/"+*created.Commit, nil, 404); p.Type != "restore_at.not_found" {
		t.Errorf("a task that did not exist: %+v", p)
	}
	if p := ts.problem("GET", "/api/tasks/"+id+"/history/nonsense", nil, 422); p.Type != "restore_at.bad_time" {
		t.Errorf("nonsense: %+v", p)
	}
}
