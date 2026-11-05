package replicate

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/storage/store"
)

// crashedAfterFetch is clone 1 behind a force pushed origin, after a sync that
// died right after its fetch: the tracking ref sits on the new head of
// origin, and origin-before-fetch has the value the ref had before. An
// earlier sync had found the force push (force-push.json).
func crashedAfterFetch(t *testing.T) (c *cluster, old plumbing.Hash) {
	t.Helper()
	c = newCluster(t, 2)
	gone := c.addTask(0, "gone")
	c.addTask(0, "keep")
	c.sync(0)
	c.sync(1)
	c.addTask(1, "mine")
	c.rewriteOrigin(gone.ULID)
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrForcePush) {
		t.Fatalf("sync = %v, want ErrForcePush", err)
	}
	s := c.stores[1]
	old = trackOf(t, c)
	r := s.Repo()
	if err := r.SetRemotes(c.opts[1].Remotes); err != nil {
		t.Fatal(err)
	}
	if h, err := r.Fetch("origin"); err != nil || h == old {
		t.Fatalf("Fetch = %s, %v; want the new head", h, err)
	}
	if err := s.WriteLocal(fetchMarker, []byte(old.String()+"\n")); err != nil {
		t.Fatal(err)
	}
	return c, old
}

func markerExists(t *testing.T, c *cluster) bool {
	t.Helper()
	return exists(t, filepath.Join(c.stores[1].Dir(), filepath.FromSlash(store.LocalDir), fetchMarker))
}

// A sync that died after its fetch is not blind to the force push the fetch
// brought: the next one puts the tracking ref back first.
func TestSyncFindsTheForcePushAfterACrashedFetch(t *testing.T) {
	t.Parallel()
	c, old := crashedAfterFetch(t)
	if _, err := Run(c.stores[1], c.opts[1]); !errors.Is(err, ErrForcePush) {
		t.Fatalf("sync = %v, want ErrForcePush", err)
	}
	if got := trackOf(t, c); got != old {
		t.Errorf("the tracking ref = %s, want %s", got, old)
	}
	if markerExists(t, c) {
		t.Error("origin-before-fetch is left")
	}
}

// And the rebuild goes from the history the store followed, not from the one
// the fetch brought, which would look as if origin followed the store.
func TestRebuildAfterACrashedFetchStillRebuilds(t *testing.T) {
	t.Parallel()
	c, _ := crashedAfterFetch(t)
	b, err := RebuildStore(c.stores[1], c.opts[1])
	if err != nil || b == nil {
		t.Fatalf("RebuildStore = %+v, %v; want a rebuild", b, err)
	}
	if b.Replayed != 1 {
		t.Errorf("replayed = %d, want 1", b.Replayed)
	}
	if markerExists(t, c) {
		t.Error("origin-before-fetch is left")
	}
}

// A marker that does not parse is dropped, and the sync goes on from the
// tracking ref.
func TestAMarkerThatDoesNotParseIsDropped(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	c.addTask(0, "a")
	c.sync(0)
	if err := c.stores[1].WriteLocal(fetchMarker, []byte("not a hash\n")); err != nil {
		t.Fatal(err)
	}
	c.sync(1)
	if markerExists(t, c) {
		t.Error("origin-before-fetch is left")
	}
}

// A sync that fetches leaves no marker behind.
func TestASyncLeavesNoMarker(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	c.addTask(0, "a")
	c.sync(0)
	c.sync(1)
	if markerExists(t, c) {
		t.Error("origin-before-fetch is left")
	}
	if _, err := os.Stat(filepath.Join(c.stores[1].Dir(), filepath.FromSlash(store.LocalDir), fetchMarker)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("stat = %v", err)
	}
}
