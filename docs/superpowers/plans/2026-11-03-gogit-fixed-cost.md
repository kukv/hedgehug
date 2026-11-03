# go-git の固定費 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** WSL でも `show`（HEAD が 1 つ進んだ直後）と `undo` を 200ms 以内にする。新しいプロセスが最初にオブジェクトを読むときの go-git の固定費と、commit のたびに tree をすべて組み立て直す費用をなくす。

**Architecture:** `internal/gitrepo` に自前のオブジェクトの読み手（`objectReader`）を置き、`cachedStorage` の `EncodedObject`・`HasEncodedObject`・`EncodedObjectSize` をこれで答える。`.idx` は fanout だけを読み、引くときに必要な範囲だけを `ReadAt` で読むので、go-git の `.idx` の全読み込みと逆引き表（`genOffsetHash`）が起きない。`Repo.Commit` は `Worktree.Commit` をやめ、HEAD の tree を基に、変わったパスの上にある tree だけを組み立てる。

**Tech Stack:** Go 1.27、go-git v5.19.3（`plumbing`、`plumbing/format/packfile` の `PatchDelta`、`plumbing/format/idxfile`、`plumbing/format/index`、`plumbing/object`）

**Spec:** `docs/superpowers/specs/2026-11-03-gogit-fixed-cost-design.md`

---

## ファイルの構成

| ファイル | 役目 |
| --- | --- |
| `internal/gitrepo/pack.go`（新規） | 1 つの pack と `.idx`（版 2）。fanout だけを読んで開き、hash からオフセットを引き、オフセットからオブジェクトの頭と zlib の中身を読む |
| `internal/gitrepo/pack_test.go`（新規） | 64 ビットのオフセットを持つ `.idx` を手で作って引く |
| `internal/gitrepo/objects.go`（新規） | `objectReader`: pack の一覧、loose、delta の解決、見つからないときの一覧の読み直し、閉じる。delta の基の LRU（`baseCache`） |
| `internal/gitrepo/objects_test.go`（新規） | go-git と全オブジェクトを比べる（loose、OFS delta、REF delta、混在、新しい pack、並行） |
| `internal/gitrepo/storage.go` | `cachedStorage` に読み手を持たせ、3 つのメソッドと `Reindex`・`Close` を足す |
| `internal/gitrepo/storage_test.go` | go-git が読めない `.idx` があっても読めること、統合の後も読めること |
| `internal/gitrepo/repo.go` | `newRepo` で読み手を付ける |
| `internal/gitrepo/commit_tree.go`（新規） | 変わった tree だけを組み立てて commit し、HEAD を進める |
| `internal/gitrepo/commit_tree_test.go`（新規） | 作った tree を go-git の `Worktree.Commit` の tree と比べる |
| `internal/gitrepo/commit.go` | `Repo.Commit` から `Worktree.Commit` を外し、`commitStaged` を呼ぶ |
| `internal/history/historytest/synth.go` | pack を window 10 で書き、`layout` を 3 にする |
| `docs/superpowers/specs/2026-11-03-gogit-fixed-cost-design.md` | 6 節に計測の結果を書く |
| `docs/superpowers/specs/2026-11-03-history-growth-design.md` | 8 節の WSL の未達の 2 項目に、本 spec を指す一文を足す |

lint は `golangci-lint`（gosec を含む）。変数のパスで `os.Open` するところには、既存のコードと同じく `//nolint:gosec // G304: ...` を付ける。テストは `go test -race ./internal/gitrepo/...` で流す。

---

### Task 1: pack と `.idx` を読む（`pack.go`）

**Files:**
- Create: `internal/gitrepo/pack.go`
- Test: `internal/gitrepo/pack_test.go`

- [ ] **Step 1: 失敗するテストを書く**

`internal/gitrepo/pack_test.go`:

```go
package gitrepo

import (
	"bytes"
	"encoding/binary"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
)

// writeIdx writes a version 2 index at base+".idx" that gives offsets, and
// an empty base+".pack". Offsets past 2 GiB go in the 64-bit table.
func writeIdx(t *testing.T, base string, offsets map[plumbing.Hash]int64) {
	t.Helper()
	hs := slices.SortedFunc(maps.Keys(offsets), func(a, b plumbing.Hash) int { return bytes.Compare(a[:], b[:]) })
	var b bytes.Buffer
	b.Write(idxMagic)
	w := func(v any) {
		if err := binary.Write(&b, binary.BigEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	w(uint32(idxVersion))
	var fanout [256]uint32
	for _, h := range hs {
		for i := int(h[0]); i < 256; i++ {
			fanout[i]++
		}
	}
	w(fanout)
	for _, h := range hs {
		b.Write(h[:])
	}
	b.Write(make([]byte, 4*len(hs))) // CRCs, not read
	var large []int64
	for _, h := range hs {
		off := offsets[h]
		if off <= 0x7fffffff {
			w(uint32(off))
			continue
		}
		w(uint32(0x80000000 | len(large)))
		large = append(large, off)
	}
	for _, off := range large {
		w(uint64(off))
	}
	b.Write(make([]byte, 40)) // checksums of the pack and of the index, not read
	if err := os.WriteFile(base+".idx", b.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(base+".pack", nil, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestPackFind(t *testing.T) {
	t.Parallel()
	offsets := map[plumbing.Hash]int64{
		plumbing.NewHash("0000000000000000000000000000000000000001"): 12,
		plumbing.NewHash("7f00000000000000000000000000000000000001"): 0x7fffffff,
		plumbing.NewHash("7f00000000000000000000000000000000000002"): 0x80000000, // the 64-bit table
		plumbing.NewHash("7f00000000000000000000000000000000000003"): 5 << 32,
		plumbing.NewHash("ff00000000000000000000000000000000000001"): 99,
	}
	base := filepath.Join(t.TempDir(), "pack-x")
	writeIdx(t, base, offsets)
	p, err := openPack(base)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.close() }()
	for h, want := range offsets {
		got, ok, err := p.find(h)
		if err != nil || !ok || got != want {
			t.Errorf("find(%s) = %d, %v, %v; want %d", h, got, ok, err, want)
		}
	}
	for _, s := range []string{
		"7f00000000000000000000000000000000000004", // same first byte, not there
		"8000000000000000000000000000000000000000", // empty fanout bucket
	} {
		if _, ok, err := p.find(plumbing.NewHash(s)); err != nil || ok {
			t.Errorf("find(%s) = %v, %v; want not found", s, ok, err)
		}
	}
}

// An index that is not version 2 is left to go-git.
func TestOpenPackSkipsOtherVersions(t *testing.T) {
	t.Parallel()
	base := filepath.Join(t.TempDir(), "pack-x")
	head := append(slices.Clone(idxMagic), 0, 0, 0, 3)
	if err := os.WriteFile(base+".idx", append(head, make([]byte, 2000)...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(base+".pack", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := openPack(base); err != errSkipPack {
		t.Errorf("openPack = %v, want errSkipPack", err)
	}
	if err := os.WriteFile(base+".idx", idxMagic, 0o600); err != nil { // cut short
		t.Fatal(err)
	}
	if _, err := openPack(base); err != errSkipPack {
		t.Errorf("openPack of a short index = %v, want errSkipPack", err)
	}
}
```

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/gitrepo -run 'TestPackFind|TestOpenPackSkipsOtherVersions'`
Expected: FAIL（`undefined: idxMagic`、`openPack` など）

- [ ] **Step 3: `pack.go` を書く**

`internal/gitrepo/pack.go`:

```go
package gitrepo

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/go-git/go-git/v5/plumbing"
)

