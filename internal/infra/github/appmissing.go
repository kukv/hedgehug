package github

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// App 未導入の見分け(D4b-11)。OAuth App の組織の制限で GitHub が返す文は、docs.github.com の
// 「About OAuth app access restrictions」に全文がない。文は利用者の報告から拾った:
// https://answers.netlify.com/t/apierror-403-related-to-organization-has-enabled-oauth-application-access-restrictions/68385
// 「...the `org` organization has enabled OAuth App access restrictions, meaning that data access to third-parties is
// limited...」。そこで、文に `OAuth App access restrictions` を含むことだけを条件にする。
const (
	notAccessibleMessage = "Resource not accessible by integration"
	oauthRestrictedText  = "OAuth App access restrictions"
	listPage             = 100
)

// hiddenByApp は、GraphQL のエラーが、App 未導入を示す FORBIDDEN の文(D4b-11 の (1)(2))か。
// リポジトリそのもの(path が repository の 1 つ)の失敗だけを見る。App が入っていて PR・Issue の権限がないときの
// 同じ文は repository.issueOrPullRequest(path が 2 つ)に来るので、App 未導入にしない。
func hiddenByApp(typ string, path []any, message string) bool {
	return typ == "FORBIDDEN" && len(path) == 1 &&
		(strings.Contains(message, notAccessibleMessage) || strings.Contains(message, oauthRestrictedText))
}

// appMissing は、repo の owner に GitHub App が入っていない失敗。
func appMissing(repo string) *ghcall.Failure {
	owner, _, _ := strings.Cut(repo, "/")
	return &ghcall.Failure{Kind: ghcall.AppNotInstalled, Org: owner, Repo: repo}
}

// attempt は、once を repo で呼び、リポジトリを読めなければ、改名・移管を確かめて 1 回だけ新しい名前で呼び直す。
// なお読めなければ、App 未導入かを確かめる(explain)。確かめる問い合わせが 403・404 以外で失敗したら、その失敗を返す。
func attempt[T any](ctx context.Context, c *Client, token, repo string, once func(repo string) (T, error)) (T, error) {
	v, err := once(repo)
	if !ghcall.Is(err, ghcall.RepoUnreadable) {
		return v, err
	}
	moved, ok, perr := c.renamed(ctx, token, repo)
	if perr != nil {
		return v, perr
	}
	if ok {
		repo = moved
		v, err = once(repo)
	}
	return v, c.explain(ctx, token, repo, err)
}

// explain は、リポジトリを読めない失敗 err を、App 未導入なら AppNotInstalled に置き換える(D4b-11 の (3))。
// GitHub は App が入っていない非公開のリポジトリを 404 で隠す。確かめる問い合わせ(導入の一覧・選んだリポジトリの
// 一覧・owner のアカウント)が 403・404 で断られたら(App のトークンでない・ない)、App 未導入とは言えないので
// err(RepoUnreadable)のまま返す。それ以外の失敗(接続できない・利用上限・トークンの拒否)は、その失敗を返す
// (読めないと言えないのに読めないとすると、「再確認」が記録を消してしまう。D4b 最終レビュー R-2)。
func (c *Client) explain(ctx context.Context, token, repo string, err error) error {
	if !ghcall.Is(err, ghcall.RepoUnreadable) || !wellFormed(repo) {
		return err
	}
	owner, name, _ := strings.Cut(repo, "/")
	missing, perr := c.appNotInstalled(ctx, token, owner, name)
	switch {
	case perr != nil:
		return perr
	case missing:
		return appMissing(repo)
	}
	return err
}

// probeErr は、確かめる問い合わせの失敗 err のうち、呼び手に返すもの。成功と 403・404 は nil。
func probeErr(err error) error {
	if err == nil || ghcall.Is(err, ghcall.Forbidden) || ghcall.Is(err, ghcall.NotFound) {
		return nil
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
// 確かめる問い合わせの失敗は probeErr で返す(403・404 なら偽で nil)。
func (c *Client) appNotInstalled(ctx context.Context, token, owner, name string) (bool, error) {
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
		return false, probeErr(err)
	}
	if found == nil {
		return c.accountExists(ctx, token, owner)
	}
	if found.RepositorySelection != "selected" {
		return false, nil
	}
	listed := false
	err = paged(ctx, c, token, "/user/installations/"+strconv.FormatInt(found.ID, 10)+"/repositories", func(p repositoryList) int {
		for _, r := range p.Repositories {
			listed = listed || strings.EqualFold(r.Name, name)
		}
		return len(p.Repositories)
	})
	if err != nil {
		return false, probeErr(err)
	}
	return !listed, nil
}

// accountExists は、owner のアカウント(ユーザーか組織)があるか。打ち間違えた owner を App 未導入にしないため
// (D4b 最終レビュー I-2)。403・404 なら、あるとは言えないので偽。ほかの失敗は probeErr で返す。
func (c *Client) accountExists(ctx context.Context, token, owner string) (bool, error) {
	var u struct {
		Login string `json:"login"`
	}
	err := c.get(ctx, token, "/users/"+url.PathEscape(owner), &u)
	return err == nil, probeErr(err)
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
