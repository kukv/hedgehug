package automations

import (
	"cmp"
	"slices"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/automation"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/history"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// statsWindow is how far back Stats counts.
const statsWindow = 7 * 24 * time.Hour

// RuleStat is how a rule went in the last 7 days. Rule is the id of a rule, or
// mirror for what the issue sync changed.
type RuleStat struct {
	Rule          string        `json:"rule"`
	Applied       int           `json:"applied"`
	Rejected      int           `json:"rejected"`
	RejectedTasks []usecase.Ref `json:"rejected_tasks" nullable:"false" doc:"The tasks it was refused on, the latest first, each once"`
	LastReason    string        `json:"last_reason,omitempty" doc:"Why it was last refused"`
}

// statOps are the ops of the commits that carry the trailers Stats counts.
var statOps = []string{"set", "gh-fetch", usecase.OpGitHubMirror}

// Stats counts what the rules did in the last 7 days, by the trailers of the
// commits. The index of the history holds no trailers: it picks the commits,
// and only those are read. A rule with nothing to count is not in the list.
func Stats(ss *usecase.Session) ([]RuleStat, error) {
	s, ix, err := ss.Open()
	if err != nil {
		return nil, err
	}
	_, e, err := ss.Entry()
	if err != nil {
		return nil, err
	}
	h, err := ss.OpenHistory(s, e.Name)
	if err != nil {
		return nil, err
	}
	since := ss.Now().Add(-statsWindow)
	picked := slices.DeleteFunc(h.All(), func(c history.Commit) bool {
		return c.When.Before(since) || !slices.Contains(statOps, c.Op)
	})
	entries, err := h.Entries(picked, 0)
	if err != nil {
		return nil, err
	}
	byRule := map[string]*RuleStat{}
	stat := func(rule string) *RuleStat {
		if byRule[rule] == nil {
			byRule[rule] = &RuleStat{Rule: rule, RejectedTasks: []usecase.Ref{}}
		}
		return byRule[rule]
	}
	seen := map[string]map[model.ULID]bool{}
	for _, c := range entries { // the latest first
		for _, t := range c.Trailers {
			switch t.Key {
			case automation.TrailerApplied:
				if rule, _, ok := automation.ParseApplied(t.Value); ok {
					stat(rule).Applied++
				}
			case automation.TrailerRejected:
				u, r, ok := automation.ParseRejected(t.Value)
				if !ok {
					continue
				}
				x := stat(r.Rule)
				x.Rejected++
				if x.LastReason == "" {
					x.LastReason = r.Reason
				}
				if seen[r.Rule] == nil {
					seen[r.Rule] = map[model.ULID]bool{}
				}
				if !seen[r.Rule][u] {
					seen[r.Rule][u] = true
					x.RejectedTasks = append(x.RejectedTasks, usecase.RefOf(ix, u))
				}
			}
		}
	}
	out := make([]RuleStat, 0, len(byRule))
	for _, x := range byRule {
		out = append(out, *x)
	}
	slices.SortFunc(out, func(a, b RuleStat) int { return cmp.Compare(a.Rule, b.Rule) })
	return out, nil
}
