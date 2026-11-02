package gitrepo

import (
	"errors"
	"fmt"
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

func looseHashes(t *testing.T, r *Repo) []plumbing.Hash {
	t.Helper()
	var hs []plumbing.Hash
	err := r.r.Storer.(storer.LooseObjectStorer).ForEachObjectHash(func(h plumbing.Hash) error {
		hs = append(hs, h)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return hs
}

// addPack commits once and moves the loose objects into a pack of their own,
// the way a fetch leaves one.
func addPack(t *testing.T, r *Repo, dir string, i int) {
	t.Helper()
	commitFiles(t, r, dir, map[string]string{fmt.Sprintf("f%d.md", i): fmt.Sprint(i)})
	hs := looseHashes(t, r)
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
		addPack(t, r, dir, i)
	}
	commitFiles(t, r, dir, map[string]string{"loose.md": "x"})
	if n, err := r.SmallPacks(); err != nil || n != 3 {
		t.Fatalf("SmallPacks = %d, %v", n, err)
	}
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

// The tests below replace package variables, so they are not parallel.

func TestConsolidateLeavesPacksFromPackCap(t *testing.T) {
	dir := t.TempDir()
	r, _ := Init(dir)
	for i := range 3 {
		addPack(t, r, dir, i)
	}
	before := packFiles(t, dir)
	packCap = 1 // every pack is "large" now
	t.Cleanup(func() { packCap = PackCap })
	if n, _ := r.SmallPacks(); n != 0 {
		t.Fatalf("SmallPacks = %d, want 0", n)
	}
	commitFiles(t, r, dir, map[string]string{"loose.md": "x"})
	if err := r.Consolidate(); err != nil {
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
				addPack(t, r, dir, i)
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
		addPack(t, r, dir, i)
	}
	commitFiles(t, r, dir, map[string]string{"a.md": "1"})
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
