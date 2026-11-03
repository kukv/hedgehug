package model

import (
	"errors"
	"strings"
)

const fence = "---\n"

// normalizeNewlines turns CRLF into LF. Editors on Windows may save CRLF.
// It repeats until no CRLF is left so that it is idempotent: "\r\r\n" must
// not leave a new "\r\n" behind.
func normalizeNewlines(s string) string {
	for strings.Contains(s, "\r\n") {
		s = strings.ReplaceAll(s, "\r\n", "\n")
	}
	return s
}

// splitFrontmatter splits a Markdown file into its YAML frontmatter and its
// body. The file must start with a "---" line, and the next "---" line ends
// the frontmatter.
func splitFrontmatter(data []byte) (front []byte, body string, err error) {
	s := normalizeNewlines(string(data))
	rest, ok := strings.CutPrefix(s, fence)
	if !ok {
		return nil, "", &DecodeError{Field: "frontmatter", Line: 1, Err: errors.New("missing: the file must start with ---")}
	}
	if b, ok := strings.CutPrefix(rest, fence); ok {
		return nil, b, nil
	}
	if rest == "---" {
		return nil, "", nil
	}
	if i := strings.Index(rest, "\n"+fence); i >= 0 {
		return []byte(rest[:i+1]), rest[i+1+len(fence):], nil
	}
	if f, ok := strings.CutSuffix(rest, "\n---"); ok {
		return []byte(f + "\n"), "", nil
	}
	return nil, "", &DecodeError{Field: "frontmatter", Line: 1, Err: errors.New("unterminated: no closing ---")}
}

// joinFrontmatter is the reverse of splitFrontmatter. front must end in a
// newline.
func joinFrontmatter(front []byte, body string) []byte {
	return []byte(fence + string(front) + fence + normalizeNewlines(body))
}
