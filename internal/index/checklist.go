package index

import (
	"regexp"
	"strings"
)

// checkItem is a list item that starts with a checkbox: a bullet or a
// number, a space, [ ], [x] or [X], then a space or the end of the line.
var checkItem = regexp.MustCompile(`^[ \t]*(?:[-*+]|[0-9]{1,9}[.)])[ \t]+\[([ xX])\](?:[ \t]|$)`)

// fence opens or closes a fenced code block: three or more backticks or
// tildes after at most three spaces.
var fence = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")

// Checklist counts the checkbox items of a Markdown body, and those that are
// checked. Items inside fenced code blocks are not counted. Indented code
// blocks are not told apart from nested lists, and are counted.
func Checklist(body string) (done, total int) {
	var open string // the fence of the code block we are in, or ""
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		if m := fence.FindStringSubmatch(line); m != nil {
			f := m[1]
			switch {
			case open == "":
				open = f
			case f[0] == open[0] && len(f) >= len(open) && strings.TrimSpace(line[len(m[0]):]) == "":
				open = ""
			}
			continue
		}
		if open != "" {
			continue
		}
		if m := checkItem.FindStringSubmatch(line); m != nil {
			total++
			if m[1] != " " {
				done++
			}
		}
	}
	return done, total
}
