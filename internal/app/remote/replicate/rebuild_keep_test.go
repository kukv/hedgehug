package replicate

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/store"
)

// oldStoreWithMarker makes a directory that stands for an old store of a
// rebuild that dropped changes: a file and the keep marker.
func oldStoreWithMarker(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(p, filepath.FromSlash(store.LocalDir)), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "f"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, filepath.FromSlash(store.LocalDir), keepMarker), []byte("keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// An old store with the keep marker that a finished swap left behind is
// renamed to .kept-, not removed.
func TestFinishLeftoverRenamesAMarkedOldStoreToKept(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "s")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	before := dir + beforePrefix + "20260101T000000.000000000"
	oldStoreWithMarker(t, before)
	l, err := store.FindLeftover(dir)
	if err != nil {
		t.Fatal(err)
	}
	kept, err := FinishLeftoverKept(dir, l)
	want := dir + keptPrefix + "20260101T000000.000000000"
	if err != nil || !slices.Equal(kept, []string{want}) {
		t.Fatalf("FinishLeftoverKept = %v, %v; want %s", kept, err, want)
	}
	if b, err := os.ReadFile(filepath.Join(want, "f")); err != nil || string(b) != "old" {
		t.Errorf("the kept store = %q, %v", b, err)
	}
	if _, err := os.Stat(before); !os.IsNotExist(err) {
		t.Errorf("the old directory is still there: %v", err)
	}
	if l, err := store.FindLeftover(dir); err != nil || !l.None() {
		t.Errorf("leftover = %+v, %v", l, err)
	}
}

// When the marked old store cannot be renamed it is left in place, and the
// error names it; it is never removed.
func TestFinishLeftoverNeverRemovesAMarkedOldStore(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "s")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	before := dir + beforePrefix + "20260101T000000.000000000"
	oldStoreWithMarker(t, before)
	// a file where the kept directory would go, and not empty to rename onto
	blocked := dir + keptPrefix + "20260101T000000.000000000"
	if err := os.MkdirAll(filepath.Join(blocked, "x"), 0o750); err != nil {
		t.Fatal(err)
	}
	l, err := store.FindLeftover(dir)
	if err != nil {
		t.Fatal(err)
	}
	err = FinishLeftover(dir, l)
	if err == nil || !strings.Contains(err.Error(), before) {
		t.Fatalf("FinishLeftover = %v; want an error naming %s", err, before)
	}
	if b, err := os.ReadFile(filepath.Join(before, "f")); err != nil || string(b) != "old" {
		t.Errorf("the old store = %q, %v", b, err)
	}
}

// A swap that was found stale, whose marked old store could not be put back,
// is finished by putting that old store back: it is not kept, removed or
// replaced by the rebuild.
func TestFinishLeftoverPutsBackAMarkedOldStoreOfAStaleRebuild(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "s")
	before := dir + beforePrefix + "20260101T000000.000000000"
	oldStoreWithMarker(t, before)
	fresh := dir + newSuffix
	if err := os.MkdirAll(fresh, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fresh, staleMarker), []byte("stale\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	l, err := store.FindLeftover(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := FinishLeftover(dir, l); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "f")); err != nil || string(b) != "old" {
		t.Errorf("the store = %q, %v; want the old one back", b, err)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(store.LocalDir), keepMarker)); !os.IsNotExist(err) {
		t.Errorf("the keep marker is left in the store: %v", err)
	}
	if m, err := filepath.Glob(dir + ".*"); err != nil || len(m) != 0 {
		t.Errorf("something is left beside the store: %v, %v", m, err)
	}
}

// The swap of a rebuild that dropped changes leaves no marker in the new
// store, and the old one is at Kept.
func TestSwapLeavesNoKeepMarkerInTheNewStore(t *testing.T) {
	t.Parallel()
	c := newCluster(t, 2)
	sc := newRewriteScene(t, c, func(sc *rewriteScene) {
		c.set(1, sc.gone.ULID, func(x *model.Task) { x.Title = "gone by b" })
	})
	c.swap(sc.rb)
	if sc.rb.Kept == "" || len(sc.rb.Warnings) != 0 {
		t.Fatalf("Kept = %q, Warnings = %v", sc.rb.Kept, sc.rb.Warnings)
	}
	if _, err := os.Stat(filepath.Join(c.stores[1].Dir(), filepath.FromSlash(store.LocalDir), keepMarker)); !os.IsNotExist(err) {
		t.Errorf("the keep marker is in the new store: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sc.rb.Kept, filepath.FromSlash(store.LocalDir), keepMarker)); err != nil {
		t.Errorf("the kept store has no keep marker: %v", err)
	}
}

// A keep marker left in the live store by a swap that died before its first
// rename does not make a later swap that dropped nothing keep the old store.
// The removal of the old store is made to fail (its .hedgehug/local cannot be
// written, from the gap before the rename), so that it is a leftover; what
// finishes the leftover must remove it, not keep it.
func TestSwapIgnoresAStaleKeepMarker(t *testing.T) {
	// 直列: パッケージ変数 beforeRename の差し替え
	if os.Geteuid() == 0 {
		t.Skip("root removes what is read-only")
	}
	c := newCluster(t, 2)
	sc := newRewriteScene(t, c, func(*rewriteScene) {})
	dir := c.stores[1].Dir()
	local := filepath.Join(dir, filepath.FromSlash(store.LocalDir))
	if err := os.WriteFile(filepath.Join(local, keepMarker), []byte("stale\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if len(sc.rb.Purged) != 0 {
		t.Fatalf("setup: Purged = %v", sc.rb.Purged)
	}
	prev := beforeRename
	beforeRename = func() {
		if err := os.Chmod(local, 0o500); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(func() { beforeRename = prev })
	c.swap(sc.rb)
	if sc.rb.Kept != "" {
		t.Errorf("Kept = %q; the old store was kept because of a stale marker", sc.rb.Kept)
	}
	l, err := store.FindLeftover(dir)
	if err != nil || l.None() {
		t.Fatalf("setup: leftover = %+v, %v; the removal of the old store should have failed", l, err)
	}
	m, err := filepath.Glob(dir + beforePrefix + "*")
	if err != nil || len(m) != 1 {
		t.Fatalf("old stores = %v, %v", m, err)
	}
	oldLocal := filepath.Join(m[0], filepath.FromSlash(store.LocalDir))
	t.Cleanup(func() { _ = os.Chmod(oldLocal, 0o750) })
	if _, err := os.Stat(filepath.Join(oldLocal, keepMarker)); !os.IsNotExist(err) {
		t.Errorf("the old store carries the stale marker: %v", err)
	}
	if err := os.Chmod(oldLocal, 0o750); err != nil {
		t.Fatal(err)
	}
	kept, err := FinishLeftoverKept(dir, l)
	if err != nil || len(kept) != 0 {
		t.Errorf("FinishLeftoverKept = %v, %v; want the old store removed", kept, err)
	}
}
