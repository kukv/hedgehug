package gitrepo

import (
	"errors"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

func TestPickGeometric(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		sizes []int64 // ascending
		want  int     // how many of the smallest to merge
	}{
		{nil, 0},
		{[]int64{10}, 0},
		{[]int64{10, 10, 10, 100}, 3}, // 10 < 2*10, 10 < 2*20: merge 3; 100 >= 2*30
		{[]int64{10, 25, 100}, 0},     // already geometric
		{[]int64{10, 15, 40, 50}, 4},  // 50 < 2*65
	} {
		if got := pickGeometric(c.sizes); got != c.want {
			t.Errorf("%v: %d, want %d", c.sizes, got, c.want)
		}
	}
}

func packFiles(t *testing.T, dir string) []string {
	t.Helper()
	m, err := filepath.Glob(filepath.Join(dir, ".git", "objects", "pack", "*"))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// putBlob writes a loose blob of n pseudo-random bytes (seeded by seed, so
// they do not compress and the same seed gives the same hash).
func putBlob(t *testing.T, r *Repo, seed int64, n int) plumbing.Hash {
	t.Helper()
	buf := make([]byte, n)
	_, _ = rand.New(rand.NewSource(seed)).Read(buf) //nolint:gosec // G404: test data
	obj := r.r.Storer.NewEncodedObject()
	obj.SetType(plumbing.BlobObject)
	w, _ := obj.Writer()
	_, _ = w.Write(buf)
	_ = w.Close()
	h, err := r.r.Storer.SetEncodedObject(obj)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// packOf moves the loose objects hs into a pack of their own, the way a
// fetch leaves one.
func packOf(t *testing.T, r *Repo, hs ...plumbing.Hash) {
	t.Helper()
	if _, err := r.writePack(hs); err != nil {
		t.Fatal(err)
	}
	r.r.Storer.(interface{ Reindex() }).Reindex()
	los := r.r.Storer.(storer.LooseObjectStorer)
	for _, h := range hs {
		if err := los.DeleteLooseObject(h); err != nil {
			t.Fatal(err)
		}
	}
}

// addPack adds a pack of one blob of 2000 bytes that do not compress, so
// the packs added this way are of about one size, whatever else changes.
func addPack(t *testing.T, r *Repo, i int) plumbing.Hash {
	t.Helper()
	h := putBlob(t, r, int64(i), 2000)
	packOf(t, r, h)
	return h
}

// packSizes returns the sizes of the packs the next Consolidate sees,
// smallest first.
func packSizes(t *testing.T, r *Repo) []int64 {
	t.Helper()
	ps, err := r.smallPacks(PackCap)
	if err != nil {
		t.Fatal(err)
	}
	var out []int64
	for _, p := range ps {
		out = append(out, p.size)
	}
	return out
}

// wantPick fails the test unless the geometric rule picks want of the packs,
// so that a change in pack sizes shows up as a broken premise.
func wantPick(t *testing.T, r *Repo, want int) []int64 {
	t.Helper()
	sz := packSizes(t, r)
	if k := pickGeometric(sz); k != want {
		t.Fatalf("the rule picks %d of the packs %v, want %d", k, sz, want)
	}
	return sz
}

func isLoose(dir string, h plumbing.Hash) bool {
	s := h.String()
	_, err := os.Stat(filepath.Join(dir, ".git", "objects", s[:2], s[2:]))
	return err == nil
}

func TestConsolidate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, _ := Init(dir)
	for i := range 3 {
		addPack(t, r, i)
	}
	commitFiles(t, r, dir, map[string]string{"loose.md": "x"})
	if n, err := r.SmallPacks(); err != nil || n != 3 {
		t.Fatalf("SmallPacks = %d, %v", n, err)
	}
	wantPick(t, r, 3)
	if err := r.Consolidate(); err != nil {
		t.Fatal(err)
	}
	if n, _ := r.SmallPacks(); n != 1 {
		t.Errorf("packs after Consolidate = %d, want 1", n)
	}
	if n, _ := r.LooseObjects(); n != 0 {
		t.Errorf("loose objects after Consolidate = %d", n)
	}
	if got := packFiles(t, dir); len(got) != 2 {
		t.Errorf("pack files = %v", got)
	}
	if err := r.Verify(); err != nil {
		t.Errorf("Verify: %v", err)
	}
	o, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := o.Verify(); err != nil {
		t.Errorf("Verify on a fresh open: %v", err)
	}
}

func TestConsolidateWritesReadOnlyPacks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, _ := Init(dir)
	commitFiles(t, r, dir, map[string]string{"a.md": "1"})
	if err := r.Consolidate(); err != nil {
		t.Fatal(err)
	}
	files := packFiles(t, dir)
	if len(files) != 2 {
		t.Fatalf("pack files = %v", files)
	}
	for _, p := range files {
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0o444 {
			t.Errorf("%s mode = %v, want 0444", filepath.Base(p), fi.Mode().Perm())
		}
	}
}

