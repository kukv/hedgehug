package server

import (
	"testing"
	"time"
)

type syncStatusBody struct {
	Store     string     `json:"store"`
	Remotes   []string   `json:"remotes"`
	NextFetch *time.Time `json:"next_fetch"`
	Unpushed  *int       `json:"unpushed"`
}

func TestSyncAPI(t *testing.T) {
	ts, _, remote := loopServer(t)
	waitFirstSync(t, ts)
	var st []syncStatusBody
	ts.call("GET", "/api/sync", nil, 200, &st)
	if len(st) != 1 || st[0].Store != "me" || st[0].NextFetch == nil {
		t.Fatalf("status = %+v", st)
	}
	var out taskWriteBody
	ts.call("POST", "/api/tasks", map[string]any{"title": "one"}, 201, &out)
	var reps []struct {
		Store  string   `json:"store"`
		Pushed []string `json:"pushed"`
		Error  string   `json:"error"`
	}
	ts.call("POST", "/api/sync", nil, 200, &reps)
	if len(reps) != 1 || reps[0].Error != "" || remoteHead(t, remote) != *out.Commit {
		t.Errorf("sync = %+v", reps)
	}
	ts.call("POST", "/api/sync?store=me", nil, 200, &reps)
	if len(reps) != 1 {
		t.Errorf("sync of me = %+v", reps)
	}
}

func TestPresenceAPI(t *testing.T) {
	ts := newTestServer(t)
	id := ts.srv.presence.Connect()
	ts.call("PUT", "/api/presence", map[string]any{"client": id, "visible": false}, 204, nil)
	if ts.srv.presence.Visible() {
		t.Error("still visible")
	}
	if p := ts.problem("PUT", "/api/presence", map[string]any{"client": "nope", "visible": true}, 404); p.Type != "presence.unknown_client" {
		t.Errorf("unknown client: %+v", p)
	}
}
