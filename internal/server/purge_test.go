package server

import (
	"testing"

	"github.com/kukv/hedgehug/internal/ops"
)

// deleteTask deletes (not purges) the task with the ID id.
func (ts *testServer) deleteTask(id string) {
	ts.t.Helper()
	ts.call("DELETE", "/api/tasks/"+id, nil, 200, nil)
}

func TestPurgeAPI(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	keep := ts.add("keep", "")
	gone := ts.add("gone", "")
	ts.deleteTask(gone)

	// The plan lists the deleted task and not the live one.
	var plan ops.PurgePlan
	ts.call("GET", "/api/purge?older_than=0d", nil, 200, &plan)
	if plan.Store != "me" || len(plan.Tasks) != 1 || plan.Tasks[0].ID != gone || plan.Tasks[0].Title != "gone" || len(plan.Forwards) != 0 {
		t.Fatalf("plan = %+v", plan)
	}
	// Nothing is old enough for the default 90d.
	var none ops.PurgePlan
	ts.call("GET", "/api/purge", nil, 200, &none)
	if len(none.Tasks) != 0 || len(none.Forwards) != 0 {
		t.Errorf("default plan = %+v; want empty", none)
	}
	if p := ts.problem("GET", "/api/purge?older_than=abc", nil, 400); p.Type != "purge.bad_period" || p.Args["value"] != "abc" {
		t.Errorf("bad period: %+v", p)
	}

	// The confirmed plan is required: none is request.invalid, an empty one
	// is confirm.required.
	if p := ts.problem("POST", "/api/purge", map[string]any{"older_than": "0d"}, 400); p.Type != "request.invalid" {
		t.Errorf("no confirmed: %+v", p)
	}
	empty := ops.PurgePlan{Store: "me", Tasks: []ops.PurgeTask{}, Forwards: []ops.PurgeForward{}}
	if p := ts.problem("POST", "/api/purge", map[string]any{"older_than": "0d", "confirmed": empty}, 400); p.Type != "confirm.required" {
		t.Errorf("empty confirmed: %+v", p)
	}

	// A task deleted after the plan was shown: purge.changed, and nothing is removed.
	late := ts.add("late", "")
	ts.deleteTask(late)
	if p := ts.problem("POST", "/api/purge", map[string]any{"older_than": "0d", "confirmed": plan}, 409); p.Type != "purge.changed" {
		t.Errorf("changed: %+v", p)
	}
	ts.call("GET", "/api/purge?older_than=0d", nil, 200, &plan)
	if len(plan.Tasks) != 2 {
		t.Fatalf("plan after the refusal = %+v; want both deleted tasks still there", plan)
	}

	// The plan as shown now goes through.
	var res struct {
		ops.PurgePlan
		Commit string `json:"commit"`
	}
	ts.call("POST", "/api/purge", map[string]any{"older_than": "0d", "confirmed": plan}, 200, &res)
	if res.Commit == "" || len(res.Tasks) != 2 {
		t.Fatalf("purge = %+v", res)
	}
	ts.call("GET", "/api/purge?older_than=0d", nil, 200, &plan)
	if len(plan.Tasks) != 0 {
		t.Errorf("plan after purge = %+v", plan)
	}
	ts.call("GET", "/api/tasks/"+keep, nil, 200, nil)
	ts.problem("GET", "/api/tasks/"+gone, nil, 404)
}
