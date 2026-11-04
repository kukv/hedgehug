package tasks_test

import (
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// restore --at a time when the task was a forward says so, with a code of
// its own, not the bare "the task moved to another store".
func TestRestoreAtWhenTheTaskWasAForward(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	if _, err := stores.MoveStore(f.TS.Session, f.In()); err != nil {
		t.Fatal(err)
	}
	atForward := f.Head(t, "org-a") // API-2 is a forward here
	if _, err := stores.MoveStore(f.TS.Session, stores.MoveStoreInput{Arg: "WEB-2", To: "org-a", Yes: true}); err != nil {
		t.Fatal(err)
	}
	a := f.TS.ForStore("org-a")
	_, err := tasks.RestoreAt(a, "API-6", atForward)
	usecasetest.MustCode(t, err, usecase.KindConflict, "restore.forward")
	if err != nil && !strings.Contains(err.Error(), "API-6 was moved to another store at that time") {
		t.Errorf("message = %q", err)
	}
	e := usecase.AsError(err)
	if e == nil || e.Args["at"] != atForward {
		t.Errorf("args = %v; want at", e)
	}
}
