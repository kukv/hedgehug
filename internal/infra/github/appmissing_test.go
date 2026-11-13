package github

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

const (
	forbiddenJSON = `{"data":{"repository":null},"errors":[{"type":"FORBIDDEN","path":["repository"],"message":"%s"}]}`
	notFoundJSON  = `{"data":{"repository":null},"errors":[{"type":"NOT_FOUND","path":["repository"],"message":"Could not resolve to a Repository"}]}`
	okReadMany    = `{"data":{"repository":{"nameWithOwner":"example-org/web"}}}`
	okCheck       = `{"data":{"repository":{"id":"R_1"}}}`
)

// githubServer は、GraphQL の答え graphql と、/user/installations 系の答えを返す偽の GitHub。
// installations が空なら /user/installations は 403(App のトークンでない)。selected は選んだリポジトリの一覧(nil なら 404)。
func githubServer(graphql, installations string, selected *string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/graphql":
			_, _ = w.Write([]byte(graphql))
		case r.URL.Path == "/user/installations" && installations != "":
			_, _ = w.Write([]byte(installations))
		case r.URL.Path == "/user/installations/1/repositories" && selected != nil:
			_, _ = w.Write([]byte(*selected))
		case strings.HasPrefix(r.URL.Path, "/repos/"): // 改名の確かめ: 改名されていない
			http.NotFound(w, r)
		default:
			http.Error(w, "x", http.StatusForbidden)
		}
	}
}

func installationsOf(login, selection string) string {
	return `{"installations":[{"id":1,"account":{"login":"` + login + `"},"repository_selection":"` + selection + `"}]}`
}

func wantAppMissing(t *testing.T, name string, err error) {
	t.Helper()
	var f *ghcall.Failure
	if !ghcall.Is(err, ghcall.AppNotInstalled) || !asFailure(err, &f) || f.Org != "example-org" || f.Repo != "example-org/web" {
		t.Errorf("%s: err = %v, want AppNotInstalled for example-org/web", name, err)
	}
}

func TestLookupAppNotAccessible(t *testing.T) {
	c, _ := newClient(t, githubServer(strings.Replace(forbiddenJSON, "%s", "Resource not accessible by integration", 1), "", nil))
	_, err := c.Lookup(context.Background(), fakeToken, "example-org/web", 1)
	wantAppMissing(t, "Resource not accessible", err)
}

func TestLookupOAuthRestricted(t *testing.T) {
	msg := "`example-org` forbids access via a personal access token (classic). The organization has enabled OAuth App access restrictions"
	c, _ := newClient(t, githubServer(strings.Replace(forbiddenJSON, "%s", msg, 1), "", nil))
	_, err := c.Lookup(context.Background(), fakeToken, "example-org/web", 1)
	wantAppMissing(t, "OAuth App access restrictions", err)
}

func TestLookupPlainForbiddenIsUnreadable(t *testing.T) {
	c, _ := newClient(t, githubServer(strings.Replace(forbiddenJSON, "%s", "Forbidden", 1), "", nil))
	_, err := c.Lookup(context.Background(), fakeToken, "example-org/web", 1)
	if !ghcall.Is(err, ghcall.RepoUnreadable) {
		t.Errorf("err = %v", err)
	}
}

func TestLookupHiddenByMissingApp(t *testing.T) {
	c, _ := newClient(t, githubServer(notFoundJSON, installationsOf("other", "all"), nil))
	_, err := c.Lookup(context.Background(), fakeToken, "example-org/web", 1)
	wantAppMissing(t, "installation of another org", err)
}

func TestLookupInstallationMatchesOwnerCaseInsensitively(t *testing.T) {
	c, _ := newClient(t, githubServer(notFoundJSON, installationsOf("Example-Org", "all"), nil))
	_, err := c.Lookup(context.Background(), fakeToken, "example-org/web", 1)
	if !ghcall.Is(err, ghcall.RepoUnreadable) {
		t.Errorf("err = %v, want RepoUnreadable (App is installed on all repositories)", err)
	}
}

func TestLookupSelectedWithoutRepo(t *testing.T) {
	without := `{"repositories":[{"name":"api"}]}`
	c, _ := newClient(t, githubServer(notFoundJSON, installationsOf("example-org", "selected"), &without))
	_, err := c.Lookup(context.Background(), fakeToken, "example-org/web", 1)
	wantAppMissing(t, "selected without the repo", err)

	with := `{"repositories":[{"name":"api"},{"name":"Web"}]}`
	c, _ = newClient(t, githubServer(notFoundJSON, installationsOf("example-org", "selected"), &with))
	_, err = c.Lookup(context.Background(), fakeToken, "example-org/web", 1)
	if !ghcall.Is(err, ghcall.RepoUnreadable) {
		t.Errorf("selected with the repo: err = %v, want RepoUnreadable", err)
	}
}

