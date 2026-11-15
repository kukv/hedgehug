//go:build e2e

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/adapter/httpapi"
	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

// /e2e/sync-log は、トークンが違えば 401、POST でなければ 405、本文が読めなければ 400。足せたら 204 で、同期の行が 1 つ。
// 足せたときだけ、本物の同期と同じく、その Store の出来事 sync を出す(画面のアクティビティが読み直す)。
func TestSyncLogHandler(t *testing.T) {
	state, err := e2eReads(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	log := state.SyncLog()
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	events := &apptest.Events{}
	h := syncLogHandler("tok", log, apptest.NewClock(time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)), events, next)
	const body = `{"store":"` + e2eStore + `","sent":2,"received":1}`
	for _, tc := range []struct {
		name, method, target, auth, body string
		want                             int
	}{
		{"他のパスは次へ", http.MethodPost, "/e2e/notice", "Bearer tok", body, http.StatusTeapot},
		{"認証なし", http.MethodPost, "/e2e/sync-log", "", body, http.StatusUnauthorized},
		{"誤ったトークン", http.MethodPost, "/e2e/sync-log", "Bearer nope", body, http.StatusUnauthorized},
		{"GET", http.MethodGet, "/e2e/sync-log", "Bearer tok", "", http.StatusMethodNotAllowed},
		{"読めない本文", http.MethodPost, "/e2e/sync-log", "Bearer tok", "{", http.StatusBadRequest},
		{"足す", http.MethodPost, "/e2e/sync-log", "Bearer tok", body, http.StatusNoContent},
	} {
		r := httptest.NewRequest(tc.method, tc.target, strings.NewReader(tc.body))
		if tc.auth != "" {
			r.Header.Set("Authorization", tc.auth)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Errorf("%s: %d, want %d", tc.name, w.Code, tc.want)
		}
	}
	lines, err := log.List(t.Context(), e2eStore)
	if err != nil || len(lines) != 1 || lines[0].Sent != 2 || lines[0].Received != 1 {
		t.Errorf("lines = %+v, %v", lines, err)
	}
	if got, want := events.List(), []port.Event{{Kind: port.EventSync, Store: e2eStore}}; !slices.Equal(got, want) {
		t.Errorf("events = %+v, want %+v", got, want)
	}
}

// e2e の組み立てでは、API からの保存と /e2e/sync-log の行が、GET /api/activity に出る。
func TestE2EWireActivity(t *testing.T) {
	deps := e2eDeps(t)
	mux := http.NewServeMux()
	httpapi.NewAPI(mux, deps)
	if deps.Versions == nil {
		t.Fatal("deps.Versions is nil; the activity would rebuild every row on each request")
	}
	h := syncLogHandler("secret", deps.SyncLog, deps.Clock, deps.Events, mux)
	do := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer secret")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	if w := do(http.MethodPost, "/e2e/sync-log", `{"store":"`+e2eStore+`","sent":1,"received":0}`); w.Code != http.StatusNoContent {
		t.Fatalf("sync-log = %d", w.Code)
	}
	if w := do(http.MethodPost, "/api/stores/"+e2eStore+"/workspaces/"+string(apptest.SampleWorkspace)+"/projects", `{"title":"新規"}`); w.Code >= 300 {
		t.Fatalf("create project = %d %s", w.Code, w.Body)
	}
	w := do(http.MethodGet, "/api/activity?store="+e2eStore, "")
	var p struct {
		Rows []struct{ Actor, Text string }
	}
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil || w.Code != http.StatusOK {
		t.Fatalf("activity = %d %s (%v)", w.Code, w.Body, err)
	}
	var syncs, saves int
	for _, r := range p.Rows {
		if r.Actor == "sync" {
			syncs++
		} else {
			saves++
		}
	}
	if syncs != 1 || saves != 1 {
		t.Errorf("rows = %+v, want one sync and one save", p.Rows)
	}
}