// idxMagic heads a pack index of version 2 or later; idxVersion is the only
// version read here, the one git and go-git write.
var idxMagic = []byte{0xff, 't', 'O', 'c'}

const (
	idxVersion = 2
	idxFanout  = 8                // offset of the fanout table
	idxHashes  = idxFanout + 1024 // offset of the sorted hashes
)

// errSkipPack means a pack index this package does not read; go-git reads
// that pack instead.
var errSkipPack = errors.New("pack index is not of version 2")

// pack is a pack and its index, both kept open. Only the fanout table of the
// index is read up front: go-git reads whole indexes on its first read and
// then builds a map from every offset to its hash, which costs tens of
// milliseconds per command on a store of 600,000 objects (spec
// gogit-fixed-cost 1.1).
type pack struct {
	base      string // the path without .idx or .pack
	idx, data *os.File
	fanout    [256]uint32
}

func openPack(base string) (*pack, error) {
	idx, err := os.Open(base + ".idx") //nolint:gosec // G304: a pack index of the repository
	if err != nil {
		return nil, err
	}
	var head [idxHashes]byte
	if _, err := idx.ReadAt(head[:], 0); err != nil {
		_ = idx.Close()
		if errors.Is(err, io.EOF) {
			return nil, errSkipPack
		}
		return nil, fmt.Errorf("read %s.idx: %w", base, err)
	}
	if !bytes.Equal(head[:4], idxMagic) || binary.BigEndian.Uint32(head[4:8]) != idxVersion {
		_ = idx.Close()
		return nil, errSkipPack
	}
	data, err := os.Open(base + ".pack") //nolint:gosec // G304: a pack of the repository
	if err != nil {
		_ = idx.Close()
		return nil, err
	}
	p := &pack{base: base, idx: idx, data: data}
	for i := range p.fanout {
		p.fanout[i] = binary.BigEndian.Uint32(head[idxFanout+4*i:])
	}
	return p, nil
}

func (p *pack) close() error {
	return errors.Join(p.idx.Close(), p.data.Close())
}

// find returns the offset of h in the pack. It reads the hashes that share
// the first byte of h (about 2,300 of 600,000) and searches them.
func (p *pack) find(h plumbing.Hash) (int64, bool, error) {
	var lo uint32
	if h[0] > 0 {
		lo = p.fanout[h[0]-1]
	}
	hi := p.fanout[h[0]]
	if lo >= hi {
		return 0, false, nil
	}
	n := int(hi - lo)
	hashes := make([]byte, n*20)
	if _, err := p.idx.ReadAt(hashes, idxHashes+int64(lo)*20); err != nil {
		return 0, false, fmt.Errorf("read %s.idx: %w", p.base, err)
	}
	k := sort.Search(n, func(i int) bool { return bytes.Compare(hashes[i*20:i*20+20], h[:]) >= 0 })
	if k == n || !bytes.Equal(hashes[k*20:k*20+20], h[:]) {
		return 0, false, nil
	}
	count := int64(p.fanout[255])
	i := int64(lo) + int64(k)
	var b [8]byte
	// After the hashes come count CRCs, count 4-byte offsets, then the
	// 8-byte offsets that do not fit in 31 bits.
	if _, err := p.idx.ReadAt(b[:4], idxHashes+count*24+i*4); err != nil {
		return 0, false, fmt.Errorf("read %s.idx: %w", p.base, err)
	}
	off := binary.BigEndian.Uint32(b[:4])
	if off&0x80000000 == 0 {
		return int64(off), true, nil
	}
	if _, err := p.idx.ReadAt(b[:], idxHashes+count*28+int64(off&0x7fffffff)*8); err != nil {
		return 0, false, fmt.Errorf("read %s.idx: %w", p.base, err)
	}
	return int64(binary.BigEndian.Uint64(b[:])), true, nil //nolint:gosec // G115: git writes offsets below 2^63
}

// entry is the head of an object in a pack.
type entry struct {
	typ     plumbing.ObjectType
	size    int64         // of the inflated data; of the delta for a delta
	data    int64         // offset of the zlib data
	baseOff int64         // an OFS delta's base, in the same pack
	baseRef plumbing.Hash // a REF delta's base
}

