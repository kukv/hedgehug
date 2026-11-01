package sync

import (
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/inbox"
	"github.com/kukv/hedgehug/internal/model"
)

// What changed in .hedgehug/local after the copy and before the swap is
// carried over, and the notices of the rebuild stay in the inbox.
func TestSwapCarriesLocalChangesMadeAfterTheCopy(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	sc := newRewriteScene(t, c, func(sc *rewriteScene) {
		c.set(1, sc.gone.ULID, func(x *model.Task) { x.Title = "gone by b" })
	})
	old := c.stores[1]
	if err := old.WriteLocal("note.json", []byte("new\n")); err != nil {
		t.Fatal(err)
	}
	later := model.NewULID()
	line := `{"ulid":"` + later.String() + `","kind":"watch","task":"` + sc.keep.ULID.String() + `","commit":"x","author":"","to":"github.com/u1","time":"2026-11-02T00:00:00Z"}` + "\n"
	cur, err := old.ReadLocal("inbox.jsonl")
	if err != nil && !strings.Contains(err.Error(), "not found") {
		t.Fatal(err)
	}
	if err := old.WriteLocal("inbox.jsonl", append(cur, line...)); err != nil {
		t.Fatal(err)
	}
	c.swap(sc.res)
	s := c.stores[1]
	if b, err := s.ReadLocal("note.json"); err != nil || string(b) != "new\n" {
		t.Errorf("note.json = %q, %v", b, err)
	}
	items, err := inbox.Open(s).List(true)
	if err != nil {
		t.Fatal(err)
	}
	var kinds []inbox.Kind
	for _, it := range items {
		kinds = append(kinds, it.Kind)
	}
	for _, want := range []inbox.Kind{inbox.RewriteApplied, inbox.PurgedChange, inbox.Watch} {
		if !slices.Contains(kinds, want) {
			t.Errorf("inbox kinds = %v, want %s", kinds, want)
		}
	}
	// the files the rebuild writes itself are the rebuild's
	applied, err := ReadApplied(s)
	if err != nil || len(applied) != 1 || !applied[0].Equal(sc.rec.At) {
		t.Errorf("applied = %v, %v", applied, err)
	}
}
