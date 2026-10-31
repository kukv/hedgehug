package index

import (
	"regexp"
	"slices"
	"strings"
)

var (
	// mentionRe finds @login and @host/login where the @ does not follow a
	// letter, digit or one of the characters of a mail address or a path.
	mentionRe = regexp.MustCompile(`(?:^|[^A-Za-z0-9_.@/-])@((?:[A-Za-z0-9][A-Za-z0-9.-]*/)?[A-Za-z0-9][A-Za-z0-9-]*)`)
	// inlineCodeRe is a span of inline code on one line.
	inlineCodeRe = regexp.MustCompile("`[^`]*`")
)

// Mentions returns the accounts body mentions, as written (login or
// host/login), once each, in the order they first appear. Fenced code
// blocks and inline code are left out.
func Mentions(body string) []string {
	var out []string
	fence := ""
	for line := range strings.SplitSeq(body, "\n") {
		if f := fenceOf(line); f != "" && (fence == "" || f == fence) {
			if fence == "" {
				fence = f
			} else {
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		for _, m := range mentionRe.FindAllStringSubmatch(inlineCodeRe.ReplaceAllString(line, " "), -1) {
			if !slices.Contains(out, m[1]) {
				out = append(out, m[1])
			}
		}
	}
	return out
}

// fenceOf returns the fence a line of a fenced code block starts or ends
// with, ``` or ~~~, or "" when the line is not one.
func fenceOf(line string) string {
	l := strings.TrimLeft(line, " ")
	for _, f := range []string{"```", "~~~"} {
		if strings.HasPrefix(l, f) {
			return f
		}
	}
	return ""
}
