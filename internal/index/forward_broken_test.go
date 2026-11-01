package index

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// A forward that does not decode names no store, so a lookup by its ULID
// finds nothing; doctor tells of the file.
func TestResolveForwardSkipsABrokenForward(t *testing.T) {
	s, dir := newStore(t)
	ulid := model.NewULID()
	commitRaw(t, s, dir, model.TaskPath(ulid), "---\nulid: "+ulid.String()+"\nmoved_to: "+destStore+"\ntitle: x\n---\n")
	ix := mustOpen(t, s)
	if _, ok := ix.Forward(ulid); !ok {
		t.Fatal("the broken forward is not in the index")
	}
	if _, err := ix.ResolveForward(ulid.String()); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("ResolveForward = %v; want not found", err)
	}
}
