package server

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase/maintenance"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
)

func TestPurgeBadPeriodOnPost(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	empty := maintenance.PurgePlan{Store: "me", Tasks: []maintenance.PurgeTask{}, Forwards: []maintenance.PurgeForward{}}
	if p := ts.problem("POST", "/api/purge", map[string]any{"older_than": "abc", "confirmed": empty}, 400); p.Type != "purge.bad_period" || p.Args["value"] != "abc" {
		t.Errorf("bad period: %+v", p)
	}
}

// ?store= picks the store; the other store is left alone.
func TestPurgeStoreParam(t *testing.T) {
	t.Parallel()
	ts := twoStores(t)
	for _, name := range []string{"me", "team"} {
		ss := ts.session()
		ss.StoreName = name
		v, err := tasks.Add(ss, tasks.AddInput{Title: "gone in " + name})
		if err != nil {
			t.Fatal(err)
		}
		ts.call("DELETE", "/api/tasks/"+v.ULID.String()+"?store="+name, nil, 200, nil)
	}
	var plan maintenance.PurgePlan
	ts.call("GET", "/api/purge?older_than=0d&store=team", nil, 200, &plan)
	if plan.Store != "team" || len(plan.Tasks) != 1 || plan.Tasks[0].Title != "gone in team" {
		t.Fatalf("team plan = %+v", plan)
	}
	ts.call("POST", "/api/purge?store=team", map[string]any{"older_than": "0d", "confirmed": plan}, 200, nil)
	ts.call("GET", "/api/purge?older_than=0d&store=team", nil, 200, &plan)
	if len(plan.Tasks) != 0 {
		t.Errorf("team plan after purge = %+v", plan)
	}
	ts.call("GET", "/api/purge?older_than=0d&store=me", nil, 200, &plan)
	if plan.Store != "me" || len(plan.Tasks) != 1 {
		t.Errorf("me plan = %+v; want its task untouched", plan)
	}
}

// A task moved to another store leaves a forward file; its moved_at goes
// through the date-time check of the schema.
func TestPurgeForwards(t *testing.T) {
	t.Parallel()
	ts := twoStores(t)
	ss := ts.session()
	ss.StoreName = "me"
	moved, err := tasks.Add(ss, tasks.AddInput{Title: "moves"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stores.MoveStore(ts.session(), stores.MoveStoreInput{Arg: moved.ULID.String(), To: "team", Yes: true}); err != nil {
		t.Fatal(err)
	}
	var plan maintenance.PurgePlan
	ts.call("GET", "/api/purge?older_than=0d&store=me", nil, 200, &plan)
	if len(plan.Forwards) != 1 || plan.Forwards[0].ULID != moved.ULID || plan.Forwards[0].MovedAt.IsZero() {
		t.Fatalf("plan = %+v", plan)
	}
	var res maintenance.PurgePlan
	ts.call("POST", "/api/purge?store=me", map[string]any{"older_than": "0d", "confirmed": plan}, 200, &res)
	if len(res.Forwards) != 1 {
		t.Errorf("purged = %+v", res)
	}
	ts.call("GET", "/api/purge?older_than=0d&store=me", nil, 200, &plan)
	if len(plan.Forwards) != 0 {
		t.Errorf("plan after purge = %+v", plan)
	}
}
