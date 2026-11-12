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
