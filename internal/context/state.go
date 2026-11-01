package context

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kukv/hedgehug/internal/atomicfile"
	"github.com/kukv/hedgehug/internal/model"
)

// fileName is the state file in the data directory.
const fileName = "context.json"

// Ref names a project in a store, by ULIDs.
type Ref struct {
	Store   model.ULID `json:"store"`
	Project model.ULID `json:"project"`
}

// State is context.json.
type State struct {
	Use    *Ref           `json:"use"`
	Recent map[string]Ref `json:"recent"` // by repo ID
}

// Load reads <dataDir>/context.json. A missing file is the empty state. A
// file that does not read, or does not decode, is the empty state too, and
// warn is told why (D11).
func Load(dataDir string, warn func(string)) State {
	b, err := os.ReadFile(filepath.Join(dataDir, fileName)) //nolint:gosec // G304: the data directory is Hedgehug's own
	if err != nil {
		if !os.IsNotExist(err) && warn != nil {
			warn("context.json cannot be read, so it is read as empty: " + err.Error())
		}
		return State{}
	}
	var st State
	if err := json.Unmarshal(b, &st); err != nil {
		if warn != nil {
			warn("context.json is broken, so it is read as empty: " + err.Error())
		}
		return State{}
	}
	return st
}

// Save writes st to <dataDir>/context.json through a temporary file and a
// rename, with no lock (the last write wins).
func Save(dataDir string, st State) error {
	if st.Recent == nil {
		st.Recent = map[string]Ref{}
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if err := atomicfile.WriteFile(filepath.Join(dataDir, fileName), append(b, '\n')); err != nil {
		return fmt.Errorf("write %s: %w", fileName, err)
	}
	return nil
}
