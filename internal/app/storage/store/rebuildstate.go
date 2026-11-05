package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/kukv/hedgehug/internal/lib/osfs"
)

// RebuildFile is the file of .hedgehug/local that says a rebuild after a
// history rewrite (replicate.RebuildStore) is under way or stopped.
const RebuildFile = "rebuild.json"

// RebuildPhase is how far a rebuild got.
type RebuildPhase string

const (
	RebuildNone  RebuildPhase = ""      // no rebuild
	RebuildApply RebuildPhase = "apply" // the work tree may be part old, part new
	RebuildPrune RebuildPhase = "prune" // the store is whole; old objects are left
)

// RebuildPhaseIn tells the phase of the rebuild of the store at dir. A
// rebuild.json that cannot be read is taken for RebuildApply: nothing says
// the store is whole.
func RebuildPhaseIn(dir string) RebuildPhase {
	b, err := osfs.ReadFile(filepath.Join(dir, filepath.FromSlash(LocalDir), RebuildFile))
	if errors.Is(err, fs.ErrNotExist) {
		return RebuildNone
	}
	var rec struct {
		Phase RebuildPhase `json:"phase"`
	}
	if err != nil || json.Unmarshal(b, &rec) != nil || rec.Phase != RebuildPrune {
		return RebuildApply
	}
	return RebuildPrune
}

// LeftoverMessage is what Hedgehug tells of an unfinished rebuild of the
// store called name.
func LeftoverMessage(name string) string {
	return fmt.Sprintf("the rebuild of %s after a history rewrite is unfinished (hedgehug doctor --fix finishes it)", name)
}

// LeftoverError says a store has a rebuild left unfinished.
type LeftoverError struct{ Name string }

func (e *LeftoverError) Error() string { return LeftoverMessage(e.Name) }
