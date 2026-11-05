package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// RateLimit is what GraphQL says of the rate limit.
type RateLimit struct {
	Limit     int       `json:"limit"`
	Remaining int       `json:"remaining"`
	ResetAt   time.Time `json:"resetAt"`
}

// Pull is a pull request as Hedgehug keeps it.
type Pull struct {
	Number    int
	Title     string
	Body      string
	Head      string // the branch
	State     string // open, draft, merged, closed
	Review    string // none, pending, approved, changes_requested
	Checks    string // success, failure, pending; empty with no checks
	MergedAt  *time.Time
	UpdatedAt time.Time
	Comments  []string
}

// Issue is an issue as Hedgehug keeps it.
type Issue struct {
	Number      int
	State       string // open, closed
	StateReason string // completed, not_planned; empty when open
	UpdatedAt   time.Time
}

// Linked is the answer of LinkedItems. The numbers GitHub does not know are
// in the Missing lists.
type Linked struct {
	Pulls         []Pull
	Issues        []Issue
	MissingPulls  []int
	MissingIssues []int
}

const pullFields = `number title body state isDraft mergedAt updatedAt headRefName reviewDecision
reviewRequests(first: 1) { totalCount }
latestOpinionatedReviews(first: 100) { nodes { state } }
commits(last: 1) { nodes { commit { statusCheckRollup { state } } } }
comments(first: 100) { nodes { body } }`

const issueFields = `number state stateReason updatedAt`

const scanPullsQuery = `query ScanPulls($owner: String!, $name: String!, $after: String) {
rateLimit { limit remaining resetAt }
repository(owner: $owner, name: $name) {
pullRequests(first: 100, after: $after, orderBy: {field: UPDATED_AT, direction: DESC}) {
pageInfo { hasNextPage endCursor }
nodes { ` + pullFields + ` }
}
}
}`

// linkedBatch is how many numbers one query of LinkedItems asks for.
const linkedBatch = 100

