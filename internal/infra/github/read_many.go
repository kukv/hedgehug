package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// readManySize は、1 回の問い合わせにまとめる番号の数(D4a-3)。
const readManySize = 100

type readManyAnswer struct {
	Data struct {
		RateLimit *gqlRate `json:"rateLimit"`
		// Repository は、nameWithOwner と別名 n0, n1, … の値(null か node)を持つ。
		Repository map[string]json.RawMessage `json:"repository"`
	} `json:"data"`
	Errors []gqlError `json:"errors"`
}

// ReadMany は port.GitHub.ReadMany。リポジトリを読めなければ、REST で改名・移管を確かめ、名前が変わっていれば
// 新しい名前で 1 回だけ読み直す。
func (c *Client) ReadMany(ctx context.Context, token, repo string, numbers []int) (port.Batch, error) {
	if numbers = unique(numbers); len(numbers) == 0 {
		return port.Batch{Repo: repo}, nil
	}
	return attempt(ctx, c, token, repo, func(repo string) (port.Batch, error) {
		return c.readManyOnce(ctx, token, repo, numbers)
	})
}

func (c *Client) readManyOnce(ctx context.Context, token, repo string, numbers []int) (port.Batch, error) {
	batch := port.Batch{Repo: repo, Found: map[int]port.Found{}}
	for start := 0; start < len(numbers); start += readManySize {
		chunk := numbers[start:min(start+readManySize, len(numbers))]
		if err := c.readChunk(ctx, token, repo, chunk, &batch); err != nil {
			return port.Batch{}, err
		}
	}
	return batch, nil
}

// readChunk は、chunk の番号を 1 回の問い合わせで読み、batch に足す。
func (c *Client) readChunk(ctx context.Context, token, repo string, chunk []int, batch *port.Batch) error {
	owner, name, _ := strings.Cut(repo, "/")
	var a readManyAnswer
	h, err := c.graphql(ctx, token, repo, map[string]any{
		"query": readManyQuery(chunk), "variables": map[string]any{"owner": owner, "name": name},
	}, &a)
	if err != nil {
		return err
	}
	if f := a.failure(repo, c.resetAt(h)); f != nil {
		return f
	}
	var full string
	_ = json.Unmarshal(a.Data.Repository["nameWithOwner"], &full)
	batch.Repo = full
	for i, number := range chunk {
		var n *node
		_ = json.Unmarshal(a.Data.Repository["n"+strconv.Itoa(i)], &n)
		if n == nil {
			batch.Missing = append(batch.Missing, number)
			continue
		}
		batch.Found[number] = found(full, n)
	}
	if a.Data.RateLimit != nil {
		batch.Rate = a.Data.RateLimit.port()
	}
	return nil
}

// readManyQuery は、番号ごとの別名 n0, n1, … を持つ問い合わせ。番号は整数なので、文字列に埋め込んでよい。
func readManyQuery(numbers []int) string {
	var sb strings.Builder
	sb.WriteString("query ReadMany($owner: String!, $name: String!) { rateLimit { limit remaining resetAt } " +
		"repository(owner: $owner, name: $name) { nameWithOwner")
	for i, n := range numbers {
		fmt.Fprintf(&sb, " n%d: issueOrPullRequest(number: %d) { ...F }", i, n)
	}
	sb.WriteString(" } }\n" + nodeFragment)
	return sb.String()
}

// failure は、答えの失敗(D3-5)。別名のパスの NOT_FOUND(長さ 2)は、その番号が Missing になるだけで失敗ではない。
func (a readManyAnswer) failure(repo string, reset time.Time) *ghcall.Failure {
	return gqlFailure(a.Errors, a.Data.Repository == nil, repo, reset)
}

// unique は、numbers から重複を除く(最初に出た順)。
func unique(numbers []int) []int {
	seen := make(map[int]bool, len(numbers))
	out := make([]int, 0, len(numbers))
	for _, n := range numbers {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}
