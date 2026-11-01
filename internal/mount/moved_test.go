package mount

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

// movedPair is org-a's API-12, which moved to org-b as WEB-1 (it was API-12
// there too, as a former id), with org-a's file left as a forward.
func movedPair(t *testing.T) (a, b config.StoreEntry, task *model.Task) {
	t.Helper()
	a, b = newStoreAt(t, "org-a", "API"), newStoreAt(t, "org-b", "WEB")
	task = twelve(t, a)
	bs, err := store.Open(b.Path)
	if err != nil {
		t.Fatal(err)
	}
	putTasks(t, b, 1, func(_ int, nt *model.Task) {
		nt.ULID, nt.FormerIDs = task.ULID, []string{"API-12"}
	})
	as, err := store.Open(a.Path)
	if err != nil {
		t.Fatal(err)
	}
	fwd := "---\nulid: " + task.ULID.String() + "\nmoved_to: " + bs.Meta().ULID.String() + "\n---\n"
	if _, err := as.Update(store.Op{Name: "move-out", Author: testAuthor}, func(tx *store.Tx) error {
		return tx.PutFile(model.TaskPath(task.ULID), []byte(fwd))
	}); err != nil {
		t.Fatal(err)
	}
	return a, b, task
}

// A task found in another store wins over a forward; only an input that is
// no task anywhere names the move.
func TestResolveForward(t *testing.T) {
	t.Parallel()
	a, b, task := movedPair(t)
	set := openSet(t, a, b)
	for _, in := range []string{"API-12", "api-12", task.ULID.String()} {
		hit, err := set.Resolve(in)
		if err != nil || hit.Store.Name != "org-b" || hit.Entry.ID != "WEB-1" {
			t.Errorf("Resolve(%q) = %v, %v; want org-b's WEB-1", in, hit, err)
		}
	}
	for _, in := range []string{"org-a:API-12", "org-a:" + task.ULID.String()} {
		_, err := set.Resolve(in)
		var me *MovedError
		if !errors.As(err, &me) {
			t.Fatalf("Resolve(%q) = %v; want *MovedError", in, err)
		}
		if me.To == nil || me.To.Name != "org-b" || me.Entry == nil || me.NowID() != "WEB-1" {
			t.Errorf("Resolve(%q): To %v, Entry %v", in, me.To, me.Entry)
		}
		if want := "API-12 moved to org-b (org-b:WEB-1)"; err.Error() != want {
			t.Errorf("message = %q; want %q", err, want)
		}
		if !errors.Is(err, store.ErrNotFound) {
			t.Errorf("a *MovedError is not found for the callers that only look")
		}
	}
}

func TestResolveForwardOfAStoreNotMounted(t *testing.T) {
	t.Parallel()
	a, _, task := movedPair(t)
	set := openSet(t, a)
	for _, in := range []string{"API-12", task.ULID.String()} {
		_, err := set.Resolve(in)
		var me *MovedError
		if !errors.As(err, &me) {
			t.Fatalf("Resolve(%q) = %v; want *MovedError", in, err)
		}
		if me.To != nil || me.Entry != nil || me.NowID() != "" {
			t.Errorf("Resolve(%q): To %v, Entry %v; want none", in, me.To, me.Entry)
		}
		want := "API-12 moved to another store (ULID " + me.Forward.MovedTo.String() + "); mount that store to follow it"
		if err.Error() != want {
			t.Errorf("message = %q; want %q", err, want)
		}
	}
	if _, err := set.Resolve("API-99"); errors.As(err, new(*MovedError)) || !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Resolve(API-99) = %v; want plain not found", err)
	}
}
