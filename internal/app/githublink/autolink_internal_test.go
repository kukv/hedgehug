package githublink

import "testing"

// いまの名前は、改名の対応を最後まで辿る。輪になっていても止まる(D4b 最終レビュー R-1)。
func TestCurrentName(t *testing.T) {
	chain := map[string]string{"org/web": "org/site", "org/site": "org/app"}
	loop := map[string]string{"org/a": "org/b", "org/b": "org/a"}
	cases := []struct {
		renames map[string]string
		repo    string
		want    string
	}{
		{chain, "Org/Web", "org/app"},
		{chain, "org/app", "org/app"},
		{chain, "org/other", "org/other"},
		{nil, "Org/Web", "org/web"},
		{loop, "org/a", "org/b"},
	}
	for _, c := range cases {
		if got := currentName(c.renames, c.repo); got != c.want {
			t.Errorf("currentName(%v, %s) = %s, want %s", c.renames, c.repo, got, c.want)
		}
	}
}
