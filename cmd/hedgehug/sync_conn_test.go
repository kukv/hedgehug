package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/gitrepo/gitrepotest"
)

func TestSyncThroughRewrite(t *testing.T) {
	t.Parallel()
	te := newTestEnv(t)
	remotes := filepath.Join(t.TempDir(), "remotes")
	if err := gitrepo.InitBare(filepath.Join(remotes, "org-a", "x.git")); err != nil {
		t.Fatal(err)
	}
	te.writeConfigYAML("url_rewrites:\n  - {from: \"https://github.com/\", to: \"" + filepath.ToSlash(remotes) + "/\"}\n")
	te.ok("store", "init", "me", "--prefix", "ME", "--remote", "https://github.com/org-a/x.git")
	te.ok("add", "one")
	te.ok("sync")
	if ok, err := gitrepo.HeadHasFile(filepath.Join(remotes, "org-a", "x.git"), ".hedgehug/store.yaml"); err != nil || !ok {
		t.Errorf("the remote has no store: %v %v", ok, err)
	}
	b, _ := os.ReadFile(filepath.Join(te.storeDir("me"), ".git", "config"))
	if !strings.Contains(string(b), "https://github.com/org-a/x.git") {
		t.Errorf(".git/config = %s", b)
	}
}

func TestSyncSendsToken(t *testing.T) {
	te := newTestEnv(t)
	d := t.TempDir()
	if err := gitrepo.InitBare(filepath.Join(d, "org-a", "x.git")); err != nil {
		t.Fatal(err)
	}
	var sessions []gitrepotest.Session
	// Not a GitHub host: store init (Task 9) makes no repository there.
	gitrepotest.InstallHTTPSSessions(t, gitrepotest.Under("https://git.example.com/", d), &sessions, nil)
	te.env["HH_TOKEN"] = "tok-1"
	te.writeConfigYAML("accounts:\n  - {name: me, host: git.example.com, login: alice, token: \"env:HH_TOKEN\"}\n")
	te.ok("store", "init", "me", "--prefix", "ME", "--remote", "https://git.example.com/org-a/x.git")
	if len(sessions) == 0 {
		t.Fatal("no git session")
	}
	// kinds checks that the sessions from the index on all carry the token,
	// and returns whether a fetch and a push were seen.
	kinds := func(from int) (fetch, push bool) {
		t.Helper()
		for _, s := range sessions[from:] {
			if b, ok := s.Auth.(*githttp.BasicAuth); !ok || b.Password != "tok-1" || b.Username != "x-access-token" {
				t.Errorf("%s auth = %#v", s.Kind, s.Auth)
			}
			fetch = fetch || s.Kind == gitrepotest.UploadPack
			push = push || s.Kind == gitrepotest.ReceivePack
		}
		return fetch, push
	}
	if _, push := kinds(0); !push {
		t.Error("store init did not push")
	}
	// A later sync fetches and pushes; both carry the token.
	te.ok("add", "one")
	n := len(sessions)
	te.ok("sync")
	if fetch, push := kinds(n); !fetch || !push {
		t.Errorf("sync: fetch with the token = %v, push with the token = %v", fetch, push)
	}
}

// The host that decides is the one reached after the rewrite.
func TestTokenGoesToTheRewrittenHost(t *testing.T) {
	for _, c := range []struct {
		name, remote, to string
		token            bool
	}{
		{"rewritten to the account's host", "https://github.com/org-a/x.git", "https://git.example.com/", true},
		{"rewritten to another host", "https://git.example.com/org-a/x.git", "https://other.example.com/", false},
		{"not rewritten, another host", "https://other.example.com/org-a/x.git", "", false},
		{"not rewritten, the account's host", "https://GIT.example.com/org-a/x.git", "", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			te := newTestEnv(t)
			d := t.TempDir()
			if err := gitrepo.InitBare(filepath.Join(d, "org-a", "x.git")); err != nil {
				t.Fatal(err)
			}
			var auths []transport.AuthMethod
			route := func(u string) (string, bool) {
				for _, h := range []string{"git.example.com", "other.example.com"} {
					if dir, ok := gitrepotest.Under("https://"+h+"/", d)(strings.ToLower(u)); ok {
						return dir, true
					}
				}
				return "", false
			}
			gitrepotest.InstallHTTPS(t, route, &auths, nil)
			te.env["HH_TOKEN"] = "tok-1"
			cfg := "accounts:\n  - {name: me, host: git.example.com, login: alice, token: \"env:HH_TOKEN\"}\n"
			if c.to != "" {
				cfg += "url_rewrites:\n  - {from: \"" + strings.TrimSuffix(c.remote, "org-a/x.git") + "\", to: \"" + c.to + "\"}\n"
			}
			te.writeConfigYAML(cfg)
			te.ok("store", "init", "me", "--prefix", "ME", "--remote", c.remote)
			if len(auths) == 0 {
				t.Fatal("no git session")
			}
			for _, a := range auths {
				if b, ok := a.(*githttp.BasicAuth); ok != c.token || ok && b.Password != "tok-1" {
					t.Errorf("auth = %#v, want token %v", a, c.token)
				}
			}
		})
	}
}

// A token that cannot be got stops the sync with the reason, not with a
// failed connection.
func TestSyncWithoutTheTokenSaysWhy(t *testing.T) {
	te := newTestEnv(t)
	d := t.TempDir()
	if err := gitrepo.InitBare(filepath.Join(d, "org-a", "x.git")); err != nil {
		t.Fatal(err)
	}
	gitrepotest.InstallHTTPS(t, gitrepotest.Under("https://git.example.com/", d), nil, nil)
	te.writeConfigYAML("accounts:\n  - {name: me, host: git.example.com, login: alice, token: \"env:HH_TOKEN\"}\n")
	code, _, errs := te.run("store", "init", "me", "--prefix", "ME", "--remote", "https://git.example.com/org-a/x.git")
	if code == 0 || !strings.Contains(errs, "HH_TOKEN is not set") {
		t.Errorf("exit %d\n%s", code, errs)
	}
}
