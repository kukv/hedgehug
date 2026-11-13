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
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/ghread"
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

// readTokens は、どの Store にもアカウント e2e-bot のトークンを返す port.GitHubTokens。
type readTokens struct{}

func (readTokens) TokenFor(context.Context, port.StoreID, string) (string, account.Account, error) {
	return "e2e-bot-token", account.Account{ID: e2eAccount, Name: "e2e", Login: "e2e-bot", Method: account.EnvVar}, nil
}
func (readTokens) Rejected(account.ID)                   {}
func (readTokens) Recheck(context.Context) (bool, error) { return false, nil }

// readLinks は、e2eStore の HH-2 に org/h-web#7(Open)をリンクし、その読み取りの記録を「いま試した」にした組み立て。
// 間隔では、まだ読む時期ではない。
func readLinks(t *testing.T) (githublink.Links, *apptest.GitHub) {
	t.Helper()
	clock := apptest.NewClock(time.Now())
	stores := apptest.NewStores(clock, apptest.NewIDs())
	stores.AddStore(e2eStore)
	if err := apptest.Sample(t.Context(), stores, e2eStore); err != nil {
		t.Fatal(err)
	}
	pr := port.Found{Kind: itemlink.PR, Repo: "org/h-web", Number: 7, Values: itemlink.Values{Title: "Add", State: itemlink.Open, Review: itemlink.NoReview}}
	err := stores.Write(t.Context(), e2eStore, port.Operation{Actor: change.User("taro"), Summary: "リンク"}, func(tx port.Tx) error {
		log, err := tx.Changes().Of(change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
		if err != nil {
			return err
		}
		ref := itemlink.Ref{Kind: itemlink.PR, Repo: pr.Repo, Number: pr.Number}
		return tx.WriteItems().Append(apptest.SampleTask, itemlink.FromLog(log).Attach(ref, pr.Values)...)
	})
	if err != nil {
		t.Fatal(err)
	}
	gh := &apptest.GitHub{}
	gh.Put("org/h-web", pr)
	state, err := e2eReads(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := state.GitHubReads().SaveRecord(t.Context(), e2eStore, "org/h-web", ghread.Record{Attempt: clock.Now()}); err != nil {
		t.Fatal(err)
	}
	return githublink.Links{
		Stores: stores, Identity: apptest.Identity{Login: "taro"}, Tokens: readTokens{}, GitHub: gh,
		StoreGH: &apptest.StoreGitHub{Stores: []port.StoreGitHubInfo{{ID: e2eStore, Name: "e2e", Account: e2eAccount}}},
		Checks:  apptest.NewChecks(), Reads: state.GitHubReads(), Scans: state.PRScans(), Missing: state.AppMissing(), Inbox: &apptest.Inbox{}, Events: &apptest.Events{}, Clock: clock,
	}, gh
}

// /e2e/github-read は、間隔によらず読む(force。D4a-19): 記録ではまだ読む時期でないリポジトリも読む。
// 間隔ごとの取り直しでは、gone で消した番号は答えにない番号(Missing)になる(問い合わせ全体を失敗させない)。
func TestE2EGitHubGoneIsMissing(t *testing.T) {
	h := githubHandler("tok", http.NotFoundHandler())
	r := httptest.NewRequest(http.MethodPost, "/e2e/github?repo=org/h-gone&number=3&gone=1", nil)
	r.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	t.Cleanup(func() { e2eGitHub.Fail("org/h-gone", 3, nil) })
	if w.Code != http.StatusNoContent {
		t.Fatalf("gone: %d", w.Code)
	}
	b, err := e2eGitHub.ReadMany(t.Context(), "e2e-bot-token", "org/h-gone", []int{3})
	if err != nil || len(b.Missing) != 1 || b.Missing[0] != 3 {
		t.Errorf("ReadMany = %+v, %v", b, err)
	}
}

func TestE2EGitHubReadHandler(t *testing.T) {
	links, gh := readLinks(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h := githubReadHandler("tok", links, next)
	for _, tc := range []struct {
		name, method, target, auth string
		want, reads                int
	}{
		{"他のパスは次へ", http.MethodPost, "/e2e/github", "Bearer tok", http.StatusTeapot, 0},
		{"認証なし", http.MethodPost, "/e2e/github-read", "", http.StatusUnauthorized, 0},
		{"誤ったトークン", http.MethodPost, "/e2e/github-read", "Bearer nope", http.StatusUnauthorized, 0},
		{"GET", http.MethodGet, "/e2e/github-read", "Bearer tok", http.StatusMethodNotAllowed, 0},
		{"時期の前でも読む", http.MethodPost, "/e2e/github-read", "Bearer tok", http.StatusNoContent, 1},
	} {
		r := httptest.NewRequest(tc.method, tc.target, nil)
		if tc.auth != "" {
			r.Header.Set("Authorization", tc.auth)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if got := gh.ReadManyCalls("org/h-web"); w.Code != tc.want || got != tc.reads {
			t.Errorf("%s: %d (ReadMany %d), want %d (ReadMany %d)", tc.name, w.Code, got, tc.want, tc.reads)
		}
	}
}
