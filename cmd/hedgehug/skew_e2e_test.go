//go:build e2e

package main

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/syncing"
)

// /e2e/clock-skew は、トークンが違えば 401、POST でなければ 405、本文が読めなければ 400。受け取れたら 204 で、
// 秒を Skew.Observe に渡す(決定 C5d-11)。出来事 sync は Observe が出す。
func TestClockSkewHandler(t *testing.T) {
	events := &apptest.Events{}
	skew := &syncing.Skew{Records: apptest.NewClockSkews(), Events: events}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h := clockSkewHandler("tok", skew, next)
	const body = `{"seconds":-420}`
	for _, tc := range []struct {
		name, method, target, auth, body string
		want                             int
	}{
		{"他のパスは次へ", http.MethodPost, "/e2e/notice", "Bearer tok", body, http.StatusTeapot},
		{"認証なし", http.MethodPost, "/e2e/clock-skew", "", body, http.StatusUnauthorized},
		{"誤ったトークン", http.MethodPost, "/e2e/clock-skew", "Bearer nope", body, http.StatusUnauthorized},
		{"GET", http.MethodGet, "/e2e/clock-skew", "Bearer tok", "", http.StatusMethodNotAllowed},
		{"読めない本文", http.MethodPost, "/e2e/clock-skew", "Bearer tok", "{", http.StatusBadRequest},
		{"受け取る", http.MethodPost, "/e2e/clock-skew", "Bearer tok", body, http.StatusNoContent},
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
	if m, err := skew.Minutes(t.Context()); err != nil || m != 7 {
		t.Errorf("Minutes = %d, %v, want 7", m, err)
	}
	if got, want := events.List(), []port.Event{{Kind: port.EventSync}}; !slices.Equal(got, want) {
		t.Errorf("events = %+v, want %+v", got, want)
	}
}
