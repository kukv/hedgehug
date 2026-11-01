package sync

import (
	"os"
	"path/filepath"
	"testing"
)

// A marker with no old store to keep is not left inside the store the
// rebuild becomes.
func TestFinishLeftoverRemovesAMarkerWithoutAnOldStore(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "s")
	fresh := dir + ".rewrite-new"
	if err := os.MkdirAll(fresh, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fresh, "f"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fresh, staleMarker), []byte("stale\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	l, err := FindLeftover(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := FinishLeftover(dir, l); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "f")); err != nil || string(b) != "new" {
		t.Errorf("the store = %q, %v", b, err)
	}
	if _, err := os.Stat(filepath.Join(dir, staleMarker)); !os.IsNotExist(err) {
		t.Errorf("the marker is left in the store: %v", err)
	}
}

// A rebuild found stale whose old store could not be put back is not put in
// its place by FinishLeftover (that would drop the commit of the gap): the
// old store is, and the rebuild goes.
func TestFinishLeftoverKeepsTheOldStoreOfAStaleRebuild(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "s")
	before := dir + ".before-rewrite-1"
	fresh := dir + ".rewrite-new"
	for _, d := range []string{before, fresh} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "f"), []byte(filepath.Base(d)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(fresh, staleMarker), []byte("stale\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	l, err := FindLeftover(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := FinishLeftover(dir, l); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "f"))
	if err != nil || string(b) != "s.before-rewrite-1" {
		t.Errorf("the store = %q, %v; want the old store", b, err)
	}
	if left, _ := FindLeftover(dir); !left.None() {
		t.Errorf("leftover = %+v", left)
	}
}