// entryAt reads the head of the object at off.
func (p *pack) entryAt(off int64) (entry, error) {
	// The type and size take at most 10 bytes, and an OFS delta's offset
	// at most 10 or a REF delta's hash 20 more.
	var b [32]byte
	n, err := p.data.ReadAt(b[:], off)
	if err != nil && (!errors.Is(err, io.EOF) || n == 0) {
		return entry{}, fmt.Errorf("read %s.pack at %d: %w", p.base, off, err)
	}
	r := headReader{buf: b[:n]}
	c := r.next()
	e := entry{typ: plumbing.ObjectType((c >> 4) & 7), size: int64(c & 15)}
	for shift := 4; c&0x80 != 0; shift += 7 {
		c = r.next()
		e.size |= int64(c&0x7f) << shift
	}
	switch e.typ {
	case plumbing.OFSDeltaObject:
		c = r.next()
		rel := int64(c & 0x7f)
		for c&0x80 != 0 {
			c = r.next()
			rel = (rel+1)<<7 | int64(c&0x7f)
		}
		e.baseOff = off - rel
		if rel <= 0 || e.baseOff < 0 {
			return entry{}, fmt.Errorf("%s.pack at %d: bad delta base offset", p.base, off)
		}
	case plumbing.REFDeltaObject:
		copy(e.baseRef[:], r.take(20))
	}
	if r.short {
		return entry{}, fmt.Errorf("%s.pack at %d: %w", p.base, off, io.ErrUnexpectedEOF)
	}
	e.data = off + int64(r.i)
	return e, nil
}

// headReader reads the bytes of an object's head; past the end it gives
// zeros and sets short.
type headReader struct {
	buf   []byte
	i     int
	short bool
}

func (r *headReader) next() byte {
	if r.i >= len(r.buf) {
		r.short = true
		return 0
	}
	r.i++
	return r.buf[r.i-1]
}

func (r *headReader) take(n int) []byte {
	if r.i+n > len(r.buf) {
		r.short = true
		return nil
	}
	r.i += n
	return r.buf[r.i-n : r.i]
}

// inflate reads the zlib data of e.
func (p *pack) inflate(e entry) ([]byte, error) {
	zr, err := zlib.NewReader(bufio.NewReader(io.NewSectionReader(p.data, e.data, 1<<62)))
	if err != nil {
		return nil, fmt.Errorf("read %s.pack at %d: %w", p.base, e.data, err)
	}
	defer func() { _ = zr.Close() }()
	out := make([]byte, e.size)
	if _, err := io.ReadFull(zr, out); err != nil {
		return nil, fmt.Errorf("read %s.pack at %d: %w", p.base, e.data, err)
	}
	return out, nil
}
```

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test ./internal/gitrepo -run 'TestPackFind|TestOpenPackSkipsOtherVersions'`
Expected: PASS（`entryAt`・`inflate` は Task 2 のテストで確かめる）

- [ ] **Step 5: コミット**

```bash
git add internal/gitrepo/pack.go internal/gitrepo/pack_test.go
git commit -m "feat(gitrepo): pack の索引を全部は読まずに、hash からオフセットを引く"
```

---

### Task 2: オブジェクトの読み手（`objects.go`）

**Files:**
- Create: `internal/gitrepo/objects.go`
- Test: `internal/gitrepo/objects_test.go`

- [ ] **Step 1: 失敗するテストを書く**

`internal/gitrepo/objects_test.go`:

```go
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
			"tasks/a/task.md": strings.Repeat("a line of task a\n", 20) + fmt.Sprintf("version %d\n", i),
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
```

`sync.WaitGroup.Go` は Go 1.25 から。

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/gitrepo -run 'TestObjectReader|TestBaseCache'`
Expected: FAIL（`undefined: newObjectReader`、`newBaseCache`）

- [ ] **Step 3: `objects.go` を書く**

`internal/gitrepo/objects.go`:

```go
package gitrepo

