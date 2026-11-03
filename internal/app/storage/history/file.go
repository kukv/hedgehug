package history

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"maps"
	"slices"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/storage/store"
)

// The file is, in this order:
//
//	magic "HHHI"
//	format          uvarint (fileFormat)
//	tip             20 bytes
//	strings         uvarint count, then each as uvarint length + bytes
//	commits         uvarint count, then each: hash (20 bytes), author time
//	                (varint Unix seconds), zone offset (varint seconds),
//	                gen (uvarint), parent (uvarint: 1 + the index of the
//	                first parent, 0 for none), then the string indices
//	                (uvarint) of Author, Email, Op, Task and Workspace
//	paths           uvarint count, then each: string index of the path,
//	                uvarint count of commits, then the commit indices
//	                ascending: the first as is, the rest as the difference
//	                from the one before
//	file id         the CRC-32 (IEEE, 4 bytes little endian) of all the
//	                above; it names the file, numbering and all
//	chunks          none or more, one for each time the index was brought
//	                to a newer HEAD and added to the file instead of
//	                writing it again: uvarint length of the body, the body,
//	                the CRC-32 (IEEE, 4 bytes little endian) of the body.
//	                The body is: the tip before (20 bytes), the tip after
//	                (20 bytes), the file id and the numbers of commits and
//	                of strings before it (4 bytes, uvarint each; the file it
//	                goes on is that file, not only one of that tip, as
//	                another file of the tip may number its strings
//	                otherwise), the new
//	                strings as above, the new commits as
//	                above (their parents and string indices count those
//	                before them), and the paths they changed, each: the
//	                path (uvarint length + bytes), the number of the new
//	                commits, their indices as above
//
// Anything that does not fit in the first part (magic, format, truncation,
// an index out of range, indices not ascending) makes load report false.
// The chunks are only a gain: the first one that does not fit (cut short,
// wrong checksum, not on the file before it, out of range) ends the file for
// the reader, who then has an older index and brings it to HEAD; the next
// save writes the file again.
const (
	fileMagic  = "HHHI"
	fileFormat = 5
	hashLen    = 20

	maxZoneOffset = 18 * 60 * 60 // seconds; an offset beyond ±18h is damage

	// The file is written again, not added to, when it has this many chunks
	// or the chunks are this part (1/maxChunkShare) of it.
	maxChunks     = 100
	maxChunkShare = 2
)

// putU and putI append into the buffer's spare capacity, so a number does
// not allocate.
func putU(b *bytes.Buffer, v uint64) { b.Write(binary.AppendUvarint(b.AvailableBuffer(), v)) }
func putI(b *bytes.Buffer, v int64)  { b.Write(binary.AppendVarint(b.AvailableBuffer(), v)) }

func putRec(b *bytes.Buffer, r rec) {
	b.Write(r.hash[:])
	putI(b, r.sec)
	putI(b, int64(r.zone))
	putU(b, uint64(r.gen))
	putU(b, uint64(r.parent))
	for _, v := range []uint32{r.author, r.email, r.op, r.task, r.workspace} {
		putU(b, uint64(v))
	}
}

func putStrings(b *bytes.Buffer, list []string) {
	putU(b, uint64(len(list)))
	for _, v := range list {
		putU(b, uint64(len(v)))
		b.WriteString(v)
	}
}

// putIndices writes ascending indices: the first as is, then the differences.
func putIndices(b *bytes.Buffer, idx []int) {
	putU(b, uint64(len(idx)))
	prev := 0
	for _, i := range idx {
		putU(b, uint64(i-prev)) //nolint:gosec // ascending
		prev = i
	}
}

// save writes the index: added to the file when it was loaded from one that
// is in order and is not due to be written again, else all of it. A failure
// is not told: the next Open does the work again.
func (h *History) save(s *store.Store) {
	if h.canAppend() && h.appendChunk(s) == nil {
		return
	}
	h.saveAll(s)
}

func (h *History) canAppend() bool {
	return h.fromFile && !h.tailBad && h.chunks < maxChunks && len(h.recs) > h.savedRecs &&
		(h.chunkBytes+1)*maxChunkShare < h.mainBytes
}

