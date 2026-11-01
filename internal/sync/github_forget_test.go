package sync

import (
	"errors"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/fakegh"
	"github.com/kukv/hedgehug/internal/github"
)

// When GitHub cannot be reached for the permission at all, the collaborators
// of an earlier check are not kept: they become null and the time moves on.
func TestCheckAccessFailureForgetsCollaborators(t *testing.T) {
	fake := fakegh.New(fakegh.State{
		Users:  []string{"alice"},
		Tokens: map[string]string{"t-alice": "alice"},
		Repos:  []*fakegh.Repo{{Owner: "org-a", Name: "x", Perms: map[string]string{"alice": "admin"}}},
	})
	srv := httptest.NewServer(fake)
	defer srv.Close()
	good := func() (*github.Client, error) { return github.New(config.Host{API: srv.URL}, "t-alice", nil), nil }
	s := newLoneStore(t)
	opt := Options{
		Remotes: []string{"https://github.com/org-a/x.git"},
		GitHub: &GitHubRepo{
			Host: "github.com", Owner: "org-a", Name: "x",
			AccountID: "github.com/alice", AccountName: "me", Client: good,
		},
	}
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	if _, _, err := CheckAccess(s, opt, now); err != nil {
		t.Fatal(err)
	}
	if err := UpdateAuthors(s); err != nil { // seeds authors and head
		t.Fatal(err)
	}
	seeded, _ := ReadMembers(s)
	if len(seeded.Collaborators) != 1 || len(seeded.Authors) == 0 || seeded.Head == "" {
		t.Fatalf("members = %+v", seeded)
	}
	failing := map[string]func() (*github.Client, error){
		"401":       func() (*github.Client, error) { return github.New(config.Host{API: srv.URL}, "bad", nil), nil },
		"no client": func() (*github.Client, error) { return nil, errors.New("no token") },
	}
	for name, client := range failing {
		now = now.Add(time.Minute)
		opt.GitHub.Client = client
		_, warns, err := CheckAccess(s, opt, now)
		if err != nil || len(warns) != 1 {
			t.Fatalf("%s: warns = %v, err = %v", name, warns, err)
		}
		m, _ := ReadMembers(s)
		if m.Collaborators != nil || !m.Checked.Equal(now) {
			t.Errorf("%s: members = %+v, want null and checked %v", name, m, now)
		}
		if !slices.Equal(m.Authors, seeded.Authors) || m.Head != seeded.Head {
			t.Errorf("%s: authors/head = %v %q, want %v %q kept", name, m.Authors, m.Head, seeded.Authors, seeded.Head)
		}
		// With a working token the list is back; the next failure drops it again.
		opt.GitHub.Client = good
		now = now.Add(time.Minute)
		if _, _, err := CheckAccess(s, opt, now); err != nil {
			t.Fatal(err)
		}
		if m, _ := ReadMembers(s); len(m.Collaborators) != 1 {
			t.Fatalf("%s: collaborators after recovery = %+v", name, m.Collaborators)
		}
	}
}
