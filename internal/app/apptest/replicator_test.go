package apptest_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

func TestReplicatorRecordsCallsAndReturnsWhatWasSet(t *testing.T) {
	boom := errors.New("boom")
	r := &apptest.Replicator{
		Results:    map[port.StoreID]port.SyncResult{"S1": {Sent: 1, Received: 2, Seen: "v1"}},
		Errs:       map[port.StoreID]error{"S2": port.ErrAuthFailed},
		MirrorErrs: map[string]error{"https://m.invalid/x": boom},
		UnsentN:    map[port.StoreID]int{"S1": 4},
	}
	ctx := context.Background()
	if res, err := r.Sync(ctx, "S1", port.Remote{URL: "u"}); err != nil || res.Sent != 1 || res.Seen != "v1" {
		t.Errorf("Sync S1 = %+v, %v", res, err)
	}
	if _, err := r.Sync(ctx, "S2", port.Remote{}); !errors.Is(err, port.ErrAuthFailed) {
		t.Errorf("Sync S2 = %v", err)
	}
	if err := r.Mirror(ctx, "S1", port.Remote{URL: "https://m.invalid/x"}); !errors.Is(err, boom) {
		t.Errorf("Mirror = %v", err)
	}
	if n, err := r.Unsent(ctx, "S1"); n != 4 || err != nil {
		t.Errorf("Unsent = %d, %v", n, err)
	}
	calls := r.Calls()
	if len(calls) != 3 || calls[0].Store != "S1" || calls[0].Remote.URL != "u" || !calls[2].Mirror {
		t.Errorf("Calls = %+v", calls)
	}
}
