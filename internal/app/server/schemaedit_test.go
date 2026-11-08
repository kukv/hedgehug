package server

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase/schemas"
)

func statusNames(w schemas.EditWrite) []string {
	var n []string
	for _, s := range w.Schema.Statuses {
		n = append(n, s.Name)
	}
	return n
}

func TestSchemaEditRoundTrip(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)

	var w schemas.EditWrite
	ts.call("POST", "/api/schema/statuses", map[string]any{"name": "review", "category": "doing", "color": "teal"}, 200, &w)
	if n := statusNames(w); n[len(n)-1] != "review" || w.Commit == nil {
		t.Fatalf("add = %v, commit %v", n, w.Commit)
	}

	w = schemas.EditWrite{}
	ts.call("PATCH", "/api/schema/statuses/review", map[string]any{"label": "レビュー", "default": true}, 200, &w)
	for _, s := range w.Schema.Statuses {
		if s.Name == "review" && (!s.Default || s.Label != "レビュー") {
			t.Errorf("review = %+v", s)
		}
		if s.Name == "in_progress" && s.Default {
			t.Errorf("in_progress is still the default")
		}
	}

	rev := statusNames(w)
	slices.Reverse(rev)
	w = schemas.EditWrite{}
	ts.call("PUT", "/api/schema/statuses/order", map[string]any{"names": rev}, 200, &w)
	if got := statusNames(w); !slices.Equal(got, rev) {
		t.Errorf("order = %v, want %v", got, rev)
	}

	ts.call("PATCH", "/api/schema/statuses/in_progress", map[string]any{"default": true}, 200, nil)
	w = schemas.EditWrite{}
	ts.call("DELETE", "/api/schema/statuses/review", nil, 200, &w)
	if slices.Contains(statusNames(w), "review") {
		t.Errorf("review is still there: %v", statusNames(w))
	}

	w = schemas.EditWrite{}
	ts.call("POST", "/api/schema/props/priority/values", map[string]any{"value": "none"}, 200, &w)
	if v := w.Schema.Props["priority"].Values; v[len(v)-1] != "none" {
		t.Errorf("values = %v", v)
	}
	ts.call("PUT", "/api/schema/props/priority/values/order", map[string]any{"names": []string{"none", "low", "mid", "high", "urgent"}}, 200, nil)
	w = schemas.EditWrite{}
	ts.call("DELETE", "/api/schema/props/priority/values/none", nil, 200, &w)
	if v := w.Schema.Props["priority"].Values; slices.Contains(v, "none") {
		t.Errorf("values = %v", v)
	}

	ts.call("POST", "/api/schema/kinds", map[string]any{"name": "spike", "role": "work", "anywhere": true}, 200, nil)
	ts.call("PATCH", "/api/schema/kinds/spike", map[string]any{"label": "調査", "parents": []string{"root", "epic"}}, 200, nil)
	ts.call("PUT", "/api/schema/kinds/order", map[string]any{"names": []string{"spike", "project", "epic", "story", "task", "bug"}}, 200, nil)
	ts.call("DELETE", "/api/schema/kinds/spike", nil, 200, nil)
	ts.call("POST", "/api/schema/props", map[string]any{"name": "area", "type": "text"}, 200, nil)
	ts.call("PATCH", "/api/schema/props/area", map[string]any{"label": "領域"}, 200, nil)
	ts.call("PUT", "/api/schema/props/order", map[string]any{"names": []string{"area", "priority", "start", "due", "estimate", "points"}}, 200, nil)
	ts.call("DELETE", "/api/schema/props/area", nil, 200, nil)

	var sc schemas.SchemaView
	ts.call("GET", "/api/schema", nil, 200, &sc)
	// ordering the others put area in the first slot of the group, so points
	// took the slot area had at the end; the relations kept their places
	want := []string{"priority", "start", "due", "estimate", "duplicates", "causes", "relates", "points"}
	if !slices.Equal(sc.PropOrder, want) {
		t.Errorf("prop_order = %v, want %v", sc.PropOrder, want)
	}
}

func TestSchemaEditErrors(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)

	if e := ts.problem("DELETE", "/api/schema/statuses/nope", nil, 404); e.Type != "schema.not_defined" {
		t.Errorf("not defined: %+v", e)
	}
	if e := ts.problem("PUT", "/api/schema/statuses/order", map[string]any{"names": []string{"todo"}}, 409); e.Type != "schema.order_stale" {
		t.Errorf("stale: %+v", e)
	}
	if e := ts.problem("DELETE", "/api/schema/statuses/todo", nil, 422); e.Type != "schema.invalid" {
		t.Errorf("default: %+v", e)
	}
	ts.call("POST", "/api/tasks", map[string]any{"title": "wait", "status": "backlog"}, 201, nil)
	if e := ts.problem("DELETE", "/api/schema/statuses/backlog", nil, 422); e.Type != "schema.in_use" {
		t.Errorf("in use: %+v", e)
	}
	ts.problem("PATCH", "/api/schema/kinds/bug", map[string]any{"parents": []string{}}, 400)
	ts.problem("PATCH", "/api/schema/kinds/bug", map[string]any{"parents": []string{"root"}, "anywhere": true}, 400)
}

func TestSchemaEditPublishesSchemaChanged(t *testing.T) {
	t.Parallel()
	ts, ch := pollServer(t)
	ts.srv.poller.settle()
	ts.call("POST", "/api/schema/statuses", map[string]any{"name": "review", "category": "doing"}, 200, nil)
	waitEvent[schemaChanged](t, ch, nil)
}
