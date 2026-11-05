// Package fakegh is a fake of the GitHub API that Hedgehug uses, for tests
// and the development environment. It keeps its state in memory and answers
// only the endpoints Hedgehug calls.
package fakegh

import (
	"encoding/json"
	"os"
	"time"
)

// State is what the fake knows: users, organizations, tokens and
// repositories. A seed file is this as JSON.
type State struct {
	Users          []string            `json:"users"`
	Orgs           map[string][]string `json:"orgs"`   // org → members
	Tokens         map[string]string   `json:"tokens"` // token → login
	Repos          []*Repo             `json:"repos"`
	DeviceInterval *int                `json:"device_interval,omitempty"` // seconds; 5 when missing
	AutoApprove    string              `json:"auto_approve,omitempty"`    // approve each device code for this login at once
}

// Repo is a repository and who may do what with it. The owner of a user's
// repository is its admin.
type Repo struct {
	Owner   string `json:"owner"`
	Name    string `json:"name"`
	Private bool   `json:"private"`
	// DefaultBranch is the branch pull requests go to; main when empty.
	DefaultBranch string            `json:"default_branch,omitempty"`
	Topics        []string          `json:"topics"`
	Perms         map[string]string `json:"perms"` // login → read, write, admin or none
	Pulls         []*Pull           `json:"pulls,omitempty"`
	Issues        []*Issue          `json:"issues,omitempty"`
	Labels        []string          `json:"labels,omitempty"`
	Milestones    []string          `json:"milestones,omitempty"` // numbered from 1, in this order

	id int // numbered once, when the server takes the repository in
}

// Pull is a pull request.
type Pull struct {
	Number    int        `json:"number"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	Head      string     `json:"head"`
	Base      string     `json:"base,omitempty"` // the default branch when empty
	State     string     `json:"state"`          // open, closed, merged
	Draft     bool       `json:"draft"`
	Review    string     `json:"review"`    // "", approved, changes_requested
	Requested bool       `json:"requested"` // a review is asked for
	Checks    string     `json:"checks"`    // "", success, failure, pending
	MergedAt  *time.Time `json:"merged_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Comments  []string   `json:"comments"`
}

// Issue is an issue.
type Issue struct {
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	State       string    `json:"state"`        // open, closed
	StateReason string    `json:"state_reason"` // completed, not_planned
	Assignees   []string  `json:"assignees,omitempty"`
	Labels      []string  `json:"labels,omitempty"`
	Milestone   string    `json:"milestone,omitempty"` // its title
	UpdatedAt   time.Time `json:"updated_at"`
}

// Load reads a seed file.
func Load(path string) (State, error) {
	var st State
	b, err := os.ReadFile(path) //nolint:gosec // the seed the developer names
	if err != nil {
		return st, err
	}
	err = json.Unmarshal(b, &st)
	return st, err
}
