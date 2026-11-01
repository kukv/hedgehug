package sync

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/fakegh"
	"github.com/kukv/hedgehug/internal/github"
	"github.com/kukv/hedgehug/internal/gitrepo/gitrepotest"
)

func TestCheckAccessGitHub(t *testing.T) {
	t.Parallel()
	fake := fakegh.New(fakegh.State{
		Users:  []string{"alice", "bob"},
		Tokens: map[string]string{"t-bob": "bob"},
		Repos:  []*fakegh.Repo{{Owner: "org-a", Name: "x", Perms: map[string]string{"alice": "admin", "bob": "read"}}},
	})
	srv := httptest.NewServer(fake)
	defer srv.Close()
	s := newLoneStore(t)
	opt := Options{Remotes: []string{"https://github.com/org-a/x.git"}, GitHub: &GitHubRepo{
		Host: "github.com", Owner: "org-a", Name: "x",
		AccountID: "github.com/bob", AccountName: "me",
		Client: func() (*github.Client, error) {
			return github.New(config.Host{API: srv.URL}, "t-bob", nil), nil
		},
	}}
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	a, warns, err := CheckAccess(s, opt, now)
	if err != nil || !a.ReadOnly || a.Reason != ReasonGitHub || len(warns) != 0 {
		t.Errorf("read = %+v %v %v", a, warns, err)
	}
	m, _ := ReadMembers(s)
	if m.Collaborators != nil {
		t.Errorf("a reader gets no collaborators: %+v", m.Collaborators)
	}
	if err := fake.SetPerm("org-a/x", "bob", "write"); err != nil {
		t.Fatal(err)
	}
	a, _, err = CheckAccess(s, opt, now.Add(time.Minute))
	if err != nil || a.ReadOnly {
		t.Errorf("write = %+v %v", a, err)
	}
	m, _ = ReadMembers(s)
	if len(m.Collaborators) != 2 || m.Collaborators[1] != (Collaborator{ID: "github.com/bob", Permission: "write"}) {
		t.Errorf("collaborators = %+v", m.Collaborators)
	}
	// An API that fails leaves the access as it was, with a warning.
	opt.GitHub.Client = func() (*github.Client, error) { return github.New(config.Host{API: srv.URL}, "bad", nil), nil }
	a, warns, err = CheckAccess(s, opt, now.Add(2*time.Minute))
	if err != nil || a.ReadOnly || len(warns) != 1 {
		t.Fatalf("failing API = %+v %v %v", a, warns, err)
	}
	// The warning says which account failed to do what, and for a 401 what to run.
	for _, want := range []string{"github.com/bob cannot read the permission of org-a/x", "401", "run hedgehug auth login me"} {
		if !strings.Contains(warns[0], want) {
			t.Errorf("warning = %q, want it to contain %q", warns[0], want)
		}
	}
	// The failure moved the time of the check on, and kept the decision.
	if got, _ := ReadAccess(s); !got.Checked.Equal(now.Add(2*time.Minute)) || got.ReadOnly {
		t.Errorf("access after a failure = %+v", got)
	}
}

// Collaborators that cannot be read, for whatever reason, are unknown (null),
// not the list of an earlier check. A 403 is no warning (it is how a token
// without push access is told); any other failure is.
func TestCheckAccessCollaboratorsUnknownOnAnyFailure(t *testing.T) {
	t.Parallel()
	fake := fakegh.New(fakegh.State{
		Users:  []string{"alice"},
		Tokens: map[string]string{"t-alice": "alice"},
		Repos:  []*fakegh.Repo{{Owner: "org-a", Name: "x", Perms: map[string]string{"alice": "admin"}}},
	})
	var broken atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if broken.Load() && strings.HasSuffix(r.URL.Path, "/collaborators") {
			http.Error(w, `{"message":"boom"}`, http.StatusInternalServerError)
			return
		}
		fake.ServeHTTP(w, r)
	}))
	defer srv.Close()
	s := newLoneStore(t)
	opt := Options{Remotes: []string{"https://github.com/org-a/x.git"}, GitHub: &GitHubRepo{
		Host: "github.com", Owner: "org-a", Name: "x",
		AccountID: "github.com/alice", AccountName: "me",
		Client: func() (*github.Client, error) {
			return github.New(config.Host{API: srv.URL}, "t-alice", nil), nil
		},
	}}
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	if _, warns, err := CheckAccess(s, opt, now); err != nil || len(warns) != 0 {
		t.Fatalf("first check = %v %v", warns, err)
	}
	if m, _ := ReadMembers(s); len(m.Collaborators) != 1 || m.Collaborators[0].ID != "github.com/alice" {
		t.Fatalf("collaborators = %+v", m.Collaborators)
	}
	broken.Store(true)
	a, warns, err := CheckAccess(s, opt, now.Add(time.Minute))
	if err != nil || a.ReadOnly {
		t.Errorf("access after a failure = %+v %v", a, err)
	}
	m, _ := ReadMembers(s)
	if m.Collaborators != nil || !m.Checked.Equal(now.Add(time.Minute)) {
		t.Errorf("members after a failure = %+v, want collaborators null and a new time", m)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], "github.com/alice cannot read the collaborators of org-a/x") {
		t.Errorf("warnings = %v", warns)
	}
}

