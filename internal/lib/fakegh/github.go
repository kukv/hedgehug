package fakegh

import (
	"cmp"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

type rateLimit struct {
	Limit     int       `json:"limit"`
	Remaining int       `json:"remaining"`
	ResetAt   time.Time `json:"resetAt"`
}

func clonePulls(in []*Pull) []*Pull {
	out := make([]*Pull, len(in))
	for i, p := range in {
		c := *p
		c.Comments = slices.Clone(p.Comments)
		out[i] = &c
	}
	return out
}

func cloneIssues(in []*Issue) []*Issue {
	out := make([]*Issue, len(in))
	for i, x := range in {
		c := *x
		c.Assignees, c.Labels = slices.Clone(x.Assignees), slices.Clone(x.Labels)
		out[i] = &c
	}
	return out
}

// SetRateLimit sets what GraphQL says of the rate limit.
func (s *Server) SetRateLimit(remaining, limit int, reset time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rate = rateLimit{Limit: limit, Remaining: remaining, ResetAt: reset}
}

func nextNumber(r *Repo) int {
	n := 0
	for _, p := range r.Pulls {
		n = max(n, p.Number)
	}
	for _, i := range r.Issues {
		n = max(n, i.Number)
	}
	return n + 1
}

// OpenPull opens a pull request and returns its number. Pull requests and
// issues share their numbers, as on GitHub.
func (s *Server) OpenPull(repo, head, title, body string, draft bool) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.find(repo)
	if r == nil {
		return 0, fmt.Errorf("no repository %s", repo)
	}
	p := &Pull{Number: nextNumber(r), Title: title, Body: body, Head: head, State: "open", Draft: draft, UpdatedAt: s.now()}
	r.Pulls = append(r.Pulls, p)
	return p.Number, nil
}

// changePull changes the pull request number of repo with fn and marks it updated.
func (s *Server) changePull(repo string, number int, fn func(*Pull) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.find(repo)
	if r == nil {
		return fmt.Errorf("no repository %s", repo)
	}
	for _, p := range r.Pulls {
		if p.Number == number {
			if err := fn(p); err != nil {
				return err
			}
			p.UpdatedAt = s.now()
			return nil
		}
	}
	return fmt.Errorf("no pull request %s#%d", repo, number)
}

// ReadyPull takes a pull request out of draft.
func (s *Server) ReadyPull(repo string, number int) error {
	return s.changePull(repo, number, func(p *Pull) error { p.Draft = false; return nil })
}

// RequestReview asks for a review.
func (s *Server) RequestReview(repo string, number int) error {
	return s.changePull(repo, number, func(p *Pull) error { p.Requested = true; return nil })
}

// ReviewPull gives the review: approved or changes_requested.
func (s *Server) ReviewPull(repo string, number int, state string) error {
	return s.changePull(repo, number, func(p *Pull) error {
		if state != "approved" && state != "changes_requested" {
			return fmt.Errorf("unknown review %q", state)
		}
		p.Review, p.Requested = state, false
		return nil
	})
}

// SetChecks sets the state of the checks: success, failure or pending.
func (s *Server) SetChecks(repo string, number int, state string) error {
	return s.changePull(repo, number, func(p *Pull) error { p.Checks = state; return nil })
}

// CommentPull adds a comment.
func (s *Server) CommentPull(repo string, number int, body string) error {
	return s.changePull(repo, number, func(p *Pull) error { p.Comments = append(p.Comments, body); return nil })
}

// MergePull merges a pull request.
func (s *Server) MergePull(repo string, number int) error {
	return s.changePull(repo, number, func(p *Pull) error {
		at := s.now()
		p.State, p.Draft, p.MergedAt = "merged", false, &at
		return nil
	})
}

