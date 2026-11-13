package textmerge

import "testing"

func TestMergeReportsWhetherBothVersionsWereKept(t *testing.T) {
	cases := []struct {
		name, base, older, newer, want string
		conflict                       bool
	}{
		{"nothing changed", "a\nb\n", "a\nb\n", "a\nb\n", "a\nb\n", false},
		{"only the newer changed", "a\nb\n", "a\nb\n", "a\nB\n", "a\nB\n", false},
		{"only the older changed", "a\nb\n", "A\nb\n", "a\nb\n", "A\nb\n", false},
		{"both changed to the same", "a\nb\n", "a\nX\n", "a\nX\n", "a\nX\n", false},
		{"different lines", "a\nb\nc\n", "A\nb\nc\n", "a\nb\nC\n", "A\nb\nC\n", false},
		{"the same line", "a\nb\nc\n", "a\nX\nc\n", "a\nY\nc\n", "a\nX\n-----\nY\nc\n", true},
		{"both insert at the same position", "a\nb\n", "a\nx\nb\n", "a\ny\nb\n", "a\nx\n-----\ny\nb\n", true},
		{"both wrote to an empty description", "", "x\n", "y\n", "x\n-----\ny\n", true},
	}
	for _, c := range cases {
		got, conflict := Merge(c.base, c.older, c.newer)
		if got != c.want || conflict != c.conflict {
			t.Errorf("%s: got (%q, %v), want (%q, %v)", c.name, got, conflict, c.want, c.conflict)
		}
		if text := MergeText(c.base, c.older, c.newer); text != got {
			t.Errorf("%s: MergeText = %q, want %q", c.name, text, got)
		}
	}
}
