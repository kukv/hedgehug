package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/ops"
)

const testToken = "test-token"

var (
	specOnce sync.Once
	specV    validator.Validator
	specErr  error
)

// specValidator is the validator of api/openapi.yaml as committed.
func specValidator(t *testing.T) validator.Validator {
	t.Helper()
	specOnce.Do(func() {
		var b []byte
		if b, specErr = os.ReadFile(filepath.Join("..", "..", "api", "openapi.yaml")); specErr != nil {
			return
		}
		doc, err := libopenapi.NewDocument(b)
		if err != nil {
			specErr = err
			return
		}
		v, errs := validator.NewValidator(doc)
		if len(errs) > 0 {
			specErr = errs[0]
			return
		}
		specV = v
	})
	if specErr != nil {
		t.Fatal(specErr)
	}
	return specV
}

// testServer is a Server over a fresh store "me" with one workspace "me"
// (prefix ME), in temporary HOME and HEDGEHUG_HOME directories, behind an
// httptest server.
type testServer struct {
	t    *testing.T
	srv  *Server
	hs   *httptest.Server
	env  map[string]string
	port int
	log  *bytes.Buffer
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	d := t.TempDir()
	home := filepath.Join(d, "home")
	if err := os.MkdirAll(home, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[user]\n\tname = Alice\n\temail = alice@example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ts := &testServer{t: t, log: &bytes.Buffer{}, env: map[string]string{
		"HOME": home, "USERPROFILE": home,
		"HEDGEHUG_HOME":      filepath.Join(d, "hh"),
		"HEDGEHUG_AUTO_SYNC": "0",
	}}
	if _, err := ts.session().StoreInit(ops.StoreInitInput{Name: "me", Prefix: "ME", Workspace: "me"}); err != nil {
		t.Fatal(err)
	}
	ts.srv = New(Options{
		Getenv:  ts.getenv,
		Now:     time.Now,
		Token:   testToken,
		Version: "test",
		Log:     ts.log,
	})
	ts.hs = httptest.NewServer(ts.srv.Handler())
	t.Cleanup(ts.hs.Close)
	_, p, err := net.SplitHostPort(ts.hs.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	ts.port, _ = strconv.Atoi(p)
	ts.srv.SetPort(ts.port)
	return ts
}

func (ts *testServer) getenv(k string) string { return ts.env[k] }

// session is an ops.Session on the same config, as the CLI would make.
func (ts *testServer) session() *ops.Session {
	return &ops.Session{Getenv: ts.getenv, Now: time.Now, Warn: func(string) {}}
}

// raw sends req as it is and returns the response, its body read into
// memory so that it can be read again.
func (ts *testServer) raw(req *http.Request) *http.Response {
	ts.t.Helper()
	resp, err := ts.hs.Client().Do(req)
	if err != nil {
		ts.t.Fatal(err)
	}
	b, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		ts.t.Fatal(err)
	}
	resp.Body = io.NopCloser(bytes.NewReader(b))
	return resp
}

// call sends method path with body as JSON (none when nil) and the token,
// checks the status is want and the response is as api/openapi.yaml says,
// and decodes the body into out (when not nil).
func (ts *testServer) call(method, path string, body any, want int, out any) *http.Response {
	ts.t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			ts.t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, ts.hs.URL+path, rd)
	if err != nil {
		ts.t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+testToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp := ts.raw(req)
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != want {
		ts.t.Fatalf("%s %s: status %d, want %d; body %s", method, path, resp.StatusCode, want, b)
	}
	resp.Body = io.NopCloser(bytes.NewReader(b))
	if ok, errs := specValidator(ts.t).ValidateHttpResponse(req, resp); !ok {
		for _, e := range errs {
			ts.t.Errorf("%s %s: response not as api/openapi.yaml says: %s (%s)", method, path, e.Message, e.Reason)
		}
	}
	resp.Body = io.NopCloser(bytes.NewReader(b))
	if out != nil {
		if err := json.Unmarshal(b, out); err != nil {
			ts.t.Fatalf("%s %s: %v; body %s", method, path, err, b)
		}
	}
	return resp
}

// problem calls method path, wants status, and returns the problem.
func (ts *testServer) problem(method, path string, body any, want int) Problem {
	ts.t.Helper()
	var p Problem
	resp := ts.call(method, path, body, want, &p) //nolint:bodyclose // raw reads the body into memory
	if ct := resp.Header.Get("Content-Type"); ct != "application/problem+json" {
		ts.t.Errorf("%s %s: Content-Type %q, want application/problem+json", method, path, ct)
	}
	return p
}

func readAll(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func mustULID(t *testing.T, s string) model.ULID {
	t.Helper()
	u, err := model.ParseULID(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func slicesEqual[T comparable](a, b []T) bool { return slices.Equal(a, b) }
