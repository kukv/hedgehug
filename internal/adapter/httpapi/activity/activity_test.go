package activity_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/activity"
	appactivity "github.com/kukv/hedgehug/internal/app/activity"
	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

const (
	s1 port.StoreID = "s1"
	s2 port.StoreID = "s2"
)

type fixture struct {
	api   humatest.TestAPI
	syncs *apptest.SyncLog
	clock *apptest.Clock
}

// setup は、Sample と SampleMobile を書いた Store s1 と、SampleTeam を書いた Store s2 のアクティビティの API を組む。
func setup(t *testing.T) *fixture {
	t.Helper()
	clock := apptest.NewClock(time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC))
	clock.Step = time.Second
	stores := apptest.NewStores(clock, apptest.NewIDs())
	log := apptest.NewCommitLog(stores, clock)
	for _, id := range []port.StoreID{s1, s2} {
		stores.AddStore(id)
	}
	for _, f := range []func() error{
		func() error { return apptest.Sample(t.Context(), log, s1) },
		func() error { return apptest.SampleMobile(t.Context(), log, s1) },
		func() error { return apptest.SampleTeam(t.Context(), log, s2) },
	} {
		if err := f(); err != nil {
			t.Fatal(err)
		}
	}
	f := &fixture{syncs: &apptest.SyncLog{}, clock: clock}
	_, f.api = humatest.New(t)
	activity.Register(f.api, appactivity.Service{
		Stores: log, Commits: log, Log: f.syncs,
		List: apptest.StoreList{Stores: []port.StoreInfo{{ID: s1, Name: "web", Color: "sky"}, {ID: s2, Name: "team", Color: "teal"}}},
	})
	return f
}

type row struct {
	Key       string `json:"key"`
	At        string `json:"at"`
	Actor     string `json:"actor"`
	Who       string `json:"who"`
	Text      string `json:"text"`
	Cursor    string `json:"cursor"`
	SHA       string `json:"sha"`
	Workspace string `json:"workspace"`
	Item      *struct {
		Display string `json:"display"`
		Link    string `json:"link"`
	} `json:"item"`
	Project *struct {
		Display string `json:"display"`
		Title   string `json:"title"`
		Link    string `json:"link"`
	} `json:"project"`
	Store struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Color string `json:"color"`
	} `json:"store"`
}

type page struct {
	Rows []row `json:"rows"`
	More bool  `json:"more"`
}

