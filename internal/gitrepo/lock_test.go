package gitrepo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
)

func TestCommitWaitsForIndexLock(t *testing.T) {
	dir := t.TempDir()
	r, _ := Init(dir)
	lock := filepath.Join(dir, ".git", "index.lock")
	if err := os.WriteFile(lock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	indexLockWait = 50 * time.Millisecond
	t.Cleanup(func() { indexLockWait = 2 * time.Second })
	writeFile(t, dir, "a.md", "a\n")
	if _, err := r.Commit(Commit{Paths: []string{"a.md"}, Subject: "s", Author: Signature{"a", "a@x"}, When: time.Now()}); !errors.Is(err, ErrGitBusy) {
		t.Errorf("Commit with index.lock = %v", err)
	}
	if _, err := os.Stat(lock); err != nil {
		t.Errorf("the index.lock of another process is gone: %v", err)
	}
	_ = os.Remove(lock)
	if _, err := r.Commit(Commit{Paths: []string{"a.md"}, Subject: "s", Author: Signature{"a", "a@x"}, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(lock); !errors.Is(err, fs.ErrNotExist) {
		t.Error("index.lock is left after the commit")
	}
}

func TestCommitTakesTheLockWhenItIsReleased(t *testing.T) {
	dir := t.TempDir()
	r, _ := Init(dir)
	lock := filepath.Join(dir, ".git", "index.lock")
	if err := os.WriteFile(lock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		time.Sleep(150 * time.Millisecond)
		_ = os.Remove(lock)
	}()
	writeFile(t, dir, "a.md", "a\n")
	_, err := r.Commit(Commit{Paths: []string{"a.md"}, Subject: "s", Author: author, When: time.Now()})
	<-done
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(lock); !errors.Is(err, fs.ErrNotExist) {
		t.Error("index.lock is left after the commit")
	}
}

func TestFastForwardWaitsForIndexLock(t *testing.T) {
	remote := newBare(t)
	a, b := cloneTwo(t, remote)
	commitHead(t, a.r, a.dir, map[string]string{"x.md": "a\n"})
	_ = a.r.Push("origin")
	theirs, _ := b.r.Fetch("origin")
	before, _ := b.r.Head()
	lock := filepath.Join(b.dir, ".git", "index.lock")
	if err := os.WriteFile(lock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	indexLockWait = 50 * time.Millisecond
	t.Cleanup(func() { indexLockWait = 2 * time.Second })
	if err := b.r.FastForward(theirs, []string{"x.md"}); !errors.Is(err, ErrGitBusy) {
		t.Errorf("FastForward with index.lock = %v", err)
	}
	if got, _ := b.r.Head(); got != before {
		t.Errorf("HEAD moved to %s", got)
	}
	if _, err := os.Stat(filepath.Join(b.dir, "x.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("x.md was written: %v", err)
	}
	if _, err := os.Stat(lock); err != nil {
		t.Errorf("the index.lock of another process is gone: %v", err)
	}
}

func TestRepack(t *testing.T) {
	dir := t.TempDir()
	r, _ := Init(dir)
	for i := range 5 {
		commitFiles(t, r, dir, map[string]string{"a.md": fmt.Sprint(i)})
	}
	n, err := r.LooseObjects()
	if err != nil || n == 0 {
		t.Fatalf("LooseObjects = %d, %v", n, err)
	}
	if err := r.Repack(); err != nil {
		t.Fatal(err)
	}
	if n, _ := r.LooseObjects(); n != 0 {
		t.Errorf("loose objects after repack = %d", n)
	}
	if err := r.Verify(); err != nil {
		t.Errorf("Verify after repack: %v", err)
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

func TestRepackKeepsPacksAndObjectsNoReferenceReaches(t *testing.T) {
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
	if err := r.Repack(); err != nil {
		t.Fatal(err)
	}
	first := packFiles(t, dir)
	if len(first) == 0 {
		t.Fatal("no pack after the first repack")
	}

	commitFiles(t, r, dir, map[string]string{"a.md": "2"})
	// A blob only the index holds.
	writeFile(t, dir, "staged.md", "staged")
	wt, _ := r.r.Worktree()
	staged, err := wt.Add("staged.md")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Repack(); err != nil {
		t.Fatal(err)
	}
	if n, _ := r.LooseObjects(); n != 0 {
		t.Errorf("loose objects after repack = %d", n)
	}
	after := packFiles(t, dir)
	for _, p := range first {
		if !slices.Contains(after, p) {
			t.Errorf("%s was removed", p)
		}
	}
	// A fresh open reads only what is on disk.
	o, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range []plumbing.Hash{orphan, staged} {
		if _, err := o.r.Storer.EncodedObject(plumbing.BlobObject, h); err != nil {
			t.Errorf("object %s after repack: %v", h, err)
		}
	}
	if err := o.Verify(); err != nil {
		t.Errorf("Verify after repack: %v", err)
	}
	if st, err := o.Status(); err != nil || len(st) != 1 || st[0].Path != "staged.md" || !st[0].Staged {
		t.Errorf("status after repack = %+v, %v", st, err)
	}
}

func TestRepackFailureLeavesNoFiles(t *testing.T) {
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
			commitFiles(t, r, dir, map[string]string{"a.md": "1"})
			tc.setup(t, dir)
			before, _ := r.LooseObjects()
			if err := r.Repack(); err == nil {
				t.Fatal("Repack succeeded")
			}
			if got := packFiles(t, dir); len(got) != 0 {
				t.Errorf("files in objects/pack after a failed repack: %v", got)
			}
			if n, _ := r.LooseObjects(); n != before {
				t.Errorf("loose objects = %d, want %d", n, before)
			}
			if err := r.Verify(); err != nil {
				t.Errorf("Verify after a failed repack: %v", err)
			}
		})
	}
}

func TestRepackWritesReadOnlyPacks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no permission bits for others")
	}
	dir := t.TempDir()
	r, _ := Init(dir)
	commitFiles(t, r, dir, map[string]string{"a.md": "1"})
	if err := r.Repack(); err != nil {
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

func TestRepackNothingLoose(t *testing.T) {
	dir := t.TempDir()
	r, _ := Init(dir)
	commitFiles(t, r, dir, map[string]string{"a.md": "1"})
	if err := r.Repack(); err != nil {
		t.Fatal(err)
	}
	before := packFiles(t, dir)
	if err := r.Repack(); err != nil {
		t.Fatal(err)
	}
	if after := packFiles(t, dir); !slices.Equal(before, after) {
		t.Errorf("packs = %v, want %v", after, before)
	}
}
