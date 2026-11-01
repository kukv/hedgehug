package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/danielgtaylor/huma/v2"

	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

func TestHealth(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	var out struct {
		Version string `json:"version"`
		PID     int    `json:"pid"`
	}
	ts.call("GET", "/api/health", nil, 200, &out)
	if out.Version != "test" || out.PID != os.Getpid() {
		t.Errorf("health = %+v", out)
	}
}

func TestTokenRequired(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	for _, auth := range []string{"", "Bearer wrong", "Basic " + testToken, testToken} {
		req, _ := http.NewRequest("GET", ts.hs.URL+"/api/health", nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		resp := ts.raw(req)
		if resp.Status != 401 {
			t.Errorf("Authorization %q: status %d, want 401", auth, resp.Status)
		}
		b := resp.Body
		if !strings.Contains(string(b), `"type":"auth.token"`) {
			t.Errorf("Authorization %q: body %s", auth, b)
		}
	}
}

func TestEmptyTokenRefusesEverything(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	s := New(Options{Getenv: ts.getenv, Version: "test"}) // Token is empty
	s.SetPort(ts.port)
	hs := httptest.NewServer(s.Handler())
	t.Cleanup(hs.Close)
	for _, auth := range []string{"Bearer ", "Bearer x"} {
		req, _ := http.NewRequest("GET", hs.URL+"/api/health", nil)
		req.Host = "127.0.0.1:" + itoa(ts.port)
		req.Header.Set("Authorization", auth)
		resp, err := hs.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Errorf("Authorization %q: status %d, want 401", auth, resp.StatusCode)
		}
	}
}

func TestHostRefused(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	for _, host := range []string{"evil.example:80", "127.0.0.1:1", "localhost", "example.com"} {
		for _, path := range []string{"/api/health", "/", "/index.html"} {
			req, _ := http.NewRequest("GET", ts.hs.URL+path, nil)
			req.Host = host
			req.Header.Set("Authorization", "Bearer "+testToken)
			resp := ts.raw(req)
			if resp.Status != 403 {
				t.Errorf("Host %q %s: status %d, want 403", host, path, resp.Status)
			}
			if ct := resp.Header.Get("Content-Type"); ct != "application/problem+json" {
				t.Errorf("Host %q %s: Content-Type %q", host, path, ct)
			}
		}
	}
	req, _ := http.NewRequest("GET", ts.hs.URL+"/api/health", nil)
	req.Host = "localhost:" + itoa(ts.port)
	req.Header.Set("Authorization", "Bearer "+testToken)
	if resp := ts.raw(req); resp.Status != 200 {
		t.Errorf("Host localhost:<port>: status %d, want 200", resp.Status)
	}
}

func TestNotBuiltPage(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t) // Options.Static is nil
	for _, path := range []string{"/", "/tasks/ME-1", "/index.html"} {
		req, _ := http.NewRequest("GET", ts.hs.URL+path, nil)
		resp := ts.raw(req)
		b := resp.Body
		if resp.Status != 200 || !strings.Contains(string(b), "make web") {
			t.Errorf("%s: status %d body %s", path, resp.Status, b)
		}
	}
	p := ts.problemNoSpec("GET", "/api/nope", nil, 404) // not in the spec, so not validated
	if p.Type != "http.404" {
		t.Errorf("unknown /api path: %+v", p)
	}
}

