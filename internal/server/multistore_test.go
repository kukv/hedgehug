package server

import (
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/inbox"
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
	now := time.Now()
	for store, id := range map[string]string{"me": "ME-1", "team": "TM-1"} {
		var show ops.ShowView
		ts.call("GET", "/api/tasks/"+id+"?store="+store, nil, 200, &show)
		ts.notifyIn(store, inbox.Item{Kind: inbox.Mention, Task: mustULID(t, show.ULID), ID: id, Commit: "c1", Time: now})
	}
	type item struct {
		ULID  string `json:"ulid"`
		Store string `json:"store"`
	}
	storesOfItems := func(items []item) []string {
		var got []string
		for _, it := range items {
			got = append(got, it.Store)
		}
		slices.Sort(got)
		return got
	}
	var items []item
	ts.call("GET", "/api/notifications", nil, 200, &items)
	if got := storesOfItems(items); !slices.Equal(got, []string{"me", "team"}) {
		t.Fatalf("notifications without ?store = %v; want me and team", got)
	}
	ts.call("GET", "/api/notifications?store=team", nil, 200, &items)
	if got := storesOfItems(items); !slices.Equal(got, []string{"team"}) {
		t.Fatalf("notifications ?store=team = %v", got)
	}
	teamItem := items[0].ULID
	// ?store= narrows the write too: the item of team is not in me.
	ts.problem("PATCH", "/api/notifications/"+teamItem+"?store=me", map[string]any{"read": true}, 404)
	ts.call("PATCH", "/api/notifications/"+teamItem+"?store=team", map[string]any{"read": true}, 204, nil)
	ts.call("GET", "/api/notifications", nil, 200, &items)
	if got := storesOfItems(items); !slices.Equal(got, []string{"me"}) {
		t.Errorf("unread after reading the item of team = %v", got)
	}
}

// Each task's checklist and each project's progress come from its own store,
// where two stores number their tasks alike.
func TestChecklistAndProgressPerStore(t *testing.T) {
	ts := twoStores(t)
	bodies := map[string]string{"me": "- [x] a\n- [ ] b\n", "team": "- [x] a\n- [x] b\n- [ ] c\n"}
	want := map[string]ops.Count{"me": {Done: 1, Total: 2}, "team": {Done: 2, Total: 3}}
	for store, body := range bodies {
		ss := ts.session()
		ss.StoreName = store
		res, err := ss.Add(ops.AddInput{Title: "p-" + store, Kind: "project"})
		if err != nil {
			t.Fatal(err)
		}
		pv, _ := res.View()
		res, err = ss.Add(ops.AddInput{Title: "c-" + store, Parent: pv.ID})
		if err != nil {
			t.Fatal(err)
		}
		cv, _ := res.View()
		if _, err := ss.Patch(cv.ID, ops.PatchInput{Body: &body}); err != nil {
			t.Fatal(err)
		}
	}
	check := func(what string, tasks []*ops.TaskView) {
		t.Helper()
		seen := 0
		for _, v := range tasks {
			switch v.Title {
			case "c-" + v.Store:
				seen++
				if v.Checklist == nil || *v.Checklist != want[v.Store] {
					t.Errorf("%s: checklist of %s = %+v; want %+v", what, v.Title, v.Checklist, want[v.Store])
				}
			case "p-" + v.Store:
				seen++
				if v.Progress == nil || v.Progress.Total != 1 {
					t.Errorf("%s: progress of %s = %+v; want 1 child", what, v.Title, v.Progress)
				}
			}
		}
		if seen != 4 {
			t.Errorf("%s: found %d of the 4 tasks", what, seen)
		}
	}
	var out listBody
	ts.call("GET", "/api/tasks", nil, 200, &out)
	check("list", out.Tasks)
	ts.call("GET", "/api/tasks?tree=true", nil, 200, &out)
	var flat []*ops.TaskView
	for _, r := range out.Roots {
		flat = append(flat, r)
		flat = append(flat, r.Children...)
	}
	check("tree", flat)
}

func TestActivityFiltersMergedList(t *testing.T) {
	ts := twoStores(t)
	ts.withAccount()
	for _, store := range []string{"me", "team", "me", "team"} {
		ss := ts.session()
		ss.StoreName = store
		if _, err := ss.Add(ops.AddInput{Title: "later"}); err != nil {
			t.Fatal(err)
		}
	}
	var out []ops.LogView
	author := "?author=" + ts.accountID()
	ts.call("GET", "/api/activity"+author, nil, 200, &out)
	if len(out) != 4 {
		t.Fatalf("author = %d commits; want the 4 adds", len(out))
	}
	// The limit cuts the merged list, not each store's.
	ts.call("GET", "/api/activity"+author+"&limit=1", nil, 200, &out)
	if len(out) != 1 {
		t.Errorf("author, limit=1 = %d commits", len(out))
	}
	ts.call("GET", "/api/activity"+author+"&limit=3", nil, 200, &out)
	if len(out) != 3 {
		t.Errorf("author, limit=3 = %d commits", len(out))
	}
	ts.call("GET", "/api/activity?limit=2", nil, 200, &out)
	if len(out) != 2 {
		t.Errorf("limit=2 = %d commits", len(out))
	}
	ts.call("GET", "/api/activity?sync=only", nil, 200, &out)
	if len(out) != 0 {
		t.Errorf("sync=only = %d commits; want none", len(out))
	}
	ts.call("GET", "/api/activity?sync=exclude&limit=5", nil, 200, &out)
	if len(out) != 5 {
		t.Errorf("sync=exclude, limit=5 = %d commits", len(out))
	}
	stores := map[string]bool{}
	for _, l := range out {
		stores[l.Store] = true
	}
	if !stores["me"] || !stores["team"] {
		t.Errorf("sync=exclude stores = %v", stores)
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
