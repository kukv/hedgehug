package httpapi_test

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/kukv/hedgehug/internal/adapter/httpapi"
	"github.com/kukv/hedgehug/internal/app/apptest"
)

func TestAccountsAreRegistered(t *testing.T) {
	srv := httptest.NewServer(httpapi.New("secret", fstest.MapFS{}, httpapi.Deps{
		Version:  "test",
		IDs:      apptest.NewIDs(),
		Accounts: apptest.NewAccounts(),
		Secrets:  &apptest.Secrets{},
		GitHub:   &apptest.GitHub{},
		Clock:    apptest.NewClock(time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)),
	}))
	t.Cleanup(srv.Close)
	code, body := get(t, srv.URL+"/api/accounts", "secret")
	if code != http.StatusOK || !strings.Contains(body, `"accounts":[]`) {
		t.Errorf("status %d, body %s", code, body)
	}
}

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	static := fstest.MapFS{
		"index.html":    {Data: []byte("<!doctype html><title>hedgehug</title>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
	srv := httptest.NewServer(httpapi.New("secret", static, httpapi.Deps{Version: "v0.0.0-test"}))
	t.Cleanup(srv.Close)
	return srv
}

// noRedirect は、リダイレクトを追わないクライアント。パスごとに返したものそのものを確かめる。
var noRedirect = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func get(t *testing.T, url, token string) (int, string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := noRedirect.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b)
}

func TestSystemNeedsToken(t *testing.T) {
	srv := newServer(t)
	for _, token := range []string{"", "wrong"} {
		if code, _ := get(t, srv.URL+"/api/system", token); code != http.StatusUnauthorized {
			t.Errorf("token %q: status %d, want 401", token, code)
		}
	}
	code, body := get(t, srv.URL+"/api/system", "secret")
	if code != http.StatusOK || strings.TrimSpace(body) != `{"version":"v0.0.0-test"}` {
		t.Errorf("status %d, body %s", code, body)
	}
}

func TestUnknownAPIIs404WithToken(t *testing.T) {
	if code, _ := get(t, newServer(t).URL+"/api/nope", "secret"); code != http.StatusNotFound {
		t.Errorf("status %d, want 404", code)
	}
}

// ない API も、トークンがなければ 401(パスの有無を見せない)。
func TestUnknownAPINeedsToken(t *testing.T) {
	if code, _ := get(t, newServer(t).URL+"/api/nope", ""); code != http.StatusUnauthorized {
		t.Errorf("status %d, want 401", code)
	}
}

func TestStaticFilesNeedNoToken(t *testing.T) {
	srv := newServer(t)
	if code, body := get(t, srv.URL+"/assets/app.js", ""); code != http.StatusOK || body != "console.log(1)" {
		t.Errorf("asset: %d %q", code, body)
	}
	// 画面のアドレス(例: /ws/HH)は、index.html を返して画面が開く。
	for _, path := range []string{"/", "/ws/HH/items"} {
		if code, body := get(t, srv.URL+path, ""); code != http.StatusOK || !strings.Contains(body, "<title>hedgehug</title>") {
			t.Errorf("%s: %d %q", path, code, body)
		}
	}
}

// ディレクトリのパスは、ファイルの一覧を見せずに index.html を返す。
func TestDirectoryPathsServeIndex(t *testing.T) {
	srv := newServer(t)
	for _, path := range []string{"/assets", "/assets/"} {
		if code, body := get(t, srv.URL+path, ""); code != http.StatusOK || !strings.Contains(body, "<title>hedgehug</title>") {
			t.Errorf("%s: %d %q", path, code, body)
		}
	}
}

// permissionFS は、secret.txt を開けない(Stat が not-exist でないエラーを返す)FS。
// Open だけを持たせ、fs.Stat も Open を通るようにする。
type permissionFS struct{ files fstest.MapFS }

func (f permissionFS) Open(name string) (fs.File, error) {
	if name == "secret.txt" {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
	}
	return f.files.Open(name)
}

// 開けないファイルのパスは、エラーを見せずに index.html を返す。
func TestUnreadablePathServesIndex(t *testing.T) {
	static := permissionFS{fstest.MapFS{
		"index.html": {Data: []byte("<!doctype html><title>hedgehug</title>")},
		"secret.txt": {Data: []byte("x")},
	}}
	srv := httptest.NewServer(httpapi.New("secret", static, httpapi.Deps{Version: "v0.0.0-test"}))
	t.Cleanup(srv.Close)
	if code, body := get(t, srv.URL+"/secret.txt", ""); code != http.StatusOK || !strings.Contains(body, "<title>hedgehug</title>") {
		t.Errorf("%d %q", code, body)
	}
}

// make web をしていない(web/dist が .gitkeep だけの)ときも、ファイルの一覧を見せない。
func TestNoIndexIs404WithoutListing(t *testing.T) {
	static := fstest.MapFS{".gitkeep": {Data: nil}}
	srv := httptest.NewServer(httpapi.New("secret", static, httpapi.Deps{Version: "v0.0.0-test"}))
	t.Cleanup(srv.Close)
	for _, path := range []string{"/", "/ws/HH/items"} {
		if code, body := get(t, srv.URL+path, ""); code != http.StatusNotFound || strings.Contains(body, ".gitkeep") {
			t.Errorf("%s: %d %q", path, code, body)
		}
	}
}

// OpenAPI と説明のページは、make generate で api/openapi.yaml に書き、サーバーでは出さない。
func TestOpenAPIIsNotServed(t *testing.T) {
	mux := http.NewServeMux()
	httpapi.NewAPI(mux, httpapi.Deps{Version: "v0.0.0-test"})
	for _, path := range []string{"/openapi.json", "/openapi.yaml", "/docs", "/schemas/SystemInfo.json", "/schemas/x.json"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", path, rec.Code)
		}
	}
}
