package gitrepo

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage"
	"github.com/go-git/go-git/v5/storage/filesystem/dotgit"
)

func TestSetReferenceKeepsOnlyTheReferenceFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	h1 := plumbing.NewHash("1111111111111111111111111111111111111111")
	h2 := plumbing.NewHash("2222222222222222222222222222222222222222")
	name := plumbing.NewBranchReferenceName("topic/a")
	if err := r.r.Storer.SetReference(plumbing.NewHashReference(name, h1)); err != nil {
		t.Fatal(err)
	}
	// The old value must match for CheckAndSetReference to write.
	err = r.r.Storer.CheckAndSetReference(plumbing.NewHashReference(name, h2), plumbing.NewHashReference(name, h2))
	if !errors.Is(err, storage.ErrReferenceHasChanged) {
		t.Errorf("CheckAndSetReference with a stale old = %v; want ErrReferenceHasChanged", err)
	}
	if err := r.r.Storer.CheckAndSetReference(plumbing.NewHashReference(name, h2), plumbing.NewHashReference(name, h1)); err != nil {
		t.Fatal(err)
	}
	got, err := r.r.Storer.Reference(name)
	if err != nil || got.Hash() != h2 {
		t.Errorf("Reference = %v, %v; want %s", got, err, h2)
	}
	if b, err := os.ReadFile(filepath.Join(dir, ".git", "refs", "heads", "topic", "a")); err != nil || string(b) != h2.String()+"\n" {
		t.Errorf("ref file = %q, %v", b, err)
	}
	left, err := filepath.Glob(filepath.Join(dir, ".git", "hh-ref-*"))
	if err != nil || len(left) != 0 {
		t.Errorf("temporary files left: %v, %v", left, err)
	}
}

func TestSetReferenceRejectsNamesOutsideRefs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	h := plumbing.NewHash("1111111111111111111111111111111111111111")
	for _, n := range []string{"hooks/x", "refs/../hooks/x", "config", "refs/heads/a\x01b", "refs/heads/a\x7fb", "refs/heads/.\xe2\x80\x8c.", "refs/heads/. "} {
		err := r.r.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(n), h))
		if !errors.Is(err, dotgit.ErrReferenceNameEscape) {
			t.Errorf("SetReference(%q) = %v; want ErrReferenceNameEscape", n, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "hooks", "x")); !os.IsNotExist(err) {
		t.Errorf("hooks/x was written: %v", err)
	}
	// An upper-case pseudo-reference is allowed.
	if err := r.r.Storer.SetReference(plumbing.NewHashReference("FETCH_HEAD", h)); err != nil {
		t.Errorf("SetReference(FETCH_HEAD) = %v", err)
	}
}

func TestSetReferenceWritesSymbolicReference(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	target := plumbing.NewBranchReferenceName("topic/a")
	if err := r.r.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, target)); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, ".git", "HEAD")); err != nil || string(b) != "ref: refs/heads/topic/a\n" {
		t.Errorf("HEAD file = %q, %v", b, err)
	}
	got, err := r.r.Storer.Reference(plumbing.HEAD)
	if err != nil || got.Type() != plumbing.SymbolicReference || got.Target() != target {
		t.Errorf("Reference(HEAD) = %v, %v", got, err)
	}
}
