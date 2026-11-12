// Package httpapi は、画面のための HTTP の API と、画面のファイルの配信を持つ。
package httpapi

import (
	"crypto/subtle"
	"io/fs"
	"net/http"
	"strings"
)

// New は、トークンで守った API と、画面のファイルを返す Handler を作る。
func New(token string, static fs.FS, deps Deps) http.Handler {
	api := http.NewServeMux()
	NewAPI(api, deps)
	root := http.NewServeMux()
	root.Handle("/api/", requireToken(token, api))
	root.Handle("/", spa(static))
	return root
}

// requireToken は、Authorization: Bearer <トークン> が合わなければ 401 を返す。
func requireToken(token string, next http.Handler) http.Handler {
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := []byte(r.Header.Get("Authorization"))
		if subtle.ConstantTimeCompare(got, want) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// spa は、ファイルがあればそれを、なければ index.html を返す(画面のアドレスは画面が解釈する)。
// ディレクトリや開けないパスも index.html を返し、ファイルの一覧やエラーは見せない。
// index.html がなければ(make web をしていなければ)404。
func spa(static fs.FS) http.Handler {
	files := http.FileServerFS(static)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info, err := fs.Stat(static, strings.TrimPrefix(r.URL.Path, "/"))
		if err == nil && !info.IsDir() {
			files.ServeHTTP(w, r)
			return
		}
		http.ServeFileFS(w, r, static, "index.html")
	})
}
