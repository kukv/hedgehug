package merge

import "testing"

func TestDiff3(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, base, a, b, want string }{
		{"only a", "1\n2\n3\n", "1\nA\n3\n", "1\n2\n3\n", "1\nA\n3\n"},
		{"apart", "1\n2\n3\n4\n", "A\n2\n3\n4\n", "1\n2\n3\nB\n", "A\n2\n3\nB\n"},
		{"same", "1\n2\n", "1\nX\n", "1\nX\n", "1\nX\n"},
		{"overlap", "1\n2\n3\n", "1\nA\n3\n", "1\nB\n3\n", "1\nA\n-----\nB\n3\n"},
		{"both append", "1\n", "1\nA\n", "1\nB\n", "1\nA\n-----\nB\n"},
		{"newline at end", "1", "1\n", "1", "1\n"},
		{"empty base", "", "A\n", "B\n", "A\n-----\nB\n"},
	} {
		if got := Diff3(c.base, c.a, c.b, true); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
	if got := Diff3("1\n", "1\nA\n", "1\nB\n", false); got != "1\nB\n-----\nA\n" {
		t.Errorf("b first: %q", got)
	}
}

func TestDiff3Regions(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ name, base, a, b, want string }{
		// a's one change overlaps two of b's, so all three become one region.
		{"bridge", "1\n2\n3\n4\n5\n", "1\nA\nA\nA\n5\n", "1\nB\n3\nB\n5\n", "1\nA\nA\nA\n-----\nB\n3\nB\n5\n"},
		{"delete and change", "1\n2\n3\n", "1\n3\n", "1\nB\n3\n", "1\n-----\nB\n3\n"},
		{"insert inside a change", "1\n2\n3\n4\n", "1\nA\nA\n4\n", "1\n2\nB\n3\n4\n", "1\nA\nA\n-----\n2\nB\n3\n4\n"},
		{"insert next to a change", "1\n2\n3\n", "1\nA\n3\n", "1\nB\n2\n3\n", "1\nB\nA\n3\n"},
		{"no newline at end", "1", "A", "B", "A\n-----\nB"},
	} {
		if got := Diff3(c.base, c.a, c.b, true); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
		// Swapping the sides and the order gives the same text.
		if got := Diff3(c.base, c.b, c.a, false); got != c.want {
			t.Errorf("%s swapped: got %q, want %q", c.name, got, c.want)
		}
	}
}
