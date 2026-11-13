package project_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/domain/project"
)

// 07 の owner/name の形。前後の空白は除いてから見る。
func TestNormalizeRepo(t *testing.T) {
	const bad = "リポジトリは owner/name の形式で入力してください"
	cases := []struct{ name, in, want, refused string }{
		{"plain", "example-org/web", "example-org/web", ""},
		{"trims", " example-org/web \n", "example-org/web", ""},
		{"dots, underscores and hyphens in the name", "a-b/c.d_e-f", "a-b/c.d_e-f", ""},
		{"no slash", "web", "", bad},
		{"two slashes", "a/b/c", "", bad},
		{"a dot in the owner", "a.b/c", "", bad},
		{"a star in the name", "a/b*", "", bad},
		{"a space inside", "a/b c", "", bad},
		{"a newline inside", "a/b\nc", "", bad},
		{"empty", "  ", "", bad},
	}
	for _, c := range cases {
		got, err := project.NormalizeRepo(c.in)
		if message(err) != c.refused || got != c.want {
			t.Errorf("%s: NormalizeRepo(%q) = %q, %v; want %q, %q", c.name, c.in, got, err, c.want, c.refused)
		}
	}
}

func TestIndexRepoIgnoresCase(t *testing.T) {
	repos := []string{"acme/api", "Example-Org/Web"}
	if got := project.IndexRepo(repos, "example-org/web"); got != 1 {
		t.Errorf("IndexRepo = %d, want 1", got)
	}
	if got := project.IndexRepo(repos, "acme/other"); got != -1 {
		t.Errorf("IndexRepo = %d, want -1", got)
	}
}
