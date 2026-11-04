package server

import (
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/inbox"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/notifications"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
)

func TestUndo(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	id := ts.add("one", "")
	var w taskWriteBody
	ts.call("PATCH", "/api/tasks/"+id, map[string]any{"title": "two"}, 200, &w)
	var out struct {
		Undone usecase.LogView `json:"undone"`
		Commit *string         `json:"commit"`
		Kept   []any           `json:"kept"`
	}
	ts.call("POST", "/api/undo", nil, 200, &out)
	if out.Commit == nil || out.Undone.Op == nil || *out.Undone.Op != "set" || out.Kept == nil {
		t.Fatalf("undo = %+v", out)
	}
	var show usecase.ShowView
	ts.call("GET", "/api/tasks/"+id, nil, 200, &show)
	if show.Title != "one" {
		t.Errorf("title = %q", show.Title)
	}
}

func TestUndoWithoutAccountIsConflict(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ts.dropDefaultAccount()
	if p := ts.problem("POST", "/api/undo", nil, 409); p.Type != "account.none" {
		t.Errorf("problem = %+v", p)
	}
}

func TestActivity(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ts.add("one", "")
	ts.add("two", "")
	var out []usecase.LogView
	ts.call("GET", "/api/activity", nil, 200, &out)
	if len(out) < 3 { // the store's first commit and the two adds
		t.Fatalf("activity = %d entries", len(out))
	}
	ts.call("GET", "/api/activity?limit=1", nil, 200, &out)
	if len(out) != 1 || out[0].Op == nil || *out[0].Op != "add" {
		t.Errorf("limit=1: %+v", out)
	}
	ts.call("GET", "/api/activity?sync=only", nil, 200, &out)
	if len(out) != 0 {
		t.Errorf("sync=only: %d entries, want none", len(out))
	}
	ts.call("GET", "/api/activity?author="+ts.accountID(), nil, 200, &out)
	if len(out) != 3 {
		t.Errorf("author: %d entries, want the init and the two adds", len(out))
	}
	ts.problem("GET", "/api/activity?sync=maybe", nil, 400)
	// a workspace by its prefix or ULID; the adds tell their workspace
	ts.call("GET", "/api/activity?workspace=ME&limit=1", nil, 200, &out)
	if len(out) != 1 || out[0].Workspace == nil || out[0].Workspace.Name != "me" {
		t.Fatalf("workspace=ME: %+v", out)
	}
	if p := ts.problem("GET", "/api/activity?workspace="+out[0].Workspace.ULID+",nope", nil, 404); p.Type != "workspace.not_found" {
		t.Errorf("problem = %+v", p)
	}
	ts.call("GET", "/api/activity?workspace="+out[0].Workspace.ULID, nil, 200, &out)
	if len(out) < 2 {
		t.Errorf("workspace=<ULID>: %d entries, want the two adds at least", len(out))
	}
}

func TestNotifications(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	id := ts.add("mine", "")
	var show usecase.ShowView
	ts.call("GET", "/api/tasks/"+id, nil, 200, &show)
	u := mustULID(t, show.ULID)
	// Another writer's title won over mine ("first"): the sync told me.
	nid := ts.notifyMe(overwrittenItem(u, id, `"first"`))
	var items []struct {
		ULID string `json:"ulid"`
		Kind string `json:"kind"`
		Read bool   `json:"read"`
	}
	ts.call("GET", "/api/notifications", nil, 200, &items)
	if len(items) != 1 || items[0].ULID != nid || items[0].Read {
		t.Fatalf("unread = %+v", items)
	}
	ts.call("PATCH", "/api/notifications/"+nid, map[string]any{"read": true}, 204, nil)
	ts.call("GET", "/api/notifications", nil, 200, &items)
	if len(items) != 0 {
		t.Errorf("unread after read = %+v", items)
	}
	ts.call("GET", "/api/notifications?all=true", nil, 200, &items)
	if len(items) != 1 || !items[0].Read {
		t.Errorf("all = %+v", items)
	}
	if p := ts.problem("PATCH", "/api/notifications/"+nid, map[string]any{"read": false}, 400); p.Type != "notification.unread" {
		t.Errorf("unread: %+v", p)
	}
	var w taskWriteBody
	ts.call("POST", "/api/notifications/"+nid+"/revert", nil, 200, &w)
	if w.Task == nil || w.Task.Title != "first" || w.Commit == nil {
		t.Errorf("revert = %+v", w)
	}
	// A mention cannot be reverted.
	mention := ts.notifyMe(inbox.Item{Kind: inbox.Mention, Task: u, ID: id, Commit: "c2", Time: time.Now()})
	if p := ts.problem("POST", "/api/notifications/"+mention+"/revert", nil, 422); p.Type != "inbox.not_revertable" {
		t.Errorf("revert of a mention: %+v", p)
	}
	missing := model.NewULID().String()
	if p := ts.problem("PATCH", "/api/notifications/"+missing, map[string]any{"read": true}, 404); p.Type != "inbox.not_found" {
		t.Errorf("missing: %+v", p)
	}
	ts.problem("POST", "/api/notifications/"+missing+"/revert", nil, 404)
	ts.problem("PATCH", "/api/notifications/short", map[string]any{"read": true}, 400)
}

