package filemerge

import "testing"

func TestMergeText(t *testing.T) {
	base := "a\nb\nc\n"
	cases := []struct {
		name, older, newer, want string
	}{
		{"only one side changed", base, "a\nb\nC\n", "a\nb\nC\n"},
		{"both changed to the same", "a\nB\nc\n", "a\nB\nc\n", "a\nB\nc\n"},
		{"different lines are both kept", "A\nb\nc\n", "a\nb\nC\n", "A\nb\nC\n"},
		{"same line keeps both, older first", "a\nX\nc\n", "a\nY\nc\n", "a\nX\n-----\nY\nc\n"},
		{"added lines at different places", "a\nb\nb2\nc\n", "a0\na\nb\nc\n", "a0\na\nb\nb2\nc\n"},
		{"one deletes a line, other edits another", "a\nc\n", "a\nb\nC\n", "a\nC\n"},
	}
	for _, c := range cases {
		if got := MergeText(base, c.older, c.newer); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestMergeTextFromEmptyBase(t *testing.T) {
	// 説明のなかったものに、2 人が別々に書いた。同じ箇所なので両方を残す。
	if got := MergeText("", "x\n", "y\n"); got != "x\n-----\ny\n" {
		t.Errorf("got %q", got)
	}
}