func TestConsolidateNothingToDo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, _ := Init(dir)
	if err := r.Consolidate(); err != nil {
		t.Fatal(err)
	}
	commitFiles(t, r, dir, map[string]string{"a.md": "1"})
	if err := r.Consolidate(); err != nil {
		t.Fatal(err)
	}
	before := packFiles(t, dir)
	if err := r.Consolidate(); err != nil {
		t.Fatal(err)
	}
	if after := packFiles(t, dir); !slices.Equal(before, after) {
		t.Errorf("packs = %v, want %v", after, before)
	}
}

func TestConsolidateKeepsObjectsNoReferenceReaches(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, _ := Init(dir)
	commitFiles(t, r, dir, map[string]string{"a.md": "1"})
	// A blob no reference reaches, as a stash or an aborted merge leaves.
	obj := r.r.Storer.NewEncodedObject()
	obj.SetType(plumbing.BlobObject)
	w, _ := obj.Writer()
	_, _ = w.Write([]byte("unreachable"))
	_ = w.Close()
	orphan, err := r.r.Storer.SetEncodedObject(obj)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Consolidate(); err != nil {
		t.Fatal(err)
	}
	commitFiles(t, r, dir, map[string]string{"a.md": "2"})
	// A blob only the index holds.
	writeFile(t, dir, "staged.md", "staged")
	wt, _ := r.r.Worktree()
	staged, err := wt.Add("staged.md")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Consolidate(); err != nil {
		t.Fatal(err)
	}
	if n, _ := r.LooseObjects(); n != 0 {
		t.Errorf("loose objects after Consolidate = %d", n)
	}
	// A fresh open reads only what is on disk.
	o, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range []plumbing.Hash{orphan, staged} {
		if _, err := o.r.Storer.EncodedObject(plumbing.BlobObject, h); err != nil {
			t.Errorf("object %s after Consolidate: %v", h, err)
		}
	}
	if err := o.Verify(); err != nil {
		t.Errorf("Verify after Consolidate: %v", err)
	}
	if st, err := o.Status(); err != nil || len(st) != 1 || st[0].Path != "staged.md" || !st[0].Staged {
		t.Errorf("status after Consolidate = %+v, %v", st, err)
	}
}

func TestConsolidateLeavesPacksFromPackCap(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, _ := Init(dir)
	for i := range 3 {
		addPack(t, r, i)
	}
	before := packFiles(t, dir)
	const packCap = 1 // every pack is "large" now
	if n, _ := r.SmallPacksBelow(packCap); n != 0 {
		t.Fatalf("SmallPacks = %d, want 0", n)
	}
	commitFiles(t, r, dir, map[string]string{"loose.md": "x"})
	if err := r.ConsolidateBelow(packCap); err != nil {
		t.Fatal(err)
	}
	after := packFiles(t, dir)
	for _, p := range before {
		if !slices.Contains(after, p) {
			t.Errorf("%s was removed", p)
		}
	}
	if len(after) != len(before)+2 {
		t.Errorf("pack files = %d, want %d", len(after), len(before)+2)
	}
	if n, _ := r.LooseObjects(); n != 0 {
		t.Errorf("loose objects = %d", n)
	}
	if err := r.Verify(); err != nil {
		t.Errorf("Verify: %v", err)
	}
}

func TestConsolidateFailureLeavesNoFiles(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, dir string)
	}{
		{"unreadable loose object", func(t *testing.T, dir string) {
			writeFile(t, dir, ".git/objects/ff/00000000000000000000000000000000000000", "not zlib")
		}},
		{"failure after the pack is written", func(t *testing.T, _ string) {
			afterPackWrite = func() error { return errors.New("injected") }
			t.Cleanup(func() { afterPackWrite = func() error { return nil } })
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			r, _ := Init(dir)
			for i := range 2 {
				addPack(t, r, i)
			}
			commitFiles(t, r, dir, map[string]string{"a.md": "1"})
			tc.setup(t, dir)
			packs := packFiles(t, dir)
			before, _ := r.LooseObjects()
			if err := r.Consolidate(); err == nil {
				t.Fatal("Consolidate succeeded")
			}
			if got := packFiles(t, dir); !slices.Equal(got, packs) {
				t.Errorf("files in objects/pack after a failed Consolidate: %v, want %v", got, packs)
			}
			if n, _ := r.LooseObjects(); n != before {
				t.Errorf("loose objects = %d, want %d", n, before)
			}
			if err := r.Verify(); err != nil {
				t.Errorf("Verify after a failed Consolidate: %v", err)
			}
		})
	}
}

