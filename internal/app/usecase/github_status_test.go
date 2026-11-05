package usecase

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/automation"
)

func TestAutomatedCommit(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		trailers map[string]string
		want     bool
	}{
		"a person":  {map[string]string{"Hedgehug-Op": "set"}, false},
		"a merge":   {map[string]string{"Hedgehug-Op": "merge"}, false},
		"a rule":    {map[string]string{"Hedgehug-Op": "set", automation.TrailerApplied: "r pr.opened"}, true},
		"a refusal": {map[string]string{"Hedgehug-Op": "merge", automation.TrailerRejected: "x"}, true},
		"fetch":     {map[string]string{"Hedgehug-Op": "gh-fetch"}, true},
		"mirror":    {map[string]string{"Hedgehug-Op": OpGitHubMirror}, true},
		"push":      {map[string]string{"Hedgehug-Op": OpGitHubPush}, true},
		"by hand":   {nil, false},
	} {
		got := AutomatedCommit(func(k string) (string, bool) { v, ok := c.trailers[k]; return v, ok })
		if got != c.want {
			t.Errorf("%s = %v", name, got)
		}
	}
}
