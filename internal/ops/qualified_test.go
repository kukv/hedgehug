package ops

import (
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/store"
)

func TestUndoKeptLabelIsQualifiedAndTaskIsNot(t *testing.T) {
	ts := newTwoStoreSessionWith(t, "XX", "XX")
	a := ts.ForStore("org-a")
	if _, err := a.Set("XX-1", []Assign{{Key: "title", Op: "=", Value: "b"}}); err != nil {
		t.Fatal(err)
	}
	// someone else changes the title after alice's write
	s, ix, err := a.Open()
	if err != nil {
		t.Fatal(err)
	}
	e, _ := a.Resolve(ix, "XX-1")
	if _, err := s.Update(store.Op{Name: "set", Task: e.ULID, Author: gitrepo.Signature{Name: "Bob", Email: "bob@example.com"}, Account: "github.com/bob"}, func(tx *store.Tx) error {
		task, err := tx.Task(e.ULID)
		if err != nil {
			return err
		}
		task.Title = "c"
		return tx.PutTask(task)
	}); err != nil {
		t.Fatal(err)
	}
	res, err := a.Undo()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Kept) != 1 || res.Kept[0].Task != "XX-1" || res.Kept[0].Label != "org-a:XX-1" {
		t.Errorf("Kept = %+v; want Task XX-1 and Label org-a:XX-1", res.Kept)
	}
}

func TestMountedPathFollowsAPrefixThatBecomesShared(t *testing.T) {
	ts := newTwoStoreSession(t) // API and WEB
	set, err := ts.Mount()
	if err != nil {
		t.Fatal(err)
	}
	m, _ := set.Get("org-b")
	ix, err := m.Index()
	if err != nil {
		t.Fatal(err)
	}
	if got := ix.QualifyID("API-1"); got != "API-1" {
		t.Fatalf("before: QualifyID = %q", got)
	}
	// org-b gets the prefix org-a has; a write of the session drops what it
	// knew of the shared prefixes
	b := ts.ForStore("org-b")
	s, bix, err := b.Open()
	if err != nil {
		t.Fatal(err)
	}
	w2 := *bix.Workspaces()[0]
	w2.IDPrefix = "API"
	if _, err := b.UpdateCommit(s, store.Op{Name: "workspace"}, func(tx *store.Tx) error { return tx.PutWorkspace(&w2) }); err != nil {
		t.Fatal(err)
	}
	if got := ix.QualifyID("API-1"); got != "org-b:API-1" {
		t.Errorf("after: the mounted index's QualifyID = %q; want org-b:API-1", got)
	}
}

func TestTaskLabelFollowsWhetherThePrefixIsShared(t *testing.T) {
	for _, tc := range []struct {
		a, b, want string
	}{
		{"XX", "XX", "org-a:XX-1"},
		{"API", "WEB", "API-1"},
	} {
		ts := newTwoStoreSessionWith(t, tc.a, tc.b)
		r, err := ts.Show("org-a:" + tc.a + "-1")
		if err != nil {
			t.Fatal(err)
		}
		u := r.Entry.ULID.String()
		if got, want := TaskLabel(r.Index, r.Entry), tc.want+" ("+u+")"; got != want {
			t.Errorf("%s/%s: TaskLabel = %q; want %q", tc.a, tc.b, got, want)
		}
		if got, want := RefName(r.Index, r.Entry.ULID), tc.want+" ("+u+") org-a task"; got != want {
			t.Errorf("%s/%s: RefName = %q; want %q", tc.a, tc.b, got, want)
		}
		if got := r.Entry.ID; got != tc.a+"-1" {
			t.Errorf("Entry.ID = %q; the id itself does not change", got)
		}
	}
}

func TestDeletedTaskPointsToRestoreWithTheStoreName(t *testing.T) {
	ts := newTwoStoreSessionWith(t, "XX", "XX")
	if _, err := ts.Remove("org-a:XX-1"); err != nil {
		t.Fatal(err)
	}
	_, err := ts.Set("org-a:XX-1", []Assign{{Key: "status", Op: "=", Value: "done"}})
	wantCode(t, "Set on a deleted task", err, KindConflict, "task.deleted")
	if err != nil && !strings.Contains(err.Error(), "restore org-a:XX-1 first") {
		t.Errorf("message %q lacks the store-qualified restore hint", err)
	}
}

func TestOperationIndexesQualifyToo(t *testing.T) {
	// an operation opens its own index (Session.openIndex), not the mount's
	ts := newTwoStoreSessionWith(t, "XX", "XX")
	s := ts.ForStore("org-b")
	_, ix, err := s.Open()
	if err != nil {
		t.Fatal(err)
	}
	if got := ix.QualifyID("XX-1"); got != "org-b:XX-1" {
		t.Errorf("QualifyID = %q", got)
	}
	ts2 := newTwoStoreSession(t)
	_, ix2, err := ts2.ForStore("org-b").Open()
	if err != nil {
		t.Fatal(err)
	}
	if got := ix2.QualifyID("WEB-1"); got != "WEB-1" {
		t.Errorf("QualifyID without overlap = %q", got)
	}
}
