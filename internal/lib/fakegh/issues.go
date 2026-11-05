package fakegh

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
)

// ReopenIssue opens a closed issue.
func (s *Server) ReopenIssue(repo string, number int) error {
	return s.changeIssue(repo, number, func(i *Issue) { i.State, i.StateReason = "open", "" })
}

// DropIssue deletes an issue.
func (s *Server) DropIssue(repo string, number int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.find(repo)
	if r == nil {
		return fmt.Errorf("no repository %s", repo)
	}
	r.Issues = slices.DeleteFunc(r.Issues, func(i *Issue) bool { return i.Number == number })
	return nil
}

// TagIssue sets the assignees, the labels and the milestone of an issue. A
// nil list or an empty milestone stays; the milestone "-" goes.
func (s *Server) TagIssue(repo string, number int, assignees, labels []string, milestone string) error {
	if assignees == nil && labels == nil && milestone == "" {
		return nil
	}
	return s.changeIssue(repo, number, func(i *Issue) {
		if assignees != nil {
			i.Assignees = slices.Clone(assignees)
		}
		if labels != nil {
			i.Labels = slices.Clone(labels)
		}
		switch milestone {
		case "":
		case "-":
			i.Milestone = ""
		default:
			i.Milestone = milestone
		}
	})
}

// AddLabel adds a label to a repository.
func (s *Server) AddLabel(repo, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.find(repo)
	if r == nil {
		return fmt.Errorf("no repository %s", repo)
	}
	r.Labels = append(r.Labels, name)
	return nil
}

// AddMilestone adds a milestone to a repository.
func (s *Server) AddMilestone(repo, title string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.find(repo)
	if r == nil {
		return fmt.Errorf("no repository %s", repo)
	}
	r.Milestones = append(r.Milestones, title)
	return nil
}

// assignable lists who an issue of x can be assigned to.
func assignable(x *Repo) []string {
	out := []string{x.Owner}
	names := make([]string, 0, len(x.Perms))
	for l := range x.Perms {
		names = append(names, l)
	}
	slices.Sort(names)
	for _, l := range names {
		if perm(x, l) != "" && l != x.Owner {
			out = append(out, l)
		}
	}
	return out
}

// issueInput is what the REST endpoints of an issue take. A key that is not
// there stays.
type issueInput struct {
	Title       *string         `json:"title"`
	Body        *string         `json:"body"`
	State       *string         `json:"state"`
	StateReason *string         `json:"state_reason"`
	Assignees   *[]string       `json:"assignees"`
	Labels      *[]string       `json:"labels"`
	Milestone   json.RawMessage `json:"milestone"`
}

// apply writes in to i, or says why GitHub would answer 422.
func (in issueInput) apply(x *Repo, i *Issue) string {
	if in.Assignees != nil {
		for _, a := range *in.Assignees {
			if !slices.Contains(assignable(x), a) {
				return "assignees: " + a + " cannot be assigned"
			}
		}
	}
	if in.Labels != nil {
		for _, l := range *in.Labels {
			if !slices.Contains(x.Labels, l) {
				return "labels: " + l + " does not exist"
			}
		}
	}
	milestone := i.Milestone
	if len(in.Milestone) > 0 {
		milestone = ""
		if string(in.Milestone) != "null" {
			n, err := strconv.Atoi(string(in.Milestone))
			if err != nil || n < 1 || n > len(x.Milestones) {
				return "milestone: no milestone " + string(in.Milestone)
			}
			milestone = x.Milestones[n-1]
		}
	}
	if in.State != nil && *in.State != "open" && *in.State != "closed" {
		return "state: " + *in.State
	}
	if in.StateReason != nil && *in.StateReason != "completed" && *in.StateReason != "not_planned" {
		return "state_reason: " + *in.StateReason
	}
	i.Milestone = milestone
	if in.Title != nil {
		i.Title = *in.Title
	}
	if in.Body != nil {
		i.Body = *in.Body
	}
	if in.Assignees != nil {
		i.Assignees = slices.Clone(*in.Assignees)
	}
	if in.Labels != nil {
		i.Labels = slices.Clone(*in.Labels)
	}
	if in.State != nil {
		i.State = *in.State
		i.StateReason = ""
		if i.State == "closed" {
			i.StateReason = "completed"
		}
	}
	if in.StateReason != nil && i.State == "closed" {
		i.StateReason = *in.StateReason
	}
	return ""
}

func (s *Server) createIssue(w http.ResponseWriter, r *http.Request, x *Repo, login string) {
	if rank(perm(x, login)) < rank("write") {
		fail(w, http.StatusForbidden, "Resource not accessible by personal access token")
		return
	}
	var in issueInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if in.Title == nil || strings.TrimSpace(*in.Title) == "" {
		unprocessable(w, "title: missing")
		return
	}
	i := &Issue{State: "open"}
	if why := in.apply(x, i); why != "" {
		unprocessable(w, why)
		return
	}
	i.Number, i.UpdatedAt = nextNumber(x), s.now()
	x.Issues = append(x.Issues, i)
	writeJSON(w, http.StatusCreated, map[string]int{"number": i.Number})
}

func (s *Server) patchIssue(w http.ResponseWriter, r *http.Request, x *Repo, number, login string) {
	if rank(perm(x, login)) < rank("write") {
		fail(w, http.StatusForbidden, "Resource not accessible by personal access token")
		return
	}
	var in issueInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	n, _ := strconv.Atoi(number)
	at := slices.IndexFunc(x.Issues, func(i *Issue) bool { return i.Number == n })
	if at < 0 {
		fail(w, http.StatusNotFound, "Not Found")
		return
	}
	if why := in.apply(x, x.Issues[at]); why != "" {
		unprocessable(w, why)
		return
	}
	x.Issues[at].UpdatedAt = s.now()
	writeJSON(w, 200, map[string]int{"number": n})
}

// openIssues answers the query OpenIssues: the open issues with the label, a
// page of 100.
func (s *Server) openIssues(w http.ResponseWriter, x *Repo, label, after string) {
	var match []*Issue
	for _, i := range x.Issues {
		if i.State == "open" && slices.Contains(i.Labels, label) {
			match = append(match, i)
		}
	}
	from, _ := strconv.Atoi(after)
	to := min(from+100, len(match))
	nodes := make([]map[string]any, 0, to-from)
	for _, i := range match[from:to] {
		nodes = append(nodes, issueJSON(i))
	}
	writeJSON(w, 200, map[string]any{"data": map[string]any{
		"rateLimit": s.rate,
		"repository": map[string]any{"issues": map[string]any{
			"pageInfo": map[string]any{"hasNextPage": to < len(match), "endCursor": strconv.Itoa(to)},
			"nodes":    nodes,
		}},
	}})
}
