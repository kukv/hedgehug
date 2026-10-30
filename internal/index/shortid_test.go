package index

import (
	"errors"
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
	return ix
}

func TestShortID(t *testing.T) {
	a := mustID(t, "01JBZ8K3QW7M2X9V4T6RAAA001")
	b := mustID(t, "01JBZ8K3QW7M2X9V4T6RBBB001") // shares the last 4 with a
	c := mustID(t, "01JBZ8K3QW7M2X9V4T6R5N8P0C")
	ix := indexOf(&Entry{ID: a}, &Entry{ID: b}, &Entry{ID: c})
	for id, want := range map[model.ID]string{a: "A001", b: "B001", c: "8P0C"} {
		if got := ix.ShortID(id); got != want {
			t.Errorf("ShortID(%s) = %q, want %q", id, got, want)
		}
	}
	// With another task ending in AAA001, a needs one more character.
	d := mustID(t, "01JBZ8K3QW7M2X9V4T6RCAA001")
	ix = indexOf(&Entry{ID: a}, &Entry{ID: b}, &Entry{ID: d})
	if got := ix.ShortID(a); got != "AAA001" {
		t.Errorf("ShortID(a) = %q, want AAA001", got)
	}
	if got := indexOf(&Entry{ID: c}).ShortID(c); got != "8P0C" {
		t.Errorf("lone task: ShortID = %q, want 4 characters", got)
	}
}

func TestResolve(t *testing.T) {
	a := mustID(t, "01JBZ8K3QW7M2X9V4T6RAAA001")
	b := mustID(t, "01JBZ8K3QW7M2X9V4T6RBBB001")
	d := mustID(t, "01JBZ8K3QW7M2X9V4T6RCAA001") // shares AA001 with a
	ix := indexOf(
		&Entry{ID: a, Key: "WEB-42", FormerKeys: []string{"API-7"}},
		&Entry{ID: b, Key: "WEB-43"},
		&Entry{ID: d},
	)
	tests := []struct {
		in   string
		want model.ID
	}{
		{"WEB-42", a},
		{"web-42", a},
		{"api-7", a},
		{"01JBZ8K3QW7M2X9V4T6RBBB001", b},
		{"01jbz8k3qw7m2x9v4t6rbbb001", b},
		{"AAA001", a},
		{"bbb001", b},
		{"B001", b},
	}
	for _, tt := range tests {
		got, err := ix.Resolve(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("Resolve(%q) = %v, %v, want %v", tt.in, got, err, tt.want)
		}
	}

	var amb *AmbiguousError
	if _, err := ix.Resolve("001"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Resolve(too short) err = %v, want ErrNotFound", err)
	}
	if _, err := ix.Resolve("A001"); !errors.As(err, &amb) {
		t.Errorf("Resolve(A001) err = %v, want AmbiguousError", err)
	} else if len(amb.Candidates) != 2 || amb.Candidates[0] != a || amb.Candidates[1] != d {
		t.Errorf("candidates = %v", amb.Candidates)
	}
	if _, err := ix.Resolve("ZZZZ"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Resolve(ZZZZ) err = %v, want ErrNotFound", err)
	}
	if _, err := ix.Resolve(""); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Resolve(empty) err = %v, want ErrNotFound", err)
	}
}
