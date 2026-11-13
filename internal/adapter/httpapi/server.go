// Package httpapi は、画面のための HTTP の API と、画面のファイルの配信を持つ。
package httpapi

import (
	"context"
	"crypto/subtle"
	"io/fs"
	"net/http"
	"net/url"
	"strings"

	"github.com/kukv/hedgehug/internal/app/port"
)

// New は、トークンで守った API と、画面のファイルを返す Handler を作る。
func New(token string, static fs.FS, deps Deps) http.Handler {
	api := http.NewServeMux()
	NewAPI(api, deps)
	root := http.NewServeMux()
	root.Handle("/api/", requireToken(token, restoredHeader(api)))
	root.Handle("/", spa(static))
	return root
}

// restoredHeader は、別の画面で削除されたものへの保存で削除を取り消した成功の応答に、取り消したものの
// 表示 ID を Hedgehug-Restored で付ける(画面がトーストを出す)。usecase は取り消したあとに断って
// 巻き戻すことがあり、そのときも ID は残るので、付けるのは 2xx のときだけ。
func restoredHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := port.WithRevived(r.Context())
		next.ServeHTTP(&restoredWriter{ResponseWriter: w, ctx: ctx}, r.WithContext(ctx))
	})
}

// restoredWriter は、最初の WriteHeader(Write が先なら 200 として)で、2xx なら表示 ID を Header に足す。
type restoredWriter struct {
	http.ResponseWriter
	ctx  context.Context //nolint:containedctx // 応答の間だけ生きる入れ物の context
	sent bool
}

func (w *restoredWriter) WriteHeader(status int) {
	if !w.sent && status >= 200 { // 1xx は最終の応答ではないので、その後の応答に付ける
		w.sent = true
		if status < 300 {
			for _, r := range port.Revived(w.ctx) {
				switch r.Kind {
				case port.RevivedID:
					w.Header().Add("Hedgehug-Restored", r.Text)
				case port.RevivedName: // 名前はコンマや空白を含みうるので、画面が url.PathUnescape で戻せる形にする
					w.Header().Add("Hedgehug-Restored-Name", url.PathEscape(r.Text))
				case port.RevivedComment:
					w.Header().Add("Hedgehug-Restored-Comment", r.Text)
				}
			}
		}
	}
	w.ResponseWriter.WriteHeader(status)
}

// Flush は、出来事の読み続け(SSE)が w.(http.Flusher) で型を確かめて呼ぶ。Flush は暗黙に 200 を送るので、
// まだ送っていなければ先に WriteHeader を通す。
func (w *restoredWriter) Flush() {
	if !w.sent {
		w.WriteHeader(http.StatusOK)
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

func (w *restoredWriter) Write(b []byte) (int, error) {
	if !w.sent {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// Unwrap は、http.NewResponseController が Flush などを包んだ先へ届けるための口。
func (w *restoredWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

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
