package rank_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/domain/rank"
)

func TestAfter(t *testing.T) {
	for in, want := range map[string]string{"": "V", "V": "W", "a0V": "a0W", "9": "A", "Z": "a", "z": "zV", "azz": "azzV"} {
		if got := rank.After(in); got != want {
			t.Errorf("After(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAfterIsAlwaysLater(t *testing.T) {
	s := ""
	for range 500 {
		next := rank.After(s)
		if next <= s {
			t.Fatalf("After(%q) = %q is not later", s, next)
		}
		s = next
	}
}

func TestBetween(t *testing.T) {
	cases := [][2]string{{"", "V"}, {"V", "W"}, {"V", "X"}, {"Vz", "W"}, {"V", "V1"}, {"", "1"}, {"a", "b"}}
	for _, c := range cases {
		got := rank.Between(c[0], c[1])
		if got <= c[0] || got >= c[1] {
			t.Errorf("Between(%q, %q) = %q", c[0], c[1], got)
		}
	}
	if got := rank.Between("V", ""); got != rank.After("V") {
		t.Errorf("no upper bound: %q", got)
	}
	// 何度あいだに詰めても、順が保たれる。
	lo, hi := "V", "W"
	for range 200 {
		m := rank.Between(lo, hi)
		if m <= lo || m >= hi {
			t.Fatalf("Between(%q, %q) = %q", lo, hi, m)
		}
		hi = m
	}
}
