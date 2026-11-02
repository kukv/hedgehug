package server

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/ops"
)

// addProject adds a project and returns its ID.
func (ts *testServer) addProject(title string) ops.TaskView {
	ts.t.Helper()
	var out taskWriteBody
	ts.call("POST", "/api/tasks", map[string]any{"title": title, "kind": "project"}, 201, &out)
	if out.Task == nil {
		ts.t.Fatalf("created = %+v", out)
	}
	return *out.Task
}

func TestScopedPutAndDelete(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	p := ts.addProject("Web")
	path := "/api/schema/scoped/" + p.ID
	four := []string{"todo", "in_progress", "done", "canceled"}

	var v ops.ScopedView
	ts.call("PUT", path, map[string]any{"statuses": four}, 200, &v)
	if v.Commit == nil || v.Scoped == nil || !slices.Equal(v.Scoped.Statuses, four) || v.Project.ULID != p.ULID || v.Project.ID != p.ID {
		t.Fatalf("statuses = %+v", v)
	}

	// what is left out stays
	v = ops.ScopedView{}
	ts.call("PUT", path, map[string]any{"transitions": map[string][]string{"todo": {"in_progress"}}, "required": map[string][]string{"done": {"points"}}}, 200, &v)
	if v.Scoped == nil || !slices.Equal(v.Scoped.Statuses, four) || !slices.Equal(v.Scoped.Workflow.Transitions["todo"], []string{"in_progress"}) || !slices.Equal(v.Scoped.Workflow.Required["done"], []string{"points"}) {
		t.Fatalf("workflow = %+v", v.Scoped)
	}

	// the same again writes nothing
	var same ops.ScopedView
	ts.call("PUT", path, map[string]any{"statuses": four}, 200, &same)
	if same.Commit != nil {
		t.Errorf("commit = %v, want null", *same.Commit)
	}

	// GET /api/schema shows it
	var sc ops.SchemaView
	ts.call("GET", "/api/schema", nil, 200, &sc)
	if len(sc.Scoped) != 1 || sc.Scoped[0].Where != "project:"+p.ULID {
		t.Errorf("schema.scoped = %+v", sc.Scoped)
	}

	// an empty list or object goes back to the store; the ULID names the project too
	var cleared ops.ScopedView
	ts.call("PUT", "/api/schema/scoped/"+p.ULID, map[string]any{"statuses": []string{}, "transitions": map[string][]string{}}, 200, &cleared)
	if cleared.Scoped == nil || cleared.Scoped.Statuses != nil || len(cleared.Scoped.Workflow.Transitions) != 0 || len(cleared.Scoped.Workflow.Required) != 1 {
		t.Fatalf("after clearing = %+v", cleared.Scoped)
	}

	var del ops.ScopedView
	ts.call("DELETE", path, nil, 200, &del)
	if del.Scoped != nil || del.Commit == nil {
		t.Errorf("delete = %+v", del)
	}
	var again ops.ScopedView
	ts.call("DELETE", path, nil, 200, &again)
	if again.Scoped != nil || again.Commit != nil {
		t.Errorf("second delete = %+v", again)
	}
}

func TestScopedPutErrors(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	p := ts.addProject("Web")
	path := "/api/schema/scoped/" + p.ID

	ts.problem("PUT", path, map[string]any{}, 400)
	ts.problem("PUT", path, map[string]any{"statuses": nil}, 400) // null means nothing: leave it out, or give []
	if e := ts.problem("PUT", path, map[string]any{"statuses": []string{"todo", "in_progress", "canceled"}}, 422); e.Type != "scoped.invalid" {
		t.Errorf("a default status hidden: %+v", e)
	}
	if e := ts.problem("PUT", path, map[string]any{"required": map[string][]string{"done": {"nope"}}}, 422); e.Type != "scoped.invalid" || e.Args["problems"] == nil {
		t.Errorf("an unknown field: %+v", e)
	}
	var out taskWriteBody
	ts.call("POST", "/api/tasks", map[string]any{"title": "plain"}, 201, &out)
	if e := ts.problem("PUT", "/api/schema/scoped/"+out.Task.ID, map[string]any{"statuses": []string{}}, 422); e.Type != "scoped.not_container" {
		t.Errorf("a task: %+v", e)
	}
	ts.problem("PUT", "/api/schema/scoped/NOPE-9", map[string]any{"statuses": []string{}}, 404)
	ts.problem("DELETE", "/api/schema/scoped/NOPE-9", nil, 404)
}

func TestScopedPutPublishesSchemaChanged(t *testing.T) {
	t.Parallel()
	ts, ch := pollServer(t)
	p := ts.addProject("Web")
	ts.srv.poller.settle()
	ts.call("PUT", "/api/schema/scoped/"+p.ID, map[string]any{"statuses": []string{"todo", "in_progress", "done", "canceled"}}, 200, nil)
	waitEvent[schemaChanged](t, ch, nil)
	ts.srv.poller.settle()
	ts.call("DELETE", "/api/schema/scoped/"+p.ID, nil, 200, nil)
	waitEvent[schemaChanged](t, ch, nil)
}