func (f *fixture) get(t *testing.T, q url.Values, wantCode int) page {
	t.Helper()
	res := f.api.Get("/api/activity?" + q.Encode())
	if res.Code != wantCode {
		t.Fatalf("GET %v = %d %s, want %d", q, res.Code, res.Body, wantCode)
	}
	var p page
	if wantCode == http.StatusOK {
		if err := json.Unmarshal(res.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

func (f *fixture) addSync(t *testing.T, store port.StoreID, at time.Time, sent, received int) {
	t.Helper()
	if err := f.syncs.Add(t.Context(), store, at, sent, received); err != nil {
		t.Fatal(err)
	}
}

func TestListActivity(t *testing.T) {
	f := setup(t)
	f.addSync(t, s1, f.clock.Now().Add(time.Hour), 2, 1)
	p := f.get(t, nil, http.StatusOK)
	if len(p.Rows) < 2 || p.More {
		t.Fatalf("rows = %d, more = %v", len(p.Rows), p.More)
	}
	first := p.Rows[0]
	if first.Actor != "sync" || first.Text != "origin と同期 (送信 2 件・受信 1 件)" ||
		first.Store.ID != "s1" || first.Store.Name != "web" || first.Store.Color != "sky" {
		t.Errorf("first = %+v", first)
	}
	at, err := time.Parse(time.RFC3339Nano, first.At)
	if err != nil || first.Cursor != at.UTC().Format("2006-01-02T15:04:05.000Z07:00")+"|"+first.Key {
		t.Errorf("at = %q, cursor = %q (%v)", first.At, first.Cursor, err)
	}
	var target row
	for _, r := range p.Rows {
		if r.Item != nil && r.Project != nil {
			target = r
			break
		}
	}
	if target.Item == nil || target.Item.Display != "TM-1" || target.Item.Link != "current" ||
		target.Project == nil || target.Project.Display != "TM-P1" || target.Project.Title != "運営" || target.Project.Link != "current" {
		t.Errorf("row with item and project = %+v (item %+v, project %+v)", target, target.Item, target.Project)
	}
	if rest := p.Rows[1]; rest.SHA == "" || rest.Actor == "" || rest.Text == "" {
		t.Errorf("second = %+v", rest)
	}
}

func TestListActivityMore(t *testing.T) {
	f := setup(t)
	base := f.clock.Now().Add(time.Hour)
	for i := range 305 {
		f.addSync(t, s1, base.Add(time.Duration(i)*time.Second), 1, 0)
	}
	first := f.get(t, nil, http.StatusOK)
	if len(first.Rows) != 300 || !first.More {
		t.Fatalf("first = %d rows, more = %v", len(first.Rows), first.More)
	}
	second := f.get(t, url.Values{"before": {first.Rows[299].Cursor}}, http.StatusOK)
	if len(second.Rows) < 5 || second.More || second.Rows[0].Key == first.Rows[299].Key {
		t.Errorf("second = %d rows, more = %v", len(second.Rows), second.More)
	}
}

func TestListActivityFilters(t *testing.T) {
	f := setup(t)
	f.addSync(t, s1, f.clock.Now(), 1, 0)
	f.addSync(t, s2, f.clock.Now(), 1, 0)
	syncRows := f.get(t, url.Values{"kind": {"sync"}}, http.StatusOK).Rows
	if len(syncRows) != 2 {
		t.Errorf("kind=sync = %d rows, want 2", len(syncRows))
	}
	for _, r := range syncRows {
		if r.Actor != "sync" {
			t.Errorf("kind=sync row = %+v", r)
		}
	}
	all := f.get(t, url.Values{"store": {string(s1)}}, http.StatusOK).Rows
	mobile := f.get(t, url.Values{"store": {string(s1)}, "workspace": {string(apptest.SampleMobileWorkspace)}}, http.StatusOK).Rows
	if len(mobile) == 0 || len(mobile) >= len(all) {
		t.Fatalf("store rows = %d, mobile rows = %d", len(all), len(mobile))
	}
	for _, r := range append(all, mobile...) {
		if r.Store.ID != string(s1) {
			t.Errorf("row of another store: %+v", r)
		}
	}
}

// 選んだワークスペースの絞り込みは、そのワークスペースを持つ行だけを出す。Web の行は Mobile の絞り込みに出ない。
func TestListActivityWorkspaceOnly(t *testing.T) {
	f := setup(t)
	of := func(id string) []row {
		return f.get(t, url.Values{"store": {string(s1)}, "workspace": {id}}, http.StatusOK).Rows
	}
	mobile := of(string(apptest.SampleMobileWorkspace))
	web := of(string(apptest.SampleWorkspace))
	inMobile := map[string]bool{}
	for _, r := range mobile {
		inMobile[r.Key] = true
	}
	for _, r := range web {
		if r.Workspace != "" && inMobile[r.Key] {
			t.Errorf("row %+v is in both the Web and the Mobile filters", r)
		}
	}
	for _, r := range mobile {
		if r.Workspace != "" && r.Workspace != "Mobile" {
			t.Errorf("mobile filter has a row of %q: %+v", r.Workspace, r)
		}
	}
}

func TestListActivityRejects(t *testing.T) {
	f := setup(t)
	for _, q := range []url.Values{
		{"kind": {"bot"}},
		{"workspace": {string(apptest.SampleWorkspace)}},
		{"before": {"yesterday"}},
	} {
		f.get(t, q, http.StatusUnprocessableEntity)
	}
}