func (h *History) saveAll(s *store.Store) {
	// The paths take the numbers after the strings of the commits, so that
	// the table is the one a chunk goes on from.
	paths := slices.Sorted(maps.Keys(h.byPath)) // so that the same index always gives the same bytes
	for _, p := range paths {
		h.intern(p)
	}
	var buf bytes.Buffer
	buf.WriteString(fileMagic)
	putU(&buf, fileFormat)
	buf.Write(h.tip[:])
	putStrings(&buf, h.strs)
	putU(&buf, uint64(len(h.recs)))
	for _, r := range h.recs {
		putRec(&buf, r)
	}
	putU(&buf, uint64(len(paths)))
	for _, p := range paths {
		putU(&buf, uint64(h.intern(p)))
		putIndices(&buf, h.byPath[p])
	}
	id := crc32.ChecksumIEEE(buf.Bytes())
	buf.Write(binary.LittleEndian.AppendUint32(nil, id))
	if s.WriteLocal(fileName, buf.Bytes()) != nil {
		return
	}
	h.fromFile, h.tailBad, h.fileID = true, false, id
	h.mainBytes, h.chunkBytes, h.chunks = buf.Len(), 0, 0
	h.savedRecs, h.savedStrs, h.savedTip = len(h.recs), len(h.strs), h.tip
	h.pending = nil
}

// appendChunk adds the commits put since the file was read or written.
func (h *History) appendChunk(s *store.Store) error {
	var body bytes.Buffer
	body.Write(h.savedTip[:])
	body.Write(h.tip[:])
	body.Write(binary.LittleEndian.AppendUint32(nil, h.fileID))
	putU(&body, uint64(h.savedRecs)) //nolint:gosec // not negative
	putU(&body, uint64(h.savedStrs)) //nolint:gosec // not negative
	putStrings(&body, h.strs[h.savedStrs:])
	putU(&body, uint64(len(h.recs)-h.savedRecs)) //nolint:gosec // not negative
	for _, r := range h.recs[h.savedRecs:] {
		putRec(&body, r)
	}
	putU(&body, uint64(len(h.pending)))
	for _, p := range slices.Sorted(maps.Keys(h.pending)) {
		putU(&body, uint64(len(p)))
		body.WriteString(p)
		putIndices(&body, h.pending[p])
	}
	var chunk bytes.Buffer
	putU(&chunk, uint64(body.Len())) //nolint:gosec // not negative
	chunk.Write(body.Bytes())
	chunk.Write(binary.LittleEndian.AppendUint32(nil, crc32.ChecksumIEEE(body.Bytes())))
	if err := s.AppendLocal(fileName, chunk.Bytes()); err != nil {
		return err
	}
	h.chunks++
	h.chunkBytes += chunk.Len()
	h.savedRecs, h.savedStrs, h.savedTip = len(h.recs), len(h.strs), h.tip
	h.pending = nil
	return nil
}

// reader reads the file; after the first problem bad is set and every read
// returns zero.
type reader struct {
	b   []byte
	bad bool
}

func (r *reader) uvarint() uint64 {
	v, n := binary.Uvarint(r.b)
	if n <= 0 {
		r.bad = true
		return 0
	}
	r.b = r.b[n:]
	return v
}

func (r *reader) varint() int64 {
	v, n := binary.Varint(r.b)
	if n <= 0 {
		r.bad = true
		return 0
	}
	r.b = r.b[n:]
	return v
}

func (r *reader) bytes(n uint64) []byte {
	if n > uint64(len(r.b)) {
		r.bad = true
		return nil
	}
	out := r.b[:n]
	r.b = r.b[n:]
	return out
}

// count reads a number of items that each take at least one byte, so that
// a damaged number cannot ask for more memory than the file holds.
func (r *reader) count() int {
	n := r.uvarint()
	if n > uint64(len(r.b)) {
		r.bad = true
		return 0
	}
	return int(n) //nolint:gosec // at most len(r.b)
}

func (r *reader) strings() []string {
	strs := make([]string, r.count())
	for i := range strs {
		strs[i] = string(r.bytes(r.uvarint()))
	}
	return strs
}

// rec reads a commit whose parent and strings are among the nrecs commits
// before it and the nstrs strings; false when it does not fit.
func (r *reader) rec(nrecs, nstrs int) (rec, bool) {
	var c rec
	copy(c.hash[:], r.bytes(hashLen))
	c.sec = r.varint()
	off := r.varint()
	if off < -maxZoneOffset || off > maxZoneOffset {
		return c, false
	}
	c.zone = int32(off)
	c.gen = uint32(r.uvarint()) //nolint:gosec // a generation
	p := r.uvarint()
	if p > uint64(nrecs) { //nolint:gosec // not negative
		return c, false
	}
	c.parent = uint32(p) //nolint:gosec // at most nrecs
	var ids [5]uint32
	for i := range ids {
		v := r.uvarint()
		if v >= uint64(nstrs) { //nolint:gosec // not negative
			return c, false
		}
		ids[i] = uint32(v) //nolint:gosec // below nstrs
	}
	c.author, c.email, c.op, c.task, c.workspace = ids[0], ids[1], ids[2], ids[3], ids[4]
	return c, !r.bad
}