func TestNotificationsReadMany(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	id := ts.add("mine", "")
	var show usecase.ShowView
	ts.call("GET", "/api/tasks/"+id, nil, 200, &show)
	u := mustULID(t, show.ULID)
	first := ts.notifyMe(overwrittenItem(u, id, `"first"`))
	second := ts.notifyMe(inbox.Item{Kind: inbox.Mention, Task: u, ID: id, Commit: "c2", Time: time.Now()})
	third := ts.notifyMe(inbox.Item{Kind: inbox.Mention, Task: u, ID: id, Commit: "c3", Time: time.Now()})
	var items []struct {
		ULID string `json:"ulid"`
	}

	// ids marks those and no others.
	var res struct {
		Read []string `json:"read"`
	}
	ts.call("POST", "/api/notifications/read", map[string]any{"ids": []string{first, second}}, 200, &res)
	if len(res.Read) != 2 {
		t.Errorf("read = %v", res.Read)
	}
	ts.call("GET", "/api/notifications", nil, 200, &items)
	if len(items) != 1 || items[0].ULID != third {
		t.Errorf("unread after ids = %+v; want only %s", items, third)
	}

	// all marks every unread item.
	ts.call("POST", "/api/notifications/read", map[string]any{"all": true}, 200, &res)
	if len(res.Read) != 1 || res.Read[0] != third {
		t.Errorf("read all = %v", res.Read)
	}
	ts.call("GET", "/api/notifications", nil, 200, &items)
	if len(items) != 0 {
		t.Errorf("unread after all = %+v", items)
	}

	// Both, neither, an empty ids and all: false are the caller's mistake.
	for _, body := range []map[string]any{
		{"all": true, "ids": []string{first}},
		{},
		{"ids": []string{}},
		{"all": false},
	} {
		if p := ts.problem("POST", "/api/notifications/read", body, 400); p.Type != "notification.read_args" {
			t.Errorf("body %v: %+v", body, p)
		}
	}
	if p := ts.problem("POST", "/api/notifications/read", map[string]any{"ids": []string{"abc"}}, 400); p.Type != "inbox.ulid" {
		t.Errorf("non-ULID id: %+v", p)
	}
	if p := ts.problem("POST", "/api/notifications/read", map[string]any{"ids": []string{model.NewULID().String()}}, 404); p.Type != "inbox.not_found" {
		t.Errorf("missing id: %+v", p)
	}
}

// ?store= picks the store whose items are marked read.
func TestNotificationsReadStoreParam(t *testing.T) {
	t.Parallel()
	ts := twoStores(t)
	ss := ts.session()
	ss.StoreName = "team"
	v, err := tasks.Add(ss, tasks.AddInput{Title: "in team"})
	if err != nil {
		t.Fatal(err)
	}
	mine := ts.notifyIn("me", overwrittenItem(v.ULID, v.ULID.String(), `"x"`))
	theirs := ts.notifyIn("team", overwrittenItem(v.ULID, v.ULID.String(), `"y"`))
	var res struct {
		Read []string `json:"read"`
	}
	ts.call("POST", "/api/notifications/read?store=team", map[string]any{"all": true}, 200, &res)
	if len(res.Read) != 1 || res.Read[0] != theirs {
		t.Fatalf("read = %v; want only %s", res.Read, theirs)
	}
	ts.call("POST", "/api/notifications/read?store=me", map[string]any{"ids": []string{mine}}, 200, &res)
	if len(res.Read) != 1 || res.Read[0] != mine {
		t.Errorf("read = %v; want only %s", res.Read, mine)
	}
}

func TestUndoNothingIsConflict(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	if p := ts.problem("POST", "/api/undo", nil, 409); p.Type != "undo.nothing" {
		t.Errorf("problem = %+v", p)
	}
}

func TestNotificationsWarnOnce(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t) // no account: NoAccountsWarning
	ts.dropDefaultAccount()
	ts.call("GET", "/api/notifications?all=true", nil, 200, nil)
	if n := strings.Count(ts.log.String(), notifications.NoAccountsWarning); n != 1 {
		t.Errorf("warned %d times: %q", n, ts.log.String())
	}
}

func TestActivityStoreDescription(t *testing.T) {
	t.Parallel()
	y, err := OpenAPIYAML()
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]map[string]struct {
			Parameters []struct {
				Name        string `yaml:"name"`
				Description string `yaml:"description"`
			} `yaml:"parameters"`
		} `yaml:"paths"`
	}
	if err := yaml.Unmarshal(y, &doc); err != nil {
		t.Fatal(err)
	}
	for _, p := range doc.Paths["/api/activity"]["get"].Parameters {
		if p.Name == "store" {
			if !strings.Contains(p.Description, "every store") {
				t.Errorf("store = %q; want it to say that without it every store is read", p.Description)
			}
			return
		}
	}
	t.Fatal("GET /api/activity has no store parameter")
}

func TestActivityWithoutStoreReadsEveryStore(t *testing.T) {
	t.Parallel()
	ts := twoStores(t)
	ss := ts.session()
	ss.StoreName = "me"
	if _, err := tasks.Add(ss, tasks.AddInput{Title: "in me"}); err != nil {
		t.Fatal(err)
	}
	ss.StoreName = "team"
	if _, err := tasks.Add(ss, tasks.AddInput{Title: "in team"}); err != nil {
		t.Fatal(err)
	}
	var out []usecase.LogView
	ts.call("GET", "/api/activity", nil, 200, &out)
	stores := map[string]bool{}
	for _, v := range out {
		stores[v.Store] = true
	}
	if !stores["me"] || !stores["team"] {
		t.Errorf("stores in the activity = %v; want me and team", stores)
	}
	ts.call("GET", "/api/activity?store=team", nil, 200, &out)
	for _, v := range out {
		if v.Store != "team" {
			t.Errorf("?store=team gave a commit of %q", v.Store)
		}
	}
}
