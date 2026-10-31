package index

import (
	"regexp"
	"slices"
	"strings"
)

// mentionRe finds @login and @host/login where the @ does not follow a
// letter, digit or one of the characters of a mail address or a path. A
// login does not end with a hyphen.
var mentionRe = regexp.MustCompile(`(?:^|[^A-Za-z0-9_.@/-])@((?:[A-Za-z0-9][A-Za-z0-9.-]*/)?[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?)`)

// Mentions returns the accounts body mentions, as written (login or
// host/login), once each, in the order they first appear. Code is left
// out, as CommonMark reads it: fenced code blocks, indented code blocks and
// inline code.
func Mentions(body string) []string {
	var out []string
	fence := ""       // the opening fence of the fenced block the line is in
	indented := false // in an indented code block
	canIndent := true // an indented line here starts a code block: no paragraph goes on
	for line := range strings.SplitSeq(body, "\n") {
		if fence != "" {
			if closesFence(line, fence) {
				fence, canIndent = "", true
			}
			continue
		}
		if f := openingFence(line); f != "" {
			fence, indented = f, false
			continue
		}
		if strings.TrimSpace(line) == "" {
			canIndent = true
			continue
		}
		if indent(line) >= 4 && (canIndent || indented) {
			indented, canIndent = true, false
			continue
		}
		indented, canIndent = false, false
		for _, m := range mentionRe.FindAllStringSubmatch(stripInlineCode(line), -1) {
			if !slices.Contains(out, m[1]) {
				out = append(out, m[1])
			}
		}
	}
	return out
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
