package ops

import (
	"slices"
	"testing"
)

func TestPurgeMovedListsAndRemoves(t *testing.T) {
	t.Parallel()
	f := newMoveFx(t)
	if _, err := f.ts.MoveStore(f.in()); err != nil {
		t.Fatal(err)
	}
	a := f.ts.ForStore("org-a")
	list, err := a.PurgeMovedList()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Fatalf("list = %+v; want the 3 tasks that moved", list)
	}
	var root *ForwardView
	for i := range list {
		if list[i].ULID == f.p {
			root = &list[i]
		}
	}
	if root == nil || root.ID != "API-2" || root.ToStore != "org-b" || root.ToID == "" || root.ToULID.IsZero() {
		t.Errorf("forward of the root = %+v; want API-2 → org-b with its id there", root)
	}

	before := len(f.log(t, "org-a"))
	_, _, err = a.PurgeMoved(false)
	mustCode(t, err, KindUsage, "confirm.required")
	if got := len(f.log(t, "org-a")); got != before {
		t.Fatalf("a refused purge wrote %d commits", got-before)
	}

	removed, commit, err := a.PurgeMoved(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 3 || commit == "" {
		t.Fatalf("removed %d, commit %q", len(removed), commit)
	}
	l := f.log(t, "org-a")
	if len(l) != before+1 {
		t.Fatalf("commits = %d; want one more than %d", len(l), before)
	}
	if op, _ := l[0].Trailer("Hedgehug-Op"); op != "purge-moved" {
		t.Errorf("Hedgehug-Op = %q", op)
	}
	if list, err = a.PurgeMovedList(); err != nil || len(list) != 0 {
		t.Errorf("list after = %+v, %v", list, err)
	}
	// the moved tasks stay in org-b
	if _, ok := mustIndex(t, f.ts, "org-b").Task(f.p); !ok {
		t.Error("the moved task is gone from org-b")
	}
}

func TestPurgeMovedNoneIsNoCommit(t *testing.T) {
	t.Parallel()
	f := newMoveFx(t)
	a := f.ts.ForStore("org-a")
	head := f.head(t, "org-a")
	for _, yes := range []bool{false, true} {
		removed, commit, err := a.PurgeMoved(yes)
		if err != nil || len(removed) != 0 || commit != "" {
			t.Errorf("yes=%v: %v, %q, %v", yes, removed, commit, err)
		}
	}
	if f.head(t, "org-a") != head {
		t.Error("head moved")
	}
}

func TestPurgeMovedForwardToAStoreNotMounted(t *testing.T) {
	t.Parallel()
	f := newMoveFx(t)
	if _, err := f.ts.MoveStore(f.in()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.ts.StoreRemove("org-b", false); err != nil {
		t.Fatal(err)
	}
	f.ts.dropMount()
	list, err := f.ts.ForStore("org-a").PurgeMovedList()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 || list[0].ToStore != "" || list[0].ToULID.IsZero() {
		t.Errorf("list = %+v; want forwards to a store that is not mounted, with its ULID", list)
	}
	if !slices.ContainsFunc(list, func(v ForwardView) bool { return v.ULID == f.p }) {
		t.Errorf("list = %+v lacks the root", list)
	}
}

// The forwards are read again under the lock: when a sync or another
// command took them away in between, there is nothing to commit.
func TestPurgeMovedGoneUnderTheLockIsNoCommit(t *testing.T) {
	t.Parallel()
	f := newMoveFx(t)
	if _, err := f.ts.MoveStore(f.in()); err != nil {
		t.Fatal(err)
	}
	a := f.ts.ForStore("org-a")
	racing := false
	a.BeforeUpdate = func() {
		if racing {
			return
		}
		racing = true
		if _, _, err := a.PurgeMoved(true); err != nil {
			t.Error(err)
		}
	}
	// the list was read before the other purge, so it is not empty
	before := len(f.log(t, "org-a"))
	removed, commit, err := a.PurgeMoved(true)
	if err != nil || len(removed) != 0 || commit != "" {
		t.Errorf("removed %v, commit %q, err %v; want nothing", removed, commit, err)
	}
	if got := len(f.log(t, "org-a")); got != before+1 {
		t.Errorf("commits = %d; want only the racing purge (%d)", got, before+1)
	}
}
