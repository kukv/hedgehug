package sync

import (
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// newLoneStore makes a store that was never synced, written by github.com/u0.
func newLoneStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Create(filepath.Join(t.TempDir(), "s"), &model.StoreMeta{ULID: model.NewULID(), Name: "s"}, nil,
		&model.Workspace{ULID: model.NewULID(), Name: "web", IDPrefix: "WEB"}, gitrepo.Signature{Name: "u0", Email: "u0@x"}, "github.com/u0", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAccessAndMembersRoundTrip(t *testing.T) {
	s := newLoneStore(t)
	if a, err := ReadAccess(s); err != nil || a.ReadOnly {
		t.Errorf("no access.json = %+v, %v", a, err)
	}
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	if err := WriteAccess(s, Access{ReadOnly: true, Reason: ReasonGitHub, Checked: now}); err != nil {
		t.Fatal(err)
	}
	if a, _ := ReadAccess(s); !a.ReadOnly || a.Why() != "the GitHub permission is read" || !a.Checked.Equal(now) {
		t.Errorf("access = %+v", a)
	}
	m, err := ReadMembers(s)
	if err != nil || m.Collaborators != nil {
		t.Errorf("no members.json = %+v, %v", m, err)
	}
	if err := UpdateAuthors(s); err != nil {
		t.Fatal(err)
	}
	m, _ = ReadMembers(s)
	if m.Head == "" || !slices.Contains(m.Authors, "github.com/u0") {
		t.Errorf("members = %+v", m)
	}
}

func TestAccessWhy(t *testing.T) {
	for reason, want := range map[string]string{
		ReasonNotWritable: "the remote cannot be written",
		ReasonPushDenied:  "the remote denied the push",
		"other":           "other",
	} {
		if got := (Access{Reason: reason}).Why(); got != want {
			t.Errorf("Why(%q) = %q, want %q", reason, got, want)
		}
	}
}

// Account IDs are the same without case: an author known as github.com/U0 is
// not added again as github.com/u0.
func TestUpdateAuthorsIgnoresCase(t *testing.T) {
	s := newLoneStore(t)
	if err := WriteMembers(s, Members{Authors: []string{"github.com/U0"}}); err != nil {
		t.Fatal(err)
	}
	if err := UpdateAuthors(s); err != nil {
		t.Fatal(err)
	}
	if m, _ := ReadMembers(s); !slices.Equal(m.Authors, []string{"github.com/U0"}) {
		t.Errorf("authors = %v", m.Authors)
	}
}