func TestLookupNotAppToken(t *testing.T) {
	c, _ := newClient(t, githubServer(notFoundJSON, "", nil))
	_, err := c.Lookup(context.Background(), fakeToken, "example-org/web", 1)
	if !ghcall.Is(err, ghcall.RepoUnreadable) {
		t.Errorf("err = %v, want RepoUnreadable when /user/installations is refused", err)
	}
}

func TestInstallationsArePaged(t *testing.T) {
	var asked []string
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/graphql":
			_, _ = w.Write([]byte(notFoundJSON))
		case "/user/installations":
			asked = append(asked, r.URL.RawQuery)
			if r.URL.Query().Get("page") == "1" {
				_, _ = w.Write([]byte(`{"installations":[` + strings.TrimSuffix(strings.Repeat(`{"id":9,"account":{"login":"x"},"repository_selection":"all"},`, 100), ",") + `]}`))
				return
			}
			_, _ = w.Write([]byte(installationsOf("example-org", "all")))
		default:
			http.NotFound(w, r)
		}
	})
	_, err := c.Lookup(context.Background(), fakeToken, "example-org/web", 1)
	if !ghcall.Is(err, ghcall.RepoUnreadable) || len(asked) != 2 || asked[1] != "per_page=100&page=2" {
		t.Errorf("err = %v, asked %v: the org on page 2 should be found (App installed)", err, asked)
	}
}

func TestReadManyAppNotInstalled(t *testing.T) {
	cases := []struct {
		name, graphql, installations string
	}{
		{"not accessible", strings.Replace(forbiddenJSON, "%s", "Resource not accessible by integration", 1), ""},
		{"oauth restricted", strings.Replace(forbiddenJSON, "%s", "OAuth App access restrictions", 1), ""},
		{"hidden", notFoundJSON, installationsOf("other", "all")},
	}
	for _, cs := range cases {
		c, _ := newClient(t, githubServer(cs.graphql, cs.installations, nil))
		_, err := c.ReadMany(context.Background(), fakeToken, "example-org/web", []int{1, 2})
		wantAppMissing(t, cs.name, err)
	}
}

func TestAppNotInstalledUsesTheRenamedRepo(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/graphql":
			_, _ = w.Write([]byte(notFoundJSON))
		case "/repos/old/web":
			_, _ = w.Write([]byte(`{"full_name":"example-org/web"}`))
		case "/user/installations":
			_, _ = w.Write([]byte(installationsOf("other", "all")))
		}
	})
	_, err := c.Lookup(context.Background(), fakeToken, "old/web", 1)
	wantAppMissing(t, "renamed", err)
}

func TestCheckRepo(t *testing.T) {
	c, _ := newClient(t, githubServer(okCheck, "", nil))
	if err := c.CheckRepo(context.Background(), fakeToken, "example-org/web"); err != nil {
		t.Errorf("readable: err = %v", err)
	}

	c, _ = newClient(t, githubServer(notFoundJSON, installationsOf("example-org", "all"), nil))
	if err := c.CheckRepo(context.Background(), fakeToken, "example-org/web"); !ghcall.Is(err, ghcall.RepoUnreadable) {
		t.Errorf("hidden with installation: err = %v, want RepoUnreadable", err)
	}

	c, _ = newClient(t, githubServer(notFoundJSON, installationsOf("other", "all"), nil))
	wantAppMissing(t, "check", c.CheckRepo(context.Background(), fakeToken, "example-org/web"))

	c, _ = newClient(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "x", http.StatusUnauthorized) })
	if err := c.CheckRepo(context.Background(), fakeToken, "example-org/web"); !ghcall.Is(err, ghcall.TokenRejected) {
		t.Errorf("401: err = %v, want TokenRejected", err)
	}
}

func TestCheckRepoRateLimited(t *testing.T) {
	c, _ := newClient(t, githubServer(`{"errors":[{"type":"RATE_LIMITED","message":"x"}]}`, "", nil))
	if err := c.CheckRepo(context.Background(), fakeToken, "example-org/web"); !ghcall.Is(err, ghcall.RateLimited) {
		t.Errorf("err = %v, want RateLimited", err)
	}
}
