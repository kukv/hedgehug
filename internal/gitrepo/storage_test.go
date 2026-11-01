package gitrepo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

// removeLoose removes the loose object h from the git directory gitDir.
func removeLoose(t *testing.T, gitDir string, h plumbing.Hash) {
	t.Helper()
	s := h.String()
	if err := os.Remove(filepath.Join(gitDir, "objects", s[:2], s[2:])); err != nil {
		t.Fatal(err)
	}
}

// blobOf returns the hash of the blob at path in the tree of HEAD.
func blobOf(t *testing.T, r *Repo, path string) plumbing.Hash {
	t.Helper()
	c, err := r.r.CommitObject(mustHead(t, r))
	if err != nil {
		t.Fatal(err)
	}
	f, err := c.File(path)
	if err != nil {
		t.Fatal(err)
	}
	return f.Hash
}

func mustHead(t *testing.T, r *Repo) plumbing.Hash {
	t.Helper()
	h, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// An object read once is served from memory: on Windows every open of an
// object file costs tens of microseconds, so go-git's opening the file before
// it looks in its cache made reads slow.
func TestReadObjectsFromMemoryOnceRead(t *testing.T) {
	t.Parallel()
	check := func(t *testing.T, r *Repo, dir string) {
		t.Helper()
		h := commitHead(t, r, dir, map[string]string{"a.md": "1\n"})
		if b, err := r.ReadAt(h, "a.md"); err != nil || string(b) != "1\n" {
			t.Fatalf("ReadAt = %q, %v", b, err)
		}
		removeLoose(t, filepath.Join(dir, ".git"), blobOf(t, r, "a.md"))
		if b, err := r.ReadAt(h, "a.md"); err != nil || string(b) != "1\n" {
			t.Errorf("ReadAt after the file is gone = %q, %v", b, err)
		}
	}
	t.Run("Init", func(t *testing.T) {
		dir := t.TempDir()
		r, err := Init(dir)
		if err != nil {
			t.Fatal(err)
		}
		check(t, r, dir)
	})
	t.Run("Open", func(t *testing.T) {
		dir := t.TempDir()
		if _, err := Init(dir); err != nil {
			t.Fatal(err)
		}
		r, err := Open(dir)
		if err != nil {
			t.Fatal(err)
		}
		check(t, r, dir)
	})
	t.Run("Clone", func(t *testing.T) {
		a, _ := cloneTwo(t, newBare(t))
		check(t, a.r, a.dir)
	})
}

// The remote side of a push or fetch reads from memory too.
func TestFileLoaderReadsObjectsFromMemoryOnceRead(t *testing.T) {
	t.Parallel()
	remote := newBare(t)
	a, _ := cloneTwo(t, remote)
	commitHead(t, a.r, a.dir, map[string]string{"a.md": "1\n"})
	if err := a.r.Push("origin"); err != nil {
		t.Fatal(err)
	}
	blob := blobOf(t, a.r, "a.md")
	ep, err := transport.NewEndpoint(remote)
	if err != nil {
		t.Fatal(err)
	}
	s, err := fileLoader{}.Load(ep)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.EncodedObject(plumbing.BlobObject, blob); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(remote, "objects")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EncodedObject(plumbing.BlobObject, blob); err != nil {
		t.Errorf("EncodedObject after the files are gone = %v", err)
	}
}
