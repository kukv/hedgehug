//go:build e2e

package main

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"

	"github.com/kukv/hedgehug/internal/app/port"
)

// syncLogHandler は、POST /e2e/sync-log を受け、本文 {store, sent, received} の同期の行を、いまの時刻で記録する
// (C4d-17)。同期は e2e のビルドで再現できないので、通しのテストは同期の行をここから入れる。ほかの要求は next に渡す。
// API と同じトークンを求める。
func syncLogHandler(token string, log port.SyncLog, clock port.Clock, next http.Handler) http.Handler {
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/e2e/sync-log" {
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
			Store    port.StoreID `json:"store"`
			Sent     int          `json:"sent"`
			Received int          `json:"received"`
		}
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := log.Add(r.Context(), b.Store, clock.Now(), b.Sent, b.Received); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
