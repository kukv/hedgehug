package mount

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kukv/hedgehug/internal/app/storage/mount/mounttest"

	"github.com/kukv/hedgehug/internal/app/storage/store"
)

func writeRebuild(t *testing.T, dir, phase string) {
	t.Helper()
	local := filepath.Join(dir, filepath.FromSlash(store.LocalDir))
	if err := os.MkdirAll(local, 0o750); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(local, store.RebuildFile)
	if err := os.WriteFile(p, []byte(`{"phase":"`+phase+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
}

// A rebuild that stopped is told of: a store with only the old objects left
// is in Unfinished, one with a work tree part old, part new is skipped with
// the same reason.
func TestOpenTellsOfAnUnfinishedRebuild(t *testing.T) {
	t.Parallel()
	prune := mounttest.NewStoreAt(t, "prune", "PRN")
	apply := mounttest.NewStoreAt(t, "apply", "APL")
	writeRebuild(t, prune.Path, "prune")
	writeRebuild(t, apply.Path, "apply")
	set := openSet(t, prune, apply, mounttest.NewStoreAt(t, "plain", "PLN"))
	if len(set.Stores) != 2 || set.Stores[0].Name != "prune" || set.Stores[1].Name != "plain" {
		t.Errorf("stores = %v", set.Stores)
	}
	var le *store.LeftoverError
	if len(set.Unfinished) != 1 || set.Unfinished[0].Name != "prune" || !errors.As(set.Unfinished[0].Err, &le) || le.Name != "prune" {
		t.Errorf("Unfinished = %+v", set.Unfinished)
	}
	if len(set.Skipped) != 1 || set.Skipped[0].Name != "apply" || !errors.As(set.Skipped[0].Err, &le) || le.Name != "apply" {
		t.Errorf("Skipped = %+v", set.Skipped)
	}
}