type pullNode struct {
	Number         int        `json:"number"`
	Title          string     `json:"title"`
	Body           string     `json:"body"`
	State          string     `json:"state"`
	IsDraft        bool       `json:"isDraft"`
	MergedAt       *time.Time `json:"mergedAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	HeadRefName    string     `json:"headRefName"`
	ReviewDecision *string    `json:"reviewDecision"`
	ReviewRequests struct {
		TotalCount int `json:"totalCount"`
	} `json:"reviewRequests"`
	LatestOpinionatedReviews struct {
		Nodes []struct {
			State string `json:"state"`
		} `json:"nodes"`
	} `json:"latestOpinionatedReviews"`
	Commits struct {
		Nodes []struct {
			Commit struct {
				StatusCheckRollup *struct {
					State string `json:"state"`
				} `json:"statusCheckRollup"`
			} `json:"commit"`
		} `json:"nodes"`
	} `json:"commits"`
	Comments struct {
		Nodes []struct {
			Body string `json:"body"`
		} `json:"nodes"`
	} `json:"comments"`
}

func (n pullNode) pull() Pull {
	p := Pull{Number: n.Number, Title: n.Title, Body: n.Body, Head: n.HeadRefName, MergedAt: n.MergedAt, UpdatedAt: n.UpdatedAt}
	switch {
	case n.State == "MERGED":
		p.State = "merged"
	case n.State == "CLOSED":
		p.State = "closed"
	case n.IsDraft:
		p.State = "draft"
	default:
		p.State = "open"
	}
	p.Review = n.review()
	if len(n.Commits.Nodes) > 0 {
		if r := n.Commits.Nodes[0].Commit.StatusCheckRollup; r != nil {
			switch r.State {
			case "SUCCESS":
				p.Checks = "success"
			case "FAILURE", "ERROR":
				p.Checks = "failure"
			default:
				p.Checks = "pending"
			}
		}
	}
	for _, c := range n.Comments.Nodes {
		p.Comments = append(p.Comments, c.Body)
	}
	return p
}

// review is the state of the review. reviewDecision is null in a repository
// with no required review, so the latest review of each reviewer decides
// then.
func (n pullNode) review() string {
	if d := n.ReviewDecision; d != nil && (*d == "APPROVED" || *d == "CHANGES_REQUESTED") {
		return strings.ToLower(*d)
	}
	approved := false
	for _, r := range n.LatestOpinionatedReviews.Nodes {
		switch r.State {
		case "CHANGES_REQUESTED":
			return "changes_requested"
		case "APPROVED":
			approved = true
		}
	}
	switch {
	case approved:
		return "approved"
	case n.ReviewRequests.TotalCount > 0:
		return "pending"
	}
	return "none"
}

type issueNode struct {
	Number      int       `json:"number"`
	State       string    `json:"state"`
	StateReason *string   `json:"stateReason"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (n issueNode) issue() Issue {
	i := Issue{Number: n.Number, State: strings.ToLower(n.State), UpdatedAt: n.UpdatedAt}
	if i.State != "closed" {
		return i // REOPENED is no reason to keep
	}
	// An issue closed before GitHub kept the reason has none: it was completed.
	i.StateReason = "completed"
	if r := n.StateReason; r != nil && (*r == "NOT_PLANNED" || *r == "DUPLICATE") {
		i.StateReason = "not_planned"
	}
	return i
}

type graphqlError struct {
	Type    string `json:"type"`
	Path    []any  `json:"path"` // names and list indexes
	Message string `json:"message"`
}

// graphqlURL is the GraphQL endpoint of the host: /graphql on github.com,
// /api/graphql on GitHub Enterprise Server, whose REST base ends in /api/v3.
func (c *Client) graphqlURL() string { return strings.TrimSuffix(c.api, "/v3") + "/graphql" }

// graphql runs query and decodes data into out. An error of one item (a
// NOT_FOUND with a path of two or more) leaves that item null in data and
// is no error here.
//
// Any other error fails the whole call even when data came with it: GitHub
// then answers part of the query, and the caller would take the part for the
// whole. The failure is a warning of the repository to the caller.
func (c *Client) graphql(query string, vars map[string]any, out any) error {
	var res struct {
		Data   json.RawMessage `json:"data"`
		Errors []graphqlError  `json:"errors"`
	}
	in := map[string]any{"query": query, "variables": vars}
	h, err := c.do("POST", c.graphqlURL(), in, &res)
	if err != nil {
		return err
	}
	for _, e := range res.Errors {
		switch {
		case e.Type == "NOT_FOUND" && len(e.Path) > 1:
		case e.Type == "NOT_FOUND":
			return fmt.Errorf("%w: %s", ErrNotFound, e.Message)
		case e.Type == "FORBIDDEN":
			return fmt.Errorf("%w: %s", ErrForbidden, e.Message)
		case e.Type == "RATE_LIMITED":
			reset := time.Now().Add(time.Minute)
			if s, err := strconv.ParseInt(h.Get("X-RateLimit-Reset"), 10, 64); err == nil {
				reset = time.Unix(s, 0)
			}
			return &RateLimitError{Reset: reset}
		default:
			return errors.New("GitHub GraphQL: " + e.Message)
		}
	}
	if len(res.Data) == 0 || string(res.Data) == "null" {
		return errors.New("GitHub GraphQL: the answer has no data")
	}
	return json.Unmarshal(res.Data, out)
}

// ScanPulls lists the pull requests of owner/name updated at since or later,
// the latest first. With a zero since it reads the latest page only.
func (c *Client) ScanPulls(owner, name string, since time.Time) ([]Pull, RateLimit, error) {
	var out []Pull
	var rl RateLimit
	vars := map[string]any{"owner": owner, "name": name}
	for {
		var data struct {
			RateLimit  RateLimit `json:"rateLimit"`
			Repository struct {
				PullRequests struct {
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
					Nodes []pullNode `json:"nodes"`
				} `json:"pullRequests"`
			} `json:"repository"`
		}
		if err := c.graphql(scanPullsQuery, vars, &data); err != nil {
			return nil, rl, err
		}
		rl = data.RateLimit
		page := data.Repository.PullRequests
		for _, n := range page.Nodes {
			if !since.IsZero() && n.UpdatedAt.Before(since) {
				return out, rl, nil
			}
			out = append(out, n.pull())
		}
		if since.IsZero() || !page.PageInfo.HasNextPage {
			return out, rl, nil
		}
		vars["after"] = page.PageInfo.EndCursor
	}
}

// LinkedItems reads the pull requests and the issues of owner/name by number,
// in as many queries as it takes.
func (c *Client) LinkedItems(owner, name string, pulls, issues []int) (Linked, RateLimit, error) {
	var out Linked
	var rl RateLimit
	pulls, issues = dedupe(pulls), dedupe(issues)
	for len(pulls)+len(issues) > 0 {
		n := min(len(pulls), linkedBatch)
		ps := pulls[:n]
		m := min(len(issues), linkedBatch-n)
		is := issues[:m]
		pulls, issues = pulls[n:], issues[m:]
		var data struct {
			RateLimit  RateLimit                  `json:"rateLimit"`
			Repository map[string]json.RawMessage `json:"repository"`
		}
		vars := map[string]any{"owner": owner, "name": name}
		if err := c.graphql(linkedQuery(ps, is), vars, &data); err != nil {
			return out, rl, err
		}
		rl = data.RateLimit
		for _, x := range ps {
			raw := data.Repository[fmt.Sprintf("p%d", x)]
			var node *pullNode
			if err := json.Unmarshal(raw, &node); err != nil && len(raw) > 0 {
				return out, rl, err
			}
			if node == nil {
				out.MissingPulls = append(out.MissingPulls, x)
				continue
			}
			out.Pulls = append(out.Pulls, node.pull())
		}
		for _, x := range is {
			raw := data.Repository[fmt.Sprintf("i%d", x)]
			var node *issueNode
			if err := json.Unmarshal(raw, &node); err != nil && len(raw) > 0 {
				return out, rl, err
			}
			if node == nil {
				out.MissingIssues = append(out.MissingIssues, x)
				continue
			}
			out.Issues = append(out.Issues, node.issue())
		}
	}
	return out, rl, nil
}

// dedupe drops the repeated numbers, keeping the order.
func dedupe(in []int) []int {
	var out []int
	for _, n := range in {
		if !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	return out
}

func linkedQuery(pulls, issues []int) string {
	var b strings.Builder
	b.WriteString("query Linked($owner: String!, $name: String!) {\nrateLimit { limit remaining resetAt }\nrepository(owner: $owner, name: $name) {\n")
	for _, n := range pulls {
		fmt.Fprintf(&b, "p%d: pullRequest(number: %d) { %s }\n", n, n, pullFields)
	}
	for _, n := range issues {
		fmt.Fprintf(&b, "i%d: issue(number: %d) { %s }\n", n, n, issueFields)
	}
	b.WriteString("}\n}")
	return b.String()
}
