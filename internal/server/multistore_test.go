package server

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/ops"
)

// twoStores is a test server with the stores "me" (prefix ME) and "team"
// (prefix TM), a task "one" in me and a task "two" in team.
func twoStores(t *testing.T) *testServer {
	t.Helper()
	ts := newTestServer(t)
	ts.call("POST", "/api/stores", map[string]any{"name": "team", "prefix": "TM"}, 201, nil)
	for store, title := range map[string]string{"me": "one", "team": "two"} {
		ss := ts.session()
		ss.StoreName = store
		if _, err := ss.Add(ops.AddInput{Title: title}); err != nil {
			t.Fatal(err)
		}
	}
	return ts
}

func storesOf(tasks []*ops.TaskView) map[string]string {
	m := map[string]string{}
	for _, v := range tasks {
		m[v.Title] = v.Store
	}
	return m
}

func TestListSpansStores(t *testing.T) {
	ts := twoStores(t)
	var out listBody
	ts.call("GET", "/api/tasks", nil, 200, &out)
	got := storesOf(out.Tasks)
	if len(out.Tasks) != 2 || got["one"] != "me" || got["two"] != "team" {
		t.Fatalf("list without ?store = %v", got)
	}
	for _, v := range out.Tasks {
		if v.Store == "" {
			t.Errorf("row without its store: %+v", v)
		}
	}
	ts.call("GET", "/api/tasks?store=team", nil, 200, &out)
	if got := storesOf(out.Tasks); len(got) != 1 || got["two"] != "team" {
		t.Errorf("?store=team = %v", got)
	}
	ts.call("GET", "/api/tasks?store=me", nil, 200, &out)
	if got := storesOf(out.Tasks); len(got) != 1 || got["one"] != "me" {
		t.Errorf("?store=me = %v", got)
	}
}

func TestListGroupsSpanStores(t *testing.T) {
	ts := twoStores(t)
	var out listBody
	ts.call("GET", "/api/tasks?q=group%3Astatus", nil, 200, &out)
	var n int
	for _, g := range out.Groups {
		n += len(g.ULIDs)
	}
	if n != 2 {
		t.Errorf("groups = %+v; want both tasks", out.Groups)
	}
}

func TestTreeSpansStores(t *testing.T) {
	ts := twoStores(t)
	var out listBody
	ts.call("GET", "/api/tasks?tree=true", nil, 200, &out)
	if got := storesOf(out.Roots); len(got) != 2 || got["one"] != "me" || got["two"] != "team" {
		t.Fatalf("tree roots = %v", got)
	}
	ts.call("GET", "/api/tasks?tree=true&store=team", nil, 200, &out)
	if got := storesOf(out.Roots); len(got) != 1 || got["two"] != "team" {
		t.Errorf("tree ?store=team = %v", got)
	}
}

func TestActivitySpansStores(t *testing.T) {
	ts := twoStores(t)
	var all []ops.LogView
	ts.call("GET", "/api/activity", nil, 200, &all)
	seen := map[string]bool{}
	for _, l := range all {
		seen[l.Store] = true
	}
	if !seen["me"] || !seen["team"] {
		t.Errorf("activity stores = %v; want me and team", seen)
	}
	var team []ops.LogView
	ts.call("GET", "/api/activity?store=team", nil, 200, &team)
	if len(team) == 0 || slices.ContainsFunc(team, func(l ops.LogView) bool { return l.Store != "team" }) {
		t.Errorf("?store=team = %+v", team)
	}
}

func TestHistoryNamesTheStore(t *testing.T) {
	ts := twoStores(t)
	var h []ops.LogView
	ts.call("GET", "/api/tasks/TM-1/history", nil, 200, &h)
	if len(h) == 0 || h[0].Store != "team" {
		t.Errorf("history of TM-1 = %+v", h)
	}
}

func TestDoctorSpansStores(t *testing.T) {
	ts := twoStores(t)
	var reps []doctorReport
	ts.call("GET", "/api/doctor", nil, 200, &reps)
	if len(reps) != 2 || reps[0].Store != "me" || reps[1].Store != "team" {
		t.Fatalf("doctor = %+v; want a report for me and for team", reps)
	}
	ts.call("POST", "/api/doctor/fix", nil, 200, &reps)
	if len(reps) != 2 {
		t.Errorf("doctor fix = %+v", reps)
	}
	ts.call("GET", "/api/doctor?store=team", nil, 200, &reps)
	if len(reps) != 1 || reps[0].Store != "team" {
		t.Errorf("doctor ?store=team = %+v", reps)
	}
}

func TestNotificationsSpanStores(t *testing.T) {
	ts := twoStores(t)
	ts.withAccount()
	var items []map[string]any
	ts.call("GET", "/api/notifications", nil, 200, &items)
	if items == nil {
		t.Errorf("notifications = %v; want an array", items)
	}
}

// With several stores a create names its store; ops does not pick one.
func TestCreateNeedsAStoreWhenSeveral(t *testing.T) {
	ts := twoStores(t)
	if p := ts.problem("POST", "/api/tasks", map[string]any{"title": "x"}, 422); p.Type != "store.choose" {
		t.Errorf("create without ?store: %+v", p)
	}
	ts.call("POST", "/api/tasks?store=team", map[string]any{"title": "x"}, 201, nil)
}
