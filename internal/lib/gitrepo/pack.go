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
	"sync"
	"sync/atomic"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/lib/osfs"
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
	buckets   [256]atomic.Pointer[idxBucket]
}

func openPack(base string) (*pack, error) {
	idx, err := osfs.Open(base + ".idx")
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
	data, err := osfs.Open(base + ".pack")
	if err != nil {
		_ = idx.Close()
		return nil, err
	}
	p := &pack{base: base, idx: idx, data: data}
	for i := range p.fanout {
		p.fanout[i] = binary.BigEndian.Uint32(head[idxFanout+4*i:])
	}
	// A fanout that goes down, or counts more objects than the index has
	// room for, would make bucket allocate past the file.
	fi, err := idx.Stat()
	ok := err == nil && idxHashes+int64(p.fanout[255])*28 <= fi.Size()
	for i := 1; ok && i < len(p.fanout); i++ {
		ok = p.fanout[i-1] <= p.fanout[i]
	}
	if !ok {
		_ = p.close()
		return nil, errors.Join(fmt.Errorf("%s.idx: bad fanout", base), err)
	}
	return p, nil
}

func (p *pack) close() error {
	return errors.Join(p.idx.Close(), p.data.Close())
}

// find returns the offset of h in the pack. It searches the hashes that
// share the first byte of h (about 2,300 of 600,000).
func (p *pack) find(h plumbing.Hash) (int64, bool, error) {
	b, err := p.bucket(h[0])
	if err != nil || b == nil {
		return 0, false, err
	}
	n := len(b.offsets) / 4
	k := sort.Search(n, func(i int) bool { return bytes.Compare(b.hashes[i*20:i*20+20], h[:]) >= 0 })
	if k == n || !bytes.Equal(b.hashes[k*20:k*20+20], h[:]) {
		return 0, false, nil
	}
	off := binary.BigEndian.Uint32(b.offsets[k*4:])
	if off&0x80000000 == 0 {
		return int64(off), true, nil
	}
	// The 8-byte offsets that do not fit in 31 bits come after the 4-byte ones.
	var large [8]byte
	if _, err := p.idx.ReadAt(large[:], idxHashes+int64(p.fanout[255])*28+int64(off&0x7fffffff)*8); err != nil {
		return 0, false, fmt.Errorf("read %s.idx: %w", p.base, err)
	}
	return int64(binary.BigEndian.Uint64(large[:])), true, nil //nolint:gosec // G115: git writes offsets below 2^63
}

// idxBucket is the hashes that share a first byte and their 4-byte offsets,
// as the index stores them.
type idxBucket struct {
	hashes, offsets []byte
}

// bucket returns the hashes of the index that start with first, read on
// first use and kept: a walk over 100,000 commits looks in every bucket many
// times. Kept whole, the buckets take as much memory as the index's hashes
// and offsets (about 14 MB for 600,000 objects), much less than go-git's map.
// It returns nil for an empty bucket.
func (p *pack) bucket(first byte) (*idxBucket, error) {
	if b := p.buckets[first].Load(); b != nil {
		return b, nil
	}
	var lo uint32
	if first > 0 {
		lo = p.fanout[first-1]
	}
	hi := p.fanout[first] // openPack checked that the fanout goes up
	if lo == hi {
		return nil, nil
	}
	n := int64(hi - lo)
	b := &idxBucket{hashes: make([]byte, n*20), offsets: make([]byte, n*4)}
	if _, err := p.idx.ReadAt(b.hashes, idxHashes+int64(lo)*20); err != nil {
		return nil, fmt.Errorf("read %s.idx: %w", p.base, err)
	}
	// After the hashes come a CRC for each object, then the offsets.
	if _, err := p.idx.ReadAt(b.offsets, idxHashes+int64(p.fanout[255])*24+int64(lo)*4); err != nil {
		return nil, fmt.Errorf("read %s.idx: %w", p.base, err)
	}
	p.buckets[first].Store(b) // two readers may both read it; either copy is right
	return b, nil
}

// packEntry is the head of an object in a pack.
type packEntry struct {
	typ     plumbing.ObjectType
	size    int64         // of the inflated data; of the delta for a delta
	data    int64         // offset of the zlib data
	baseOff int64         // an OFS delta's base, in the same pack
	baseRef plumbing.Hash // a REF delta's base
}

