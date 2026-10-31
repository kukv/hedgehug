package inbox

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
)

// A record names the commit the driver merged in. Once the merge is aborted
// that commit never reaches HEAD, and the record is dropped; while the merge
// is under way the record waits.
func TestImportDropsAbortedMerge(t *testing.T) {
	_, b := twoClones(t)
	box := Open(b.s)
	inHead := head(t, b).String()
	elsewhere := "0123456789abcdef0123456789abcdef01234567"
	write := func(commit string) string {
		rec := `{"path":"tasks/0A/P0/01JBZ8K3QW7M2X9V4T6R5N8P0A.md","field":"title","value":"\"mine\"","author":"github.com/b","commit":"` + commit + `","time":"2026-10-31T10:00:00Z"}`
		name := "overwritten/" + model.NewULID().String() + ".json"
		if err := b.s.WriteLocal(name, []byte(rec)); err != nil {
			t.Fatal(err)
		}
		return name
	}
	gone := func(name string) bool {
		_, err := b.s.ReadLocal(name)
		return errors.Is(err, fs.ErrNotExist)
	}
	aborted, merged := write(elsewhere), write(inHead)
	if err := box.Import([]string{"github.com/b"}); err != nil {
		t.Fatal(err)
	}
	items, _ := box.List(true)
	if len(items) != 1 || items[0].Commit != inHead || !gone(aborted) || !gone(merged) {
		t.Errorf("items = %+v, aborted gone %v, merged gone %v", items, gone(aborted), gone(merged))
	}

	// With a merge under way the record stays.
	pending := write(elsewhere)
	mergeHead := filepath.Join(b.s.Dir(), ".git", "MERGE_HEAD")
	if err := os.WriteFile(mergeHead, []byte(elsewhere+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := box.Import([]string{"github.com/b"}); err != nil {
		t.Fatal(err)
	}
	if gone(pending) {
		t.Error("a record of a merge under way was dropped")
	}
	if err := os.Remove(mergeHead); err != nil {
		t.Fatal(err)
	}
	if err := box.Import([]string{"github.com/b"}); err != nil {
		t.Fatal(err)
	}
	if items, _ := box.List(true); len(items) != 1 || !gone(pending) {
		t.Errorf("after the abort: items = %+v, gone %v", items, gone(pending))
	}
}
