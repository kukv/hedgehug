package usecase

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/automation"
	"github.com/kukv/hedgehug/internal/app/storage/ghstate"
	"github.com/kukv/hedgehug/internal/app/storage/store"
)

// AutomatedCommit reports whether GitHub or a rule wrote a commit, not a
// person: by what its trailer says. A fetch that only notes a refused rule is
// one, as is a commit of an op of GitHub.
func AutomatedCommit(trailer func(string) (string, bool)) bool {
	if op, _ := trailer("Hedgehug-Op"); slices.Contains([]string{"gh-fetch", OpGitHubMirror, OpGitHubPush}, op) {
		return true
	}
	if _, ok := trailer(automation.TrailerApplied); ok {
		return true
	}
	_, ok := trailer(automation.TrailerRejected)
	return ok
}

// GitHubRepoView is how the reads of one repository of a store go.
type GitHubRepoView struct {
	Store       string     `json:"store"`
	Repo        string     `json:"repo" doc:"<host>/<owner>/<repo>"`
	Account     string     `json:"account,omitempty" doc:"The name of the account that reads it; absent when none can"`
	LastFetch   *time.Time `json:"last_fetch,omitempty" doc:"When it was last read, absent before the first read"`
	NextFetch   *time.Time `json:"next_fetch,omitempty" doc:"The first time it may be read again; absent or past means at the next sync"`
	Warnings    []string   `json:"warnings" nullable:"false"`
	Limit       int        `json:"limit" doc:"The rate limit, 0 before the first read"`
	Remaining   int        `json:"remaining"`
	ResetAt     *time.Time `json:"reset_at,omitempty"`
	SlowedUntil *time.Time `json:"slowed_until,omitempty" doc:"While the reads are held back by the rate limit: when it recovers"`
	AppMissing  bool       `json:"app_missing" doc:"GitHub said the GitHub App is not installed where the repository is"`
}

// GitHubSummary is how the reads of a store go, for its status line.
type GitHubSummary struct {
	LastFetch   *time.Time `json:"last_fetch,omitempty" doc:"The latest read of its repositories"`
	NextFetch   *time.Time `json:"next_fetch,omitempty" doc:"The first read to come"`
	Limit       int        `json:"limit" doc:"The rate limit of the repository with the least left"`
	Remaining   int        `json:"remaining"`
	SlowedUntil *time.Time `json:"slowed_until,omitempty" doc:"While a read is held back by the rate limit: the last time it recovers"`
	Warnings    int        `json:"warnings" doc:"The repositories with a warning"`
	AppMissing  int        `json:"app_missing" doc:"The repositories the GitHub App is not installed for"`
}

func repoView(store, repo string, r ghstate.Repo, now time.Time) GitHubRepoView {
	v := GitHubRepoView{
		Store: store, Repo: repo, Warnings: []string{}, Limit: r.Limit, Remaining: r.Remaining,
		LastFetch: TimePtr(r.LastFetch), AppMissing: r.AppMissing,
	}
	if r.Warning != "" {
		v.Warnings = strings.Split(r.Warning, "\n")
	}
	v.NextFetch = TimePtr(r.NextAt(now))
	v.ResetAt = TimePtr(r.ResetAt)
	v.SlowedUntil = TimePtr(r.SlowedUntil(now))
	return v
}

// GitHubStatus reads how the reads of the repositories go, store by store: of
// ss.StoreName, or of every store. The repositories are those a fetch reads,
// so one that was not read yet is here too. A store that does not open has
// none, nor has a read-only store, which is not fetched.
func GitHubStatus(ss *Session) ([]GitHubRepoView, error) {
	c, _, err := ss.Config()
	if err != nil {
		return nil, err
	}
	entries, err := ss.SyncTargets(c, true)
	if err != nil {
		return nil, err
	}
	now := ss.Now()
	out := []GitHubRepoView{}
	for _, e := range entries {
		es := ss.ForStore(e.Name)
		s, err := es.OpenStoreAt(e.Path)
		if err != nil {
			continue
		}
		views, err := func(s *store.Store) ([]GitHubRepoView, error) {
			defer func() { _ = s.Repo().Close() }()
			var ro *ReadOnlyError
			if err := es.CheckWritable(s); errors.As(err, &ro) {
				return nil, nil
			} else if err != nil {
				return nil, err
			}
			ix, err := es.IndexOf(e.Name, s)
			if err != nil {
				return nil, err
			}
			state, err := ghstate.Read(s)
			if err != nil {
				return nil, err
			}
			var vs []GitHubRepoView
			for _, repo := range slices.Sorted(maps.Keys(targetsOf(ix))) {
				v := repoView(e.Name, repo, state.Repos[repo], now)
				if a, err := es.AccountForRepo(c, e, repo); err == nil {
					v.Account = a.Name
				}
				vs = append(vs, v)
			}
			return vs, nil
		}(s)
		if err != nil {
			return nil, err
		}
		out = append(out, views...)
	}
	return out, nil
}

// githubSummary sums up state; nil when no repository was tried yet.
func githubSummary(state ghstate.State, now time.Time) *GitHubSummary {
	if len(state.Repos) == 0 {
		return nil
	}
	var sum GitHubSummary
	for _, r := range state.Repos {
		if !r.LastFetch.IsZero() && (sum.LastFetch == nil || r.LastFetch.After(*sum.LastFetch)) {
			sum.LastFetch = TimePtr(r.LastFetch)
		}
		if next := r.NextAt(now); !next.IsZero() && (sum.NextFetch == nil || next.Before(*sum.NextFetch)) {
			sum.NextFetch = TimePtr(next)
		}
		if r.Limit > 0 && (sum.Limit == 0 || r.Remaining*sum.Limit < sum.Remaining*r.Limit) {
			sum.Limit, sum.Remaining = r.Limit, r.Remaining
		}
		if u := r.SlowedUntil(now); !u.IsZero() && (sum.SlowedUntil == nil || u.After(*sum.SlowedUntil)) {
			sum.SlowedUntil = TimePtr(u)
		}
		if r.Warning != "" {
			sum.Warnings++
		}
		if r.AppMissing {
			sum.AppMissing++
		}
	}
	return &sum
}