// A token without push access gets a 403 for the collaborators: no warning,
// and the collaborators are unknown.
func TestCheckAccessForbiddenCollaboratorsIsNoWarning(t *testing.T) {
	t.Parallel()
	fake := fakegh.New(fakegh.State{
		Users:  []string{"alice", "bob"},
		Tokens: map[string]string{"t-bob": "bob"},
		Repos:  []*fakegh.Repo{{Owner: "org-a", Name: "x", Perms: map[string]string{"alice": "admin", "bob": "write"}}},
	})
	var forbid atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if forbid.Load() && strings.HasSuffix(r.URL.Path, "/collaborators") {
			http.Error(w, `{"message":"Must have push access"}`, http.StatusForbidden)
			return
		}
		fake.ServeHTTP(w, r)
	}))
	defer srv.Close()
	s := newLoneStore(t)
	opt := Options{Remotes: []string{"https://github.com/org-a/x.git"}, GitHub: &GitHubRepo{
		Host: "github.com", Owner: "org-a", Name: "x",
		AccountID: "github.com/bob", AccountName: "me",
		Client: func() (*github.Client, error) { return github.New(config.Host{API: srv.URL}, "t-bob", nil), nil },
	}}
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	if _, _, err := CheckAccess(s, opt, now); err != nil {
		t.Fatal(err)
	}
	forbid.Store(true)
	_, warns, err := CheckAccess(s, opt, now.Add(time.Minute))
	if err != nil || len(warns) != 0 {
		t.Errorf("403 = %v %v, want no warning", warns, err)
	}
	if m, _ := ReadMembers(s); m.Collaborators != nil {
		t.Errorf("collaborators = %+v, want null", m.Collaborators)
	}
}

func TestCheckAccessLocal(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 1)
	a, _, err := CheckAccess(c.stores[0], c.opts[0], time.Now())
	if err != nil || a.ReadOnly {
		t.Errorf("writable bare = %+v %v", a, err)
	}
	gitrepotest.LockObjects(t, c.remote)
	a, _, err = CheckAccess(c.stores[0], c.opts[0], time.Now())
	if err != nil || !a.ReadOnly || a.Reason != ReasonNotWritable {
		t.Errorf("read-only bare = %+v %v", a, err)
	}
}

// A sync whose GitHub call failed does not call GitHub again before
// infoInterval has passed, and does again after.
func TestRefreshInfoFailureAdvancesChecked(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 1)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"boom"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()
	now := time.Now().Add(time.Hour)
	opt := c.opts[0]
	opt.Now = func() time.Time { return now }
	opt.GitHub = &GitHubRepo{
		Host: "github.com", Owner: "org-a", Name: "x", AccountID: "github.com/u0", AccountName: "me",
		Client: func() (*github.Client, error) {
			calls.Add(1)
			return github.New(config.Host{API: srv.URL}, "t", nil), nil
		},
	}
	res, err := Run(c.stores[0], opt)
	if err != nil || calls.Load() != 1 || len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "cannot read the permission of org-a/x") {
		t.Fatalf("first sync = %+v %v, calls %d", res, err, calls.Load())
	}
	now = now.Add(time.Minute)
	if res, err = Run(c.stores[0], opt); err != nil || calls.Load() != 1 || len(res.Warnings) != 0 {
		t.Errorf("sync right after a failure = %+v %v, calls %d, want no call", res, err, calls.Load())
	}
	now = now.Add(infoInterval)
	if _, err = Run(c.stores[0], opt); err != nil || calls.Load() != 2 {
		t.Errorf("sync after the interval = %v, calls %d, want a call", err, calls.Load())
	}
}