// indices reads what putIndices wrote: ascending, from first up to below
// limit.
func (r *reader) indices(limit int) ([]int, bool) {
	m := r.count()
	idx := make([]int, 0, m)
	prev := 0
	for j := range m {
		d := r.uvarint()
		if d >= uint64(limit) || (j > 0 && d == 0) { //nolint:gosec // not negative
			return nil, false
		}
		prev += int(d) //nolint:gosec // below limit
		if prev >= limit {
			return nil, false
		}
		idx = append(idx, prev)
	}
	return idx, !r.bad
}

// load reads the saved index into h and reports whether it could. On false
// h is not to be used.
func (h *History) load(s *store.Store) bool {
	data, err := s.ReadLocal(fileName)
	if err != nil || len(data) < len(fileMagic) || string(data[:len(fileMagic)]) != fileMagic {
		return false
	}
	r := &reader{b: data[len(fileMagic):]}
	if r.uvarint() != fileFormat {
		return false
	}
	copy(h.tip[:], r.bytes(hashLen))
	h.strs = r.strings()
	n := r.count()
	h.recs = make([]rec, 0, n)
	for range n {
		c, ok := r.rec(len(h.recs), len(h.strs))
		if !ok {
			return false
		}
		h.recs = append(h.recs, c)
	}
	np := r.count()
	for range np {
		i := r.uvarint()
		if i >= uint64(len(h.strs)) {
			return false
		}
		idx, ok := r.indices(len(h.recs))
		if !ok {
			return false
		}
		h.byPath[h.strs[i]] = idx
	}
	if r.bad {
		return false
	}
	h.fileID = crc32.ChecksumIEEE(data[:len(data)-len(r.b)])
	if id := r.bytes(4); r.bad || binary.LittleEndian.Uint32(id) != h.fileID {
		return false
	}
	h.mainBytes = len(data) - len(r.b)
	h.fromFile = true
	h.savedRecs, h.savedStrs, h.savedTip = len(h.recs), len(h.strs), h.tip
	for len(r.b) > 0 {
		before := len(r.b)
		if !h.applyChunk(r) {
			h.tailBad = true
			break
		}
		h.chunks++
		h.chunkBytes += before - len(r.b)
	}
	h.savedRecs, h.savedStrs, h.savedTip = len(h.recs), len(h.strs), h.tip
	if h.tip.IsZero() {
		return len(h.recs) == 0
	}
	// the tip was the last commit added, or near it
	for i := len(h.recs) - 1; i >= 0; i-- {
		if h.recs[i].hash == h.tip {
			return true
		}
	}
	return false
}

// applyChunk adds the chunk at the start of r to h, or changes nothing and
// reports false.
func (h *History) applyChunk(r *reader) bool {
	n := r.uvarint()
	body := r.bytes(n)
	sum := r.bytes(4)
	if r.bad || binary.LittleEndian.Uint32(sum) != crc32.ChecksumIEEE(body) {
		return false
	}
	b := &reader{b: body}
	var before, after plumbing.Hash
	copy(before[:], b.bytes(hashLen))
	copy(after[:], b.bytes(hashLen))
	if b.bad || before != h.tip {
		return false
	}
	if id := b.bytes(4); b.bad || binary.LittleEndian.Uint32(id) != h.fileID {
		return false
	}
	if b.uvarint() != uint64(len(h.recs)) || b.uvarint() != uint64(len(h.strs)) || b.bad {
		return false
	}
	strs := b.strings()
	nstrs := len(h.strs) + len(strs)
	m := b.count()
	recs := make([]rec, 0, m)
	for range m {
		c, ok := b.rec(len(h.recs)+len(recs), nstrs)
		if !ok {
			return false
		}
		recs = append(recs, c)
	}
	total := len(h.recs) + len(recs)
	np := b.count()
	paths := make(map[string][]int, np)
	for range np {
		p := string(b.bytes(b.uvarint()))
		idx, ok := b.indices(total)
		if !ok || len(idx) > 0 && idx[0] < len(h.recs) {
			return false
		}
		paths[p] = idx
	}
	if b.bad || len(b.b) != 0 {
		return false
	}
	h.strs = append(h.strs, strs...)
	h.recs = append(h.recs, recs...)
	for p, idx := range paths {
		h.byPath[p] = append(h.byPath[p], idx...)
	}
	h.tip = after
	return true
}
