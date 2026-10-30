package index

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

func mustULID(t *testing.T, s string) model.ULID {
	t.Helper()
	id, err := model.ParseULID(s)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// indexOf builds an index of entries without a store.
func indexOf(es ...*Entry) *Index {
	ws := &model.Workspace{ULID: model.NewULID(), Name: "test", KeyPrefix: "TST"}
	ix := &Index{sc: schema.Default(), tasks: map[model.ULID]*Entry{}, workspaces: map[model.ULID]*model.Workspace{ws.ULID: ws}}
	for i, e := range es {
		// An entry without a key or a workspace gets one, so the problems of
		// the tests are only the ones they are about.
		if e.Key == "" {
			e.Key = fmt.Sprintf("TST-%d", i+1)
		}
		if e.Workspace.IsZero() {
			e.Workspace = ws.ULID
		}
		ix.tasks[e.ULID] = e
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
	for in, want := range map[string]model.ULID{"TST-1": a.ULID, "tst-2": b.ULID, "old-3": b.ULID, a.ULID.String(): a.ULID, strings.ToLower(b.ULID.String()): b.ULID} {
		if got, err := ix.Resolve(in); err != nil || got != want {
			t.Errorf("Resolve(%q) = %v %v", in, got, err)
		}
	}
	tail := a.ULID.String()[22:]
	if _, err := ix.Resolve(tail); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("the end of a ULID resolves: %v", err)
	}
	if _, err := ix.Resolve(""); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Resolve(empty) err = %v, want ErrNotFound", err)
	}
}

func TestResolveAmbiguous(t *testing.T) {
	a := mustULID(t, "01JBZ8K3QW7M2X9V4T6RAAA001")
	d := mustULID(t, "01JBZ8K3QW7M2X9V4T6RCAA001")
	ix := indexOf(
		&Entry{ULID: a, Key: "WEB-42"},
		&Entry{ULID: d, Key: "WEB-42"},
		&Entry{ULID: mustULID(t, "01JBZ8K3QW7M2X9V4T6RDDD001"), Key: "API-1"},
	)
	var amb *AmbiguousError
	if _, err := ix.Resolve("WEB-42"); !errors.As(err, &amb) {
		t.Fatalf("Resolve(WEB-42) err = %v, want AmbiguousError", err)
	}
	if len(amb.Candidates) != 2 || amb.Candidates[0] != a || amb.Candidates[1] != d {
		t.Errorf("candidates = %v", amb.Candidates)
	}
}
