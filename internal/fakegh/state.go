// Package fakegh is a fake of the GitHub API that Hedgehug uses, for tests
// and the development environment. It keeps its state in memory and answers
// only the endpoints Hedgehug calls.
package fakegh

import (
	"encoding/json"
	"os"
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
	Owner   string            `json:"owner"`
	Name    string            `json:"name"`
	Private bool              `json:"private"`
	Topics  []string          `json:"topics"`
	Perms   map[string]string `json:"perms"` // login → read, write, admin or none

	id int // numbered once, when the server takes the repository in
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
