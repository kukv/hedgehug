package server

import (
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/inbox"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/ops"
)

func TestUndo(t *testing.T) {
	ts := newTestServer(t)
	ts.withAccount()
	id := ts.add("one", "")
	var w taskWriteBody
	ts.call("PATCH", "/api/tasks/"+id, map[string]any{"title": "two"}, 200, &w)
	var out struct {
		Undone ops.LogView `json:"undone"`
		Commit *string     `json:"commit"`
		Kept   []any       `json:"kept"`
	}
	ts.call("POST", "/api/undo", nil, 200, &out)
	if out.Commit == nil || out.Undone.Op == nil || *out.Undone.Op != "set" || out.Kept == nil {
		t.Fatalf("undo = %+v", out)
	}
	var show ops.ShowView
	ts.call("GET", "/api/tasks/"+id, nil, 200, &show)
	if show.Title != "one" {
		t.Errorf("title = %q", show.Title)
	}
}

func TestUndoWithoutAccountIsConflict(t *testing.T) {
	ts := newTestServer(t)
	if p := ts.problem("POST", "/api/undo", nil, 409); p.Type != "account.none" {
		t.Errorf("problem = %+v", p)
	}
}

func TestActivity(t *testing.T) {
	ts := newTestServer(t)
	ts.withAccount()
	ts.add("one", "")
	ts.add("two", "")
	var out []ops.LogView
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
	if len(out) != 2 {
		t.Errorf("author: %d entries, want the two adds", len(out))
	}
	ts.problem("GET", "/api/activity?sync=maybe", nil, 400)
}

func TestNotifications(t *testing.T) {
	ts := newTestServer(t)
	id := ts.add("mine", "")
	var show ops.ShowView
	ts.call("GET", "/api/tasks/"+id, nil, 200, &show)
	u := mustULID(t, show.ULID)
	// Another writer's title won over mine ("first"): the sync told me.
	nid := ts.notifyMe(overwrittenItem(u, id, "title", `"first"`))
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

func TestUndoNothingIsConflict(t *testing.T) {
	ts := newTestServer(t)
	ts.withAccount()
	if p := ts.problem("POST", "/api/undo", nil, 409); p.Type != "undo.nothing" {
		t.Errorf("problem = %+v", p)
	}
}

func TestNotificationsWarnOnce(t *testing.T) {
	ts := newTestServer(t) // no account: NoAccountsWarning
	ts.call("GET", "/api/notifications?all=true", nil, 200, nil)
	if n := strings.Count(ts.log.String(), ops.NoAccountsWarning); n != 1 {
		t.Errorf("warned %d times: %q", n, ts.log.String())
	}
}