import (
	"bufio"
	"compress/zlib"
	"container/list"
	"errors"
	"fmt"
	"io"
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
	typ, data, err := o.raw(h)
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
func (o *objectReader) raw(h plumbing.Hash) (plumbing.ObjectType, []byte, error) {
	for _, again := range []bool{false, true} {
		p, off, err := o.find(h, again)
		if err != nil {
			return plumbing.InvalidObject, nil, err
		}
		if p != nil {
			return o.at(p, off, false)
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
// the base of a delta is kept in the base cache.
func (o *objectReader) at(p *pack, off int64, asBase bool) (plumbing.ObjectType, []byte, error) {
	if typ, data, ok := o.bases.get(p, off); ok {
		return typ, data, nil
	}
	e, err := p.entryAt(off)
	if err != nil {
		return plumbing.InvalidObject, nil, err
	}
	data, err := p.inflate(e)
	if err != nil {
		return plumbing.InvalidObject, nil, err
	}
	typ := e.typ
	switch e.typ {
	case plumbing.CommitObject, plumbing.TreeObject, plumbing.BlobObject, plumbing.TagObject:
	case plumbing.OFSDeltaObject, plumbing.REFDeltaObject:
		var base []byte
		if e.typ == plumbing.OFSDeltaObject {
			typ, base, err = o.at(p, e.baseOff, true)
		} else {
			typ, base, err = o.raw(e.baseRef)
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
	data := make([]byte, n)
	if _, err := io.ReadFull(br, data); err != nil {
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
```

注意: `at` が返す `data` は基の LRU と共有する。`object` は `MemoryObject.Write` でコピーするので、呼び出し側が書き換えても LRU は壊れない。`PatchDelta` は新しい slice を返す。

- [ ] **Step 4: テストが通ることを確かめる**

Run: `go test -race ./internal/gitrepo -run 'TestObjectReader|TestBaseCache|TestPack|TestOpenPack' -v 2>&1 | tail -20`
Expected: PASS（`TestObjectReaderOFSDeltas` と `TestObjectReaderREFDeltas` が「delta がない」で落ちたら、`makeHistory` の回数を増やす。delta なしで比べても意味がないため）

- [ ] **Step 5: コミット**

```bash
git add internal/gitrepo/objects.go internal/gitrepo/objects_test.go
git commit -m "feat(gitrepo): loose と pack のオブジェクトを、go-git の索引の全読み込みなしに読む"
```

---

### Task 3: `cachedStorage` で読み手を使う

**Files:**
- Modify: `internal/gitrepo/storage.go`
- Modify: `internal/gitrepo/repo.go`（`newRepo`）
- Test: `internal/gitrepo/storage_test.go`

- [ ] **Step 1: 失敗するテストを書く**

`internal/gitrepo/storage_test.go` の末尾に足す（import に `"errors"`、`"strings"`、`"github.com/go-git/go-billy/v5/osfs"`、`"github.com/go-git/go-git/v5/plumbing/cache"`、`"github.com/go-git/go-git/v5/storage/filesystem"` を足す）:

```go
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
	bad := filepath.Join(dir, ".git", "objects", "pack", "pack-"+strings.Repeat("0", 40))
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

// An object the reader cannot find is asked of go-git, which finds it (here,
// in a pack whose index is of a version the reader does not read).
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
```

`TestReadFallsBackToGoGit` は、`cachedStorage` の型の確認と、見つからないときと種類が違うときの振る舞いを確かめる。go-git に任せた先で見つかる場合は、alternates がないと作れない。ここでは「見つからない」が go-git を通っても `ErrObjectNotFound` になることだけを確かめる。

- [ ] **Step 2: テストが失敗することを確かめる**

Run: `go test ./internal/gitrepo -run 'TestReadWithoutGoGitIndexes|TestReadAfterConsolidate|TestReadFallsBackToGoGit'`
Expected: FAIL（`cs.reader undefined`、`TestReadWithoutGoGitIndexes` は go-git が `.idx` を読めずに失敗）

- [ ] **Step 3: `storage.go` を変える**

`internal/gitrepo/storage.go` を次の内容にする:

```go
package gitrepo

import (
	"errors"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

// cachedStorage looks an object up in memory before it touches the disk.
// go-git opens the object file, or the packfile, before it looks in its
// cache, and on Windows each open and each stat of the path costs tens of
// microseconds. An object never changes under its hash, so the copy in
// memory is the object whether or not the file still exists.
//
// With a reader, objects are read by it and only those it cannot find by
// go-git (spec gogit-fixed-cost 2): go-git loads every pack index whole on
// its first read. Writes and walks over every object stay with go-git.
type cachedStorage struct {
	*filesystem.Storage
	objects cache.Object
	gitDir  string        // the git directory; references are written there (refs.go)
	reader  *objectReader // nil: go-git reads every object
}

func newCachedStorage(s *filesystem.Storage) *cachedStorage {
	return &cachedStorage{Storage: s, objects: cache.NewObjectLRUDefault(), gitDir: s.Filesystem().Root()}
}

func (s *cachedStorage) EncodedObject(t plumbing.ObjectType, h plumbing.Hash) (plumbing.EncodedObject, error) {
	if o, ok := s.objects.Get(h); ok && (t == plumbing.AnyObject || o.Type() == t) {
		return o, nil
	}
	o, err := s.read(t, h)
	if err != nil {
		return nil, err
	}
	s.objects.Put(o)
	return o, nil
}

// read reads h with the reader, and with go-git when the reader does not
// find it.
func (s *cachedStorage) read(t plumbing.ObjectType, h plumbing.Hash) (plumbing.EncodedObject, error) {
	if s.reader != nil {
		o, err := s.reader.object(h)
		switch {
		case err == nil:
			if t != plumbing.AnyObject && o.Type() != t {
				return nil, plumbing.ErrObjectNotFound
			}
			return o, nil
		case !errors.Is(err, plumbing.ErrObjectNotFound):
			return nil, err
		}
	}
	return s.Storage.EncodedObject(t, h)
}

func (s *cachedStorage) HasEncodedObject(h plumbing.Hash) error {
	if s.reader != nil {
		ok, err := s.reader.has(h)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
	return s.Storage.HasEncodedObject(h)
}

func (s *cachedStorage) EncodedObjectSize(h plumbing.Hash) (int64, error) {
	if s.reader == nil {
		return s.Storage.EncodedObjectSize(h)
	}
	o, err := s.EncodedObject(plumbing.AnyObject, h)
	if err != nil {
		return 0, err
	}
	return o.Size(), nil
}

// Reindex makes go-git and the reader list the packs anew.
func (s *cachedStorage) Reindex() {
	s.Storage.Reindex()
	if s.reader != nil {
		s.reader.stale()
	}
}

// Close lets go of the packs go-git and the reader hold open. Both open them
// again when they next read.
func (s *cachedStorage) Close() error {
	err := s.Storage.Close()
	if s.reader != nil {
		err = errors.Join(err, s.reader.close())
	}
	return err
}
```

- [ ] **Step 4: `newRepo` で読み手を付ける**

`internal/gitrepo/repo.go` の `newRepo` の中の

```go
		s = filesystem.NewStorageWithOptions(s.Filesystem(), cache.NewObjectLRUDefault(), filesystem.Options{KeepDescriptors: true})
		r.Storer = newCachedStorage(s)
```

を次にする:

```go
		s = filesystem.NewStorageWithOptions(s.Filesystem(), cache.NewObjectLRUDefault(), filesystem.Options{KeepDescriptors: true})
		cs := newCachedStorage(s)
		cs.reader = newObjectReader(cs.gitDir)
		r.Storer = cs
```

`fileLoader`（`transport.go`、remote 側の bare リポジトリ）は読み手を付けない。閉じる人がいないので、pack を開いたままにしないためである。

- [ ] **Step 5: テストが通ることを確かめる**

Run: `go test -race ./internal/gitrepo/...`
Expected: PASS（既存のテストも含めてすべて）

- [ ] **Step 6: パッケージ全体のテストを流す**

Run: `go test -race ./...`
Expected: PASS

- [ ] **Step 7: コミット**

```bash
git add internal/gitrepo/storage.go internal/gitrepo/storage_test.go internal/gitrepo/repo.go
git commit -m "perf(gitrepo): オブジェクトを自前の読み手で読み、go-git の pack の索引の読み込みを避ける"
```

---

### Task 4: 変わった tree だけを組み立てる commit

**Files:**
- Create: `internal/gitrepo/commit_tree.go`
- Modify: `internal/gitrepo/commit.go`（`Repo.Commit`）
- Test: `internal/gitrepo/commit_tree_test.go`

- [ ] **Step 1: テストを書く**

`internal/gitrepo/commit_tree_test.go`:

```go
package gitrepo

import (
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// goGitTree returns the tree go-git's Worktree.Commit makes from the index
// of the repository at dir, which is what Repo.Commit made before (spec
// gogit-fixed-cost 3). HEAD is put back after.
func goGitTree(t *testing.T, dir string) plumbing.Hash {
	t.Helper()
	g, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	head, err := g.Head()
	if err != nil {
		t.Fatal(err)
	}
	w, err := g.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	sig := &object.Signature{Name: "oracle", Email: "oracle@example.com", When: time.Now()}
	h, err := w.Commit("oracle", &git.CommitOptions{Author: sig, AllowEmptyCommits: true})
	if err != nil {
		t.Fatal(err)
	}
	c, err := g.CommitObject(h)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Storer.SetReference(plumbing.NewHashReference(head.Name(), head.Hash())); err != nil {
		t.Fatal(err)
	}
	return c.TreeHash
}

func TestCommitMakesTheTreeGoGitMakes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	steps := []struct {
		name  string
		files map[string]string // empty content removes the path
	}{
		{"first", map[string]string{"schema.yaml": "s1", "tasks/a/task.md": "a1", "tasks/b/task.md": "b1"}},
		{"modify", map[string]string{"tasks/a/task.md": "a2"}},
		{"add deep", map[string]string{"tasks/a/comments/c1.md": "c1", "x/y/z/w.md": "w"}},
		{"several", map[string]string{"schema.yaml": "s2", "tasks/b/task.md": "b2", "tasks/c/task.md": "c1"}},
		{"delete empties a directory", map[string]string{"tasks/a/comments/c1.md": ""}},
		{"delete empties a chain", map[string]string{"x/y/z/w.md": ""}},
		// "a.md" < "a/" < "a0.md" in git's order, which compares a
		// directory's name with a "/" after it.
		{"order", map[string]string{"tasks/a.md": "f", "tasks/a0.md": "g", "tasks/a-b/task.md": "h"}},
		{"remove a file", map[string]string{"tasks/a.md": ""}},
		{"a directory where the file was", map[string]string{"tasks/a.md/task.md": "i"}},
		{"delete everything", map[string]string{
			"schema.yaml": "", "tasks/a/task.md": "", "tasks/b/task.md": "", "tasks/c/task.md": "",
			"tasks/a0.md": "", "tasks/a-b/task.md": "", "tasks/a.md/task.md": "",
		}},
	}
	for _, s := range steps {
		got := commitFiles(t, r, dir, s.files)
		if want := goGitTree(t, dir); got != want {
			t.Fatalf("%s: tree %s, go-git makes %s", s.name, got, want)
		}
	}
}

// A merge commit with no path has the tree of HEAD and both parents.
func TestCommitMergeKeepsTheTree(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	first := commitHead(t, r, dir, map[string]string{"a.md": "1"})
	second := commitHead(t, r, dir, map[string]string{"b.md": "2"})
	tree, err := r.HeadTree()
	if err != nil {
		t.Fatal(err)
	}
	h, err := r.Commit(Commit{Parents: []plumbing.Hash{second, first}, Subject: "merge", Author: author, When: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	c, err := r.r.CommitObject(h)
	if err != nil {
		t.Fatal(err)
	}
	if c.TreeHash != tree || len(c.ParentHashes) != 2 || c.ParentHashes[0] != second || c.ParentHashes[1] != first {
		t.Errorf("merge: tree %s parents %v; want tree %s parents [%s %s]", c.TreeHash, c.ParentHashes, tree, second, first)
	}
	if got := mustHead(t, r); got != h {
		t.Errorf("HEAD = %s, want the merge %s", got, h)
	}
}

func TestCommitAllowEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	before := commitHead(t, r, dir, map[string]string{"a.md": "1"})
	h, err := r.Commit(Commit{AllowEmpty: true, Subject: "replay", Author: author, When: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	c, err := r.r.CommitObject(h)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.ParentHashes) != 1 || c.ParentHashes[0] != before {
		t.Errorf("parents = %v, want [%s]", c.ParentHashes, before)
	}
}

// Only c.Paths are committed: a path staged by hand stays out (spec
// gogit-fixed-cost 3.2).
func TestCommitLeavesOutPathsStagedByHand(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	commitFiles(t, r, dir, map[string]string{"a.md": "1"})
	writeFile(t, dir, "other.md", "by hand")
	w, err := r.r.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Add("other.md"); err != nil {
		t.Fatal(err)
	}
	commitFiles(t, r, dir, map[string]string{"a.md": "2"})
	c, err := r.r.CommitObject(mustHead(t, r))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.File("other.md"); err == nil {
		t.Error("other.md, staged by hand, was committed")
	}
}

// The author's name and email lose what git does not allow in them, as
// go-git's Worktree.Commit did.
func TestCommitSanitizesTheAuthor(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	writeFile(t, dir, "a.md", "1")
	h, err := r.Commit(Commit{Paths: []string{"a.md"}, Subject: "x", Author: Signature{Name: "A <b>\n", Email: "<a@example.com>"}, When: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	c, err := r.r.CommitObject(h)
	if err != nil {
		t.Fatal(err)
	}
	if c.Author.Name != "A b" || c.Author.Email != "a@example.com" || c.Committer.Name != "A b" || c.Committer.Email != "a@example.com" {
		t.Errorf("author %q <%q>, committer %q <%q>", c.Author.Name, c.Author.Email, c.Committer.Name, c.Committer.Email)
	}
}
```

- [ ] **Step 2: テストを流す（今の go-git の実装で通ること）**

Run: `go test ./internal/gitrepo -run 'TestCommit'`
Expected: `TestCommitLeavesOutPathsStagedByHand` だけ FAIL（今は手で stage した `other.md` も commit される）。ほかは PASS。これで、比べる相手の go-git の動きをテストが正しく捉えていることを確かめる。

- [ ] **Step 3: `commit_tree.go` を書く**

`internal/gitrepo/commit_tree.go`:

```go
package gitrepo

import (
	"errors"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// commitStaged commits c, whose paths are staged already, and moves HEAD to
// it. The tree is HEAD's with c.Paths as in the index; only the trees above
// c.Paths are made again. go-git's Worktree.Commit builds every tree of the
// index and asks the disk for each whether it exists, about 5,000 opens per
// commit on a store of 5,000 tasks (spec gogit-fixed-cost 3). The index
// outside c.Paths is taken to be HEAD's tree, as it is after every hh write.
func (r *Repo) commitStaged(c Commit) (plumbing.Hash, error) {
	head, err := r.r.Head()
	if err != nil && !errors.Is(err, plumbing.ErrReferenceNotFound) {
		return plumbing.ZeroHash, err
	}
	var base plumbing.Hash
	parents := c.Parents
	if head != nil {
		hc, err := r.r.CommitObject(head.Hash())
		if err != nil {
			return plumbing.ZeroHash, err
		}
		base = hc.TreeHash
		if len(parents) == 0 {
			parents = []plumbing.Hash{head.Hash()}
		}
	}
	idx, err := r.r.Storer.Index()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	edits := make(map[string]*index.Entry, len(c.Paths))
	for _, p := range c.Paths {
		e, err := idx.Entry(p)
		switch {
		case err == nil:
			edits[p] = e
		case errors.Is(err, index.ErrEntryNotFound):
			edits[p] = nil
		default:
			return plumbing.ZeroHash, err
		}
	}
	tree, empty, err := r.editTree(base, edits, true)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	if len(parents) < 2 && !c.AllowEmpty {
		if len(parents) == 0 && empty {
			return plumbing.ZeroHash, ErrNothingToCommit
		}
		if len(parents) > 0 {
			pc, err := r.r.CommitObject(parents[0])
			if err != nil {
				return plumbing.ZeroHash, err
			}
			if pc.TreeHash == tree {
				return plumbing.ZeroHash, ErrNothingToCommit
			}
		}
	}
	sig := object.Signature{Name: badSignature.ReplaceAllString(c.Author.Name, ""), Email: badSignature.ReplaceAllString(c.Author.Email, ""), When: c.When}
	commit := &object.Commit{Author: sig, Committer: sig, Message: message(c), TreeHash: tree, ParentHashes: parents}
	o := r.r.Storer.NewEncodedObject()
	if err := commit.Encode(o); err != nil {
		return plumbing.ZeroHash, err
	}
	h, err := r.r.Storer.SetEncodedObject(o)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return h, r.moveHead(h)
}

// badSignature is what git does not allow in a name or an email; go-git's
// Worktree.Commit drops it too.
var badSignature = regexp.MustCompile(`[<>\n]`)

// moveHead points HEAD, or the branch it is on, at h.
func (r *Repo) moveHead(h plumbing.Hash) error {
	ref, err := r.r.Storer.Reference(plumbing.HEAD)
	if err != nil {
		return err
	}
	name := plumbing.HEAD
	if ref.Type() != plumbing.HashReference {
		name = ref.Target()
	}
	return r.r.Storer.SetReference(plumbing.NewHashReference(name, h))
}

// editTree returns the tree base (zero: an empty tree) with edits made, and
// whether it is empty. edits maps a path relative to base to its index entry,
// or to nil to remove it. Only the trees on the edited paths are read and
// written. An empty tree is dropped from its parent; only the root is
// written when empty.
func (r *Repo) editTree(base plumbing.Hash, edits map[string]*index.Entry, root bool) (plumbing.Hash, bool, error) {
	var entries []object.TreeEntry
	if !base.IsZero() {
		t, err := r.r.TreeObject(base)
		if err != nil {
			return plumbing.ZeroHash, false, err
		}
		entries = slices.Clone(t.Entries)
	}
	at := func(name string) int {
		return slices.IndexFunc(entries, func(e object.TreeEntry) bool { return e.Name == name })
	}
	subs := map[string]map[string]*index.Entry{}
	for p, e := range edits {
		name, rest, nested := strings.Cut(p, "/")
		if nested {
			if subs[name] == nil {
				subs[name] = map[string]*index.Entry{}
			}
			subs[name][rest] = e
			continue
		}
		if i := at(name); i >= 0 {
			entries = slices.Delete(entries, i, i+1)
		}
		if e != nil {
			entries = append(entries, object.TreeEntry{Name: name, Mode: e.Mode, Hash: e.Hash})
		}
	}
	for name, sub := range subs {
		i := at(name)
		var cur plumbing.Hash
		if i >= 0 && entries[i].Mode == filemode.Dir {
			cur = entries[i].Hash
		}
		h, empty, err := r.editTree(cur, sub, false)
		if err != nil {
			return plumbing.ZeroHash, false, err
		}
		dir := object.TreeEntry{Name: name, Mode: filemode.Dir, Hash: h}
		switch {
		case !empty && i >= 0:
			entries[i] = dir
		case !empty:
			entries = append(entries, dir)
		case i >= 0 && entries[i].Mode == filemode.Dir:
			entries = slices.Delete(entries, i, i+1)
		}
	}
	if len(entries) == 0 && !root {
		return plumbing.ZeroHash, true, nil
	}
	// git orders a directory as if its name ended in "/".
	key := func(e object.TreeEntry) string {
		if e.Mode == filemode.Dir {
			return e.Name + "/"
		}
		return e.Name
	}
	sort.Slice(entries, func(i, j int) bool { return key(entries[i]) < key(entries[j]) })
	o := r.r.Storer.NewEncodedObject()
	if err := (&object.Tree{Entries: entries}).Encode(o); err != nil {
		return plumbing.ZeroHash, false, err
	}
	if o.Hash() == base {
		return base, len(entries) == 0, nil // unchanged; already stored
	}
	h, err := r.r.Storer.SetEncodedObject(o)
	return h, len(entries) == 0, err
}
```

- [ ] **Step 4: `Repo.Commit` から `Worktree.Commit` を外す**

`internal/gitrepo/commit.go` の `Repo.Commit` の

```go
	if err := r.stage(w, c.Paths); err != nil {
		return plumbing.ZeroHash, err
	}
	sig := &object.Signature{Name: c.Author.Name, Email: c.Author.Email, When: c.When}
	h, err = w.Commit(message(c), &git.CommitOptions{
		Author: sig, Committer: sig, Parents: c.Parents, AllowEmptyCommits: len(c.Parents) >= 2 || c.AllowEmpty,
	})
	if errors.Is(err, git.ErrEmptyCommit) {
		return plumbing.ZeroHash, ErrNothingToCommit
	}
	return h, err
}
```

を次にする:

```go
	if err := r.stage(w, c.Paths); err != nil {
		return plumbing.ZeroHash, err
	}
	h, err = r.commitStaged(c)
	return h, err
}
```

`Repo.Commit` の説明の 2 段落目（「Paths are staged one by one ...」）の後に次の段落を足す:

```go
// The tree is made from HEAD's with only the trees above c.Paths made again
// (commitStaged), not from the whole index.
```

import から使わなくなった `"github.com/go-git/go-git/v5/plumbing/object"` を消す（`git` は `stage` の `git.Worktree`・`git.AddOptions` でまだ使う）。

- [ ] **Step 5: テストが通ることを確かめる**

Run: `go test -race ./internal/gitrepo/...`
Expected: PASS（`TestCommitLeavesOutPathsStagedByHand` も通る）

- [ ] **Step 6: 全体のテストを流す**

Run: `go test -race ./...`
Expected: PASS

- [ ] **Step 7: コミット**

```bash
git add internal/gitrepo/commit_tree.go internal/gitrepo/commit_tree_test.go internal/gitrepo/commit.go
git commit -m "perf(gitrepo): commit で変わったパスの上の tree だけを組み立てる"
```

---

### Task 4b: tree の差分を hash が違うところだけ読む（spec 3.3）

実装中の計測（mac、v2 の合成 Store）で追加した。Task 1〜4 の後、`undo` 1 回で約 1,200 個のオブジェクトを読み、約 1,100 個が `Repo.DiffTrees` だった。go-git の `object.DiffTree` が、hash が同じ兄弟のディレクトリの tree まで開くためである。

**Files:**
- Modify: `internal/gitrepo/changed.go`（`diffTrees` が消えたかどうかも渡す）
- Modify: `internal/gitrepo/tree.go`（`DiffTrees` が `diffTrees` を使う）
- Test: `internal/gitrepo/tree_test.go`

- [ ] **Step 1: go-git と比べるテストを書く**

`tree_test.go` に、いくつかの tree の組について `Repo.DiffTrees` の結果が go-git の `object.DiffTree` から今のコードと同じ方法で作った `[]Change`（パスの順）と一致することを確かめるテストを足す。組: 足す、変える、消す、ディレクトリごと消す、深いディレクトリを足す、ファイルからディレクトリに変える、ディレクトリからファイルに変える、mode だけを変える（実行ビット。`os.Chmod` で 0o755 にして commit）、`prefix` での絞り込み、同じ tree（空）。go-git 側の結果を作るヘルパーは、今の `DiffTrees` の本体（`object.DiffTree` → `Action` → `Change`）をテストに移したものにする。

- [ ] **Step 2: テストを流し、今のコードで通ることを確かめる（比べる相手の確認）**

- [ ] **Step 3: `diffTrees` を `fn func(path string, deleted bool)` にし、`DiffTrees` を書き換える**

`diffTrees` の中で、A にあるファイルは「B に同じ名前のファイルがなければ deleted」、B にあるファイルは今と同じ条件で deleted=false として渡す。`ChangedFiles` は deleted を見ない。`DiffTrees` は `r.tree(from)`・`r.tree(to)` を読み、`diffTrees` で集め、`prefix` で絞り、パスの順に並べて返す。`tree.go` で使わなくなった `merkletrie` の import を消す。

- [ ] **Step 4: `go test -race -count=1 ./...` と `golangci-lint run ./...` が通ることを確かめる**

- [ ] **Step 5: コミット**（`perf(gitrepo): tree の差分で、hash が同じディレクトリを読まない`）

---

### Task 5: 計測用の合成 Store の pack を delta 付きにする

**Files:**
- Modify: `internal/history/historytest/synth.go`（`layout` と `writePack`）

- [ ] **Step 1: `layout` を上げる**

`internal/history/historytest/synth.go` の

```go
const layout = 2
```

を

```go
const layout = 3
```

にする。

- [ ] **Step 2: pack を window 10 で書く**

同じファイルの `writePack` の

```go
	if _, err := packfile.NewEncoder(w, m.mem, false).Encode(m.hashes, 0); err != nil {
```

を次にする:

```go
	// With deltas, as the packs of a real store are (a fetch, or
	// Consolidate with the default pack window).
	if _, err := packfile.NewEncoder(w, m.mem, false).Encode(m.hashes, config.DefaultPackWindow); err != nil {
```

`historytest` は `internal/config` を `config` の名前で import しているので、go-git の config は別名で import に足す: `gitconfig "github.com/go-git/go-git/v5/config"`。上のコードの `config.DefaultPackWindow` は `gitconfig.DefaultPackWindow` と書く。

- [ ] **Step 3: ビルドと通常のテストを確かめる**

Run: `go vet ./internal/history/... && go test ./internal/history/...`
Expected: PASS（`HH_BENCH` がないのでベンチは飛ばされる。小さい合成を使うテストがあれば、それも delta 付きで通る）

- [ ] **Step 4: 合成 Store を作り、delta があることを確かめる**

Run: `HH_BENCH=1 go test ./internal/history -run '^$' -bench 'BenchmarkShow$' -benchtime 1x -timeout 120m`
Expected: PASS。合成に時間がかかる（delta を探すため）。かかった時間を控えておく（6 節に書く）。

続けて、作った pack に delta があることを確かめる。合成 Store は `$HH_BENCH_CACHE`、なければユーザーのキャッシュディレクトリの `hh-bench` にある（mac は `~/Library/Caches/hh-bench`、Linux は `~/.cache/hh-bench`）。mac では:

```bash
cd "$HOME/Library/Caches/hh-bench/c100000-t5000-s1-v3/store"
git verify-pack -s "$(git rev-parse --git-dir)"/objects/pack/pack-*.idx | head -3
```

Expected: `non delta:` の個数が全体（約 60 万）より十分少なく、`chain length = 1:` などの行が出る。

- [ ] **Step 5: 古い合成 Store を消す**

`c100000-t5000-s1-v2` はもう使わないので、`hh-bench` から消す（`rm -rf "$HOME/Library/Caches/hh-bench/c100000-t5000-s1-v2"`）。

- [ ] **Step 6: コミット**

```bash
git add internal/history/historytest/synth.go
git commit -m "test(history): 計測用の合成 Store の pack を、実際の Store と同じく delta 付きで書く"
```

---

### Task 6: 計測と記録

**Files:**
- Modify: `docs/superpowers/specs/2026-11-03-gogit-fixed-cost-design.md`（6 節）
- Modify: `docs/superpowers/specs/2026-11-03-history-growth-design.md`（8 節）

- [ ] **Step 1: mac で計測する**

Run: `make bench-history 2>&1 | tee <スクラッチパッド>/bench-mac.txt`（出力はリポジトリの外に置き、コミットしない）
Expected: すべての Benchmark が PASS。`BenchmarkShowAfterOneCommit` と `BenchmarkUndo/mine` が 200ms（200,000,000 ns/op）以内。手元が静かなときに流す（spec history-growth 8 節の注意のとおり、ほかのプロセスでぶれる）。

200ms を超えた項目があれば、`-cpuprofile` を付けてその Benchmark だけを流し、`go tool pprof -top -cum` で原因を探して直す。直したら Step 1 からやり直す。

- [ ] **Step 2: WSL での計測を利用者に頼む**

利用者に、WSL で `git pull` してから `make bench-history` を流し、出力を貼ってもらうよう頼む。合成 Store は WSL でも作り直しになる（layout 3）。

- [ ] **Step 3: 6 節を書く**

`docs/superpowers/specs/2026-11-03-gogit-fixed-cost-design.md` の

```markdown
## 6. 計測の結果

（実装の後に記録する）
```

を、history-growth spec の 8 節と同じ形の表（環境の表と、項目・目標・WSL・mac・判定の表）にする。項目は history-growth の 8 節の表と同じものをすべて並べ、「固定費: 開いて最初のコミットを読む」と「固定費: 書き込み 1 回」は `BenchmarkFixedCost/firstRead`・`write` の値を入れる。表の下に次を書く:

- 合成 Store が delta 付き（layout 3）になったので、history-growth の 8 節の値とは同じ条件ではないこと。
- 合成にかかった時間（Task 5 の Step 4）。
- 判定が「未達」の項目があれば、その原因（Step 1 のプロファイル）。

- [ ] **Step 4: history-growth の 8 節に一文を足す**

`docs/superpowers/specs/2026-11-03-history-growth-design.md` の 8 節の、「WSL で未達の 2 項目は、決定 9 の固定費のためだけに届かない。」で始まる段落の終わりに、次の一文を足す:

```markdown
この固定費は、`2026-11-03-gogit-fixed-cost-design.md` で直した（結果は同 spec の 6 節）。
```

- [ ] **Step 5: lint と全体の確認**

Run: `make tidy-check lint fmt-check test`
Expected: すべて成功。（`web-check` は Go だけの変更なので、`make check` の残りとして最後に一度流す: `make web-check`）

- [ ] **Step 6: コミット**

```bash
git add docs/superpowers/specs/2026-11-03-gogit-fixed-cost-design.md docs/superpowers/specs/2026-11-03-history-growth-design.md
git commit -m "docs(spec): go-git の固定費を直した後の計測の結果を書く"
```

- [ ] **Step 7: 親設計に反映する**

親設計（https://claude.ai/artifact/8d4dcVbePAxR2BxhMtkqnc）を Artifact の `read` で読み、「フェーズ7の完了条件（性能目標）」の決定 9 の固定費についての記述を、本 spec の結果（WSL と mac の両方で達成）に置き換えて publish する。サブエージェントではなく、メインのセッションで行う。
