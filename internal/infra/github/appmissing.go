package github

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// App 未導入の見分け(D4b-11)。OAuth App の組織の制限(docs.github.com の「OAuth app access restrictions」)で
// GitHub が返す文は、docs の本文から全体を確かめられなかった。そこで、文に `OAuth App access restrictions` を
// 含むことだけを条件にする。
const (
	notAccessibleMessage = "Resource not accessible by integration"
	oauthRestrictedText  = "OAuth App access restrictions"
	listPage             = 100
)

// hiddenByApp は、GraphQL のエラーが、App 未導入を示す FORBIDDEN の文(D4b-11 の (1)(2))か。
func hiddenByApp(typ, message string) bool {
	return typ == "FORBIDDEN" &&
		(strings.Contains(message, notAccessibleMessage) || strings.Contains(message, oauthRestrictedText))
}

// appMissing は、repo の owner に GitHub App が入っていない失敗。
func appMissing(repo string) *ghcall.Failure {
	owner, _, _ := strings.Cut(repo, "/")
	return &ghcall.Failure{Kind: ghcall.AppNotInstalled, Org: owner, Repo: repo}
}

// attempt は、once を repo で呼び、リポジトリを読めなければ、改名・移管を確かめて 1 回だけ新しい名前で呼び直す。
// なお読めなければ、App 未導入かを確かめる(explain)。
func attempt[T any](ctx context.Context, c *Client, token, repo string, once func(repo string) (T, error)) (T, error) {
	v, err := once(repo)
	if !ghcall.Is(err, ghcall.RepoUnreadable) {
		return v, err
	}
	if moved, ok := c.renamed(ctx, token, repo); ok {
		repo = moved
		v, err = once(repo)
	}
	return v, c.explain(ctx, token, repo, err)
}

// explain は、リポジトリを読めない失敗 err を、App 未導入なら AppNotInstalled に置き換える(D4b-11 の (3))。
// GitHub は App が入っていない非公開のリポジトリを 404 で隠す。導入の一覧を読めない(App のトークンでない)ときは、
// 何も分からないので err のまま返す。
func (c *Client) explain(ctx context.Context, token, repo string, err error) error {
	if !ghcall.Is(err, ghcall.RepoUnreadable) || !wellFormed(repo) {
		return err
	}
	owner, name, _ := strings.Cut(repo, "/")
	if c.appNotInstalled(ctx, token, owner, name) {
		return appMissing(repo)
	}
	return err
}

type installation struct {
	ID      int64 `json:"id"`
	Account struct {
		Login string `json:"login"`
	} `json:"account"`
	RepositorySelection string `json:"repository_selection"`
}

type installationList struct {
	Installations []installation `json:"installations"`
}

type repositoryList struct {
	Repositories []struct {
		Name string `json:"name"`
	} `json:"repositories"`
}

// appNotInstalled は、token の App の導入に owner がない、または導入が選んだリポジトリだけで name を含まないとき真。
func (c *Client) appNotInstalled(ctx context.Context, token, owner, name string) bool {
	var found *installation
	err := paged(ctx, c, token, "/user/installations", func(p installationList) int {
		for i := range p.Installations {
			if strings.EqualFold(p.Installations[i].Account.Login, owner) {
				found = &p.Installations[i]
			}
		}
		return len(p.Installations)
	})
	if err != nil {
		return false
	}
	if found == nil {
		return true
	}
	if found.RepositorySelection != "selected" {
		return false
	}
	listed := false
	err = paged(ctx, c, token, "/user/installations/"+strconv.FormatInt(found.ID, 10)+"/repositories", func(p repositoryList) int {
		for _, r := range p.Repositories {
			listed = listed || strings.EqualFold(r.Name, name)
		}
		return len(p.Repositories)
	})
	return err == nil && !listed
}

// paged は、100 件ずつの一覧を、page が受け取った件数が 100 に満たなくなるまでめくる。
func paged[P any](ctx context.Context, c *Client, token, path string, page func(P) int) error {
	for n := 1; ; n++ {
		var p P
		if err := c.get(ctx, token, path+"?per_page="+strconv.Itoa(listPage)+"&page="+strconv.Itoa(n), &p); err != nil {
			return err
		}
		if page(p) < listPage {
			return nil
		}
	}
}

// checkQuery は、リポジトリを読めるかだけを確かめる GraphQL。
const checkQuery = `query Check($owner: String!, $name: String!) { rateLimit { limit remaining resetAt }
repository(owner: $owner, name: $name) { id } }`

// CheckRepo は port.GitHub.CheckRepo。
func (c *Client) CheckRepo(ctx context.Context, token, repo string) error {
	_, err := attempt(ctx, c, token, repo, func(repo string) (struct{}, error) {
		return struct{}{}, c.checkOnce(ctx, token, repo)
	})
	return err
}

func (c *Client) checkOnce(ctx context.Context, token, repo string) error {
	owner, name, _ := strings.Cut(repo, "/")
	body, _ := json.Marshal(map[string]any{
		"query": checkQuery, "variables": map[string]any{"owner": owner, "name": name},
	})
	var a readManyAnswer
	h, err := c.do(ctx, http.MethodPost, token, "/graphql", bytes.NewReader(body), &a)
	if err != nil {
		return withRepo(err, repo)
	}
	if f := a.failure(repo, c.resetAt(h)); f != nil {
		return f
	}
	return nil
}
