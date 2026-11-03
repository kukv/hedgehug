package merge

import (
	"strings"
	"unicode/utf8"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// separator is the line put between the two versions of an overlap.
const separator = "-----\n"

// Diff3 merges text line by line. Overlapping changes keep both, older first.
func Diff3(base, a, b string, aFirst bool) string {
	return merge3(base, a, b, func(x, y string) string {
		if !aFirst {
			x, y = y, x
		}
		if x != "" && !strings.HasSuffix(x, "\n") {
			x += "\n"
		}
		return x + separator + y
	})
}

// merge3 merges a and b into base line by line. Where their changes overlap,
// conflict gets the two versions of the overlap and returns the text to use.
func merge3(base, a, b string, conflict func(a, b string) string) string {
	lines := splitLines(base)
	ha, hb := hunks(base, a), hunks(base, b)
	var out strings.Builder
	pos := 0
	for len(ha)+len(hb) > 0 {
		// The first hunk of either side opens a region, which grows while a
		// hunk of either side overlaps it.
		var ra, rb []hunk
		var first hunk
		if len(hb) == 0 || len(ha) > 0 && before(ha[0], hb[0]) {
			first, ha = ha[0], ha[1:]
			ra = []hunk{first}
		} else {
			first, hb = hb[0], hb[1:]
			rb = []hunk{first}
		}
		start, end := first.start, first.end
		for grew := true; grew; {
			grew = false
			if len(ha) > 0 && overlaps(ha[0], start, end) {
				end = max(end, ha[0].end)
				ra, ha = append(ra, ha[0]), ha[1:]
				grew = true
			}
			if len(hb) > 0 && overlaps(hb[0], start, end) {
				end = max(end, hb[0].end)
				rb, hb = append(rb, hb[0]), hb[1:]
				grew = true
			}
		}
		out.WriteString(strings.Join(lines[pos:start], ""))
		va, vb := apply(lines, start, end, ra), apply(lines, start, end, rb)
		switch {
		case len(rb) == 0 || va == vb:
			out.WriteString(va)
		case len(ra) == 0:
			out.WriteString(vb)
		default:
			out.WriteString(conflict(va, vb))
		}
		pos = end
	}
	out.WriteString(strings.Join(lines[pos:], ""))
	return out.String()
}

// hunk replaces the base lines [start, end) with lines. An insertion has
// start == end.
type hunk struct {
	start, end int
	lines      string
}

// hunks returns the changes from base to x, in order.
func hunks(base, x string) []hunk {
	dmp := diffmatchpatch.New()
	dmp.DiffTimeout = 0 // a diff cut short by time would depend on the machine
	bc, xc, lineArray := dmp.DiffLinesToChars(base, x)
	diffs := dmp.DiffMain(bc, xc, false)
	text := dmp.DiffCharsToLines(diffs, lineArray)
	var hs []hunk
	pos, open := 0, false
	for i, d := range diffs {
		n := utf8.RuneCountInString(d.Text) // one rune per line
		if d.Type == diffmatchpatch.DiffEqual {
			pos += n
			open = false
			continue
		}
		if !open {
			hs = append(hs, hunk{start: pos, end: pos})
			open = true
		}
		h := &hs[len(hs)-1]
		if d.Type == diffmatchpatch.DiffDelete {
			pos += n
			h.end = pos
		} else {
			h.lines += text[i].Text
		}
	}
	return hs
}

// before orders hunks by where they start; an insertion goes before a
// range that starts at the same line.
func before(x, y hunk) bool {
	return x.start < y.start || x.start == y.start && x.end < y.end
}

// overlaps reports whether h touches the region [start, end): their ranges
// share a line, or both insert at the same place.
func overlaps(h hunk, start, end int) bool {
	return h.start < end && start < h.end || h.start == h.end && start == end && h.start == start
}

// apply returns the base lines [start, end) with the hunks, which lie in
// that range, applied.
func apply(lines []string, start, end int, hs []hunk) string {
	var b strings.Builder
	pos := start
	for _, h := range hs {
		b.WriteString(strings.Join(lines[pos:h.start], ""))
		b.WriteString(h.lines)
		pos = h.end
	}
	b.WriteString(strings.Join(lines[pos:end], ""))
	return b.String()
}

// splitLines splits s after each newline, as DiffLinesToChars does: a last
// line without a newline is a line of its own.
func splitLines(s string) []string {
	lines := strings.SplitAfter(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
