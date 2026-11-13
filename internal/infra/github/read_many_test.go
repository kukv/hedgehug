package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

const rateJSON = `"rateLimit":{"limit":5000,"remaining":4321,"resetAt":"2026-11-13T11:00:00Z"}`

// nodeJSON は、番号 n の Open な PR の答え。
func nodeJSON(n int) string {
	return fmt.Sprintf(`{"__typename":"PullRequest","number":%d,"title":"t%d","state":"OPEN","isDraft":false,"reviewDecision":null,`+
		`"reviewRequests":{"totalCount":0},"latestOpinionatedReviews":{"nodes":[]},"commits":{"nodes":[]}}`, n, n)
}

// readManyServer は、GraphQL の本文を queries に溜め、answer(owner, name, 何回目か) の JSON を返す。
func readManyServer(t *testing.T, queries *[]string, answer func(owner, name string, call int) string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Query     string `json:"query"`
			Variables struct{ Owner, Name string }
		}
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &body); err != nil {
			t.Errorf("body = %s", b)
		}
		*queries = append(*queries, body.Query)
		_, _ = w.Write([]byte(answer(body.Variables.Owner, body.Variables.Name, len(*queries))))
	}
}

// answerRange は、番号 first..last を別名 n0, n1, … に入れた答え。
func answerRange(first, last int) string {
	var sb strings.Builder
	for i := first; i <= last; i++ {
		fmt.Fprintf(&sb, `,"n%d":%s`, i-first, nodeJSON(i))
	}
	return `{"data":{` + rateJSON + `,"repository":{"nameWithOwner":"org/web"` + sb.String() + `}}}`
}

func TestReadManyBatches(t *testing.T) {
	var queries []string
	c, _ := newClient(t, readManyServer(t, &queries, func(_, _ string, call int) string {
		if call == 1 {
			return answerRange(1, 100)
		}
		return answerRange(101, 150)
	}))
	numbers := make([]int, 150)
	for i := range numbers {
		numbers[i] = i + 1
	}
	b, err := c.ReadMany(context.Background(), fakeToken, "org/web", numbers)
	if err != nil {
		t.Fatal(err)
	}
	if len(queries) != 2 {
		t.Fatalf("queries = %d, want 2", len(queries))
	}
	for _, want := range []string{"n0: issueOrPullRequest(number: 1)", "n99: issueOrPullRequest(number: 100)", "rateLimit { limit remaining resetAt }"} {
		if !strings.Contains(queries[0], want) {
			t.Errorf("query 1 lacks %q", want)
		}
	}
	if strings.Contains(queries[0], "n100:") || !strings.Contains(queries[1], "n49: issueOrPullRequest(number: 150)") {
		t.Errorf("chunks are wrong: %s", queries[1])
	}
	if b.Repo != "org/web" || len(b.Found) != 150 || len(b.Missing) != 0 || b.Found[150].Kind != itemlink.PR || b.Found[150].Title != "t150" {
		t.Errorf("Batch = %+v", b)
	}
	if b.Rate.Limit != 5000 || b.Rate.Remaining != 4321 || b.Rate.ResetAt.Unix() != 1794567600 {
		t.Errorf("Rate = %+v", b.Rate)
	}
}

func TestReadManyMissing(t *testing.T) {
	c, _ := newClient(t, readManyServer(t, new([]string), func(string, string, int) string {
		return `{"data":{` + rateJSON + `,"repository":{"nameWithOwner":"org/web","n0":` + nodeJSON(1) + `,"n1":null,"n2":null,"n3":` + nodeJSON(4) + `}},` +
			`"errors":[{"type":"NOT_FOUND","path":["repository","n2"]}]}`
	}))
	b, err := c.ReadMany(context.Background(), fakeToken, "org/web", []int{1, 2, 3, 4})
	if err != nil || len(b.Found) != 2 || !slices.Equal(b.Missing, []int{2, 3}) {
		t.Errorf("Batch = %+v, %v", b, err)
	}
}

func TestReadManyRepoUnreadable(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/org/web" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"data":{` + rateJSON + `,"repository":null},"errors":[{"type":"NOT_FOUND","path":["repository"]}]}`))
	})
	_, err := c.ReadMany(context.Background(), fakeToken, "org/web", []int{1})
	var f *ghcall.Failure
	if !asFailure(err, &f) || f.Kind != ghcall.RepoUnreadable || f.Repo != "org/web" {
		t.Errorf("err = %v", err)
	}
}

