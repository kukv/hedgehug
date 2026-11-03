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

// Every hash of a bucket of many is found, from the bucket read once.
func TestPackFindInALargeBucket(t *testing.T) {
	t.Parallel()
	offsets := map[plumbing.Hash]int64{}
	var hs []plumbing.Hash
	for i := range 1000 {
		var h plumbing.Hash
		h[0] = 0x42
		binary.BigEndian.PutUint32(h[1:], uint32(i*7)) //nolint:gosec // G115: small
		offsets[h] = int64(12 + i)
		hs = append(hs, h)
	}
	base := filepath.Join(t.TempDir(), "pack-x")
	writeIdx(t, base, offsets)
	p, err := openPack(base)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.close() }()
	for _, h := range hs {
		got, ok, err := p.find(h)
		if err != nil || !ok || got != offsets[h] {
			t.Fatalf("find(%s) = %d, %v, %v; want %d", h, got, ok, err, offsets[h])
		}
		missing := h
		missing[19] = 1 // between two hashes of the bucket
		if _, ok, err := p.find(missing); err != nil || ok {
			t.Fatalf("find(%s) = %v, %v; want not found", missing, ok, err)
		}
	}
	var past plumbing.Hash
	past[0], past[1] = 0x42, 0xff // after every hash of the bucket
	if _, ok, err := p.find(past); err != nil || ok {
		t.Errorf("find(%s) = %v, %v; want not found", past, ok, err)
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

// A fanout that decreases, or that exceeds the object count, is an error and
// not a huge read.
func TestOpenPackBadFanout(t *testing.T) {
	t.Parallel()
	h := plumbing.NewHash("7f00000000000000000000000000000000000001") // in bucket 0x7f
	for name, set := range map[string][2]uint32{
		"decreasing":    {0x7e, 5}, // fanout[0x7e] = 5 > fanout[0x7f] = 1
		"too large":     {0x7f, 1000},
		"past the file": {0xff, 1 << 30}, // more objects than the index has room for
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			base := filepath.Join(t.TempDir(), "pack-x")
			writeIdx(t, base, map[plumbing.Hash]int64{h: 12})
			b, err := os.ReadFile(base + ".idx")
			if err != nil {
				t.Fatal(err)
			}
			binary.BigEndian.PutUint32(b[idxFanout+4*int(set[0]):], set[1])
			if err := os.WriteFile(base+".idx", b, 0o600); err != nil {
				t.Fatal(err)
			}
			if p, err := openPack(base); err == nil {
				_ = p.close()
				t.Error("openPack of a bad fanout succeeded")
			}
		})
	}
}

// A size varint that overflows int64 is an error.
func TestPackEntryAtBadSize(t *testing.T) {
	t.Parallel()
	base := filepath.Join(t.TempDir(), "pack-x")
	writeIdx(t, base, map[plumbing.Hash]int64{plumbing.NewHash("0000000000000000000000000000000000000001"): 12})
	head := []byte{0xbf, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f}
	if err := os.WriteFile(base+".pack", head, 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := openPack(base)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.close() }()
	if _, _, err := p.entryAt(0, make([]byte, readAhead)); err == nil {
		t.Error("entryAt of an overflowing size succeeded")
	}
}
