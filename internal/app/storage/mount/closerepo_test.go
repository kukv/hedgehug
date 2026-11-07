package mount

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/storage/mount/mounttest"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/store"
)

// newStoresIn creates the stores named names in one directory.
func newStoresIn(t *testing.T, root string, names ...string) []config.StoreEntry {
	t.Helper()
	var out []config.StoreEntry
	for i, name := range names {
		dir := filepath.Join(root, name)
		ws := &model.Workspace{ULID: model.NewULID(), Name: name, IDPrefix: string(rune('A'+i)) + "XX"}
		if _, err := store.Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: name}, nil, ws, mounttest.Author, "", time.Now); err != nil {
			t.Fatal(err)
		}
		out = append(out, config.StoreEntry{Name: name, Path: dir})
	}
	return out
}
