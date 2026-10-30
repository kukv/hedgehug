package model

import "testing"

func TestSplitFrontmatter(t *testing.T) {
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
		if _, _, err := splitFrontmatter([]byte(bad)); err == nil {
			t.Errorf("split(%q) succeeded", bad)
		}
	}
}

func TestJoinFrontmatter(t *testing.T) {
	if got, want := string(joinFrontmatter([]byte("a: 1\n"), "body\r\n")), "---\na: 1\n---\nbody\n"; got != want {
		t.Errorf("join = %q, want %q", got, want)
	}
	if got, want := string(joinFrontmatter([]byte("a: 1\n"), "")), "---\na: 1\n---\n"; got != want {
		t.Errorf("join = %q, want %q", got, want)
	}
}
