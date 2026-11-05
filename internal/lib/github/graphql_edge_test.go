package github

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type reviewNodes = struct {
	Nodes []struct {
		State string `json:"state"`
	} `json:"nodes"`
}

func reviews(states ...string) reviewNodes {
	var r reviewNodes
	for _, s := range states {
		r.Nodes = append(r.Nodes, struct {
			State string `json:"state"`
		}{s})
	}
	return r
}

func str(s string) *string { return &s }

func TestPullReview(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name     string
		decision *string
		latest   []string
		requests int
		want     string
	}{
		{"decision approved", str("APPROVED"), nil, 0, "approved"},
		{"decision changes", str("CHANGES_REQUESTED"), []string{"APPROVED"}, 0, "changes_requested"},
		{"no rule, latest changes", nil, []string{"APPROVED", "CHANGES_REQUESTED"}, 0, "changes_requested"},
		{"no rule, latest approved", nil, []string{"COMMENTED", "APPROVED"}, 1, "approved"},
		{"no rule, requested", nil, []string{"COMMENTED"}, 1, "pending"},
		{"no rule, nothing", nil, nil, 0, "none"},
		{"required but nobody asked", str("REVIEW_REQUIRED"), nil, 0, "none"},
		{"required and asked", str("REVIEW_REQUIRED"), nil, 2, "pending"},
		{"required, approved by one", str("REVIEW_REQUIRED"), []string{"APPROVED"}, 1, "approved"},
	} {
		n := pullNode{ReviewDecision: c.decision, LatestOpinionatedReviews: reviews(c.latest...)}
		n.ReviewRequests.TotalCount = c.requests
		if got := n.pull().Review; got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
}

func TestIssueStateReason(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		state  string
		reason *string
		want   string
	}{
		{"OPEN", nil, ""},
		{"OPEN", str("REOPENED"), ""},
		{"CLOSED", str("COMPLETED"), "completed"},
		{"CLOSED", str("NOT_PLANNED"), "not_planned"},
		{"CLOSED", str("DUPLICATE"), "not_planned"},
		{"CLOSED", nil, "completed"},
	} {
		got := issueNode{State: c.state, StateReason: c.reason}.issue()
		if got.StateReason != c.want {
			t.Errorf("%s %v: %q, want %q", c.state, c.reason, got.StateReason, c.want)
		}
	}
}

// answering serves body with the status and headers for every request.
func answering(t *testing.T, status int, headers map[string]string, body string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return New(Options{API: srv.URL}, "t", nil)
}

func TestGraphQLPathWithIndex(t *testing.T) {
	t.Parallel()
	c := answering(t, 200, nil, `{"data":{"rateLimit":{"limit":1,"remaining":1,"resetAt":"2026-11-05T12:00:00Z"},"repository":{"p1":null}},
"errors":[{"type":"NOT_FOUND","path":["repository","pullRequests","nodes",0,"x"],"message":"gone"},{"type":"NOT_FOUND","path":["repository","p1"],"message":"gone"}]}`)
	got, _, err := c.LinkedItems("o", "r", []int{1}, nil)
	if err != nil || len(got.MissingPulls) != 1 {
		t.Fatalf("%+v, %v", got, err)
	}
}

func TestGraphQLRateLimitedResetHeader(t *testing.T) {
	t.Parallel()
	c := answering(t, 200, map[string]string{"X-RateLimit-Reset": "1793880000"}, `{"data":null,"errors":[{"type":"RATE_LIMITED","message":"used up"}]}`)
	_, _, err := c.ScanPulls("o", "r", time.Time{})
	var rl *RateLimitError
	if !errors.As(err, &rl) || !rl.Reset.Equal(time.Unix(1793880000, 0)) {
		t.Fatalf("err = %v", err)
	}
	c = answering(t, 200, nil, `{"data":null,"errors":[{"type":"RATE_LIMITED","message":"used up"}]}`)
	_, _, err = c.ScanPulls("o", "r", time.Time{})
	if !errors.As(err, &rl) || time.Until(rl.Reset) < 30*time.Second {
		t.Fatalf("no header: %v", err)
	}
}

func TestGraphQLStatusErrors(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		status  int
		headers map[string]string
		check   func(error) bool
	}{
		{401, nil, func(e error) bool { return errors.Is(e, ErrUnauthorized) }},
		{403, nil, func(e error) bool { return errors.Is(e, ErrForbidden) }},
		{429, map[string]string{"Retry-After": "7"}, func(e error) bool { var r *RateLimitError; return errors.As(e, &r) }},
		{502, nil, func(e error) bool { return e != nil && strings.Contains(e.Error(), "502") }},
	} {
		_, _, err := answering(t, c.status, c.headers, `{"message":"x"}`).ScanPulls("o", "r", time.Time{})
		if !c.check(err) {
			t.Errorf("%d: %v", c.status, err)
		}
	}
}

func TestGraphQLNoData(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`{"data":null}`, `{}`} {
		if _, _, err := answering(t, 200, nil, body).ScanPulls("o", "r", time.Time{}); err == nil {
			t.Errorf("%s: no error", body)
		}
	}
}

func TestLinkedItemsDeduplicates(t *testing.T) {
	t.Parallel()
	c, reqs := recorded(t, "linked.json")
	if _, _, err := c.LinkedItems("org-a", "web", []int{12, 12, 13}, []int{45, 45}); err != nil {
		t.Fatal(err)
	}
	q, _ := (*reqs)[0]["query"].(string)
	if len(*reqs) != 1 || strings.Count(q, "p12:") != 1 || strings.Count(q, "i45:") != 1 {
		t.Errorf("%d requests:\n%s", len(*reqs), q)
	}
}