func TestConsolidateKeepsObjectsWrittenMeanwhile(t *testing.T) {
	dir := t.TempDir()
	r, _ := Init(dir)
	for i := range 2 {
		addPack(t, r, i)
	}
	commitFiles(t, r, dir, map[string]string{"a.md": "1"})
	wantPick(t, r, 2)
	var late plumbing.Hash
	afterPackWrite = func() error {
		obj := r.r.Storer.NewEncodedObject()
		obj.SetType(plumbing.BlobObject)
		w, _ := obj.Writer()
		_, _ = w.Write([]byte("written during the consolidation"))
		_ = w.Close()
		h, err := r.r.Storer.SetEncodedObject(obj)
		late = h
		return err
	}
	t.Cleanup(func() { afterPackWrite = func() error { return nil } })
	if err := r.Consolidate(); err != nil {
		t.Fatal(err)
	}
	if late.IsZero() || !isLoose(dir, late) {
		t.Errorf("the object written meanwhile (%s) is not loose any more", late)
	}
	if n, _ := r.LooseObjects(); n != 1 {
		t.Errorf("loose objects = %d, want 1", n)
	}
	if n, _ := r.SmallPacks(); n != 1 {
		t.Errorf("packs = %d, want 1", n)
	}
}

func TestConsolidateLeavesTheBiggerPackOfMixedSizes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, _ := Init(dir)
	var small []plumbing.Hash
	for i := range 3 {
		small = append(small, addPack(t, r, i))
	}
	big := putBlob(t, r, 10, 40000)
	packOf(t, r, big)
	wantPick(t, r, 3) // 2000, 2000, 2000, 40000: the big one is over twice the rest
	before := packFiles(t, dir)
	if err := r.Consolidate(); err != nil {
		t.Fatal(err)
	}
	after := packFiles(t, dir)
	if len(after) != 4 { // the big pack and the new one, .pack and .idx each
		t.Fatalf("pack files = %v", after)
	}
	kept := 0
	for _, p := range before {
		if slices.Contains(after, p) {
			kept++
		}
	}
	if kept != 2 {
		t.Errorf("%d of the old files are left, want only the two of the big pack", kept)
	}
	o, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range append(small, big) {
		if _, err := o.r.Storer.EncodedObject(plumbing.BlobObject, h); err != nil {
			t.Errorf("object %s: %v", h, err)
		}
	}
}

// B holds everything A holds and one more object, so the pack the merge
// writes is B itself: it must stay, and A must go.
func TestConsolidateKeepsThePackThatEqualsTheNewOne(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, _ := Init(dir)
	x := addPack(t, r, 1)
	// The same blob again, loose, with one more, packed together.
	if h := putBlob(t, r, 1, 2000); h != x {
		t.Fatal("same seed, different hash")
	}
	y := putBlob(t, r, 2, 100)
	packOf(t, r, x, y)
	wantPick(t, r, 2)
	before := packFiles(t, dir)
	if err := r.Consolidate(); err != nil {
		t.Fatal(err)
	}
	after := packFiles(t, dir)
	if len(after) != 2 {
		t.Fatalf("pack files %v, want one pack (were %v)", after, before)
	}
	if !slices.Contains(before, after[0]) || !slices.Contains(before, after[1]) {
		t.Errorf("the pack left, %v, is not B of %v: the case was not reached", after, before)
	}
	o, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range []plumbing.Hash{x, y} {
		if _, err := o.r.Storer.EncodedObject(plumbing.BlobObject, h); err != nil {
			t.Errorf("object %s: %v", h, err)
		}
	}
}

func TestConsolidateMergesAboutPackCapAtMost(t *testing.T) {
	t.Parallel()
	three := func() (*Repo, string) {
		dir := t.TempDir()
		r, _ := Init(dir)
		for i := range 3 {
			addPack(t, r, i)
		}
		return r, dir
	}

	// two fit under the cap, three do not: two are merged
	r, dir := three()
	sz := wantPick(t, r, 3)
	packCap := sz[0] + sz[1] + 1
	if sz[2] >= packCap {
		t.Fatalf("a pack is over the cap: %v, %d", sz, packCap)
	}
	if err := r.ConsolidateBelow(packCap); err != nil {
		t.Fatal(err)
	}
	// counted by the files: the merged pack may come out at the cap or under it
	if n := len(slices.DeleteFunc(packFiles(t, dir), func(p string) bool { return filepath.Ext(p) != ".pack" })); n != 2 {
		t.Errorf("packs = %d, want 2 (one left, one new)", n)
	}
	if err := r.Verify(); err != nil {
		t.Errorf("Verify: %v", err)
	}

	// not even two fit under the cap: nothing is merged
	r, dir = three()
	sz = wantPick(t, r, 3)
	packCap = sz[2] + 1
	if sz[0]+sz[1] <= packCap {
		t.Fatalf("two packs fit under the cap: %v, %d", sz, packCap)
	}
	before := packFiles(t, dir)
	if err := r.ConsolidateBelow(packCap); err != nil {
		t.Fatal(err)
	}
	if after := packFiles(t, dir); !slices.Equal(before, after) {
		t.Errorf("packs = %v, want %v", after, before)
	}
}
