package ops

import (
	"strings"
	"testing"

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
	data := "---\nulid: " + u.String() + "\nmoved_to: " + model.NewULID().String() + "\ntitle: keep me\n---\n"
	if _, err := a.Update(s, store.Op{Name: "raw"}, func(tx *store.Tx) error {
		return tx.PutFile(model.TaskPath(u), []byte(data))
	}); err != nil {
		t.Fatal(err)
	}
	return u
}

// A forward file that does not decode may still hold a task's content: purge
// --moved leaves it, tells of it, and the lookup of a moved task does not
// answer with it.
func TestPurgeMovedLeavesABrokenForward(t *testing.T) {
	t.Parallel()
	f := newMoveFx(t)
	if _, err := f.ts.MoveStore(f.in()); err != nil {
		t.Fatal(err)
	}
	broken := f.brokenForward(t)
	var warned []string
	a := f.ts.ForStore("org-a")
	a.Warn = func(m string) { warned = append(warned, m) }

	list, err := a.PurgeMovedList()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Errorf("list = %+v; want the 3 forwards that decode", list)
	}
	if len(warned) != 1 || !strings.Contains(warned[0], "1 broken forward files left; see hedgehug doctor") {
		t.Errorf("warnings = %v", warned)
	}
	removed, _, err := a.PurgeMoved(true)
	if err != nil || len(removed) != 3 {
		t.Fatalf("removed %d, %v; want 3", len(removed), err)
	}
	ix := mustIndex(t, f.ts, "org-a")
	if _, ok := ix.Forward(broken); !ok {
		t.Error("the broken forward file was removed")
	}
}

// A confirmed removal refuses a forward that came in after the list was shown.
func TestPurgeMovedConfirmedRefusesChanges(t *testing.T) {
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
	var confirmed []model.ULID
	for _, v := range list {
		confirmed = append(confirmed, v.ULID)
	}
	late := model.NewULID()
	s, err := a.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	fwd := "---\nulid: " + late.String() + "\nmoved_to: " + model.NewULID().String() + "\n---\n"
	if _, err := a.Update(s, store.Op{Name: "raw"}, func(tx *store.Tx) error {
		return tx.PutFile(model.TaskPath(late), []byte(fwd))
	}); err != nil {
		t.Fatal(err)
	}
	before := len(f.log(t, "org-a"))
	_, _, err = a.PurgeMovedConfirmed(confirmed)
	mustCode(t, err, KindConflict, "purge.changed")
	if got := len(f.log(t, "org-a")); got != before {
		t.Errorf("a refused purge wrote %d commits", got-before)
	}
	// asked again over the list as it is now
	list, err = a.PurgeMovedList()
	if err != nil {
		t.Fatal(err)
	}
	confirmed = confirmed[:0]
	for _, v := range list {
		confirmed = append(confirmed, v.ULID)
	}
	removed, commit, err := a.PurgeMovedConfirmed(confirmed)
	if err != nil || len(removed) != 4 || commit == "" {
		t.Errorf("removed %d, commit %q, %v; want 4", len(removed), commit, err)
	}
	_, _, err = a.PurgeMovedConfirmed(nil)
	mustCode(t, err, KindUsage, "confirm.required")
}
