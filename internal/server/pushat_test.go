package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	hhsync "github.com/kukv/hedgehug/internal/sync"
)

type pushAtBody struct {
	Store        string     `json:"store"`
	PendingSince *time.Time `json:"pending_since"`
	PushAt       *time.Time `json:"push_at"`
	NextFetch    *time.Time `json:"next_fetch"`
}

func TestSyncAPIPushAtIsAbsentWhenNothingWaits(t *testing.T) {
	t.Parallel()
	ts, _, _ := loopServer(t)
	waitFirstSync(t, ts)
	var st []pushAtBody
	ts.call("GET", "/api/sync", nil, 200, &st)
	if len(st) != 1 || st[0].PendingSince != nil || st[0].PushAt != nil {
		t.Fatalf("nothing waits to be pushed, so push_at is absent: %+v", st)
	}
}

// push_at is when the loop pushes (Loop.due): push_interval after the first
// write that waits, but not before push_interval after the last sync and not
// after the next fetch. The loop's clock is fake and the first sync starts at
// t0, with no client on screen, so the fetch is 10 minutes after it.
func TestSyncAPIPushAt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		config  string        // appended to config.yaml
		pending time.Duration // after t0
		want    time.Duration // after t0
	}{
		{"default push_interval of 30s", "", time.Minute, 90 * time.Second},
		{"push_interval of 5s", "sync:\n  push_interval: 5s\n", time.Minute, 65 * time.Second},
		{"written as the last sync starts", "", 0, 30 * time.Second},
		{"still pending from before the last sync (a push that failed)", "", -time.Minute, 30 * time.Second},
		{"the fetch comes first", "sync:\n  push_interval: 20m\n", time.Minute, 10 * time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ts, _, _ := loopServer(t)
			waitFirstSync(t, ts)
			eventually(t, "the loop to end the sync", ts.srv.loop.parked)
			if tc.config != "" {
				f, err := os.OpenFile(filepath.Join(ts.env["HEDGEHUG_HOME"], "config.yaml"), os.O_APPEND|os.O_WRONLY, 0o600)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := f.WriteString(tc.config); err != nil {
					t.Fatal(err)
				}
				if err := f.Close(); err != nil {
					t.Fatal(err)
				}
			}
			s, err := ts.session().OpenStore()
			if err != nil {
				t.Fatal(err)
			}
			if err := hhsync.MarkPending(s, t0.Add(tc.pending)); err != nil {
				t.Fatal(err)
			}
			var st []pushAtBody
			ts.call("GET", "/api/sync", nil, 200, &st)
			if len(st) != 1 || st[0].PendingSince == nil || st[0].PushAt == nil {
				t.Fatalf("status = %+v", st)
			}
			if want := t0.Add(tc.want); !st[0].PushAt.Equal(want) {
				t.Errorf("push_at = %v; want t0 + %v", st[0].PushAt, tc.want)
			}
			if st[0].NextFetch != nil && st[0].PushAt.After(*st[0].NextFetch) {
				t.Errorf("push_at %v is after next_fetch %v", st[0].PushAt, st[0].NextFetch)
			}
		})
	}
}

// A store the loop has not synced yet is synced at once, so no time for a
// push is told.
func TestSyncAPIPushAtBeforeTheFirstSync(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	s, err := ts.session().OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if err := hhsync.MarkPending(s, time.Now()); err != nil {
		t.Fatal(err)
	}
	var st []pushAtBody
	ts.call("GET", "/api/sync", nil, 200, &st)
	if len(st) != 1 || st[0].PendingSince == nil || st[0].PushAt != nil {
		t.Errorf("status = %+v; want pending_since and no push_at", st)
	}
}
