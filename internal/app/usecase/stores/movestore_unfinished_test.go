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
		t.Skip("a directory that cannot be written to needs a permission that is enforced")
	}
	f := usecasetest.NewMoveFx(t)
	as, _ := f.TS.ForStore("org-a").OpenStore()
	// The source cannot drop the file of G, so the move stops after the
	// destination was written.
	gDir := filepath.Dir(filepath.Join(as.Dir(), filepath.FromSlash(model.TaskPath(f.G))))
	fi, err := os.Stat(gDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(gDir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(gDir, fi.Mode().Perm()) }) // so that the directory of the test can be removed
	_, err = stores.MoveStore(f.TS.Session, f.In())
	usecasetest.MustCode(t, err, usecase.KindConflict, "move.unfinished")
	if e := usecase.AsError(err); e == nil || e.Args["store"] != "org-b" {
		t.Errorf("args = %v; want store org-b", e)
	}
}
