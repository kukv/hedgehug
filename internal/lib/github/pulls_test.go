package github

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRepoDefaultBranch(t *testing.T) {
	t.Parallel()
	_, h := fake(t)
	r, err := New(h, "t-alice", nil).Repo("org-a", "hh-shared")
	if err != nil || r.DefaultBranch != "main" {
		t.Errorf("Repo = %+v, %v", r, err)
	}
}

func TestCreatePullRequest(t *testing.T) {
	t.Parallel()
	var method, path string
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"number":7}`)
	}))
	t.Cleanup(srv.Close)
	c := New(Options{API: srv.URL}, "tok", nil)
	n, err := c.CreatePull("o", "r", NewPull{Title: "T", Body: "B", Head: "hh/x", Base: "main", Draft: true})
	if err != nil || n != 7 {
		t.Fatalf("CreatePull = %d, %v", n, err)
	}
	if method != "POST" || path != "/repos/o/r/pulls" {
		t.Errorf("request = %s %s", method, path)
	}
	want := map[string]any{"title": "T", "body": "B", "head": "hh/x", "base": "main", "draft": true}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("body[%s] = %v, want %v", k, got[k], v)
		}
	}
}

func TestCreatePullAgainstFake(t *testing.T) {
	t.Parallel()
	s, h := fake(t)
	c := New(h, "t-alice", nil)
	n, err := c.CreatePull("org-a", "hh-shared", NewPull{Title: "T", Head: "hh/x", Base: "main"})
	if err != nil || n != 1 {
		t.Fatalf("CreatePull = %d, %v", n, err)
	}
	r, _ := s.Repo("org-a/hh-shared")
	if len(r.Pulls) != 1 || r.Pulls[0].Draft {
		t.Errorf("pulls = %+v", r.Pulls)
	}
	// A second pull for the branch is explained, not a repository clash.
	_, err = c.CreatePull("org-a", "hh-shared", NewPull{Title: "T2", Head: "hh/x", Base: "main"})
	if !errors.Is(err, ErrPullExists) || errors.Is(err, ErrExists) {
		t.Errorf("duplicate = %v; want ErrPullExists", err)
	}
	// A missing field says what the API said.
	_, err = c.CreatePull("org-a", "hh-shared", NewPull{Head: "hh/y", Base: "main"})
	if err == nil || !strings.Contains(err.Error(), "422") || !strings.Contains(err.Error(), "title, head and base are required") {
		t.Errorf("422 = %v; want the reason", err)
	}
}

func TestCreatePullDenied(t *testing.T) {
	t.Parallel()
	_, h := fake(t)
	_, err := New(h, "t-bob", nil).CreatePull("org-a", "hh-shared", NewPull{Title: "T", Head: "hh/x", Base: "main"})
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("by a reader = %v; want ErrForbidden", err)
	}
	_, err = New(h, "t-alice", nil).CreatePull("org-a", "nope", NewPull{Title: "T", Head: "hh/x", Base: "main"})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("no repository = %v; want ErrNotFound", err)
	}
}

func TestOpenPullFor(t *testing.T) {
	t.Parallel()
	_, h := fake(t)
	c := New(h, "t-alice", nil)
	if n, ok, err := c.OpenPullFor("org-a", "hh-shared", "hh/x"); err != nil || ok || n != 0 {
		t.Fatalf("before = %d, %v, %v", n, ok, err)
	}
	made, err := c.CreatePull("org-a", "hh-shared", NewPull{Title: "T", Head: "hh/x", Base: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if n, ok, err := c.OpenPullFor("org-a", "hh-shared", "hh/x"); err != nil || !ok || n != made {
		t.Errorf("after = %d, %v, %v; want %d", n, ok, err, made)
	}
	if _, ok, err := c.OpenPullFor("org-a", "hh-shared", "hh/other"); err != nil || ok {
		t.Errorf("another branch = %v, %v", ok, err)
	}
	// Reading is allowed to a reader.
	if n, ok, err := New(h, "t-bob", nil).OpenPullFor("org-a", "hh-shared", "hh/x"); err != nil || !ok || n != made {
		t.Errorf("by a reader = %d, %v, %v", n, ok, err)
	}
}
