package github

import (
	"errors"
	"net/url"
)

// ErrPullExists means an open pull request already has that head branch.
var ErrPullExists = errors.New("an open pull request already exists for that branch")

// NewPull is a pull request to open: Head is a branch of the repository, Base
// the branch it goes to.
type NewPull struct {
	Title, Body, Head, Base string
	Draft                   bool
}

// CreatePull opens a pull request in owner/name and returns its number. It
// returns ErrPullExists when the head branch has an open one.
func (c *Client) CreatePull(owner, name string, p NewPull) (int, error) {
	in := map[string]any{"title": p.Title, "body": p.Body, "head": p.Head, "base": p.Base, "draft": p.Draft}
	var out struct {
		Number int `json:"number"`
	}
	if _, err := c.do("POST", c.repoURL(owner, name)+"/pulls", in, &out); err != nil {
		if errors.Is(err, ErrExists) {
			return 0, ErrPullExists
		}
		return 0, err
	}
	return out.Number, nil
}

// OpenPullFor returns the number of the open pull request whose head is the
// branch of owner/name; ok is false when there is none.
func (c *Client) OpenPullFor(owner, name, branch string) (int, bool, error) {
	q := url.Values{"state": {"open"}, "head": {owner + ":" + branch}}
	var out []struct {
		Number int `json:"number"`
	}
	if _, err := c.do("GET", c.repoURL(owner, name)+"/pulls?"+q.Encode(), nil, &out); err != nil {
		return 0, false, err
	}
	if len(out) == 0 {
		return 0, false, nil
	}
	return out[0].Number, true, nil
}
