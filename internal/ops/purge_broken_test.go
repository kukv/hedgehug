package ops

import (
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// brokenForward puts a forward file with a key it must not have in org-a.
func (f *moveFx) brokenForward(t *testing.T) model.ULID {
	t.Helper()
	u := model.NewULID()
	a := f.ts.ForStore("org-a")
	s, err := a.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	data := "---\nulid: " + u.String() + "\nmoved_to: " + model.NewULID().String() + "\nmoved_at: 2026-10-31T10:00:00Z\ntitle: keep me\n---\n"
	if _, err := a.Update(s, store.Op{Name: "raw"}, func(tx *store.Tx) error {
		return tx.PutFile(model.TaskPath(u), []byte(data))
	}); err != nil {
		t.Fatal(err)
	}
	return u
}

// oldForward puts a forward file that moved at in org-a.
func (f *moveFx) oldForward(t *testing.T, at time.Time) model.ULID {
	t.Helper()
	u := model.NewULID()
	a := f.ts.ForStore("org-a")
	s, err := a.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	data, err := (&model.Forward{ULID: u, MovedTo: model.NewULID(), MovedAt: at}).Encode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Update(s, store.Op{Name: "raw"}, func(tx *store.Tx) error {
		return tx.PutFile(model.TaskPath(u), data)
	}); err != nil {
		t.Fatal(err)
	}
	return u
}

func TestPurgeForwardsByTheirMovedAt(t *testing.T) {
	t.Parallel()
	f := newMoveFx(t)
	if _, err := f.ts.MoveStore(f.in()); err != nil { // 3 tasks move now: 3 forwards, moved_at = the clock
		t.Fatal(err)
	}
	a := f.ts.ForStore("org-a")
	old := f.oldForward(t, a.Now().AddDate(0, 0, -100))

	plan, err := a.PurgeList(90)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Tasks) != 0 || len(plan.Forwards) != 1 || plan.Forwards[0].ULID != old {
		t.Fatalf("plan = %+v; want only the old forward", plan)
	}
	plan, err = a.PurgeList(0)
	if err != nil || len(plan.Forwards) != 4 {
		t.Fatalf("plan(0d) = %d forwards, %v; want 4", len(plan.Forwards), err)
	}
	for _, p := range plan.Forwards {
		if p.ULID != old && !p.MovedAt.Equal(a.Now().UTC().Truncate(time.Second)) {
			t.Errorf("moved_at of %s = %v; want the time of the move", p.ULID, p.MovedAt)
		}
	}

	before := len(f.log(t, "org-a"))
	res, err := a.Purge(0, true)
	if err != nil || len(res.Forwards) != 4 || res.Commit == "" {
		t.Fatalf("Purge = %+v, %v", res, err)
	}
	l := f.log(t, "org-a")
	if len(l) != before+1 {
		t.Fatalf("commits = %d; want one more than %d", len(l), before)
	}
	if op, _ := l[0].Trailer("Hedgehug-Op"); op != "purge" {
		t.Errorf("Hedgehug-Op = %q; want purge", op)
	}
	if plan, err = a.PurgeList(0); err != nil || !plan.Empty() {
		t.Errorf("plan after = %+v, %v", plan, err)
	}
	// the moved tasks stay in org-b
	if _, ok := mustIndex(t, f.ts, "org-b").Task(f.p); !ok {
		t.Error("the moved task is gone from org-b")
	}
}

// A forward that does not decode may still hold a task's content: purge
// leaves it and tells of it, and the lookup of a moved task does not answer
// with it.
func TestPurgeLeavesABrokenForward(t *testing.T) {
	t.Parallel()
	f := newMoveFx(t)
	if _, err := f.ts.MoveStore(f.in()); err != nil {
		t.Fatal(err)
	}
	broken := f.brokenForward(t)
	var warned []string
	a := f.ts.ForStore("org-a")
	a.Warn = func(m string) { warned = append(warned, m) }

	plan, err := a.PurgeList(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Forwards) != 3 {
		t.Errorf("plan = %+v; want the 3 forwards that decode", plan)
	}
	if len(warned) != 1 || !strings.Contains(warned[0], "1 broken forward files left; see hedgehug doctor") {
		t.Errorf("warnings = %v", warned)
	}
	res, err := a.Purge(0, true)
	if err != nil || len(res.Forwards) != 3 {
		t.Fatalf("Purge = %+v, %v; want 3", res, err)
	}
	if _, ok := mustIndex(t, f.ts, "org-a").Forward(broken); !ok {
		t.Error("the broken forward file was removed")
	}
}

// A forward with no moved_at is a broken one.
func TestAForwardWithoutMovedAtIsBroken(t *testing.T) {
	t.Parallel()
	f := newMoveFx(t)
	u := model.NewULID()
	a := f.ts.ForStore("org-a")
	s, err := a.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	data := "---\nulid: " + u.String() + "\nmoved_to: " + model.NewULID().String() + "\n---\n"
	if _, err := a.Update(s, store.Op{Name: "raw"}, func(tx *store.Tx) error { return tx.PutFile(model.TaskPath(u), []byte(data)) }); err != nil {
		t.Fatal(err)
	}
	plan, err := a.PurgeList(0)
	if err != nil || !plan.Empty() {
		t.Errorf("plan = %+v, %v; want nothing (the file is broken, not a forward to purge)", plan, err)
	}
	if fe, ok := mustIndex(t, f.ts, "org-a").Forward(u); !ok || fe.Broken == "" {
		t.Errorf("forward = %+v, %v; want a broken one", fe, ok)
	}
}

// A forward that is gone under the lock is no commit.
func TestPurgeGoneUnderTheLockIsNoCommit(t *testing.T) {
	t.Parallel()
	f := newMoveFx(t)
	if _, err := f.ts.MoveStore(f.in()); err != nil {
		t.Fatal(err)
	}
	a := f.ts.ForStore("org-a")
	plan, err := a.PurgeList(0)
	if err != nil {
		t.Fatal(err)
	}
	// someone else purges first
	if _, err := a.Purge(0, true); err != nil {
		t.Fatal(err)
	}
	res, err := a.PurgeConfirmed(0, plan)
	wantErr(t, err, KindConflict, "purge.changed")
	if res.Commit != "" {
		t.Errorf("commit = %q", res.Commit)
	}
}