// OpenIssue opens an issue and returns its number.
func (s *Server) OpenIssue(repo, title, body string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.find(repo)
	if r == nil {
		return 0, fmt.Errorf("no repository %s", repo)
	}
	i := &Issue{Number: nextNumber(r), Title: title, Body: body, State: "open", UpdatedAt: s.now()}
	r.Issues = append(r.Issues, i)
	return i.Number, nil
}

// changeIssue changes the issue number of repo with fn and marks it updated.
func (s *Server) changeIssue(repo string, number int, fn func(*Issue)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.find(repo)
	if r == nil {
		return fmt.Errorf("no repository %s", repo)
	}
	for _, i := range r.Issues {
		if i.Number == number {
			fn(i)
			i.UpdatedAt = s.now()
			return nil
		}
	}
	return fmt.Errorf("no issue %s#%d", repo, number)
}

// CloseIssue closes an issue: completed or not_planned.
func (s *Server) CloseIssue(repo string, number int, reason string) error {
	return s.changeIssue(repo, number, func(i *Issue) { i.State, i.StateReason = "closed", reason })
}

// EditIssue changes the title and the body of an issue; an empty one is left
// as it is.
func (s *Server) EditIssue(repo string, number int, title, body string) error {
	return s.changeIssue(repo, number, func(i *Issue) {
		if title != "" {
			i.Title = title
		}
		if body != "" {
			i.Body = body
		}
	})
}

func pullJSON(p *Pull) map[string]any {
	state := strings.ToUpper(p.State)
	var decision any
	if p.Review != "" {
		decision = strings.ToUpper(p.Review)
	}
	requests := 0
	if p.Requested {
		requests = 1
	}
	latest := []map[string]string{}
	if p.Review != "" {
		latest = append(latest, map[string]string{"state": strings.ToUpper(p.Review)})
	}
	var rollup any
	if p.Checks != "" {
		rollup = map[string]string{"state": strings.ToUpper(p.Checks)}
	}
	comments := make([]map[string]string, len(p.Comments))
	for i, c := range p.Comments {
		comments[i] = map[string]string{"body": c}
	}
	return map[string]any{
		"number": p.Number, "title": p.Title, "body": p.Body, "state": state, "isDraft": p.Draft,
		"mergedAt": p.MergedAt, "updatedAt": p.UpdatedAt, "headRefName": p.Head, "reviewDecision": decision,
		"reviewRequests":           map[string]int{"totalCount": requests},
		"latestOpinionatedReviews": map[string]any{"nodes": latest},
		"commits":                  map[string]any{"nodes": []any{map[string]any{"commit": map[string]any{"statusCheckRollup": rollup}}}},
		"comments":                 map[string]any{"nodes": comments},
	}
}

func issueJSON(i *Issue) map[string]any {
	var reason any
	if i.StateReason != "" {
		reason = strings.ToUpper(i.StateReason)
	}
	assignees := make([]map[string]string, len(i.Assignees))
	for n, a := range i.Assignees {
		assignees[n] = map[string]string{"login": a}
	}
	labels := make([]map[string]string, len(i.Labels))
	for n, l := range i.Labels {
		labels[n] = map[string]string{"name": l}
	}
	var milestone any
	if i.Milestone != "" {
		milestone = map[string]string{"title": i.Milestone}
	}
	return map[string]any{
		"number": i.Number, "title": i.Title, "body": i.Body, "state": strings.ToUpper(i.State),
		"stateReason": reason, "updatedAt": i.UpdatedAt,
		"assignees": map[string]any{"nodes": assignees},
		"labels":    map[string]any{"nodes": labels},
		"milestone": milestone,
	}
}

