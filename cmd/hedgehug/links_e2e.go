//go:build e2e

package main

import (
	"crypto/subtle"
	"net/http"
	"strconv"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
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
