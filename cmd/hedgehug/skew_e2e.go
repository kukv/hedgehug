//go:build e2e

package main

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"time"

	"github.com/kukv/hedgehug/internal/app/syncing"
)

// clockSkewHandler は、POST /e2e/clock-skew を受け、本文 {seconds}(GitHub の時刻 − このマシンの時計の秒)を
// skew.Observe に渡す(決定 C5d-11)。e2e のビルドの GitHub は偽物で時計のずれを測らないので、通しのテストはずれを
// ここから入れる。ほかの要求は next に渡す。API と同じトークンを求める。出来事 sync は Observe が出す。
func clockSkewHandler(token string, skew *syncing.Skew, next http.Handler) http.Handler {
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/e2e/clock-skew" {
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
		var b struct {
			Seconds int `json:"seconds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		skew.Observe(time.Duration(b.Seconds) * time.Second)
		w.WriteHeader(http.StatusNoContent)
	})
}
