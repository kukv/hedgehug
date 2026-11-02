package history

import (
	"bytes"
	"encoding/binary"
	"maps"
	"slices"

	"github.com/kukv/hedgehug/internal/store"
)

// The file is, in this order:
//
//	magic "HHHI"
//	format          uvarint (fileFormat)
//	tip             20 bytes
//	strings         uvarint count, then each as uvarint length + bytes
//	commits         uvarint count, then each: hash (20 bytes), author time
//	                (varint Unix seconds), zone offset (varint seconds),
//	                gen (uvarint), then the string indices (uvarint) of
//	                Author, Email, Op, Task and Workspace
//	paths           uvarint count, then each: string index of the path,
//	                uvarint count of commits, then the commit indices
//	                ascending: the first as is, the rest as the difference
//	                from the one before
//
// Anything that does not fit (magic, format, truncation, an index out of
// range, indices not ascending) makes load report false.
const (
	fileMagic  = "HHHI"
	fileFormat = 1
	hashLen    = 20

	maxZoneOffset = 18 * 60 * 60 // seconds; an offset beyond ±18h is damage
)

// strTable gives each distinct string a number.
type strTable struct {
	list []string
	idx  map[string]int
}

func (t *strTable) id(s string) int {
	if i, ok := t.idx[s]; ok {
		return i
	}
	if t.idx == nil {
		t.idx = map[string]int{}
	}
	t.idx[s] = len(t.list)
	t.list = append(t.list, s)
	return len(t.list) - 1
}

// putU and putI append into the buffer's spare capacity, so a number does
// not allocate.
func putU(b *bytes.Buffer, v uint64) { b.Write(binary.AppendUvarint(b.AvailableBuffer(), v)) }
func putI(b *bytes.Buffer, v int64)  { b.Write(binary.AppendVarint(b.AvailableBuffer(), v)) }

func (h *History) save(s *store.Store) {
	// The strings of the commits come first, as the index numbers them; the
	// paths are numbered while the body is written, so the body goes to its
	// own buffer and the table is written before it.
	strs := strTable{list: slices.Clone(h.strs), idx: make(map[string]int, len(h.strs))}
	for i, v := range h.strs {
		strs.idx[v] = i
	}
	var body bytes.Buffer
	putU(&body, uint64(len(h.recs)))
	for _, r := range h.recs {
		body.Write(r.hash[:])
		putI(&body, r.sec)
		putI(&body, int64(r.zone))
		putU(&body, uint64(r.gen))
		for _, v := range []uint32{r.author, r.email, r.op, r.task, r.workspace} {
			putU(&body, uint64(v))
		}
	}
	putU(&body, uint64(len(h.byPath)))
	// Sorted, so that the same index always gives the same bytes.
	for _, p := range slices.Sorted(maps.Keys(h.byPath)) {
		idx := h.byPath[p]
		putU(&body, uint64(strs.id(p))) //nolint:gosec // an index
		putU(&body, uint64(len(idx)))
		prev := 0
		for _, i := range idx {
			putU(&body, uint64(i-prev)) //nolint:gosec // ascending
			prev = i
		}
	}

	var buf bytes.Buffer
	buf.WriteString(fileMagic)
	putU(&buf, fileFormat)
	buf.Write(h.tip[:])
	putU(&buf, uint64(len(strs.list)))
	for _, v := range strs.list {
		putU(&buf, uint64(len(v)))
		buf.WriteString(v)
	}
	buf.Write(body.Bytes())
	_ = s.WriteLocal(fileName, buf.Bytes())
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
	strs := make([]string, r.count())
	for i := range strs {
		strs[i] = string(r.bytes(r.uvarint()))
	}
	h.strs = strs
	str := func() uint32 {
		i := r.uvarint()
		if i >= uint64(len(strs)) {
			r.bad = true
			return 0
		}
		return uint32(i) //nolint:gosec // below len(strs)
	}
	n := r.count()
	h.recs = make([]rec, 0, n)
	for range n {
		var c rec
		copy(c.hash[:], r.bytes(hashLen))
		c.sec = r.varint()
		off := r.varint()
		if off < -maxZoneOffset || off > maxZoneOffset {
			return false
		}
		c.zone = int32(off)
		c.gen = uint32(r.uvarint()) //nolint:gosec // a generation
		c.author, c.email, c.op, c.task, c.workspace = str(), str(), str(), str(), str()
		if r.bad {
			return false
		}
		h.recs = append(h.recs, c)
	}
	np := r.count()
	for range np {
		p := str()
		m := r.count()
		idx := make([]int, 0, m)
		prev := 0
		for j := range m {
			d := r.uvarint()
			if (j > 0 && d == 0) || d >= uint64(len(h.recs)) {
				return false
			}
			prev += int(d) //nolint:gosec // below len(h.recs)
			if prev >= len(h.recs) {
				return false
			}
			idx = append(idx, prev)
		}
		if r.bad {
			return false
		}
		h.byPath[strs[p]] = idx
	}
	if r.bad || len(r.b) != 0 {
		return false
	}
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
