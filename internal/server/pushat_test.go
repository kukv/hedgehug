package server

import (
	"testing"
	"time"

	hhsync "github.com/kukv/hedgehug/internal/sync"
)

type pushAtBody struct {
	Store        string     `json:"store"`
	PendingSince *time.Time `json:"pending_since"`
	PushAt       *time.Time `json:"push_at"`
}

func TestSyncAPIPushAt(t *testing.T) {
	t.Parallel()
	ts, _, _ := loopServer(t)
	waitFirstSync(t, ts)

	var st []pushAtBody
	ts.call("GET", "/api/sync", nil, 200, &st)
	if len(st) != 1 || st[0].PendingSince != nil || st[0].PushAt != nil {
		t.Fatalf("nothing waits to be pushed, so push_at is absent: %+v", st)
	}

	s, err := ts.session().OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	since := time.Now().Truncate(time.Second)
	if err := hhsync.MarkPending(s, since); err != nil {
		t.Fatal(err)
	}
	ts.call("GET", "/api/sync", nil, 200, &st)
	if len(st) != 1 || st[0].PendingSince == nil || st[0].PushAt == nil {
		t.Fatalf("status = %+v", st)
	}
	// sync.push_interval is 30 seconds unless config.yaml says otherwise.
	if want := st[0].PendingSince.Add(30 * time.Second); !st[0].PushAt.Equal(want) {
		t.Errorf("push_at = %v; want pending_since + 30s = %v", st[0].PushAt, want)
	}
}