// graphql answers the two queries of internal/lib/github: ScanPulls and
// Linked.
func (s *Server) graphql(w http.ResponseWriter, r *http.Request, login string) {
	var body struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	owner, _ := body.Variables["owner"].(string)
	name, _ := body.Variables["name"].(string)
	if s.restricted[strings.ToLower(owner)] {
		writeJSON(w, 200, map[string]any{
			"data":   map[string]any{"rateLimit": s.rate, "repository": nil},
			"errors": []map[string]any{{"type": "FORBIDDEN", "path": []string{"repository"}, "message": "Resource not accessible by integration"}},
		})
		return
	}
	x := s.find(owner + "/" + name)
	if x == nil || perm(x, login) == "" || s.uninstalled[strings.ToLower(owner)] {
		writeJSON(w, 200, map[string]any{
			"data":   map[string]any{"rateLimit": s.rate, "repository": nil},
			"errors": []map[string]any{{"type": "NOT_FOUND", "path": []string{"repository"}, "message": "Could not resolve to a Repository"}},
		})
		return
	}
	kind, top, err := parseQuery(body.Query)
	var problems []string
	switch {
	case err != nil:
		problems = []string{err.Error()}
	case kind == "ScanPulls":
		problems = check(top, scanShape, "Query")
	case kind == "Linked":
		problems = checkLinked(top)
	case kind == "OpenIssues":
		problems = check(top, openIssuesShape, "Query")
	default:
		problems = []string{"unknown query " + kind}
	}
	if len(problems) > 0 {
		errs := make([]map[string]any, len(problems))
		for i, m := range problems {
			errs[i] = map[string]any{"message": m}
		}
		writeJSON(w, 200, map[string]any{"errors": errs})
		return
	}
	after, _ := body.Variables["after"].(string)
	switch kind {
	case "ScanPulls":
		s.scanPulls(w, x, after)
	case "OpenIssues":
		label, _ := body.Variables["label"].(string)
		s.openIssues(w, x, label, after)
	default:
		s.linked(w, x, top)
	}
}

func (s *Server) scanPulls(w http.ResponseWriter, x *Repo, after string) {
	pulls := slices.Clone(x.Pulls)
	slices.SortStableFunc(pulls, func(a, b *Pull) int {
		if c := b.UpdatedAt.Compare(a.UpdatedAt); c != 0 {
			return c
		}
		return cmp.Compare(b.Number, a.Number)
	})
	from, _ := strconv.Atoi(after)
	to := min(from+100, len(pulls))
	nodes := make([]map[string]any, 0, to-from)
	for _, p := range pulls[from:to] {
		nodes = append(nodes, pullJSON(p))
	}
	writeJSON(w, 200, map[string]any{"data": map[string]any{
		"rateLimit": s.rate,
		"repository": map[string]any{"pullRequests": map[string]any{
			"pageInfo": map[string]any{"hasNextPage": to < len(pulls), "endCursor": strconv.Itoa(to)},
			"nodes":    nodes,
		}},
	}})
}

// linked answers the query that checkLinked passed.
func (s *Server) linked(w http.ResponseWriter, x *Repo, top []*field) {
	repo := map[string]any{}
	var errs []map[string]any
	var aliases []*field
	for _, f := range top {
		if f.name == "repository" {
			aliases = f.kids
		}
	}
	for _, a := range aliases {
		n, _ := strconv.Atoi(strings.TrimPrefix(a.args, "number:"))
		var node map[string]any
		if a.name == "pullRequest" {
			if i := slices.IndexFunc(x.Pulls, func(p *Pull) bool { return p.Number == n }); i >= 0 {
				node = pullJSON(x.Pulls[i])
			}
		} else if i := slices.IndexFunc(x.Issues, func(p *Issue) bool { return p.Number == n }); i >= 0 {
			node = issueJSON(x.Issues[i])
		}
		if node == nil {
			repo[a.alias] = nil
			errs = append(errs, map[string]any{"type": "NOT_FOUND", "path": []string{"repository", a.alias}, "message": "Could not resolve"})
			continue
		}
		repo[a.alias] = node
	}
	out := map[string]any{"data": map[string]any{"rateLimit": s.rate, "repository": repo}}
	if errs != nil {
		out["errors"] = errs
	}
	writeJSON(w, 200, out)
}
