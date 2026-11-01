package server

import (
	"net/url"
	"testing"

	"github.com/kukv/hedgehug/internal/ops"
)

func TestViews(t *testing.T) {
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
	ts := newTestServer(t)
	ts.call("PUT", "/api/views/a", map[string]any{"layout": "table"}, 200, nil)
	ts.call("PUT", "/api/views/b", map[string]any{"layout": "table"}, 200, nil)
	if p := ts.problem("PATCH", "/api/views/a", map[string]any{"name": "b"}, 409); p.Type != "view.name_taken" {
		t.Errorf("taken: %+v", p)
	}
}
