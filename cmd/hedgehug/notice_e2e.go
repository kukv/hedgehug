//go:build e2e

package main

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"

	"github.com/kukv/hedgehug/internal/app/port"
)

// noticeHandler は、POST /e2e/notice を受け、本文の port.Notice(JSON)を受信箱に足す(C4c2-12)。同期は e2e のビルドで
// 再現できないので、通しのテストは値付きの上書きの行をここから入れる。ほかの要求は next に渡す。API と同じトークンを求める。
func noticeHandler(token string, book port.Inbox, next http.Handler) http.Handler {
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/e2e/notice" {
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
		var n port.Notice
		if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := book.Add(r.Context(), n); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
