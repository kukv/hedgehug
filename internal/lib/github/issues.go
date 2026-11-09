package github

import "strconv"

// NewIssue is an issue to open. A Milestone of 0 is none.
type NewIssue struct {
	Title, Body string
	Assignees   []string
	Milestone   int
}

// IssueUpdate says what to change of an issue: a nil field stays. A Milestone
// of 0 takes the milestone off, and StateReason goes with a closed State.
type IssueUpdate struct {
	Title, Body, State, StateReason *string
	Assignees                       *[]string
	Milestone                       *int
}

// Milestone is a milestone of a repository.
type Milestone struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
}

// CreateIssue opens an issue in owner/name and returns its number.
func (c *Client) CreateIssue(owner, name string, in NewIssue) (int, error) {
	body := map[string]any{"title": in.Title, "body": in.Body}
	if len(in.Assignees) > 0 {
		body["assignees"] = in.Assignees
	}
	if in.Milestone != 0 {
		body["milestone"] = in.Milestone
	}
	var out struct {
		Number int `json:"number"`
	}
	if _, err := c.do("POST", c.repoURL(owner, name)+"/issues", body, &out); err != nil {
		return 0, err
	}
	return out.Number, nil
}

// UpdateIssue changes the issue number of owner/name.
func (c *Client) UpdateIssue(owner, name string, number int, u IssueUpdate) error {
	body := map[string]any{}
	put := func(key string, v *string) {
		if v != nil {
			body[key] = *v
		}
	}
	put("title", u.Title)
	put("body", u.Body)
	put("state", u.State)
	put("state_reason", u.StateReason)
	if u.Assignees != nil {
		body["assignees"] = nonNil(*u.Assignees)
	}
	if u.Milestone != nil {
		body["milestone"] = nil
		if *u.Milestone != 0 {
			body["milestone"] = *u.Milestone
		}
	}
	_, err := c.do("PATCH", c.repoURL(owner, name)+"/issues/"+strconv.Itoa(number), body, nil)
	return err
}

// nonNil makes a list that is empty encode as [], which clears it on GitHub.
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// Assignees lists the logins that the issues of owner/name can be assigned to.
func (c *Client) Assignees(owner, name string) ([]string, error) {
	all, err := getAll[struct {
		Login string `json:"login"`
	}](c, c.repoURL(owner, name)+"/assignees?per_page=100")
	if err != nil {
		return nil, err
	}
	out := make([]string, len(all))
	for i, l := range all {
		out[i] = l.Login
	}
	return out, nil
}

// Milestones lists the milestones of owner/name, open or closed.
func (c *Client) Milestones(owner, name string) ([]Milestone, error) {
	return getAll[Milestone](c, c.repoURL(owner, name)+"/milestones?state=all&per_page=100")
}

// OpenIssues lists the open issues of owner/name that have the label, or all
// of them when label is empty.
func (c *Client) OpenIssues(owner, name, label string) ([]Issue, RateLimit, error) {
	var out []Issue
	var rl RateLimit
	vars := map[string]any{"owner": owner, "name": name, "labels": nil}
	if label != "" {
		vars["labels"] = []string{label}
	}
	for {
		var data struct {
			RateLimit  RateLimit `json:"rateLimit"`
			Repository struct {
				Issues struct {
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
					Nodes []issueNode `json:"nodes"`
				} `json:"issues"`
			} `json:"repository"`
		}
		if err := c.graphql(openIssuesQuery, vars, &data); err != nil {
			return nil, rl, err
		}
		rl = data.RateLimit
		page := data.Repository.Issues
		for _, n := range page.Nodes {
			out = append(out, n.issue())
		}
		if !page.PageInfo.HasNextPage {
			return out, rl, nil
		}
		vars["after"] = page.PageInfo.EndCursor
	}
}

const openIssuesQuery = `query OpenIssues($owner: String!, $name: String!, $labels: [String!], $after: String) {
rateLimit { limit remaining resetAt }
repository(owner: $owner, name: $name) {
issues(first: 100, after: $after, states: [OPEN], labels: $labels) {
pageInfo { hasNextPage endCursor }
nodes { ` + issueFields + ` }
}
}
}`
