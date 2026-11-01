package ops

import (
	"strings"
	"testing"
)

// restore --at a time when the task was a forward says so, with a code of
// its own, not the bare "the task moved to another store".
func TestRestoreAtWhenTheTaskWasAForward(t *testing.T) {
	f := newMoveFx(t)
	if _, err := f.ts.MoveStore(f.in()); err != nil {
		t.Fatal(err)
	}
	atForward := f.head(t, "org-a") // API-2 is a forward here
	if _, err := f.ts.MoveStore(MoveStoreInput{Arg: "WEB-2", To: "org-a", Yes: true}); err != nil {
		t.Fatal(err)
	}
	a := f.ts.ForStore("org-a")
	_, err := a.RestoreAt("API-6", atForward)
	mustCode(t, err, KindConflict, "restore.forward")
	if err != nil && !strings.Contains(err.Error(), "API-6 was moved to another store at that time") {
		t.Errorf("message = %q", err)
	}
	e := AsError(err)
	if e == nil || e.Args["at"] != atForward {
		t.Errorf("args = %v; want at", e)
	}
}