func TestReadManyRenamed(t *testing.T) {
	var queries, asked []string
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/old/web" {
			_, _ = w.Write([]byte(`{"full_name":"new/web"}`))
			return
		}
		readManyServer(t, &queries, func(owner, name string, _ int) string {
			asked = append(asked, owner+"/"+name)
			if owner == "old" {
				return `{"data":{"repository":null},"errors":[{"type":"NOT_FOUND","path":["repository"]}]}`
			}
			return `{"data":{` + rateJSON + `,"repository":{"nameWithOwner":"new/web","n0":` + nodeJSON(7) + `}}}`
		})(w, r)
	})
	b, err := c.ReadMany(context.Background(), fakeToken, "old/web", []int{7})
	if err != nil || b.Repo != "new/web" || len(b.Found) != 1 || b.Found[7].Repo != "new/web" || strings.Join(asked, ",") != "old/web,new/web" {
		t.Errorf("Batch = %+v, %v, asked %v", b, err, asked)
	}
}

func TestReadManyRateLimited(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Reset", "1793000000")
		_, _ = w.Write([]byte(`{"errors":[{"type":"RATE_LIMITED","message":"x"}]}`))
	})
	_, err := c.ReadMany(context.Background(), fakeToken, "org/web", []int{1})
	var f *ghcall.Failure
	if !asFailure(err, &f) || f.Kind != ghcall.RateLimited || f.ResetAt.Unix() != 1793000000 {
		t.Errorf("err = %v", err)
	}
}

func TestReadManyNoNumbers(t *testing.T) {
	c, _ := newClient(t, func(http.ResponseWriter, *http.Request) { t.Error("asked GitHub") })
	b, err := c.ReadMany(context.Background(), fakeToken, "org/web", nil)
	if err != nil || b.Repo != "org/web" || len(b.Found) != 0 || len(b.Missing) != 0 {
		t.Errorf("Batch = %+v, %v", b, err)
	}
}

// パスに整数(リストの添字)を含む入れ子のエラーがあっても、null でない node は読める。
func TestReadManyNestedErrorWithIntegerPath(t *testing.T) {
	c, _ := newClient(t, readManyServer(t, new([]string), func(string, string, int) string {
		return `{"data":{` + rateJSON + `,"repository":{"nameWithOwner":"org/web","n0":` + nodeJSON(1) + `}},` +
			`"errors":[{"type":"INTERNAL","path":["repository","n0","commits","nodes",0,"commit","statusCheckRollup"]}]}`
	}))
	b, err := c.ReadMany(context.Background(), fakeToken, "org/web", []int{1})
	if err != nil || len(b.Found) != 1 || b.Found[1].Title != "t1" {
		t.Errorf("Batch = %+v, %v", b, err)
	}
}

func TestReadManySecondChunkRateLimited(t *testing.T) {
	c, _ := newClient(t, readManyServer(t, new([]string), func(_, _ string, call int) string {
		if call == 1 {
			return answerRange(1, 100)
		}
		return `{"errors":[{"type":"RATE_LIMITED","message":"x"}]}`
	}))
	numbers := make([]int, 150)
	for i := range numbers {
		numbers[i] = i + 1
	}
	b, err := c.ReadMany(context.Background(), fakeToken, "org/web", numbers)
	if !ghcall.Is(err, ghcall.RateLimited) || b.Found != nil || b.Repo != "" || b.Missing != nil {
		t.Errorf("Batch = %+v, %v", b, err)
	}
}

func TestReadManyRepositoryForbidden(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/org/web" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte(`{"data":{"repository":null},"errors":[{"type":"FORBIDDEN","path":["repository"]}]}`))
	})
	_, err := c.ReadMany(context.Background(), fakeToken, "org/web", []int{1})
	var f *ghcall.Failure
	if !asFailure(err, &f) || f.Kind != ghcall.RepoUnreadable || f.Repo != "org/web" {
		t.Errorf("err = %v", err)
	}
}

func TestReadManyDeduplicatesNumbers(t *testing.T) {
	var queries []string
	c, _ := newClient(t, readManyServer(t, &queries, func(string, string, int) string {
		return `{"data":{` + rateJSON + `,"repository":{"nameWithOwner":"org/web","n0":null,"n1":` + nodeJSON(5) + `}}}`
	}))
	b, err := c.ReadMany(context.Background(), fakeToken, "org/web", []int{3, 5, 3, 5})
	if err != nil || !slices.Equal(b.Missing, []int{3}) || len(b.Found) != 1 || strings.Contains(queries[0], "n2:") {
		t.Errorf("Batch = %+v, %v, query %s", b, err, queries[0])
	}
}
