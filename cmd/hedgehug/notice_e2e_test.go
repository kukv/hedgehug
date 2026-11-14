//go:build e2e

package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

// /e2e/notice は、トークンが違えば 401、POST でなければ 405、本文が読めなければ 400。足せたら 204 で、受信箱に 1 行。
func TestNoticeHandler(t *testing.T) {
	state, err := e2eReads(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	book := state.Inbox(apptest.NewClock(time.Now()))
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h := noticeHandler("tok", book, next)
	const body = `{"Kind":"overwrite","Store":"` + e2eStore + `","Workspace":"` + string(apptest.SampleWorkspace) +
		`","Item":"` + string(apptest.SampleTask) + `","Text":"ステータスが上書きされました","Display":"HH-2","Sender":"hanako",` +
		`"Field":"status","Mine":"` + string(apptest.SampleStatusTodo) + `","Kept":"` + string(apptest.SampleStatusDone) + `"}`
	for _, tc := range []struct {
		name, method, target, auth, body string
		want                             int
	}{
		{"他のパスは次へ", http.MethodPost, "/e2e/github", "Bearer tok", body, http.StatusTeapot},
		{"認証なし", http.MethodPost, "/e2e/notice", "", body, http.StatusUnauthorized},
		{"誤ったトークン", http.MethodPost, "/e2e/notice", "Bearer nope", body, http.StatusUnauthorized},
		{"GET", http.MethodGet, "/e2e/notice", "Bearer tok", "", http.StatusMethodNotAllowed},
		{"読めない本文", http.MethodPost, "/e2e/notice", "Bearer tok", "{", http.StatusBadRequest},
		{"足す", http.MethodPost, "/e2e/notice", "Bearer tok", body, http.StatusNoContent},
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
	rows, err := book.List(t.Context(), port.NoticeQuery{Stores: []port.StoreID{e2eStore}})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want one", rows)
	}
	n := rows[0]
	if n.Kind != port.NoticeOverwrite || n.Item != apptest.SampleTask || n.Field != "status" || n.Sender != "hanako" ||
		string(n.Mine) != `"`+string(apptest.SampleStatusTodo)+`"` || string(n.Kept) != `"`+string(apptest.SampleStatusDone)+`"` {
		t.Errorf("row = %+v", n)
	}
}
