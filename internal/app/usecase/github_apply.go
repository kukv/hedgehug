package usecase

import (
	"bytes"
	"slices"
	"time"

	"go.yaml.in/yaml/v4"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/lib/github"
)

// fetched is what GitHub said, by ref.
type fetched struct {
	pulls  map[string]github.Pull
	issues map[string]github.Issue
}

// apply brings t up to date: it adds the links in auto that it does not have
// in any form (an unlinked one stays unlinked), and rewrites gh from what was
// fetched. What was not fetched keeps its value; an item of gh with no link
// in use goes. It reports whether t changed.
func apply(t *model.Task, got fetched, auto []string) (bool, error) {
	linked := false
	for _, ref := range auto {
		if !slices.ContainsFunc(t.Links, func(l model.Link) bool { return model.CanonRef(l.Ref) == model.CanonRef(ref) }) {
			t.Links = append(t.Links, model.Link{Type: model.LinkPR, Ref: ref, Sync: model.SyncMirror})
			linked = true
		}
	}
	old, err := t.GitHub()
	if err != nil {
		return false, err
	}
	var next model.GitHubState
	for _, l := range t.Links {
		if !l.Live() {
			continue
		}
		switch l.Type {
		case model.LinkPR:
			ref := model.CanonRef(l.Ref)
			i := slices.IndexFunc(old.PRs, func(p model.PRState) bool { return model.CanonRef(p.Ref) == ref })
			p, ok := got.pulls[ref]
			switch {
			case ok && i >= 0:
				next.PRs = append(next.PRs, withPull(old.PRs[i], p))
			case ok:
				next.PRs = append(next.PRs, withPull(model.PRState{Ref: ref}, p))
			case i >= 0:
				next.PRs = append(next.PRs, old.PRs[i])
			}
		case model.LinkIssue:
			ref := model.CanonRef(l.Ref)
			i := slices.IndexFunc(old.Issues, func(x model.IssueState) bool { return model.CanonRef(x.Ref) == ref })
			x, ok := got.issues[ref]
			switch {
			case ok && i >= 0:
				next.Issues = append(next.Issues, withIssue(old.Issues[i], x))
			case ok:
				next.Issues = append(next.Issues, withIssue(model.IssueState{Ref: ref}, x))
			case i >= 0:
				next.Issues = append(next.Issues, old.Issues[i])
			}
		}
	}
	same, err := sameState(old, next)
	if err != nil {
		return false, err
	}
	if same {
		return linked, nil
	}
	return true, t.SetGitHub(next)
}

func withPull(s model.PRState, p github.Pull) model.PRState {
	s.State, s.Review, s.Checks, s.MergedAt = p.State, p.Review, p.Checks, utc(p.MergedAt)
	return s
}

func withIssue(s model.IssueState, x github.Issue) model.IssueState {
	s.State, s.StateReason = x.State, nil
	if x.StateReason != "" {
		s.StateReason = &x.StateReason
	}
	return s
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

// sameState compares what the two write, so that a time read from the file
// equals the same time read from GitHub.
func sameState(a, b model.GitHubState) (bool, error) {
	x, err := yaml.Marshal(a)
	if err != nil {
		return false, err
	}
	y, err := yaml.Marshal(b)
	if err != nil {
		return false, err
	}
	return bytes.Equal(x, y), nil
}
