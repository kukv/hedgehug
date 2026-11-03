package gitrepo

import (
	"bufio"
	"compress/zlib"
	"container/list"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
)

// baseCacheBytes is how much of the objects that deltas were made from is
// kept in memory.
const baseCacheBytes = 16 << 20

// maxDeltaDepth bounds a chain of deltas, so that a corrupt pack whose deltas
// point at each other ends in an error. git's own limit is 50.
const maxDeltaDepth = 64

// objectReader reads the objects of a git directory, loose and packed. It
// opens a pack's index without reading all of it and never needs the map
// from offsets to hashes that go-git builds on its first read (spec
// gogit-fixed-cost 2). It is safe for concurrent use.
type objectReader struct {
	dir   string // the objects directory
	bases *baseCache

	mu      sync.Mutex
	packs   []*pack
	listed  bool    // packs is the list of the directory, unless stale was called
	retired []*pack // packs gone from the directory; closed by close, as a read may be in one
}

func newObjectReader(gitDir string) *objectReader {
	return &objectReader{dir: filepath.Join(gitDir, "objects"), bases: newBaseCache(baseCacheBytes)}
}

// object returns the object h, or plumbing.ErrObjectNotFound.
func (o *objectReader) object(h plumbing.Hash) (plumbing.EncodedObject, error) {
	typ, data, err := o.raw(h, false, 0)
	if err != nil {
		return nil, err
	}
	obj := &plumbing.MemoryObject{}
	obj.SetType(typ)
	obj.SetSize(int64(len(data)))
	if _, err := obj.Write(data); err != nil {
		return nil, err
	}
	return obj, nil
}

// raw returns the type and the content of h. It looks in the packs, then in
// the loose objects, then in the packs again after listing them anew: a
// fetch or a consolidation may have written a pack since they were listed.
// The returned slice may be shared with the base cache; do not modify it.
// With asBase an object found in a pack is kept in the base cache; depth is
// the length of the chain of deltas so far.
func (o *objectReader) raw(h plumbing.Hash, asBase bool, depth int) (plumbing.ObjectType, []byte, error) {
	for _, again := range []bool{false, true} {
		p, off, err := o.find(h, again)
		if err != nil {
			return plumbing.InvalidObject, nil, err
		}
		if p != nil {
			return o.at(p, off, asBase, depth)
		}
		if !again {
			typ, data, err := readLoose(o.dir, h)
			if !errors.Is(err, fs.ErrNotExist) {
				return typ, data, err
			}
		}
	}
	return plumbing.InvalidObject, nil, plumbing.ErrObjectNotFound
}

// has reports whether h is in the repository, as raw looks for it.
func (o *objectReader) has(h plumbing.Hash) (bool, error) {
	for _, again := range []bool{false, true} {
		p, _, err := o.find(h, again)
		if err != nil || p != nil {
			return p != nil, err
		}
		if !again {
			_, err := os.Stat(loosePath(o.dir, h))
			if err == nil {
				return true, nil
			}
			if !errors.Is(err, fs.ErrNotExist) {
				return false, err
			}
		}
	}
	return false, nil
}

// find returns the pack that holds h and its offset there, or a nil pack.
// With again it lists the packs anew first.
func (o *objectReader) find(h plumbing.Hash, again bool) (*pack, int64, error) {
	packs, err := o.list(again)
	if err != nil {
		return nil, 0, err
	}
	for _, p := range packs {
		off, ok, err := p.find(h)
		if err != nil {
			return nil, 0, err
		}
		if ok {
			return p, off, nil
		}
	}
	return nil, 0, nil
}

