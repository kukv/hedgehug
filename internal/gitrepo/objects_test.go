package gitrepo

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/format/idxfile"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

// makeHistory makes n commits from the i-th on that rewrite similar files, so
// that a pack of them holds deltas.
func makeHistory(t *testing.T, r *Repo, dir string, from, n int) {
	t.Helper()
	for i := from; i < from+n; i++ {
		commitFiles(t, r, dir, map[string]string{
			"tasks/a/task.md":                     strings.Repeat("a line of task a\n", 20) + fmt.Sprintf("version %d\n", i),
			fmt.Sprintf("tasks/t%d/task.md", i%5): strings.Repeat("the body of a task\n", 30) + fmt.Sprint(i),
		})
	}
}

// packAll writes the loose objects of r into one pack with deltas (REF
// deltas when refDeltas) and removes them. It returns the path of the pack.
func packAll(t *testing.T, r *Repo, refDeltas bool) string {
	t.Helper()
	los := r.r.Storer.(storer.LooseObjectStorer)
	var hs []plumbing.Hash
	if err := los.ForEachObjectHash(func(h plumbing.Hash) error {
		hs = append(hs, h)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(r.dir, ".git", "objects", "pack")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	f, err := os.CreateTemp(dir, "tmp_test_pack_*")
	if err != nil {
		t.Fatal(err)
	}
	sum, err := packfile.NewEncoder(f, r.r.Storer, refDeltas).Encode(hs, 10)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		t.Fatal(err)
	}
	idx, err := indexPack(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(dir, "pack-"+sum.String())
	if err := os.Rename(f.Name(), base+".pack"); err != nil {
		t.Fatal(err)
	}
	ix, err := os.Create(base + ".idx")
	if err != nil {
		t.Fatal(err)
	}
	_, err = idxfile.NewEncoder(ix).Encode(idx)
	if cerr := ix.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hs {
		if err := los.DeleteLooseObject(h); err != nil {
			t.Fatal(err)
		}
	}
	r.r.Storer.(interface{ Reindex() }).Reindex()
	return base + ".pack"
}

// deltas counts the OFS and REF deltas in the pack at path.
func deltas(t *testing.T, path string) (ofs, ref int) {
	t.Helper()
	f, err := os.Open(path) //nolint:gosec // G304: a pack the test wrote
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	s := packfile.NewScanner(f)
	_, n, err := s.Header()
	if err != nil {
		t.Fatal(err)
	}
	for range n {
		h, err := s.NextObjectHeader()
		if err != nil {
			t.Fatal(err)
		}
		switch h.Type {
		case plumbing.OFSDeltaObject:
			ofs++
		case plumbing.REFDeltaObject:
			ref++
		}
	}
	return ofs, ref
}

// goGitObjects returns every object of the repository at dir as go-git
// reads it, by hash.
func goGitObjects(t *testing.T, dir string) map[plumbing.Hash]plumbing.EncodedObject {
	t.Helper()
	s := filesystem.NewStorage(osfs.New(filepath.Join(dir, ".git")), cache.NewObjectLRUDefault())
	defer func() { _ = s.Close() }()
	it, err := s.IterEncodedObjects(plumbing.AnyObject)
	if err != nil {
		t.Fatal(err)
	}
	out := map[plumbing.Hash]plumbing.EncodedObject{}
	if err := it.ForEach(func(o plumbing.EncodedObject) error {
		out[o.Hash()] = o
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatal("the repository has no object")
	}
	return out
}

func content(t *testing.T, o plumbing.EncodedObject) []byte {
	t.Helper()
	rd, err := o.Reader()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rd.Close() }()
	b, err := io.ReadAll(rd)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// checkAll reads every object of the repository at dir with rd and compares
// it with what go-git reads.
func checkAll(t *testing.T, dir string, rd *objectReader) {
	t.Helper()
	for h, want := range goGitObjects(t, dir) {
		got, err := rd.object(h)
		if err != nil {
			t.Errorf("object(%s): %v", h, err)
			continue
		}
		if got.Type() != want.Type() || got.Hash() != h || !bytes.Equal(content(t, got), content(t, want)) {
			t.Errorf("object(%s) = %s %s, want %s", h, got.Type(), got.Hash(), want.Type())
		}
		if ok, err := rd.has(h); err != nil || !ok {
			t.Errorf("has(%s) = %v, %v", h, ok, err)
		}
	}
}

func newReader(t *testing.T, dir string) *objectReader {
	t.Helper()
	rd := newObjectReader(filepath.Join(dir, ".git"))
	t.Cleanup(func() { _ = rd.close() })
	return rd
}

func historyRepo(t *testing.T) (*Repo, string) {
	t.Helper()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	makeHistory(t, r, dir, 0, 40)
	return r, dir
}

func TestObjectReaderLoose(t *testing.T) {
	t.Parallel()
	_, dir := historyRepo(t)
	checkAll(t, dir, newReader(t, dir))
}

func TestObjectReaderOFSDeltas(t *testing.T) {
	t.Parallel()
	r, dir := historyRepo(t)
	if ofs, _ := deltas(t, packAll(t, r, false)); ofs == 0 {
		t.Fatal("the pack has no OFS delta")
	}
	checkAll(t, dir, newReader(t, dir))
}

func TestObjectReaderREFDeltas(t *testing.T) {
	t.Parallel()
	r, dir := historyRepo(t)
	if _, ref := deltas(t, packAll(t, r, true)); ref == 0 {
		t.Fatal("the pack has no REF delta")
	}
	checkAll(t, dir, newReader(t, dir))
}

// Two packs and loose objects.
func TestObjectReaderMixed(t *testing.T) {
	t.Parallel()
	r, dir := historyRepo(t)
	packAll(t, r, false)
	makeHistory(t, r, dir, 40, 20)
	packAll(t, r, true)
	makeHistory(t, r, dir, 60, 5)
	checkAll(t, dir, newReader(t, dir))
}

// A pack written after the reader listed the packs (a fetch) is found.
func TestObjectReaderSeesNewPack(t *testing.T) {
	t.Parallel()
	r, dir := historyRepo(t)
	packAll(t, r, false)
	rd := newReader(t, dir)
	if _, err := rd.object(mustHead(t, r)); err != nil {
		t.Fatal(err)
	}
	makeHistory(t, r, dir, 40, 10)
	packAll(t, r, false)
	checkAll(t, dir, rd)
}

func TestObjectReaderNotFound(t *testing.T) {
	t.Parallel()
	r, dir := historyRepo(t)
	packAll(t, r, false)
	rd := newReader(t, dir)
	h := plumbing.NewHash("0123456789012345678901234567890123456789")
	if _, err := rd.object(h); !errors.Is(err, plumbing.ErrObjectNotFound) {
		t.Errorf("object = %v, want ErrObjectNotFound", err)
	}
	if ok, err := rd.has(h); err != nil || ok {
		t.Errorf("has = %v, %v; want false", ok, err)
	}
}

// The server reads from many goroutines while the list of packs is read again.
func TestObjectReaderConcurrent(t *testing.T) {
	t.Parallel()
	r, dir := historyRepo(t)
	packAll(t, r, false)
	objs := goGitObjects(t, dir)
	rd := newReader(t, dir)
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for h := range objs {
				if _, err := rd.object(h); err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	wg.Go(func() {
		for range 50 {
			rd.stale()
		}
	})
	wg.Wait()
}

func TestBaseCacheEvicts(t *testing.T) {
	t.Parallel()
	c := newBaseCache(10)
	p := &pack{}
	c.put(p, 1, plumbing.BlobObject, []byte("12345"))
	c.put(p, 2, plumbing.BlobObject, []byte("12345"))
	if _, _, ok := c.get(p, 1); !ok { // 1 is now the newest
		t.Fatal("1 is not kept")
	}
	c.put(p, 3, plumbing.BlobObject, []byte("123"))
	if _, _, ok := c.get(p, 2); ok {
		t.Error("2, the oldest, is kept")
	}
	if typ, b, ok := c.get(p, 1); !ok || typ != plumbing.BlobObject || string(b) != "12345" {
		t.Errorf("get(1) = %s %q %v", typ, b, ok)
	}
	c.put(p, 4, plumbing.BlobObject, make([]byte, 11)) // larger than the whole cache
	if _, _, ok := c.get(p, 4); ok {
		t.Error("an object larger than the cache is kept")
	}
}
