package syncing_test

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/syncing"
	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	appsyncing "github.com/kukv/hedgehug/internal/app/syncing"
)

type env struct {
	api  humatest.TestAPI
	rep  *apptest.Replicator
	auto *appsyncing.Auto
}

func newEnv(t *testing.T) env {
	t.Helper()
	rep := &apptest.Replicator{
		Results:    map[port.StoreID]port.SyncResult{"S1": {Sent: 1, Received: 2}},
		Errs:       map[port.StoreID]error{"S3": port.ErrWriteDenied},
		MirrorErrs: map[string]error{"https://github.com/o/m": port.ErrMirrorAhead},
		UnsentN:    map[port.StoreID]int{"S1": 3},
	}
	svc := &appsyncing.Service{
		Targets: &apptest.SyncTargets{Targets: []port.SyncTarget{
			{ID: "S1", Name: "team", Remote: "https://github.com/o/team", Mirrors: []string{"https://github.com/o/m"}, Account: "A1"},
			{ID: "S2", Name: "solo"},
			{ID: "S3", Name: "web", Remote: "https://github.com/o/web", Account: "A1"},
		}},
		Tokens:     &apptest.SyncTokens{Tokens: map[port.StoreID]string{"S1": "tok-taro", "S3": "tok-taro"}},
		Replicator: rep,
		State:      apptest.NewSyncState(),
		Events:     &apptest.Events{},
		Clock:      apptest.NewClock(time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)),
	}
	auto := appsyncing.NewAuto(svc, &apptest.SyncSettings{Value: port.SyncTiming{Auto: true, Interval: 5 * time.Minute, AfterWrite: 30 * time.Second}})
	_, api := humatest.New(t)
	syncing.Register(api, syncing.Services{Sync: svc, Auto: auto})
	return env{api: api, rep: rep, auto: auto}
}

func decode[T any](t *testing.T, body []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("%v: %s", err, body)
	}
	return v
}

func TestSyncStoreResults(t *testing.T) {
	e := newEnv(t)
	cases := []struct {
		store, result, message, mirror string
	}{
		{"S1", "synced", "team を同期しました(送信 1 件・受信 2 件)", "ミラー 1 件に送れませんでした"},
		{"S2", "skipped", "solo には remote がないため、同期するものはありません", ""},
		{"S3", "failed", "web の同期に失敗しました: remote への書き込みが許可されていません", ""},
	}
	for _, c := range cases {
		res := e.api.Post("/api/stores/" + c.store + "/sync")
		if res.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", c.store, res.Code, res.Body)
		}
		got := decode[syncing.SyncStoreResult](t, res.Body.Bytes())
		if got.Result != c.result || got.Message != c.message || got.MirrorWarning != c.mirror {
			t.Errorf("%s: %+v", c.store, got)
		}
	}
	if res := e.api.Post("/api/stores/S9/sync"); res.Code != http.StatusNotFound {
		t.Errorf("unknown store: %d", res.Code)
	}
}

func TestSyncAllAndStatus(t *testing.T) {
	e := newEnv(t)
	res := e.api.Post("/api/sync")
	all := decode[syncing.SyncAllResult](t, res.Body.Bytes())
	if res.Code != http.StatusOK || all.Result != "failed" || !slices.Equal(all.Lines, []string{"成功 1 件・失敗 1 件", "web: remote への書き込みが許可されていません"}) {
		t.Errorf("sync-all: %d %+v", res.Code, all)
	}
	res = e.api.Get("/api/sync")
	st := decode[syncing.SyncStatusList](t, res.Body.Bytes())
	if res.Code != http.StatusOK || len(st.Stores) != 2 {
		t.Fatalf("status: %d %s", res.Code, res.Body)
	}
	if s := st.Stores[0]; s.Store != "S1" || s.Error != "" || s.Unsent != 3 || s.LastSuccess != "2026-11-13T10:00:00Z" {
		t.Errorf("S1 = %+v", s)
	}
	if s := st.Stores[1]; s.Store != "S3" || s.Error != "remote への書き込みが許可されていません" || s.LastSuccess != "" {
		t.Errorf("S3 = %+v", s)
	}
}

// 決定 C2-18・C2-18b: タブの待つ条件とネットワークの状態を、自動の同期に伝える。
func TestHoldAndOnline(t *testing.T) {
	e := newEnv(t)
	if res := e.api.Put("/api/sync/hold", map[string]any{"tab": "t1", "hold": true}); res.Code != http.StatusNoContent {
		t.Fatalf("hold: %d %s", res.Code, res.Body)
	}
	if err := e.auto.Step(context.Background()); err != nil || len(e.rep.Calls()) != 0 {
		t.Fatalf("synced while held: %v %+v", err, e.rep.Calls())
	}
	if res := e.api.Put("/api/sync/hold", map[string]any{"tab": "t1", "hold": false}); res.Code != http.StatusNoContent {
		t.Fatalf("release: %d", res.Code)
	}
	if res := e.api.Put("/api/sync/network", map[string]any{"tab": "t1", "online": true}); res.Code != http.StatusNoContent {
		t.Fatalf("network: %d %s", res.Code, res.Body)
	}
	if err := e.auto.Step(context.Background()); err != nil || len(e.rep.Calls()) == 0 {
		t.Errorf("not synced after release: %v", err)
	}
}
