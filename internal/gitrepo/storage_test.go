package gitrepo

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage/filesystem"
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

// A command's first read does not load go-git's pack indexes: a pack index
// go-git cannot load (here, of another version) does not stop it.
func TestReadWithoutGoGitIndexes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	makeHistory(t, r, dir, 0, 10)
	head := mustHead(t, r)
	packAll(t, r, false)
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(dir, ".git", "objects", "pack", "pack-"+strings.Repeat("1", 40))
	if err := os.WriteFile(bad+".idx", append([]byte{0xff, 't', 'O', 'c', 0, 0, 0, 3}, make([]byte, 2000)...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad+".pack", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	g := filesystem.NewStorage(osfs.New(filepath.Join(dir, ".git")), cache.NewObjectLRUDefault())
	if _, err := g.EncodedObject(plumbing.CommitObject, head); err == nil {
		t.Fatal("go-git reads the commit despite the bad index; the test proves nothing")
	}
	_ = g.Close()
	r, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	if b, err := r.ReadAt(head, "tasks/a/task.md"); err != nil || !strings.Contains(string(b), "version 9") {
		t.Errorf("ReadAt = %q, %v", b, err)
	}
}

// After Consolidate moved the loose objects into a pack and closed the
// repository, every object is read from the new pack.
func TestReadAfterConsolidate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	makeHistory(t, r, dir, 0, 10)
	if _, err := r.HeadTree(); err != nil { // the reader lists the packs: none yet
		t.Fatal(err)
	}
	if err := r.Consolidate(); err != nil {
		t.Fatal(err)
	}
	checkAll(t, dir, r.r.Storer.(*cachedStorage).reader)
}

// An object the reader cannot find is asked of go-git, which does not find
// it either; an object of another type is not found. EncodedObjectSize is
// go-git's own.
func TestReadFallsBackToGoGit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	commitFiles(t, r, dir, map[string]string{"a.md": "1\n"})
	cs := r.r.Storer.(*cachedStorage)
	missing := plumbing.NewHash("0123456789012345678901234567890123456789")
	if _, err := cs.EncodedObject(plumbing.AnyObject, missing); !errors.Is(err, plumbing.ErrObjectNotFound) {
		t.Errorf("EncodedObject = %v, want ErrObjectNotFound", err)
	}
	if err := cs.HasEncodedObject(missing); !errors.Is(err, plumbing.ErrObjectNotFound) {
		t.Errorf("HasEncodedObject = %v, want ErrObjectNotFound", err)
	}
	blob := blobOf(t, r, "a.md")
	if n, err := cs.EncodedObjectSize(blob); err != nil || n != 2 {
		t.Errorf("EncodedObjectSize = %d, %v", n, err)
	}
	if _, err := cs.EncodedObject(plumbing.TreeObject, blob); !errors.Is(err, plumbing.ErrObjectNotFound) {
		t.Errorf("EncodedObject of a blob as a tree = %v, want ErrObjectNotFound", err)
	}
}
