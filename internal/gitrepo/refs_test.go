package gitrepo

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage"
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
