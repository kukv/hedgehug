package server

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase/schemas"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
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
	values := []map[string]any{{"name": "none"}, {"name": "low", "from": []string{"low"}}, {"name": "mid", "from": []string{"mid"}}, {"name": "high", "from": []string{"high"}}}
	ts.call("PATCH", "/api/schema/props/priority", map[string]any{"values": values}, 200, &w)
	if v := w.Schema.Props["priority"].Values; !slices.Equal(v, []string{"none", "low", "mid", "high"}) {
		t.Errorf("values = %v", v)
	}

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
	if e := ts.problem("POST", "/api/schema/props", map[string]any{"name": "area", "type": "enum", "values": []string{"a", "a"}}, 422); e.Type != "schema.value_invalid" {
		t.Errorf("values twice: %+v", e)
	}
	if e := ts.problem("PATCH", "/api/schema/props/priority", map[string]any{"values": []map[string]any{{"name": "a"}, {"name": "a"}}}, 422); e.Type != "schema.value_invalid" {
		t.Errorf("values given twice: %+v", e)
	}
	if e := ts.problem("PATCH", "/api/schema/props/priority", map[string]any{"values": []map[string]any{{"name": "a", "from": []string{"nope"}}}}, 422); e.Type != "schema.value_invalid" {
		t.Errorf("from not a value: %+v", e)
	}
	if e := ts.problem("PATCH", "/api/schema/props/start", map[string]any{"values": []map[string]any{{"name": "a"}}}, 422); e.Type != "schema.not_enum" {
		t.Errorf("values of a date: %+v", e)
	}
	if e := ts.problem("PATCH", "/api/schema/statuses/todo", map[string]any{"name": "done"}, 409); e.Type != "schema.name_taken" {
		t.Errorf("name taken: %+v", e)
	}
	ts.problem("PUT", "/api/schema/statuses/order", map[string]any{"names": nil}, 400)
	for _, r := range []struct{ method, path string }{
		{"POST", "/api/schema/kinds"}, {"PATCH", "/api/schema/kinds/bug"}, {"DELETE", "/api/schema/kinds/bug"}, {"PUT", "/api/schema/kinds/order"},
	} {
		// The routes are not in api/openapi.yaml, so the reply is not checked against it.
		req, err := http.NewRequest(r.method, ts.hs.URL+r.path, strings.NewReader(`{"name":"x"}`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+testToken)
		req.Header.Set("Content-Type", "application/json")
		if got := ts.raw(req); got.Status != 404 && got.Status != 405 {
			t.Errorf("%s %s: status %d, body %s", r.method, r.path, got.Status, got.Body)
		}
	}
	ts.problem("POST", "/api/schema/migrate", map[string]any{"target": "kind", "old": "task", "new": "item"}, 400)
}

func TestSchemaEditPublishesSchemaChanged(t *testing.T) {
	t.Parallel()
	ts, ch := pollServer(t)
	ts.srv.poller.settle()
	ts.call("POST", "/api/schema/statuses", map[string]any{"name": "review", "category": "doing"}, 200, nil)
	waitEvent[schemaChanged](t, ch, nil)
}

func TestSchemaEditRenameRewritesTasksInOneCommit(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	var out taskWriteBody
	ts.call("POST", "/api/tasks", map[string]any{"title": "one", "status": "todo"}, 201, &out)
	u := mustULID(t, out.Task.ULID)
	before := ts.uc.Head()

	var w schemas.EditWrite
	ts.call("PATCH", "/api/schema/statuses/todo", map[string]any{"name": "open", "label": "Open"}, 200, &w)
	if w.Commit == nil || ts.uc.Head() == before {
		t.Fatalf("commit = %v", w.Commit)
	}
	if got := ts.uc.TaskByULID("me", u).Status; got != "open" {
		t.Errorf("status = %s", got)
	}
	if usecasetest.LastOp(t, ts.uc) != "schema" {
		t.Errorf("last op = %s", usecasetest.LastOp(t, ts.uc))
	}
	for _, st := range w.Schema.Statuses {
		if st.Name == "todo" {
			t.Error("todo is still defined")
		}
		if st.Name == "open" && (st.Label != "Open" || !st.Default) {
			t.Errorf("open = %+v", st)
		}
	}

	ts.call("PATCH", "/api/schema/props/priority", map[string]any{"name": "importance"}, 200, &w)
	if _, ok := w.Schema.Props["importance"]; !ok {
		t.Errorf("props = %v", w.Schema.PropOrder)
	}
}

func TestSchemaEditValuesMergeRenameAndInUse(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	var urgent, mid taskWriteBody
	ts.call("POST", "/api/tasks", map[string]any{"title": "u", "props": map[string]any{"priority": "urgent"}}, 201, &urgent)
	ts.call("POST", "/api/tasks", map[string]any{"title": "m", "props": map[string]any{"priority": "mid"}}, 201, &mid)

	// dropping a value a task uses is refused, with the value
	drop := []map[string]any{{"name": "low", "from": []string{"low"}}, {"name": "high", "from": []string{"high"}}, {"name": "urgent", "from": []string{"urgent"}}}
	if e := ts.problem("PATCH", "/api/schema/props/priority", map[string]any{"values": drop}, 422); e.Type != "schema.in_use" || e.Args["value"] != "mid" || e.Args["count"] != float64(1) {
		t.Fatalf("in use: %+v", e)
	}

	before := ts.uc.Head()
	edit := []map[string]any{
		{"name": "none"},
		{"name": "high", "from": []string{"high", "urgent"}},
		{"name": "medium", "from": []string{"mid"}},
		{"name": "low", "from": []string{"low"}},
	}
	var w schemas.EditWrite
	ts.call("PATCH", "/api/schema/props/priority", map[string]any{"values": edit}, 200, &w)
	if v := w.Schema.Props["priority"].Values; !slices.Equal(v, []string{"none", "high", "medium", "low"}) {
		t.Errorf("values = %v", v)
	}
	if usecasetest.LastOp(t, ts.uc) != "schema" || ts.uc.Head() == before {
		t.Error("want a commit of op schema")
	}
	if got := ts.uc.TaskByULID("me", mustULID(t, urgent.Task.ULID)).Props.Get("priority").Value; got != "high" {
		t.Errorf("urgent task = %s", got)
	}
	if got := ts.uc.TaskByULID("me", mustULID(t, mid.Task.ULID)).Props.Get("priority").Value; got != "medium" {
		t.Errorf("mid task = %s", got)
	}
}
