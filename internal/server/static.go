package server

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// notBuiltPage is what / serves when the binary was built without the GUI
// (go build without make web).
const notBuiltPage = `<!doctype html>
<html lang="ja"><head><meta charset="utf-8"><title>Hedgehug</title></head>
<body><p>GUI がビルドされていません。make web を実行してください。</p></body></html>
`

// staticHandler serves the built GUI in dist: a file when there is one,
// index.html for any other path (the SPA routes itself), and a 404 problem
// under /api. Without index.html it serves notBuiltPage.
func staticHandler(dist fs.FS) http.Handler {
	built := dist != nil
	if built {
		if _, err := fs.Stat(dist, "index.html"); err != nil {
			built = false
		}
	}
	var files http.Handler
	if built {
		files = http.FileServerFS(dist)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			writeProblem(w, newProblem(http.StatusNotFound, "http.404", "no such API: "+r.URL.Path))
			return
		}
		if !built {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(notBuiltPage))
			return
		}
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "" || p == "index.html" {
			http.ServeFileFS(w, r, dist, "index.html")
			return
		}
		if st, err := fs.Stat(dist, p); err != nil || st.IsDir() {
			http.ServeFileFS(w, r, dist, "index.html")
			return
		}
		files.ServeHTTP(w, r)
	})
}
