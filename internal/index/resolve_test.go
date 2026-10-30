package index

import (
	"errors"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

func mustID(t *testing.T, s string) model.ID {
	t.Helper()
	id, err := model.ParseID(s)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// indexOf builds an index of entries without a store.
func indexOf(es ...*Entry) *Index {
	ix := &Index{sc: schema.Default(), tasks: map[model.ID]*Entry{}}
	for _, e := range es {
		ix.tasks[e.ID] = e
	}
	ix.derive()
	ix.problems = ix.findProblems()
	return ix
}

func TestResolve(t *testing.T) {
	s, _ := newStore(t)
	a := newTask("a")
	put(t, s, a) // gets TST-1
	b := newTask("b")
	b.FormerKeys = []string{"OLD-3"}
	put(t, s, b) // gets TST-2
	ix := mustOpen(t, s)
	for in, want := range map[string]model.ID{"TST-1": a.ID, "tst-2": b.ID, "old-3": b.ID, a.ID.String(): a.ID, strings.ToLower(b.ID.String()): b.ID} {
		if got, err := ix.Resolve(in); err != nil || got != want {
			t.Errorf("Resolve(%q) = %v %v", in, got, err)
		}
	}
	tail := a.ID.String()[22:]
	if _, err := ix.Resolve(tail); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("the end of a ULID resolves: %v", err)
	}
	if _, err := ix.Resolve(""); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Resolve(empty) err = %v, want ErrNotFound", err)
	}
}

func TestResolveAmbiguous(t *testing.T) {
	a := mustID(t, "01JBZ8K3QW7M2X9V4T6RAAA001")
	d := mustID(t, "01JBZ8K3QW7M2X9V4T6RCAA001")
	ix := indexOf(
		&Entry{ID: a, Key: "WEB-42"},
		&Entry{ID: d, Key: "WEB-42"},
		&Entry{ID: mustID(t, "01JBZ8K3QW7M2X9V4T6RDDD001"), Key: "API-1"},
	)
	var amb *AmbiguousError
	if _, err := ix.Resolve("WEB-42"); !errors.As(err, &amb) {
		t.Fatalf("Resolve(WEB-42) err = %v, want AmbiguousError", err)
	}
	if len(amb.Candidates) != 2 || amb.Candidates[0] != a || amb.Candidates[1] != d {
		t.Errorf("candidates = %v", amb.Candidates)
	}
}
