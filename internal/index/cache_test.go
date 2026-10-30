package index

import (
	"bytes"
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/store"
)

func TestOpenKeepsCacheOfHead(t *testing.T) {
	s, _ := newStore(t)
	put(t, s, newTask("original"))
	mustOpen(t, s)
	// Trailing bytes are ignored by the decoder, but a rewrite drops them.
	b, err := s.ReadLocal(cacheName)
	if err != nil {
		t.Fatal(err)
	}
	marked := append(slices.Clone(b), "sentinel"...)
	if err := s.WriteLocal(cacheName, marked); err != nil {
		t.Fatal(err)
	}
	mustOpen(t, s)
	got, err := s.ReadLocal(cacheName)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, marked) {
		t.Error("Open rewrote a cache that was for HEAD")
	}
}

func TestResolveEmpty(t *testing.T) {
	ix := indexOf(&Entry{ID: mustID(t, "01JBZ8K3QW7M2X9V4T6R5N8P0C"), FormerKeys: []string{""}})
	if _, err := ix.Resolve(""); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Resolve(\"\") err = %v, want ErrNotFound", err)
	}
}
