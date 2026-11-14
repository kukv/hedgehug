package mention_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/domain/mention"
)

func TestHas(t *testing.T) {
	cases := []struct {
		body string
		want bool
	}{
		{"@hanako 見て", true},
		{"cc:@Hanako", true},
		{"mail@hanako", true},
		{"@hanako2", false},
		{"@hana", false},
		{"hanako", false},
		{"@hanako-", false},
		{"@hanako。", true},
		{"@hana @hanako", true},
		{"@hanakoo @hanako", true},
	}
	for _, c := range cases {
		if got := mention.Has(c.body, "hanako"); got != c.want {
			t.Errorf("Has(%q) = %v, want %v", c.body, got, c.want)
		}
	}
}
