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

// packEntry is the head of an object in a pack.
type packEntry struct {
	typ     plumbing.ObjectType
	size    int64         // of the inflated data; of the delta for a delta
	data    int64         // offset of the zlib data
	baseOff int64         // an OFS delta's base, in the same pack
	baseRef plumbing.Hash // a REF delta's base
}

// entryAt reads the head of the object at off.
func (p *pack) entryAt(off int64) (packEntry, error) {
	// The type and size take at most 10 bytes, and an OFS delta's offset
	// at most 10 or a REF delta's hash 20 more.
	var b [32]byte
	n, err := p.data.ReadAt(b[:], off)
	if err != nil && (!errors.Is(err, io.EOF) || n == 0) {
		return packEntry{}, fmt.Errorf("read %s.pack at %d: %w", p.base, off, err)
	}
	r := headReader{buf: b[:n]}
	c := r.next()
	e := packEntry{typ: plumbing.ObjectType((c >> 4) & 7), size: int64(c & 15)}
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
			return packEntry{}, fmt.Errorf("%s.pack at %d: bad delta base offset", p.base, off)
		}
	case plumbing.REFDeltaObject:
		copy(e.baseRef[:], r.take(20))
	}
	if r.short {
		return packEntry{}, fmt.Errorf("%s.pack at %d: %w", p.base, off, io.ErrUnexpectedEOF)
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
func (p *pack) inflate(e packEntry) ([]byte, error) {
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
