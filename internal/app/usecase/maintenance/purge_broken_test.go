package maintenance_test

import (
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/maintenance"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestPurgeForwardsByTheirMovedAt(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	if _, err := stores.MoveStore(f.TS.Session, f.In()); err != nil { // 3 tasks move now: 3 forwards, moved_at = the clock
		t.Fatal(err)
	}
	a := f.TS.ForStore("org-a")
	old := f.OldForward(t, a.Now().AddDate(0, 0, -100))

	plan, err := maintenance.PurgeList(a, 90)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Tasks) != 0 || len(plan.Forwards) != 1 || plan.Forwards[0].ULID != old {
		t.Fatalf("plan = %+v; want only the old forward", plan)
	}
	plan, err = maintenance.PurgeList(a, 0)
	if err != nil || len(plan.Forwards) != 4 {
		t.Fatalf("plan(0d) = %d forwards, %v; want 4", len(plan.Forwards), err)
	}
	for _, p := range plan.Forwards {
		if p.ULID != old && !p.MovedAt.Equal(a.Now().UTC().Truncate(time.Second)) {
			t.Errorf("moved_at of %s = %v; want the time of the move", p.ULID, p.MovedAt)
		}
	}

	before := len(f.Log(t, "org-a"))
	res, err := maintenance.Purge(a, 0, true)
	if err != nil || len(res.Forwards) != 4 || res.Commit == "" {
		t.Fatalf("Purge = %+v, %v", res, err)
	}
	l := f.Log(t, "org-a")
	if len(l) != before+1 {
		t.Fatalf("commits = %d; want one more than %d", len(l), before)
	}
	if op, _ := l[0].Trailer("Hedgehug-Op"); op != "purge" {
		t.Errorf("Hedgehug-Op = %q; want purge", op)
	}
	if plan, err = maintenance.PurgeList(a, 0); err != nil || !plan.Empty() {
		t.Errorf("plan after = %+v, %v", plan, err)
	}
	// the moved tasks stay in org-b
	if _, ok := usecasetest.MustIndex(t, f.TS, "org-b").Task(f.P); !ok {
		t.Error("the moved task is gone from org-b")
	}
}

// A forward that does not decode may still hold a task's content: purge
// leaves it and tells of it, and the lookup of a moved task does not answer
// with it.
func TestPurgeLeavesABrokenForward(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	if _, err := stores.MoveStore(f.TS.Session, f.In()); err != nil {
		t.Fatal(err)
	}
	broken := f.BrokenForward(t)
	var warned []string
	a := f.TS.ForStore("org-a")
	a.Warn = func(m string) { warned = append(warned, m) }

	plan, err := maintenance.PurgeList(a, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Forwards) != 3 {
		t.Errorf("plan = %+v; want the 3 forwards that decode", plan)
	}
	if len(warned) != 1 || !strings.Contains(warned[0], "1 broken forward file left; see hedgehug doctor") {
		t.Errorf("warnings = %v", warned)
	}
	res, err := maintenance.Purge(a, 0, true)
	if err != nil || len(res.Forwards) != 3 {
		t.Fatalf("Purge = %+v, %v; want 3", res, err)
	}
	if _, ok := usecasetest.MustIndex(t, f.TS, "org-a").Forward(broken); !ok {
		t.Error("the broken forward file was removed")
	}
}

// A forward with no moved_at is a broken one.
func TestAForwardWithoutMovedAtIsBroken(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	u := model.NewULID()
	a := f.TS.ForStore("org-a")
	s, err := a.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	data := "---\nulid: " + u.String() + "\nmoved_to: " + model.NewULID().String() + "\n---\n"
	if _, err := a.Update(s, store.Op{Name: "raw"}, func(tx *store.Tx) error { return tx.PutFile(model.TaskPath(u), []byte(data)) }); err != nil {
		t.Fatal(err)
	}
	plan, err := maintenance.PurgeList(a, 0)
	if err != nil || !plan.Empty() {
		t.Errorf("plan = %+v, %v; want nothing (the file is broken, not a forward to purge)", plan, err)
	}
	if fe, ok := usecasetest.MustIndex(t, f.TS, "org-a").Forward(u); !ok || fe.Broken == "" {
		t.Errorf("forward = %+v, %v; want a broken one", fe, ok)
	}
}

// A forward that is gone under the lock is no commit.
func TestPurgeGoneUnderTheLockIsNoCommit(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	if _, err := stores.MoveStore(f.TS.Session, f.In()); err != nil {
		t.Fatal(err)
	}
	a := f.TS.ForStore("org-a")
	plan, err := maintenance.PurgeList(a, 0)
	if err != nil {
		t.Fatal(err)
	}
	// someone else purges first
	if _, err := maintenance.Purge(a, 0, true); err != nil {
		t.Fatal(err)
	}
	headBefore, logBefore := f.Head(t, "org-a"), len(f.Log(t, "org-a"))
	res, err := maintenance.PurgeConfirmed(a, 0, plan)
	usecasetest.WantErr(t, err, usecase.KindConflict, "purge.changed")
	if res.Commit != "" {
		t.Errorf("commit = %q", res.Commit)
	}
	if head, n := f.Head(t, "org-a"), len(f.Log(t, "org-a")); head != headBefore || n != logBefore {
		t.Errorf("the refused purge moved HEAD: %s -> %s, %d -> %d commits", headBefore, head, logBefore, n)
	}
}
