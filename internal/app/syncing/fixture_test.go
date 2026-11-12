package syncing_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/syncing"
)

var at = time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)

// testClock は、テストが進める時計。
type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type fixture struct {
	targets *apptest.SyncTargets
	tokens  *apptest.SyncTokens
	rep     *apptest.Replicator
	state   *apptest.SyncState
	events  *apptest.Events
	clock   *testClock
	svc     *syncing.Service

	mu    sync.Mutex
	after []port.StoreID
}

// S1 team: remote・ミラー・アカウントあり。S2 solo: remote なし。S3 docs: アカウントなし。
// S4 ro: 閲覧のみ(ミラーあり)。S5 hidden: 非表示。
func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{
		targets: &apptest.SyncTargets{Targets: []port.SyncTarget{
			{ID: "S1", Name: "team", Remote: "https://github.com/o/team.git", Mirrors: []string{"https://github.com/o/team-mirror"}, Account: "A1"},
			{ID: "S2", Name: "solo"},
			{ID: "S3", Name: "docs", Remote: "git@github.com:o/docs.git"},
			{ID: "S4", Name: "ro", Remote: "https://github.com/o/ro", Mirrors: []string{"https://m.invalid/ro"}, Account: "A1", ReadOnly: true},
			{ID: "S5", Name: "hidden", Remote: "https://github.com/o/hidden", Account: "A1", Hidden: true},
		}},
		tokens: &apptest.SyncTokens{Tokens: map[port.StoreID]string{"S1": "tok-taro", "S4": "tok-taro", "S5": "tok-taro"}},
		rep: &apptest.Replicator{
			Results: map[port.StoreID]port.SyncResult{"S1": {Sent: 2, Received: 1, Seen: "v2"}, "S4": {Received: 3, Seen: "r1"}},
			Errs:    map[port.StoreID]error{},
			UnsentN: map[port.StoreID]int{"S1": 4},
		},
		state:  apptest.NewSyncState(),
		events: &apptest.Events{},
		clock:  &testClock{now: at},
	}
	f.svc = &syncing.Service{
		Targets: f.targets, Tokens: f.tokens, Replicator: f.rep, State: f.state, Events: f.events, Clock: f.clock,
		AfterSuccess: func(_ context.Context, id port.StoreID) {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.after = append(f.after, id)
		},
	}
	return f
}

// syncs は、Store ごとの Sync(ミラーを除く)の回数。
func (f *fixture) syncs() map[port.StoreID]int {
	n := map[port.StoreID]int{}
	for _, c := range f.rep.Calls() {
		if !c.Mirror {
			n[c.Store]++
		}
	}
	return n
}

func (f *fixture) afterCalls() []port.StoreID {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]port.StoreID(nil), f.after...)
}
