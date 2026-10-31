package query

import (
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/index"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

func TestRunErrorUnwraps(t *testing.T) {
	s := newStore(t)
	for _, ulid := range []string{"01JBZ8K3QW7M2X9V4T6RAAA001", "01JBZ8K3QW7M2X9V4T6RBBB001"} {
		taskULID, err := model.ParseULID(ulid)
		if err != nil {
			t.Fatal(err)
		}
		writeRaw(t, s, model.TaskPath(taskULID), "---\nulid: "+ulid+"\nkind: task\nid: WEB-1\ntitle: t\nstatus: todo\n---\n")
	}
	ix := mustOpen(t, s)
	runQ := func(q string) error {
		parsed, err := Parse(q)
		if err != nil {
			t.Fatal(err)
		}
		_, err = Run(ix, parsed, Options{})
		return err
	}
	var amb *index.AmbiguousError
	if err := runQ("parent:WEB-1"); !errors.As(err, &amb) || len(amb.Candidates) != 2 {
		t.Errorf("parent:WEB-1 err = %v, want an AmbiguousError of 2 candidates", err)
	}
	if err := runQ("parent:WEB-9"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("parent:WEB-9 err = %v, want ErrNotFound", err)
	}
}

func TestRunBlockedByClosedOrGone(t *testing.T) {
	s := newStore(t)
	done, canceled, deleted := newTask("done"), newTask("canceled"), newTask("deleted")
	done.Status, canceled.Status = "done", "canceled"
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	deleted.DeletedAt = &at
	broken, missing := model.NewULID(), model.NewULID()
	writeRaw(t, s, model.TaskPath(broken), "not a task file\n")
	put(t, s, done, canceled, deleted)
	for title, ulid := range map[string]model.ULID{"byDone": done.ULID, "byCanceled": canceled.ULID, "byDeleted": deleted.ULID, "byBroken": broken, "byMissing": missing} {
		task := newTask(title)
		task.BlockedBy = []model.ULID{ulid}
		// Past the store, which refuses a blocker that is not there.
		writeRaw(t, s, model.TaskPath(task.ULID), "---\nulid: "+task.ULID.String()+"\nkind: task\ntitle: "+title+"\nstatus: todo\nblocked_by: ["+ulid.String()+"]\n---\n")
	}
	got := run(t, mustOpen(t, s), "is:blocked", Options{})
	if len(got) != 1 || len(got[0]) != 0 {
		t.Errorf("is:blocked = %v, want none", got)
	}
}

func TestRunNumberNotFinite(t *testing.T) {
	f := newFixture(t)
	for _, q := range []string{"points:NaN", "points>Inf", "points<-inf"} {
		parsed, err := Parse(q)
		if err != nil {
			t.Fatal(err)
		}
		_, err = Run(f.ix, parsed, Options{})
		var qe *Error
		if !errors.As(err, &qe) {
			t.Errorf("Run(%q) err = %v, want an Error", q, err)
		}
	}
}
