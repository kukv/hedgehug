//go:build e2e

package main

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/kukv/hedgehug/internal/app/githublink"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
	"github.com/kukv/hedgehug/internal/domain/prlink"
)

// githubHandler は、POST /e2e/github?repo=…&number=…&kind=…&title=…&state=…&review=…&checks=…&moved=…&gone=… を受け、
// 偽の GitHub(e2eGitHub)に PR・Issue を置く(D3-23)。ほかの要求は next に渡す。API と同じトークンを求める。
func githubHandler(token string, next http.Handler) http.Handler {
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/e2e/github" {
			next.ServeHTTP(w, r)
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		q := r.URL.Query()
		n, err := strconv.Atoi(q.Get("number"))
		if r.Method != http.MethodPost || err != nil || q.Get("repo") == "" {
			http.Error(w, "POST with repo and number", http.StatusBadRequest)
			return
		}
		if q.Get("gone") != "" { // 番号を消したことにする(取り直しの失敗の確かめ)
			e2eGitHub.Fail(q.Get("repo"), n, &ghcall.Failure{Kind: ghcall.NotFound, Repo: q.Get("repo"), Number: n})
			w.WriteHeader(http.StatusNoContent)
			return
		}
		e2eGitHub.Fail(q.Get("repo"), n, nil)
		now := q.Get("moved")
		if now == "" {
			now = q.Get("repo")
		}
		review := itemlink.Review(q.Get("review"))
		if review == "" {
			review = itemlink.NoReview
		}
		e2eGitHub.Put(q.Get("repo"), port.Found{
			Kind: itemlink.Kind(q.Get("kind")), Repo: now, Number: n,
			Values: itemlink.Values{Title: q.Get("title"), State: itemlink.State(q.Get("state")), Review: review},
			Checks: itemlink.Checks(q.Get("checks")),
		})
		w.WriteHeader(http.StatusNoContent)
	})
}

// pullsHandler は、PR の自動リンクの通しのテストの入口を受ける(D4b-16)。ほかの要求は next に渡す。
// API と同じトークンを求める。
//   - POST /e2e/github-pull?repo&number&branch&title&body&state&comment&updated: 偽の PR を置く
//     (updated は RFC 3339。なければいまの時刻)
//   - POST /e2e/github-app?owner&missing=1|0: その owner のリポジトリを App 未導入にする・戻す
//   - POST /e2e/github-repo?repo&unreadable=1|0: リポジトリ全体を読めなくする・戻す
func pullsHandler(token string, next http.Handler) http.Handler {
	want := []byte("Bearer " + token)
	routes := map[string]func(url.Values) error{
		"/e2e/github-pull": putPull,
		"/e2e/github-app":  missApp,
		"/e2e/github-repo": unreadableRepo,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route, ok := routes[r.URL.Path]
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := route(r.URL.Query()); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// putPull は、偽の GitHub に PR を置く。
func putPull(q url.Values) error {
	n, err := strconv.Atoi(q.Get("number"))
	if err != nil || q.Get("repo") == "" {
		return errors.New("repo and number are required")
	}
	updated := time.Now()
	if v := q.Get("updated"); v != "" {
		if updated, err = time.Parse(time.RFC3339, v); err != nil {
			return err
		}
	}
	text := prlink.Text{Branch: q.Get("branch"), Title: q.Get("title"), Body: q.Get("body")}
	if c := q.Get("comment"); c != "" {
		text.Comments = []string{c}
	}
	e2eGitHub.PutPull(q.Get("repo"), port.Pull{
		Number: n, Text: text, UpdatedAt: updated,
		Values: itemlink.Values{Title: q.Get("title"), State: itemlink.State(q.Get("state")), Review: itemlink.NoReview},
	})
	return nil
}

// missApp は、owner のリポジトリを App 未導入にする(missing=0 で戻す)。
func missApp(q url.Values) error {
	if q.Get("owner") == "" {
		return errors.New("owner is required")
	}
	e2eGitHub.MissApp(q.Get("owner"), q.Get("missing") == "1")
	return nil
}

// unreadableRepo は、リポジトリ全体を読めなくする(unreadable=0 で戻す)。
func unreadableRepo(q url.Values) error {
	repo := q.Get("repo")
	if repo == "" {
		return errors.New("repo is required")
	}
	var err error
	if q.Get("unreadable") == "1" {
		err = &ghcall.Failure{Kind: ghcall.RepoUnreadable, Repo: repo}
	}
	e2eGitHub.Fail(repo, 0, err)
	return nil
}

// e2eGitHubEvery は、e2e の入口の読み取りに渡す「GitHub を読む間隔」(既定の 5 分。D4a-7)。
const e2eGitHubEvery = 5 * time.Minute

// githubReadHandler は、POST /e2e/github-read を受け、間隔によらず 1 回の読み取り(ReadOnce)を行う(D4a-19)。
// ほかの要求は next に渡す。API と同じトークンを求める。
func githubReadHandler(token string, links githublink.Links, next http.Handler) http.Handler {
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/e2e/github-read" {
			next.ServeHTTP(w, r)
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := links.ReadOnce(r.Context(), e2eGitHubEvery, true); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
