//go:build e2e

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/adapter/httpapi"
	"github.com/kukv/hedgehug/internal/app/apptest"
)

// /e2e/receive は、トークンが違えば 401、POST でなければ 405、本文が読めなければ 400、アイテムがなければ 404。
// 書けたら 204 で、そのアイテムの最後の行の op が、同期で受け取った保存になる。
func TestReceiveHandler(t *testing.T) {
	deps, judged := e2eWire(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h := receiveHandler("tok", deps.Stores, judged, deps.IDs, deps.Clock, next)
	body := `{"store":"` + e2eStore + `","item":"` + string(apptest.SampleTask) + `"}`
	for _, tc := range []struct {
		name, method, target, auth, body string
		want                             int
	}{
		{"他のパスは次へ", http.MethodPost, "/e2e/notice", "Bearer tok", body, http.StatusTeapot},
		{"認証なし", http.MethodPost, "/e2e/receive", "", body, http.StatusUnauthorized},
		{"誤ったトークン", http.MethodPost, "/e2e/receive", "Bearer nope", body, http.StatusUnauthorized},
		{"GET", http.MethodGet, "/e2e/receive", "Bearer tok", "", http.StatusMethodNotAllowed},
		{"読めない本文", http.MethodPost, "/e2e/receive", "Bearer tok", "{", http.StatusBadRequest},
		{"ないアイテム", http.MethodPost, "/e2e/receive", "Bearer tok", `{"store":"` + e2eStore + `","item":"01NOPE00000000000000000000"}`, http.StatusNotFound},
		{"書く", http.MethodPost, "/e2e/receive", "Bearer tok", body, http.StatusNoContent},
		{"もう一度書く", http.MethodPost, "/e2e/receive", "Bearer tok", body, http.StatusNoContent},
	} {
		r := httptest.NewRequest(tc.method, tc.target, strings.NewReader(tc.body))
		if tc.auth != "" {
			r.Header.Set("Authorization", tc.auth)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Errorf("%s: %d, want %d (%s)", tc.name, w.Code, tc.want, w.Body)
		}
	}
}

// e2e の組み立てでは、/e2e/receive のあと、GET の履歴でそのアイテムのいちばん新しい行が received になる。
func TestE2EWireReceive(t *testing.T) {
	deps, judged := e2eWire(t)
	mux := http.NewServeMux()
	httpapi.NewAPI(mux, deps)
	h := wrap("secret", deps, judged, mux)
	do := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer secret")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	history := func() []bool {
		w := do(http.MethodGet, "/api/stores/"+e2eStore+"/items/"+string(apptest.SampleTask)+"/history", "")
		var rows []struct{ Received bool }
		if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil || w.Code != http.StatusOK || len(rows) == 0 {
			t.Fatalf("history = %d %s (%v)", w.Code, w.Body, err)
		}
		out := make([]bool, len(rows))
		for i, r := range rows {
			out[i] = r.Received
		}
		return out
	}
	if got := history(); got[0] {
		t.Fatalf("before: received = %v", got)
	}
	if w := do(http.MethodPost, "/e2e/receive", `{"store":"`+e2eStore+`","item":"`+string(apptest.SampleTask)+`"}`); w.Code != http.StatusNoContent {
		t.Fatalf("receive = %d %s", w.Code, w.Body)
	}
	if got := history(); !got[0] {
		t.Errorf("after: received = %v", got)
	}
}
