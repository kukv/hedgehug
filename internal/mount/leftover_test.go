package mount

import (
	"errors"
	"os"
	"testing"

	"github.com/kukv/hedgehug/internal/sync"
)

// A rebuild left beside a store is told of: the store that opens is in
// Unfinished, the one whose directory is missing is skipped with the same
// reason.
func TestOpenTellsOfALeftoverRebuild(t *testing.T) {
	here := newStoreAt(t, "here", "HRE")
	away := newStoreAt(t, "away", "AWY")
	if err := os.MkdirAll(here.Path+".rewrite-new", 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(away.Path, away.Path+".before-rewrite-1"); err != nil {
		t.Fatal(err)
	}
	set := openSet(t, here, away, newStoreAt(t, "plain", "PLN"))
	if len(set.Stores) != 2 || set.Stores[0].Name != "here" || set.Stores[1].Name != "plain" {
		t.Errorf("stores = %v", set.Stores)
	}
	var le *sync.LeftoverError
	if len(set.Unfinished) != 1 || set.Unfinished[0].Name != "here" || !errors.As(set.Unfinished[0].Err, &le) || le.Name != "here" {
		t.Errorf("Unfinished = %+v", set.Unfinished)
	}
	if len(set.Skipped) != 1 || set.Skipped[0].Name != "away" || !errors.As(set.Skipped[0].Err, &le) || le.Name != "away" {
		t.Errorf("Skipped = %+v", set.Skipped)
	}
}
