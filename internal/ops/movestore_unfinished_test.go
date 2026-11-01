package ops

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
)

// A move that stops after the destination was written because of a plain I/O
// error is move.unfinished with the kind of a conflict, so that the
// server answers 409 with the code and the store, not a 500.
func TestMoveStoreUnfinishedByAnIOErrorIsAConflict(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("a file that cannot be read needs a permission that is enforced")
	}
	f := newMoveFx(t)
	as, _ := f.ts.ForStore("org-a").OpenStore()
	gPath := filepath.Join(as.Dir(), filepath.FromSlash(model.TaskPath(f.g)))
	calls := 0
	f.ts.BeforeUpdate = func() {
		if calls++; calls == 2 { // the write into the source
			_ = os.Chmod(gPath, 0)
		}
	}
	_, err := f.ts.MoveStore(f.in())
	f.ts.BeforeUpdate = nil
	_ = os.Chmod(gPath, 0o600)
	mustCode(t, err, KindConflict, "move.unfinished")
	if e := AsError(err); e == nil || e.Args["store"] != "org-b" {
		t.Errorf("args = %v; want store org-b", e)
	}
}
