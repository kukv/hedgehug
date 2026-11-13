//go:build e2e

package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/githublink"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

func TestE2EGitHubHandler(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h := githubHandler("tok", next)
	do := func(method, target, auth string) int {
		r := httptest.NewRequest(method, target, nil)
		if auth != "" {
			r.Header.Set("Authorization", auth)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	const put = "/e2e/github?repo=org/h-web&number=7&kind=pr&title=Add&state=open&review=approved&checks=success"
	for _, tc := range []struct {
		name, method, target, auth string
		want                       int
	}{
		{"他のパスは次へ", http.MethodGet, "/api/accounts", "", http.StatusTeapot},
		{"認証なし", http.MethodPost, put, "", http.StatusUnauthorized},
		{"誤ったトークン", http.MethodPost, put, "Bearer nope", http.StatusUnauthorized},
		{"GET", http.MethodGet, put, "Bearer tok", http.StatusBadRequest},
		{"番号が数でない", http.MethodPost, "/e2e/github?repo=org/h-web&number=x", "Bearer tok", http.StatusBadRequest},
		{"repo がない", http.MethodPost, "/e2e/github?number=7", "Bearer tok", http.StatusBadRequest},
		{"置く", http.MethodPost, put, "Bearer tok", http.StatusNoContent},
	} {
		if got := do(tc.method, tc.target, tc.auth); got != tc.want {
			t.Errorf("%s: %d, want %d", tc.name, got, tc.want)
		}
	}
	f, err := e2eGitHub.Lookup(t.Context(), "e2e-bot-token", "org/h-web", 7)
	if err != nil || f.Kind != itemlink.PR || f.Title != "Add" || f.Review != itemlink.Approved || f.Checks != itemlink.Success || f.Repo != "org/h-web" {
		t.Fatalf("Lookup = %+v, %v", f, err)
	}

	if got := do(http.MethodPost, put+"&moved=org/h-renamed", "Bearer tok"); got != http.StatusNoContent {
		t.Fatalf("moved: %d", got)
	}
	if f, _ := e2eGitHub.Lookup(t.Context(), "e2e-bot-token", "org/h-web", 7); f.Repo != "org/h-renamed" {
		t.Errorf("moved の Repo = %q", f.Repo)
	}

	if got := do(http.MethodPost, "/e2e/github?repo=org/h-web&number=7&gone=1", "Bearer tok"); got != http.StatusNoContent {
		t.Fatalf("gone: %d", got)
	}
	_, err = e2eGitHub.Lookup(t.Context(), "e2e-bot-token", "org/h-web", 7)
	var fail *ghcall.Failure
	if !errors.As(err, &fail) || fail.Kind != ghcall.NotFound {
		t.Errorf("gone の Lookup = %v", err)
	}
	if got := do(http.MethodPost, put, "Bearer tok"); got != http.StatusNoContent {
		t.Fatalf("restore: %d", got)
	}
	if _, err := e2eGitHub.Lookup(t.Context(), "e2e-bot-token", "org/h-web", 7); err != nil {
		t.Errorf("gone を戻したあと: %v", err)
	}
}

// countingStoreGitHub は、List を呼んだ回数を数える(ReadOnce は 1 回ごとに 1 度だけ List を呼ぶ)。
type countingStoreGitHub struct {
	apptest.StoreGitHub
	lists int
}

func (c *countingStoreGitHub) List(ctx context.Context) ([]port.StoreGitHubInfo, error) {
	c.lists++
	return c.StoreGitHub.List(ctx)
}

func TestE2EGitHubReadHandler(t *testing.T) {
	infos := &countingStoreGitHub{}
	links := githublink.Links{StoreGH: infos, Clock: apptest.NewClock(time.Now())}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h := githubReadHandler("tok", links, next)
	for _, tc := range []struct {
		name, method, target, auth string
		want, lists                int
	}{
		{"他のパスは次へ", http.MethodPost, "/e2e/github", "Bearer tok", http.StatusTeapot, 0},
		{"認証なし", http.MethodPost, "/e2e/github-read", "", http.StatusUnauthorized, 0},
		{"誤ったトークン", http.MethodPost, "/e2e/github-read", "Bearer nope", http.StatusUnauthorized, 0},
		{"GET", http.MethodGet, "/e2e/github-read", "Bearer tok", http.StatusMethodNotAllowed, 0},
		{"読む", http.MethodPost, "/e2e/github-read", "Bearer tok", http.StatusNoContent, 1},
	} {
		r := httptest.NewRequest(tc.method, tc.target, nil)
		if tc.auth != "" {
			r.Header.Set("Authorization", tc.auth)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want || infos.lists != tc.lists {
			t.Errorf("%s: %d (lists %d), want %d (lists %d)", tc.name, w.Code, infos.lists, tc.want, tc.lists)
		}
	}
}
