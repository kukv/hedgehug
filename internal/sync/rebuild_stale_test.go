package sync

import (
	"os"
	"path/filepath"
	"testing"
)

// A rebuild found stale whose old store could not be put back is not put in
// its place by FinishLeftover (that would drop the commit of the gap): the
// old store is, and the rebuild goes.
func TestFinishLeftoverKeepsTheOldStoreOfAStaleRebuild(t *testing.T) {
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
