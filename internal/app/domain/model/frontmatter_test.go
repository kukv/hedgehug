package model

import (
	"errors"
	"testing"
)

func TestSplitFrontmatter(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, in, front, body string
	}{
		{"basic", "---\na: 1\n---\nbody\n", "a: 1\n", "body\n"},
		{"crlf", "---\r\na: 1\r\n---\r\nbody\r\n", "a: 1\n", "body\n"},
		{"empty front", "---\n---\nbody", "", "body"},
		{"no body", "---\na: 1\n---\n", "a: 1\n", ""},
		{"closing at EOF", "---\na: 1\n---", "a: 1\n", ""},
		{"body with a rule", "---\na: 1\n---\nx\n---\ny\n", "a: 1\n", "x\n---\ny\n"},
	} {
		front, body, err := splitFrontmatter([]byte(tc.in))
		if err != nil || string(front) != tc.front || body != tc.body {
			t.Errorf("%s: split = %q, %q, %v; want %q, %q", tc.name, front, body, err, tc.front, tc.body)
		}
	}
	for _, bad := range []string{"a: 1\n", "---\na: 1\n", ""} {
		_, _, err := splitFrontmatter([]byte(bad))
		var de *DecodeError
		if !errors.As(err, &de) || de.Field != "frontmatter" || de.Line != 1 {
			t.Errorf("split(%q) error = %#v, want DecodeError frontmatter at line 1", bad, err)
		}
	}
}

func TestJoinFrontmatter(t *testing.T) {
	t.Parallel()
	if got, want := string(joinFrontmatter([]byte("a: 1\n"), "body\r\n")), "---\na: 1\n---\nbody\n"; got != want {
		t.Errorf("join = %q, want %q", got, want)
	}
	if got, want := string(joinFrontmatter([]byte("a: 1\n"), "")), "---\na: 1\n---\n"; got != want {
		t.Errorf("join = %q, want %q", got, want)
	}
}
