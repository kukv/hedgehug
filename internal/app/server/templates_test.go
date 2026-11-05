package server

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/templates"
)

const bugTemplate = "---\nkind: task\ntitle: Bug\nchildren:\n  - title: Reproduce\n---\n"

func TestTemplateAPI(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	var f templates.TemplateFile
	ts.call("GET", "/api/templates/bug", nil, 200, &f)
	if f.Exists || f.Content == "" {
		t.Fatalf("new name = %+v", f)
	}
	var w templates.TemplateWrite
	ts.call("PUT", "/api/templates/bug", map[string]any{"content": bugTemplate}, 200, &w)
	if w.Template.Children != 1 || w.Commit == nil {
		t.Fatalf("written = %+v", w)
	}
	ts.problem("PUT", "/api/templates/bad", map[string]any{"content": "---\nkind: nope\n---\n"}, 422)

	var vs []templates.TemplateView
	ts.call("GET", "/api/templates", nil, 200, &vs)
	if len(vs) != 1 || vs[0].Name != "bug" {
		t.Fatalf("list = %+v", vs)
	}

	var added usecase.TaskWrite
	ts.call("POST", "/api/tasks", map[string]any{"template": "bug"}, 201, &added)
	if added.Task.Title != "Bug" || added.Children != 1 {
		t.Fatalf("added = %+v", added)
	}
	ts.problem("POST", "/api/tasks", map[string]any{"template": "nope", "title": "x"}, 404)

	ts.call("DELETE", "/api/templates/bug", nil, 200, &w)
	ts.call("GET", "/api/templates", nil, 200, &vs)
	if len(vs) != 0 {
		t.Fatalf("list = %+v", vs)
	}
	ts.problem("DELETE", "/api/templates/bug", nil, 404)
}

func TestScopedTemplatesAPI(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ts.call("PUT", "/api/templates/bug", map[string]any{"content": "---\nlabels: [x]\n---\n"}, 200, nil)
	var p usecase.TaskWrite
	ts.call("POST", "/api/tasks", map[string]any{"title": "p", "kind": "project"}, 201, &p)
	ts.problem("PUT", "/api/schema/scoped/"+p.Task.ULID, map[string]any{"templates": []string{"nope"}}, 404)
	ts.call("PUT", "/api/schema/scoped/"+p.Task.ULID, map[string]any{"templates": []string{"bug"}, "template_defaults": map[string]string{"task": "bug"}}, 200, nil)
	var c usecase.TaskWrite
	ts.call("POST", "/api/tasks", map[string]any{"title": "c", "parent": p.Task.ULID}, 201, &c)
	if len(c.Task.Labels) != 1 || c.Task.Labels[0] != "x" {
		t.Fatalf("labels = %v", c.Task.Labels)
	}
}

func TestBulkAPI(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ulidOf := func(id string) string {
		var v usecase.ShowView
		ts.call("GET", "/api/tasks/"+id, nil, 200, &v)
		return v.ULID
	}
	a, b := ulidOf(ts.add("a", "")), ulidOf(ts.add("b", ""))
	var r tasks.BulkResult

	ts.problem("POST", "/api/tasks/bulk", map[string]any{"labels": []string{"x"}}, 400)
	ts.problem("POST", "/api/tasks/bulk", map[string]any{"query": "x", "ulids": []string{a}, "labels": []string{"x"}}, 400)

	ts.call("POST", "/api/tasks/bulk", map[string]any{"ulids": []string{a, b}, "status": "nope", "dry_run": true}, 200, &r)
	if len(r.Tasks) != 2 || len(r.Violations) != 2 || len(r.Commits) != 0 {
		t.Fatalf("dry run = %+v", r)
	}
	ts.problem("POST", "/api/tasks/bulk", map[string]any{"ulids": []string{a, b}, "status": "nope"}, 422)

	ts.call("POST", "/api/tasks/bulk", map[string]any{"ulids": []string{a, b}, "labels": []string{"x"}}, 200, &r)
	if len(r.Commits) != 1 {
		t.Fatalf("result = %+v", r)
	}
	ts.call("POST", "/api/tasks/bulk", map[string]any{"query": "label:x", "status": "done"}, 200, &r)
	if len(r.Tasks) != 2 || len(r.Commits) != 1 {
		t.Fatalf("by query = %+v", r)
	}
	ts.call("POST", "/api/tasks/bulk", map[string]any{"query": "label:none-of-these", "status": "done"}, 200, &r)
	if len(r.Tasks) != 0 {
		t.Fatalf("no match = %+v", r)
	}
	ts.problem("POST", "/api/tasks/bulk", map[string]any{"query": "status:(", "status": "done"}, 400)
}
