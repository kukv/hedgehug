package index

import (
	"iter"
	"regexp"
	"slices"
	"strings"

	"github.com/kukv/hedgehug/internal/model"
)

// mentionRe finds @login and @host/login where the @ does not follow a
// letter, digit or one of the characters of a mail address or a path. A
// login does not end with a hyphen.
var mentionRe = regexp.MustCompile(`(?:^|[^A-Za-z0-9_.@/-])@((?:[A-Za-z0-9][A-Za-z0-9.-]*/)?[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?)`)

// taskRefRe finds #<ID> where the # does not follow a letter, digit or
// underscore. TaskRefs checks what follows the id.
var taskRefRe = regexp.MustCompile(`(?:^|[^A-Za-z0-9_])#([A-Z][A-Z0-9]{1,9}-[1-9][0-9]*)`)

// Mentions returns the accounts body mentions, as written (login or
// host/login), once each, in the order they first appear. Code is left
// out, as prose reads it.
func Mentions(body string) []string {
	var out []string
	for l := range prose(body) {
		for _, m := range mentionRe.FindAllStringSubmatch(l, -1) {
			if !slices.Contains(out, m[1]) {
				out = append(out, m[1])
			}
		}
	}
	return out
}

// TaskRefs returns the ids body refers to as #<ID>, such as #WEB-42, once
// each, in the order they first appear. An id followed by a letter, digit,
// underscore or hyphen is not one. Code is left out, as prose reads it.
func TaskRefs(body string) []string {
	var out []string
	for l := range prose(body) {
		for _, m := range taskRefRe.FindAllStringSubmatchIndex(l, -1) {
			if end := m[3]; end < len(l) && (isAlnum(l[end]) || l[end] == '_' || l[end] == '-') {
				continue
			}
			if id := l[m[2]:m[3]]; !slices.Contains(out, id) {
				out = append(out, id)
			}
		}
	}
	return out
}

// Backlinks returns the tasks that refer to ulid by a ref, in ULID order: a
// ref points at the task Resolve finds for it, so by its id first, then by
// a former id. The task itself and deleted tasks are left out.
func (ix *Index) Backlinks(ulid model.ULID) []*Entry {
	target, ok := ix.tasks[ulid]
	if !ok {
		return nil
	}
	ids := append([]string{target.ID}, target.FormerIDs...)
	points := map[string]bool{} // by ref, whether it points at ulid
	var out []*Entry
	for _, e := range ix.tasks {
		if e.ULID == ulid || e.DeletedAt != nil {
			continue
		}
		for _, r := range e.Refs {
			if !slices.ContainsFunc(ids, func(id string) bool { return strings.EqualFold(id, r) }) {
				continue
			}
			p, seen := points[r]
			if !seen {
				got, err := ix.Resolve(r)
				p = err == nil && got == ulid
				points[r] = p
			}
			if p {
				out = append(out, e)
				break
			}
		}
	}
	slices.SortFunc(out, func(a, b *Entry) int { return strings.Compare(a.ULID.String(), b.ULID.String()) })
	return out
}

func isAlnum(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9'
}

// MentionNames reports whether the mention m, as written, names the account
// a: @login any account with that login, @host/login that account. A bare
// login a, as a query may give, is named by a mention of that login on any
// host. Case does not count.
func MentionNames(m, a string) bool {
	if !strings.Contains(a, "/") {
		return strings.EqualFold(m[strings.LastIndex(m, "/")+1:], a)
	}
	if strings.Contains(m, "/") {
		return strings.EqualFold(m, a)
	}
	return strings.EqualFold(m, a[strings.LastIndex(a, "/")+1:])
}

// prose returns the lines of body that are not code, with their inline
// code blanked. Code is as CommonMark reads it: fenced code blocks,
// indented code blocks and inline code. The lines of a list item are read
// from the item's content column, so its continuation is text and only
// what is indented four more is code.
func prose(body string) iter.Seq[string] {
	return func(yield func(string) bool) { walkProse(body, yield) }
}

func walkProse(body string, yield func(string) bool) {
	fence := ""       // the opening fence of the fenced block the line is in
	indented := false // in an indented code block
	canIndent := true // an indented line here starts a code block: no paragraph goes on
	list := 0         // the content column of the list item the lines go on; 0 outside a list
	for line := range strings.SplitSeq(body, "\n") {
		if fence != "" {
			if closesFence(dedent(line, list), fence) {
				fence, canIndent = "", true
			}
			continue
		}
		if strings.TrimSpace(line) == "" {
			canIndent = true
			continue
		}
		base := list
		if indent(line) < list {
			base = 0
			// A line left of the item ends the list after a blank line, or
			// when it starts a block; only a paragraph's text goes on lazily.
			if canIndent || startsBlock(line) {
				list = 0
			}
		}
		l := dedent(line, base)
		if f := openingFence(l); f != "" {
			fence, indented = f, false
			continue
		}
		if indent(l) >= 4 && (canIndent || indented) {
			indented, canIndent = true, false
			continue
		}
		indented, canIndent = false, false
		if w := listItem(l); w > 0 {
			list = base + w
		} else if atxHeading.MatchString(l) {
			canIndent = true // a heading is no paragraph to go on
		}
		if !yield(stripInlineCode(l)) {
			return
		}
	}
}

