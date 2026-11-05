// Package ghstate keeps, on this machine, when each repository was last read
// from GitHub and how much of the rate limit is left. It is a cache: losing
// it reads the repositories again.
package ghstate

import (
	"encoding/json"
	"errors"
	"io/fs"
	"time"

	"github.com/kukv/hedgehug/internal/app/storage/store"
)

const (
	fileName = "github.json"

	// Interval is how often a repository is read.
	Interval = 5 * time.Minute
	// slowFactor stretches Interval while the rate limit runs low.
	slowFactor = 3
	// maxAhead is how far ahead NotBefore and ResetAt are believed.
	maxAhead = time.Hour
)

// Repo is what is known of one repository.
type Repo struct {
	// LastFetch is when the repository was last read, by the clock of this
	// machine. It tells the interval only.
	LastFetch time.Time `json:"last_fetch"`
	// Since is the latest UpdatedAt GitHub reported in a scan: the next scan
	// reads what was updated at or after it. It is GitHub's clock, not ours.
	Since time.Time `json:"since,omitzero"`
	// LastTry and Warning are the last read that failed: it is not tried
	// again within the interval, and Warning is told again meanwhile. A read
	// that succeeds clears them; Warning may then hold what the read found
	// (an item that is not on GitHub), told until the next read.
	LastTry   time.Time `json:"last_try,omitzero"`
	Warning   string    `json:"warning,omitempty"` // one warning per line
	Limit     int       `json:"limit"`
	Remaining int       `json:"remaining"`
	ResetAt   time.Time `json:"reset_at"`
	NotBefore time.Time `json:"not_before,omitzero"` // GitHub refused: the rate limit is used up until then
	// AppMissing says the last read was refused because the GitHub App is not
	// installed where the repository is. A read that succeeds clears it.
	AppMissing bool `json:"app_missing,omitempty"`
}

// State is the file, by repo ID.
type State struct {
	Repos map[string]Repo `json:"repos"`
}

// Due reports whether the repository is to be read at now. The interval
// counts from the later of LastFetch and LastTry. Under 10% of the
// rate limit the interval is three times as long, until the limit resets.
// The file is only a cache, so a clock that went back, or a time further
// ahead than maxAhead, is not trusted: the repository is read.
func (r Repo) Due(now time.Time) bool { return !now.Before(r.NextAt(now)) }

// NextAt is the first time the repository may be read, as Due sees it at now.
// It is the zero time when nothing holds it back. It does not move with now
// while the state stays, so that a watcher sees no change in it.
func (r Repo) NextAt(now time.Time) time.Time {
	var next time.Time
	if r.held(now) {
		next = r.NotBefore
	}
	last := r.LastFetch
	if r.LastTry.After(last) {
		last = r.LastTry
	}
	if last.IsZero() || last.After(now) {
		return next
	}
	wait := Interval
	if r.low(now) {
		wait *= slowFactor
	}
	if at := last.Add(wait); at.After(next) {
		return at
	}
	return next
}

// held reports whether GitHub refused until a time still to come.
func (r Repo) held(now time.Time) bool {
	return now.Before(r.NotBefore) && !r.NotBefore.After(now.Add(maxAhead))
}

// low reports whether the rate limit runs low until a time still to come.
func (r Repo) low(now time.Time) bool {
	return r.Limit > 0 && r.Remaining*10 < r.Limit && now.Before(r.ResetAt) && !r.ResetAt.After(now.Add(maxAhead))
}

// SlowedUntil is when the rate limit recovers, while it holds reads back; the
// zero time when it does not.
func (r Repo) SlowedUntil(now time.Time) time.Time {
	switch {
	case r.held(now):
		return r.NotBefore
	case r.low(now):
		return r.ResetAt
	}
	return time.Time{}
}

// Read reads the file of s. A missing or broken file is an empty state.
func Read(s *store.Store) (State, error) {
	st := State{Repos: map[string]Repo{}}
	b, err := s.ReadLocal(fileName)
	if errors.Is(err, fs.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	if json.Unmarshal(b, &st) != nil || st.Repos == nil {
		return State{Repos: map[string]Repo{}}, nil
	}
	return st, nil
}

// Write replaces the file of s.
func Write(s *store.Store, st State) error {
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return s.WriteLocal(fileName, b)
}
