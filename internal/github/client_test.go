package github

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/fakegh"
)

func fake(t *testing.T) (*fakegh.Server, config.Host) {
	t.Helper()
	zero := 0
	s := fakegh.New(fakegh.State{
		Users:  []string{"alice", "bob"},
		Orgs:   map[string][]string{"org-a": {"alice"}},
		Tokens: map[string]string{"t-alice": "alice", "t-bob": "bob"},
		Repos: []*fakegh.Repo{
			{Owner: "org-a", Name: "hh-shared", Private: true, Topics: []string{"hedgehug-store"}, Perms: map[string]string{"alice": "admin", "bob": "read"}},
		},
		DeviceInterval: &zero,
	})
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	return s, config.Host{Host: "github.com", API: srv.URL, Web: srv.URL}
}

func TestClient(t *testing.T) {
	s, h := fake(t)
	c := New(h, "t-alice", nil)
	if l, err := c.User(); err != nil || l != "alice" {
		t.Errorf("User = %q, %v", l, err)
	}
	r, err := c.CreateRepo("org-a", "hh-new", false)
	if err != nil || r.FullName != "org-a/hh-new" || r.CloneURL != "https://github.com/org-a/hh-new.git" {
		t.Fatalf("CreateRepo = %+v, %v", r, err)
	}
	if _, err := c.CreateRepo("org-a", "hh-new", false); !errors.Is(err, ErrExists) {
		t.Errorf("again = %v, want ErrExists", err)
	}
	if err := c.SetTopics("org-a", "hh-new", []string{"hedgehug-store"}); err != nil {
		t.Fatal(err)
	}
	if x, _ := s.Repo("org-a/hh-new"); len(x.Topics) != 1 {
		t.Errorf("topics = %v", x.Topics)
	}
	repos, err := c.UserRepos()
	if err != nil || len(repos) != 2 {
		t.Fatalf("UserRepos = %v, %v", repos, err)
	}
	cs, err := c.Collaborators("org-a", "hh-shared")
	if err != nil || len(cs) != 2 || cs[1] != (Collaborator{Login: "bob", RoleName: "read"}) {
		t.Errorf("Collaborators = %+v, %v", cs, err)
	}
	bob := New(h, "t-bob", nil)
	if r, err := bob.Repo("org-a", "hh-shared"); err != nil || r.Permissions.Push {
		t.Errorf("bob's Repo = %+v, %v", r, err)
	}
	if _, err := bob.Collaborators("org-a", "hh-shared"); !errors.Is(err, ErrForbidden) {
		t.Errorf("bob's Collaborators = %v, want ErrForbidden", err)
	}
	if _, err := bob.Repo("org-a", "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing = %v", err)
	}
	if _, err := New(h, "bad", nil).User(); !errors.Is(err, ErrUnauthorized) {
		t.Errorf("bad token = %v", err)
	}
}

func TestUserReposPages(t *testing.T) {
	s, h := fake(t)
	for i := range 150 {
		name := fmt.Sprintf("org-a/r%03d", i)
		if err := s.AddRepo(name, nil, false); err != nil {
			t.Fatal(err)
		}
		if err := s.SetPerm(name, "alice", "read"); err != nil {
			t.Fatal(err)
		}
	}
	repos, err := New(h, "t-alice", nil).UserRepos()
	if err != nil || len(repos) != 151 {
		t.Errorf("UserRepos = %d, %v", len(repos), err)
	}
}

func TestRateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", "1793500000")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
	}))
	defer srv.Close()
	_, err := New(config.Host{API: srv.URL}, "t", nil).User()
	var rl *RateLimitError
	if !errors.As(err, &rl) || !rl.Reset.Equal(time.Unix(1793500000, 0)) {
		t.Errorf("err = %v", err)
	}
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv2.Close()
	if _, err := New(config.Host{API: srv2.URL}, "t", nil).User(); !errors.As(err, &rl) {
		t.Errorf("429 = %v", err)
	}
}

func TestDescribe(t *testing.T) {
	err := Describe(ErrUnauthorized, "github.com/alice-corp", "work", "read the permission of org-a/hh-shared")
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("Describe lost the cause: %v", err)
	}
	for _, want := range []string{"github.com/alice-corp cannot read the permission of org-a/hh-shared", "401", "run hedgehug auth login work"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to contain %q", err, want)
		}
	}
	// Only a 401 is cured by logging in again.
	if err := Describe(ErrForbidden, "github.com/bob", "me", "list the collaborators of org-a/x"); !errors.Is(err, ErrForbidden) || strings.Contains(err.Error(), "auth login") {
		t.Errorf("403 = %v", err)
	}
	if Describe(nil, "github.com/bob", "me", "do it") != nil {
		t.Error("Describe(nil) is not nil")
	}
}

func TestDevice(t *testing.T) {
	s, h := fake(t)
	dc, err := RequestDeviceCode(nil, h.Web, "Iv23x")
	if err != nil || dc.UserCode == "" {
		t.Fatalf("RequestDeviceCode = %+v, %v", dc, err)
	}
	tr, err := PollToken(nil, h.Web, "Iv23x", dc.DeviceCode)
	if err != nil || tr.Error != "authorization_pending" {
		t.Errorf("before approval = %+v, %v", tr, err)
	}
	if err := s.ApproveDevice(dc.UserCode, "alice"); err != nil {
		t.Fatal(err)
	}
	tr, err = PollToken(nil, h.Web, "Iv23x", dc.DeviceCode)
	if err != nil || tr.AccessToken == "" || tr.RefreshToken == "" || tr.ExpiresIn == 0 {
		t.Fatalf("token = %+v, %v", tr, err)
	}
	tr2, err := RefreshToken(nil, h.Web, "Iv23x", tr.RefreshToken)
	if err != nil || tr2.AccessToken == "" || tr2.AccessToken == tr.AccessToken {
		t.Errorf("refresh = %+v, %v", tr2, err)
	}
}