// list returns the packs of the directory. It reads the directory when
// again, when it never did, or after stale. Packs still there stay open;
// packs gone are retired.
func (o *objectReader) list(again bool) ([]*pack, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.listed && !again {
		return o.packs, nil
	}
	des, err := os.ReadDir(filepath.Join(o.dir, "pack"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	open := make(map[string]*pack, len(o.packs))
	for _, p := range o.packs {
		open[p.base] = p
	}
	var packs []*pack
	for _, de := range des {
		name := de.Name()
		if !strings.HasPrefix(name, "pack-") || !strings.HasSuffix(name, ".idx") {
			continue
		}
		base := filepath.Join(o.dir, "pack", strings.TrimSuffix(name, ".idx"))
		if p, ok := open[base]; ok {
			packs = append(packs, p)
			delete(open, base)
			continue
		}
		p, err := openPack(base)
		switch {
		case errors.Is(err, errSkipPack), errors.Is(err, fs.ErrNotExist):
			continue // go-git reads it; or it was removed meanwhile
		case err != nil:
			return nil, err
		}
		packs = append(packs, p)
	}
	for _, p := range open {
		o.retired = append(o.retired, p)
	}
	o.packs, o.listed = packs, true
	return packs, nil
}

// stale makes the next read list the packs anew.
func (o *objectReader) stale() {
	o.mu.Lock()
	o.listed = false
	o.mu.Unlock()
}

// close closes every pack. The reader lists and opens them again on its next read.
func (o *objectReader) close() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	var errs []error
	for _, p := range append(o.packs, o.retired...) {
		errs = append(errs, p.close())
	}
	o.packs, o.retired, o.listed = nil, nil, false
	o.bases.clear()
	return errors.Join(errs...)
}

// at returns the object at off in p, its delta applied. An object read as
// the base of a delta is kept in the base cache. The returned slice may be
// shared with the base cache; do not modify it.
func (o *objectReader) at(p *pack, off int64, asBase bool, depth int) (plumbing.ObjectType, []byte, error) {
	if depth > maxDeltaDepth {
		return plumbing.InvalidObject, nil, fmt.Errorf("%s.pack at %d: deltas nested deeper than %d", p.base, off, maxDeltaDepth)
	}
	if typ, data, ok := o.bases.get(p, off); ok {
		return typ, data, nil
	}
	ob := objectBufs.Get().(*objectBuf)
	e, rest, err := p.entryAt(off, ob.ahead[:])
	var data []byte
	if err == nil {
		data, err = p.inflate(e, rest, ob)
	}
	objectBufs.Put(ob)
	if err != nil {
		return plumbing.InvalidObject, nil, err
	}
	typ := e.typ
	switch e.typ {
	case plumbing.CommitObject, plumbing.TreeObject, plumbing.BlobObject, plumbing.TagObject:
	case plumbing.OFSDeltaObject, plumbing.REFDeltaObject:
		var base []byte
		if e.typ == plumbing.OFSDeltaObject {
			typ, base, err = o.at(p, e.baseOff, true, depth+1)
		} else {
			typ, base, err = o.raw(e.baseRef, true, depth+1)
			if err != nil {
				err = fmt.Errorf("%s.pack at %d: delta base %s: %w", p.base, off, e.baseRef, err)
			}
		}
		if err != nil {
			return plumbing.InvalidObject, nil, err
		}
		if data, err = packfile.PatchDelta(base, data); err != nil {
			return plumbing.InvalidObject, nil, fmt.Errorf("%s.pack at %d: %w", p.base, off, err)
		}
	default:
		return plumbing.InvalidObject, nil, fmt.Errorf("%s.pack at %d: object type %d", p.base, off, e.typ)
	}
	if asBase {
		o.bases.put(p, off, typ, data)
	}
	return typ, data, nil
}

func loosePath(dir string, h plumbing.Hash) string {
	s := h.String()
	return filepath.Join(dir, s[:2], s[2:])
}

// readLoose reads the loose object h; an error wrapping fs.ErrNotExist means
// there is none.
func readLoose(dir string, h plumbing.Hash) (plumbing.ObjectType, []byte, error) {
	f, err := os.Open(loosePath(dir, h)) //nolint:gosec // G304: an object file of the repository
	if err != nil {
		return plumbing.InvalidObject, nil, err
	}
	defer func() { _ = f.Close() }()
	zr, err := zlib.NewReader(bufio.NewReader(f))
	if err != nil {
		return plumbing.InvalidObject, nil, fmt.Errorf("loose object %s: %w", h, err)
	}
	defer func() { _ = zr.Close() }()
	br := bufio.NewReader(zr)
	head, err := br.ReadString(0)
	if err != nil {
		return plumbing.InvalidObject, nil, fmt.Errorf("loose object %s: %w", h, err)
	}
	kind, size, _ := strings.Cut(strings.TrimSuffix(head, "\x00"), " ")
	typ, err := plumbing.ParseObjectType(kind)
	if err != nil {
		return plumbing.InvalidObject, nil, fmt.Errorf("loose object %s: %w", h, err)
	}
	n, err := strconv.ParseInt(size, 10, 64)
	if err != nil || n < 0 {
		return plumbing.InvalidObject, nil, fmt.Errorf("loose object %s: bad size %q", h, size)
	}
	data, err := readSized(br, n)
	if err != nil {
		return plumbing.InvalidObject, nil, fmt.Errorf("loose object %s: %w", h, err)
	}
	return typ, data, nil
}

// baseCache keeps the objects deltas were made from, by pack and offset, so
// that deltas with the same bases do not inflate them again. It holds up to
// max bytes, dropping the least recently used.
type baseCache struct {
	mu        sync.Mutex
	max, size int
	ll        *list.List // of *baseItem, the most recent first
	items     map[baseKey]*list.Element
}

type baseKey struct {
	p   *pack
	off int64
}

type baseItem struct {
	key  baseKey
	typ  plumbing.ObjectType
	data []byte
}

func newBaseCache(maxBytes int) *baseCache {
	return &baseCache{max: maxBytes, ll: list.New(), items: map[baseKey]*list.Element{}}
}

func (c *baseCache) get(p *pack, off int64) (plumbing.ObjectType, []byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[baseKey{p, off}]
	if !ok {
		return plumbing.InvalidObject, nil, false
	}
	c.ll.MoveToFront(el)
	it := el.Value.(*baseItem)
	return it.typ, it.data, true
}

func (c *baseCache) put(p *pack, off int64, typ plumbing.ObjectType, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	k := baseKey{p, off}
	if len(data) > c.max || c.items[k] != nil {
		return
	}
	c.items[k] = c.ll.PushFront(&baseItem{key: k, typ: typ, data: data})
	c.size += len(data)
	for c.size > c.max {
		el := c.ll.Back()
		it := el.Value.(*baseItem)
		c.ll.Remove(el)
		delete(c.items, it.key)
		c.size -= len(it.data)
	}
}

func (c *baseCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ll.Init()
	c.items = map[baseKey]*list.Element{}
	c.size = 0
}
