package github

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

// pullJSON は、番号 n・更新 updated の PR の答え。extra は、PR のフィールドに足す JSON(状態など)。
func pullJSON(n int, updated time.Time, extra string) string {
	state := `"state":"OPEN","isDraft":false,"reviewDecision":null,"reviewRequests":{"totalCount":0}`
	if extra != "" {
		state = extra
	}
	return fmt.Sprintf(`{"__typename":"PullRequest","number":%d,"title":"t%d",%s,`+
		`"latestOpinionatedReviews":{"nodes":[]},"commits":{"nodes":[]},`+
		`"headRefName":"hh/web-%d","body":"body%d","updatedAt":%q,`+
		`"comments":{"nodes":[{"body":"c%d-1"},{"body":"c%d-2"}]}}`, n, n, state, n, n, updated.Format(time.RFC3339), n, n)
}

// pullsPageJSON は、nodes(PR の JSON)を 1 ページにした答え。
func pullsPageJSON(next bool, cursor string, nodes ...string) string {
	return fmt.Sprintf(`{"data":{%s,"repository":{"nameWithOwner":"org/web","pullRequests":{`+
		`"pageInfo":{"hasNextPage":%t,"endCursor":%q},"nodes":[%s]}}}}`, rateJSON, next, cursor, strings.Join(nodes, ","))
}

func TestReadPullsFirst(t *testing.T) {
	var queries []string
	t0 := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	draft := `"state":"OPEN","isDraft":true,"reviewDecision":null,"reviewRequests":{"totalCount":0}`
	merged := `"state":"MERGED","isDraft":false,"reviewDecision":null,"reviewRequests":{"totalCount":0}`
	pending := `"state":"OPEN","isDraft":false,"reviewDecision":null,"reviewRequests":{"totalCount":1}`
	c, _ := newClient(t, readManyServer(t, &queries, func(_, _ string, _ int) string {
		nodes := []string{pullJSON(1, t0, draft), pullJSON(2, t0.Add(-time.Hour), merged), pullJSON(3, t0.Add(-2*time.Hour), pending)}
		for i := 4; i <= 100; i++ {
			nodes = append(nodes, pullJSON(i, t0.Add(-time.Duration(i)*time.Hour), ""))
		}
		return pullsPageJSON(true, "cur1", nodes...)
	}))
	got, err := c.ReadPulls(context.Background(), fakeToken, "org/web", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(queries) != 1 || len(got.Pulls) != 100 {
		t.Fatalf("queries = %d, pulls = %d; want 1, 100", len(queries), len(got.Pulls))
	}
	for _, want := range []string{"first: 100", "orderBy: {field: UPDATED_AT, direction: DESC}", "headRefName", "comments(first: 100)"} {
		if !strings.Contains(queries[0], want) {
			t.Errorf("query lacks %q", want)
		}
	}
	p := got.Pulls[0]
	if p.Number != 1 || p.Branch != "hh/web-1" || p.Text.Title != "t1" || p.Values.Title != "t1" || p.Body != "body1" || !p.UpdatedAt.Equal(t0) ||
		len(p.Comments) != 2 || p.Comments[0] != "c1-1" || p.Comments[1] != "c1-2" {
		t.Errorf("Pull = %+v", p)
	}
	for i, want := range []itemlink.State{itemlink.Draft, itemlink.Merged, itemlink.Open} {
		if got.Pulls[i].State != want {
			t.Errorf("Pulls[%d].State = %v, want %v", i, got.Pulls[i].State, want)
		}
	}
	if got.Pulls[2].Review != itemlink.Pending || got.Pulls[0].Review != itemlink.NoReview || got.Pulls[0].Checks != itemlink.NoChecks {
		t.Errorf("review/checks = %v %v %v", got.Pulls[2].Review, got.Pulls[0].Review, got.Pulls[0].Checks)
	}
}

func TestReadPullsSince(t *testing.T) {
	var queries []string
	since := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	c, _ := newClient(t, readManyServer(t, &queries, func(_, _ string, call int) string {
		if call == 1 {
			return pullsPageJSON(true, "cur1", pullJSON(1, since.Add(2*time.Hour), ""), pullJSON(2, since.Add(time.Hour), ""))
		}
		return pullsPageJSON(true, "cur2", pullJSON(3, since, ""), pullJSON(4, since.Add(-time.Second), ""), pullJSON(5, since.Add(-time.Hour), ""))
	}))
	got, err := c.ReadPulls(context.Background(), fakeToken, "org/web", since)
	if err != nil {
		t.Fatal(err)
	}
	if len(queries) != 2 {
		t.Fatalf("queries = %d, want 2", len(queries))
	}
	var numbers []int
	for _, p := range got.Pulls {
		numbers = append(numbers, p.Number)
	}
	if fmt.Sprint(numbers) != "[1 2 3]" {
		t.Errorf("numbers = %v, want [1 2 3] (since ちょうどを含み、古いものを含まない)", numbers)
	}
}

func TestReadPullsRenamed(t *testing.T) {
	calls := 0
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/graphql":
			calls++
			if calls == 1 {
				_, _ = w.Write([]byte(`{"data":{"repository":null},"errors":[{"type":"NOT_FOUND","path":["repository"],"message":"x"}]}`))
				return
			}
			_, _ = w.Write([]byte(strings.Replace(pullsPageJSON(false, "", pullJSON(1, now, "")), "org/web", "new/web", 1)))
		case "/repos/old/web":
			_, _ = w.Write([]byte(`{"full_name":"new/web"}`))
		default:
			http.NotFound(w, r)
		}
	})
	got, err := c.ReadPulls(context.Background(), fakeToken, "old/web", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Repo != "new/web" || len(got.Pulls) != 1 {
		t.Errorf("Pulls = %+v", got)
	}
}

func TestReadPullsRate(t *testing.T) {
	var queries []string
	since := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	c, _ := newClient(t, readManyServer(t, &queries, func(_, _ string, call int) string {
		if call == 1 {
			return strings.Replace(pullsPageJSON(true, "c", pullJSON(1, since.Add(time.Hour), "")), `"remaining":4321`, `"remaining":4000`, 1)
		}
		return pullsPageJSON(false, "", pullJSON(2, since, ""))
	}))
	got, err := c.ReadPulls(context.Background(), fakeToken, "org/web", since)
	if err != nil {
		t.Fatal(err)
	}
	if got.Rate.Limit != 5000 || got.Rate.Remaining != 4321 || got.Rate.ResetAt.Unix() != 1794567600 {
		t.Errorf("Rate = %+v, want 最後のページの値", got.Rate)
	}
}

func TestReadPullsRateLimited(t *testing.T) {
	var queries []string
	c, _ := newClient(t, readManyServer(t, &queries, func(_, _ string, _ int) string {
		return `{"data":null,"errors":[{"type":"RATE_LIMITED","message":"x"}]}`
	}))
	_, err := c.ReadPulls(context.Background(), fakeToken, "org/web", time.Time{})
	if !ghcall.Is(err, ghcall.RateLimited) {
		t.Errorf("err = %v, want RateLimited", err)
	}
}
