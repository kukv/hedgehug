package ops

import (
	"strings"
	"testing"
)

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
