package server

import (
	"net/url"
	"testing"

	"github.com/kukv/hedgehug/internal/ops"
)

func TestViews(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	var list []ops.SavedView
	ts.call("GET", "/api/views", nil, 200, &list)
	if list == nil || len(list) != 0 {
		t.Fatalf("views = %+v", list)
	}
	var out struct {
		View   ops.SavedView `json:"view"`
		Commit *string       `json:"commit"`
	}
	name := url.PathEscape("今週")
	ts.call("PUT", "/api/views/"+name, map[string]any{"layout": "table", "query": "status:todo", "columns": []string{"id", "title"}}, 200, &out)
	if out.View.Name != "今週" || out.View.Layout != "table" || len(out.View.Table.Columns) != 2 || out.Commit == nil {
		t.Fatalf("saved = %+v", out)
	}
	ts.call("PUT", "/api/views/"+name, map[string]any{"query": "status:done"}, 200, &out)
	if out.View.Layout != "table" || out.View.Query != "status:done" {
		t.Errorf("partial save = %+v", out)
	}
	var v ops.SavedView
	ts.call("GET", "/api/views/"+name, nil, 200, &v)
	if v.Query != "status:done" {
		t.Errorf("get = %+v", v)
	}
	ts.call("PATCH", "/api/views/"+name, map[string]any{"name": "来週"}, 200, &out)
	if out.View.Name != "来週" {
		t.Errorf("renamed = %+v", out)
	}
	ts.problem("GET", "/api/views/"+name, nil, 404)
	if p := ts.problem("PUT", "/api/views/"+url.PathEscape("a/b"), map[string]any{"layout": "table"}, 400); p.Type != "view.bad_name" {
		t.Errorf("bad name: %+v", p)
	}
	ts.call("DELETE", "/api/views/"+url.PathEscape("来週"), nil, 200, &out)
	ts.call("GET", "/api/views", nil, 200, &list)
	if len(list) != 0 {
		t.Errorf("after delete = %+v", list)
	}
}

func TestViewNameTaken(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ts.call("PUT", "/api/views/a", map[string]any{"layout": "table"}, 200, nil)
	ts.call("PUT", "/api/views/b", map[string]any{"layout": "table"}, 200, nil)
	if p := ts.problem("PATCH", "/api/views/a", map[string]any{"name": "b"}, 409); p.Type != "view.name_taken" {
		t.Errorf("taken: %+v", p)
	}
}

func TestViewPatchUpdatesOnlyAnExistingView(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	var out struct {
		View   ops.SavedView `json:"view"`
		Commit *string       `json:"commit"`
	}
	ts.call("PUT", "/api/views/w", map[string]any{"layout": "table", "query": "status:todo", "columns": []string{"id", "title"}}, 200, nil)
	ts.call("PATCH", "/api/views/w", map[string]any{"layout": "kanban"}, 200, &out)
	if out.View.Layout != "kanban" || out.View.Query != "status:todo" || len(out.View.Table.Columns) != 2 || out.Commit == nil {
		t.Errorf("patched = %+v", out)
	}
	ts.call("PATCH", "/api/views/w", map[string]any{"query": "kind:task", "swimlane": "assignee"}, 200, &out)
	if out.View.Query != "kind:task" || out.View.Kanban.Swimlane != "assignee" || out.View.Layout != "kanban" {
		t.Errorf("patched = %+v", out)
	}
	// A view that is not there is not made.
	if p := ts.problem("PATCH", "/api/views/gone", map[string]any{"layout": "table"}, 404); p.Type != "view.not_found" {
		t.Errorf("missing: %+v", p)
	}
	ts.problem("GET", "/api/views/gone", nil, 404)
	// The view that results is checked as a save checks it.
	ts.call("PUT", "/api/views/t", map[string]any{"layout": "table", "query": "sort:due"}, 200, nil)
	if p := ts.problem("PATCH", "/api/views/t", map[string]any{"layout": "tree"}, 400); p.Type != "view.tree_order" {
		t.Errorf("tree order: %+v", p)
	}
	// A rename and settings are two operations.
	if p := ts.problem("PATCH", "/api/views/w", map[string]any{"name": "x", "layout": "table"}, 400); p.Type != "view.rename_with_settings" {
		t.Errorf("both: %+v", p)
	}
	ts.problem("GET", "/api/views/x", nil, 404)
}