var (
	atxHeading = regexp.MustCompile(`^ {0,3}#{1,6}(?:[ \t]|$)`)
	listMarker = regexp.MustCompile(`^( {0,3})([-*+]|[0-9]{1,9}[.)])( *)`)
)

// startsBlock reports whether line starts a block that ends a paragraph:
// an ATX heading, a thematic break, a fence or a list item.
func startsBlock(line string) bool {
	return atxHeading.MatchString(line) || thematicBreak(line) || openingFence(line) != "" || listItem(line) > 0
}

// thematicBreak reports whether line is three or more of one of - * _,
// after at most three spaces, with only spaces between them.
func thematicBreak(line string) bool {
	l := strings.TrimLeft(line, " ")
	if len(line)-len(l) > 3 || l == "" || !strings.ContainsRune("-*_", rune(l[0])) {
		return false
	}
	rest := strings.ReplaceAll(strings.TrimRight(l, " \t"), " ", "")
	return len(rest) >= 3 && strings.Trim(rest, l[:1]) == ""
}

// listItem returns the content column of the list item line starts, or 0
// when it starts none: the marker and one to four spaces after it (one when
// more follow, as the content is then code).
func listItem(line string) int {
	m := listMarker.FindStringSubmatch(line)
	if m == nil {
		return 0
	}
	n := len(m[1]) + len(m[2])
	switch rest := line[len(m[0]):]; {
	case rest == "":
		return n + 1
	case m[3] == "":
		return 0 // -x is no list item
	case len(m[3]) > 4:
		return n + 1
	}
	return n + len(m[3])
}

// dedent removes up to n columns of the line's leading spaces and tabs.
func dedent(line string, n int) string {
	w := 0
	for i, r := range line {
		if w >= n || r != ' ' && r != '\t' {
			return line[i:]
		}
		if r == '\t' {
			w += 4
		} else {
			w++
		}
	}
	return ""
}

// indent is the width of the line's leading spaces and tabs, a tab
// counting four.
func indent(line string) int {
	n := 0
	for _, r := range line {
		switch r {
		case ' ':
			n++
		case '\t':
			n += 4
		default:
			return n
		}
	}
	return n
}

// openingFence returns the run of three or more backticks or tildes that
// opens a fenced code block on line, after at most three spaces, or "".
func openingFence(line string) string {
	l := strings.TrimLeft(line, " ")
	if len(line)-len(l) > 3 || l == "" || l[0] != '`' && l[0] != '~' {
		return ""
	}
	n := len(l) - len(strings.TrimLeft(l, l[:1]))
	if n < 3 || l[0] == '`' && strings.Contains(l[n:], "`") {
		return "" // a backtick fence's info string has no backtick
	}
	return l[:n]
}

// closesFence reports whether line closes the block fence opened: at most
// three spaces, a run of the same character at least as long, then only
// spaces.
func closesFence(line, fence string) bool {
	l := strings.TrimLeft(line, " ")
	if len(line)-len(l) > 3 {
		return false
	}
	rest := strings.TrimLeft(l, fence[:1])
	return len(l)-len(rest) >= len(fence) && strings.TrimRight(rest, " \t") == ""
}

// stripInlineCode blanks the code spans of line: a run of backticks up to
// the next run of the same length. A run that closes nothing is text.
func stripInlineCode(line string) string {
	var b strings.Builder
	for i := 0; i < len(line); {
		if line[i] != '`' {
			b.WriteByte(line[i])
			i++
			continue
		}
		n := len(line[i:]) - len(strings.TrimLeft(line[i:], "`"))
		if end := closingRun(line[i+n:], n); end >= 0 {
			b.WriteByte(' ')
			i += n + end + n
			continue
		}
		b.WriteString(line[i : i+n])
		i += n
	}
	return b.String()
}

// closingRun returns where in s the first run of exactly n backticks
// starts, or -1.
func closingRun(s string, n int) int {
	for i := 0; i < len(s); {
		if s[i] != '`' {
			i++
			continue
		}
		m := len(s[i:]) - len(strings.TrimLeft(s[i:], "`"))
		if m == n {
			return i
		}
		i += m
	}
	return -1
}
