package stores_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// A move that stops after the destination was written because of a plain I/O
// error is move.unfinished with the kind of a conflict, so that the
// server answers 409 with the code and the store, not a 500.
func TestMoveStoreUnfinishedByAnIOErrorIsAConflict(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("a file that cannot be read needs a permission that is enforced")
	}
	f := usecasetest.NewMoveFx(t)
	as, _ := f.TS.ForStore("org-a").OpenStore()
	gPath := filepath.Join(as.Dir(), filepath.FromSlash(model.TaskPath(f.G)))
	calls := 0
	f.TS.BeforeUpdate = func() {
		if calls++; calls == 2 { // the write into the source
			_ = os.Chmod(gPath, 0)
		}
	}
	_, err := stores.MoveStore(f.TS.Session, f.In())
	f.TS.BeforeUpdate = nil
	_ = os.Chmod(gPath, 0o600)
	usecasetest.MustCode(t, err, usecase.KindConflict, "move.unfinished")
	if e := usecase.AsError(err); e == nil || e.Args["store"] != "org-b" {
		t.Errorf("args = %v; want store org-b", e)
	}
}
