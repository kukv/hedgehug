//go:build live

package github

import (
	"os"
	"testing"
	"time"
)

// TestLiveQueries runs the queries against GitHub: the fields of the queries
// must exist, and the answers must decode. .github/workflows/github-live.yaml
// runs it on a schedule. It only reads.
func TestLiveQueries(t *testing.T) {
	org, token := os.Getenv("HEDGEHUG_LIVE_ORG"), os.Getenv("HEDGEHUG_LIVE_TOKEN")
	if org == "" || token == "" {
		t.Log("HEDGEHUG_LIVE_ORG or HEDGEHUG_LIVE_TOKEN is not set: nothing to check")
		return
	}
	c := New(Options{API: "https://api.github.com"}, token, nil)
	pulls, rl, err := c.ScanPulls(org, "hh-live", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if rl.Limit == 0 || rl.ResetAt.IsZero() {
		t.Errorf("rate limit = %+v", rl)
	}
	if len(pulls) == 0 {
		t.Fatal("hh-live has no pull request to read")
	}
	for _, p := range pulls {
		if p.State == "" || p.Review == "" || p.UpdatedAt.IsZero() {
			t.Errorf("pull #%d did not decode: %+v", p.Number, p)
		}
	}
	got, _, err := c.LinkedItems(org, "hh-live", []int{pulls[0].Number, 999999}, []int{999998})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Pulls) != 1 || got.Pulls[0].Number != pulls[0].Number {
		t.Errorf("pulls = %+v", got.Pulls)
	}
	if len(got.MissingPulls) != 1 || len(got.MissingIssues) != 1 {
		t.Errorf("missing = %+v", got)
	}
	if _, _, err := c.OpenIssues(org, "hh-live", "hh-live"); err != nil {
		t.Errorf("open issues: %v", err)
	}
}
