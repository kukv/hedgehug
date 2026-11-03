package plural

import "testing"

func TestCount(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		n    int
		noun string
		want string
	}{
		{0, "task", "0 tasks"},
		{1, "task", "1 task"},
		{2, "task", "2 tasks"},
		{1, "forward file", "1 forward file"},
		{3, "forward file", "3 forward files"},
	} {
		if got := Count(c.n, c.noun); got != c.want {
			t.Errorf("Count(%d, %q) = %q; want %q", c.n, c.noun, got, c.want)
		}
	}
}

func TestPick(t *testing.T) {
	t.Parallel()
	if got := Pick(1, "has", "have"); got != "has" {
		t.Errorf("Pick(1) = %q", got)
	}
	for _, n := range []int{0, 2, 10} {
		if got := Pick(n, "has", "have"); got != "have" {
			t.Errorf("Pick(%d) = %q", n, got)
		}
	}
}
