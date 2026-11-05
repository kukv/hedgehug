package github

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// recorded answers /graphql with the file, and hands back the requests.
func recorded(t *testing.T, file string) (*Client, *[]map[string]any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", file)) //nolint:gosec // a file of the test
	if err != nil {
		t.Fatal(err)
	}
	var reqs []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/graphql" {
			t.Errorf("path %s", r.URL.Path)
		}
		var b map[string]any
		_ = json.NewDecoder(r.Body).Decode(&b)
		reqs = append(reqs, b)
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return New(Options{API: srv.URL}, "t", nil), &reqs
}

func TestScanPulls(t *testing.T) {
	t.Parallel()
	c, _ := recorded(t, "scan-pulls.json")
	since := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	pulls, rl, err := c.ScanPulls("org-a", "web", since)
	if err != nil {
		t.Fatal(err)
	}
	if rl.Remaining != 4812 || rl.Limit != 5000 {
		t.Errorf("rate limit = %+v", rl)
	}
	if len(pulls) != 2 {
		t.Fatalf("pulls = %+v", pulls)
	}
	p := pulls[0]
	if p.Number != 12 || p.State != "open" || p.Review != "pending" || p.Checks != "success" || p.Head != "hh/WEB-42-login" || len(p.Comments) != 1 {
		t.Errorf("first = %+v", p)
	}
	q := pulls[1]
	if q.State != "merged" || q.Review != "approved" || q.Checks != "" || q.MergedAt == nil {
		t.Errorf("second = %+v", q)
	}
}

func TestScanPullsFirstTimeReadsOnePage(t *testing.T) {
	t.Parallel()
	c, reqs := recorded(t, "scan-pulls.json")
	pulls, _, err := c.ScanPulls("org-a", "web", time.Time{})
	if err != nil || len(pulls) != 3 || len(*reqs) != 1 || pulls[2].State != "draft" || pulls[2].Checks != "failure" || pulls[2].Review != "none" {
		t.Fatalf("pulls = %+v, %d requests, %v", pulls, len(*reqs), err)
	}
}

func TestLinkedItems(t *testing.T) {
	t.Parallel()
	c, reqs := recorded(t, "linked.json")
	got, rl, err := c.LinkedItems("org-a", "web", []int{12, 13}, []int{45})
	if err != nil {
		t.Fatal(err)
	}
	if rl.Remaining != 90 {
		t.Errorf("rate limit = %+v", rl)
	}
	if len(got.Pulls) != 1 || got.Pulls[0].State != "closed" || got.Pulls[0].Review != "changes_requested" {
		t.Errorf("pulls = %+v", got.Pulls)
	}
	if len(got.Issues) != 1 || got.Issues[0].State != "closed" || got.Issues[0].StateReason != "not_planned" {
		t.Errorf("issues = %+v", got.Issues)
	}
	if len(got.MissingPulls) != 1 || got.MissingPulls[0] != 13 {
		t.Errorf("missing = %+v", got)
	}
	q, _ := (*reqs)[0]["query"].(string)
	if !strings.Contains(q, "p13: pullRequest(number: 13)") || !strings.Contains(q, "i45: issue(number: 45)") {
		t.Errorf("query:\n%s", q)
	}
}

func TestLinkedItemsAskInBatches(t *testing.T) {
	t.Parallel()
	c, reqs := recorded(t, "linked.json")
	numbers := make([]int, 150)
	for i := range numbers {
		numbers[i] = i + 1
	}
	if _, _, err := c.LinkedItems("org-a", "web", numbers, nil); err != nil {
		t.Fatal(err)
	}
	if len(*reqs) != 2 {
		t.Errorf("%d requests", len(*reqs))
	}
}

func graphqlErr(t *testing.T, answer string) error {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(answer)) }))
	t.Cleanup(srv.Close)
	_, _, err := New(Options{API: srv.URL}, "t", nil).ScanPulls("o", "r", time.Time{})
	return err
}

func TestGraphQLErrors(t *testing.T) {
	t.Parallel()
	if err := graphqlErr(t, `{"data":{"repository":null},"errors":[{"type":"NOT_FOUND","path":["repository"],"message":"no"}]}`); !errors.Is(err, ErrNotFound) {
		t.Errorf("not found: %v", err)
	}
	if err := graphqlErr(t, `{"data":null,"errors":[{"type":"FORBIDDEN","message":"Resource not accessible by integration"}]}`); !errors.Is(err, ErrForbidden) {
		t.Errorf("forbidden: %v", err)
	}
	var rl *RateLimitError
	if err := graphqlErr(t, `{"data":null,"errors":[{"type":"RATE_LIMITED","message":"used up"}]}`); !errors.As(err, &rl) {
		t.Errorf("rate limited: %v", err)
	}
	if err := graphqlErr(t, `{"data":null,"errors":[{"message":"boom"}]}`); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("other: %v", err)
	}
}

func TestGraphQLURL(t *testing.T) {
	t.Parallel()
	for api, want := range map[string]string{
		"https://api.github.com":     "https://api.github.com/graphql",
		"https://ghe.example/api/v3": "https://ghe.example/api/graphql",
	} {
		if got := New(Options{API: api}, "t", nil).graphqlURL(); got != want {
			t.Errorf("%s: %s", api, got)
		}
	}
}
