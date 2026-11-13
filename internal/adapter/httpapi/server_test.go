package httpapi_test

import (
	"bufio"
	"context"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/kukv/hedgehug/internal/adapter/httpapi"
	"github.com/kukv/hedgehug/internal/adapter/httpapi/events"
	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
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

// revivingHandler は、削除済みのアイテム HH-2 の削除を取り消してから、finish で応答する Handler。
func revivingHandler(t *testing.T, finish func(w http.ResponseWriter)) http.HandlerFunc {
	t.Helper()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteItems(t.Context(), s, "S1", apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		op := port.Operation{Actor: change.User("taro"), Summary: "保存"}
		if err := s.Write(r.Context(), "S1", op, func(tx port.Tx) error { return port.ReviveItem(r.Context(), tx, apptest.SampleTask) }); err != nil {
			t.Error(err)
		}
		finish(w)
	}
}

// serveRestored は、restoredHeader で包んだ h の応答を返す。
func serveRestored(t *testing.T, h http.HandlerFunc) http.Header {
	t.Helper()
	rec := httptest.NewRecorder()
	httpapi.RestoredHeader(h).ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/", nil))
	return rec.Header()
}

func TestRestoredHeaderOnSuccess(t *testing.T) {
	finishes := map[string]func(w http.ResponseWriter){
		"本文を書く":     func(w http.ResponseWriter) { _, _ = io.WriteString(w, "ok") },
		"200":       func(w http.ResponseWriter) { w.WriteHeader(http.StatusOK) },
		"本文なしの 204": func(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) },
	}
	for name, finish := range finishes {
		h := serveRestored(t, revivingHandler(t, finish))
		if got := h.Values("Hedgehug-Restored"); !slices.Equal(got, []string{"HH-2"}) {
			t.Errorf("%s: %v", name, got)
		}
	}
}

func TestRestoredHeaderNotOnFailure(t *testing.T) {
	h := serveRestored(t, revivingHandler(t, func(w http.ResponseWriter) { w.WriteHeader(http.StatusUnprocessableEntity) }))
	if got := h.Values("Hedgehug-Restored"); len(got) != 0 {
		t.Errorf("422: %v", got)
	}
}

func TestRestoredHeaderKeepsFlusher(t *testing.T) {
	_ = serveRestored(t, func(w http.ResponseWriter, _ *http.Request) {
		f, ok := w.(http.Flusher)
		if !ok {
			t.Error("包んだ ResponseWriter が http.Flusher を満たさない")
			return
		}
		f.Flush()
	})
	// Flush が最初の送信になるときも、取り消した ID を付ける。
	h := serveRestored(t, revivingHandler(t, func(w http.ResponseWriter) { w.(http.Flusher).Flush() }))
	if got := h.Values("Hedgehug-Restored"); !slices.Equal(got, []string{"HH-2"}) {
		t.Errorf("flush first: %v", got)
	}
}

// 本物の組み立て(httpapi.New)を通しても、出来事の読み続けは接続直後に終わらず、出来事を送れる。
func TestEventsStreamThroughNew(t *testing.T) {
	hub := events.NewHub()
	srv := httptest.NewServer(httpapi.New("secret", fstest.MapFS{}, httpapi.Deps{Version: "test", Events: hub}))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer secret")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	for deadline := time.Now().Add(2 * time.Second); hub.Listeners() == 0; time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("接続直後に終わった(listener がいない)")
		}
	}
	hub.Publish(port.Event{Kind: port.EventStore, Store: "S1"})
	sc := bufio.NewScanner(res.Body)
	if !sc.Scan() || sc.Text() != "event: store" {
		t.Errorf("first line = %q", sc.Text())
	}
}