// readAhead is how much of an object one read takes: its head and, for most
// objects of a store, all of its compressed data. A walk over 100,000
// commits makes one read per object instead of two.
const readAhead = 4096

// objectBuf is what reading one object needs, kept for the next.
type objectBuf struct {
	ahead [readAhead]byte
	rest  bytes.Reader
	br    *bufio.Reader
	zr    io.ReadCloser // a zlib reader, Reset for each object
}

var objectBufs = sync.Pool{New: func() any {
	return &objectBuf{br: bufio.NewReaderSize(nil, readAhead)}
}}

// entryAt reads the head of the object at off into buf, which is long enough
// for any head (32 bytes), and returns it with the bytes of buf after it.
func (p *pack) entryAt(off int64, buf []byte) (packEntry, []byte, error) {
	n, err := p.data.ReadAt(buf, off)
	if err != nil && (!errors.Is(err, io.EOF) || n == 0) {
		return packEntry{}, nil, fmt.Errorf("read %s.pack at %d: %w", p.base, off, err)
	}
	// The type and size take at most 10 bytes, and an OFS delta's offset
	// at most 10 or a REF delta's hash 20 more.
	r := headReader{buf: buf[:min(n, 32)]}
	c := r.next()
	e := packEntry{typ: plumbing.ObjectType((c >> 4) & 7), size: int64(c & 15)}
	for shift := 4; c&0x80 != 0; shift += 7 {
		c = r.next()
		e.size |= int64(c&0x7f) << shift
	}
	if e.size < 0 { // a varint that overflowed
		return packEntry{}, nil, fmt.Errorf("%s.pack at %d: bad object size", p.base, off)
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
			return packEntry{}, nil, fmt.Errorf("%s.pack at %d: bad delta base offset", p.base, off)
		}
	case plumbing.REFDeltaObject:
		copy(e.baseRef[:], r.take(20))
	}
	if r.short {
		return packEntry{}, nil, fmt.Errorf("%s.pack at %d: %w", p.base, off, io.ErrUnexpectedEOF)
	}
	e.data = off + int64(r.i)
	return e, buf[r.i:n], nil
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

// inflate reads the zlib data of e, whose first bytes, already read, are
// rest; ob holds the readers it reuses.
func (p *pack) inflate(e packEntry, rest []byte, ob *objectBuf) ([]byte, error) {
	ob.rest.Reset(rest)
	ob.br.Reset(io.MultiReader(&ob.rest, io.NewSectionReader(p.data, e.data+int64(len(rest)), 1<<62)))
	var err error
	if ob.zr == nil {
		ob.zr, err = zlib.NewReader(ob.br)
	} else {
		err = ob.zr.(zlib.Resetter).Reset(ob.br, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("read %s.pack at %d: %w", p.base, e.data, err)
	}
	out, err := readSized(ob.zr, e.size)
	if err != nil {
		return nil, fmt.Errorf("read %s.pack at %d: %w", p.base, e.data, err)
	}
	return out, nil
}

// readSized reads r to its end, which must come after exactly size bytes.
// size comes from the file, so it sizes a buffer only up to 1 MiB.
func readSized(r io.Reader, size int64) ([]byte, error) {
	var out []byte
	if size <= 1<<20 {
		out = make([]byte, size)
		if _, err := io.ReadFull(r, out); err != nil {
			if errors.Is(err, io.EOF) { // nothing at all
				err = io.ErrUnexpectedEOF
			}
			return nil, fmt.Errorf("shorter than the %d bytes its header says: %w", size, err)
		}
	} else {
		var err error
		if out, err = io.ReadAll(io.LimitReader(r, size)); err != nil {
			return nil, err
		}
		if int64(len(out)) != size {
			return nil, fmt.Errorf("%d bytes, header says %d: %w", len(out), size, io.ErrUnexpectedEOF)
		}
	}
	// Reading to the end makes zlib check its checksum.
	var extra [1]byte
	switch n, err := io.ReadFull(r, extra[:]); {
	case n > 0:
		return nil, fmt.Errorf("longer than the %d bytes its header says", size)
	case !errors.Is(err, io.EOF):
		return nil, err
	}
	return out, nil
}