func TestStaticAndSPAFallback(t *testing.T) {
	t.Parallel()
	dist := fstest.MapFS{
		"index.html":    {Data: []byte("<html>app</html>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
	h := staticHandler(dist)
	for path, want := range map[string]string{
		"/":              "<html>app</html>",
		"/tasks/ME-1":    "<html>app</html>",
		"/assets/app.js": "console.log(1)",
	} {
		rec := httptestRecorder(h, "GET", path)
		if rec.Code != 200 || !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Errorf("%s: %d %q", path, rec.Code, rec.Body.String())
		}
	}
	if rec := httptestRecorder(h, "GET", "/api/nope"); rec.Code != 404 {
		t.Errorf("/api/nope: %d", rec.Code)
	}
}

func TestBadRequestBodyIsRequestInvalid(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	// Any operation with a body will do; PUT /presence comes in Task 7, so
	// this test uses the body of a test-only operation registered below.
	registerEcho(ts.srv)
	p := ts.problemNoSpec("POST", "/api/test-echo", map[string]any{"n": "not a number"}, 400)
	if p.Type != "request.invalid" || len(p.Errors) == 0 {
		t.Errorf("problem = %+v", p)
	}
}

func TestOpenAPIUpToDate(t *testing.T) {
	t.Parallel()
	got, err := OpenAPIYAML()
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("..", "..", "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("api/openapi.yaml is not what the server generates; run go generate ./internal/server")
	}
	if !bytes.HasPrefix(got, []byte("openapi: 3.1")) && !bytes.Contains(got, []byte("\nopenapi: 3.1")) {
		t.Errorf("not OpenAPI 3.1: %.80s", got)
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func httptestRecorder(h http.Handler, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

// registerEcho adds POST /api/test-echo, which takes {n: integer}; it is not
// in api/openapi.yaml, so its responses are checked without the spec.
func registerEcho(s *Server) {
	type in struct {
		Body struct {
			N int `json:"n"`
		}
	}
	huma.Register(s.api, huma.Operation{OperationID: "test-echo", Method: http.MethodPost, Path: "/api/test-echo"},
		func(_ context.Context, _ *in) (*struct{}, error) { return nil, nil })
}

// problemNoSpec is problem for an operation that is not in the spec.
func (ts *testServer) problemNoSpec(method, path string, body any, want int) Problem {
	ts.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, ts.hs.URL+path, rd)
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Content-Type", "application/json")
	resp := ts.raw(req)
	raw := resp.Body
	if resp.Status != want {
		ts.t.Fatalf("%s %s: status %d, want %d; body %s", method, path, resp.Status, want, raw)
	}
	var p Problem
	if err := json.Unmarshal(raw, &p); err != nil {
		ts.t.Fatal(err)
	}
	return p
}

func TestProblemOf(t *testing.T) {
	t.Parallel()
	cases := []struct {
		err    error
		status int
		typ    string
	}{
		{&ops.Error{Kind: ops.KindNotFound, Code: "task.not_found", Err: errors.New("x")}, 404, "task.not_found"},
		{&ops.Error{Kind: ops.KindAmbiguous, Code: "task.ambiguous", Err: errors.New("x")}, 409, "task.ambiguous"},
		{&ops.Error{Kind: ops.KindInvalid, Code: "status.unknown", Err: errors.New("x")}, 422, "status.unknown"},
		{&ops.Error{Kind: ops.KindConflict, Code: "task.deleted", Err: errors.New("x")}, 409, "task.deleted"},
		{&ops.Error{Kind: ops.KindUsage, Code: "patch.empty", Err: errors.New("x")}, 400, "patch.empty"},
		{store.ErrLocked, 409, "store.locked"},
		{errors.New("boom"), 500, "internal"},
	}
	for _, c := range cases {
		p := problemOf(c.err)
		if p.Status != c.status || p.Type != c.typ || p.Title != http.StatusText(c.status) {
			t.Errorf("%v: %+v, want %d %s", c.err, p, c.status, c.typ)
		}
	}
	if p := problemOf(store.ErrLocked); p.Detail != "another hedgehug is writing to the store; try again" {
		t.Errorf("locked detail = %q", p.Detail)
	}
	ve := &store.ValidationError{Violations: []schema.Violation{{Code: "required", Field: "title", Msg: "is required"}}}
	p := problemOf(ve)
	vs, _ := p.Args["violations"].([]map[string]string)
	if p.Status != 422 || p.Type != "schema.violation" || len(vs) != 1 || vs[0]["field"] != "title" {
		t.Errorf("validation: %+v", p)
	}
}
