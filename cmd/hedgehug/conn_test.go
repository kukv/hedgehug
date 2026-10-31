package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/gitrepo/gitrepotest"
	"github.com/kukv/hedgehug/internal/ops"
)

func TestSyncThroughRewrite(t *testing.T) {
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
	var auths []transport.AuthMethod
	// Not a GitHub host: store init (Task 9) makes no repository there.
	gitrepotest.InstallHTTPS(t, gitrepotest.Under("https://git.example.com/", d), &auths, nil)
	te.env["HH_TOKEN"] = "tok-1"
	te.writeConfigYAML("accounts:\n  - {name: me, host: git.example.com, login: alice, token: \"env:HH_TOKEN\"}\n")
	te.ok("store", "init", "me", "--prefix", "ME", "--remote", "https://git.example.com/org-a/x.git")
	if len(auths) == 0 {
		t.Fatal("no git session")
	}
	checkAuths := func(from int) {
		t.Helper()
		for _, a := range auths[from:] {
			if b, ok := a.(*githttp.BasicAuth); !ok || b.Password != "tok-1" || b.Username != "x-access-token" {
				t.Errorf("auth = %#v", a)
			}
		}
	}
	checkAuths(0)
	// A later sync fetches and pushes, and each session carries the token.
	te.ok("add", "one")
	n := len(auths)
	te.ok("sync")
	if len(auths) < n+2 {
		t.Fatalf("sync made %d sessions, want a fetch and a push", len(auths)-n)
	}
	checkAuths(n)
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

func TestConnFor(t *testing.T) {
	te := newTestEnv(t)
	te.env["HH_TOKEN"] = "tok-1"
	te.writeConfigYAML("accounts:\n" +
		"  - {name: a, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n" +
		"  - {name: b, host: github.com, login: bob}\n" +
		"url_rewrites:\n  - {from: \"https://github.com/\", to: \"/srv/remotes/\"}\n")
	newApp := func() *app {
		e := &env{getenv: te.getenv, now: func() time.Time { return te.now }}
		return &app{env: e, ss: &ops.Session{Getenv: e.getenv, Now: e.now}}
	}
	a := newApp()
	c, _, err := a.ss.Config()
	if err != nil {
		t.Fatal(err)
	}
	conn := a.connFor(c, config.StoreEntry{Account: "a"})
	if got := conn.Rewrite("https://github.com/org/x.git"); got != "/srv/remotes/org/x.git" {
		t.Errorf("Rewrite = %q", got)
	}
	for _, u := range []string{"git@github.com:org/x.git", "ssh://git@github.com/org/x.git", "/srv/remotes/org/x.git", "file:///srv/x.git"} {
		if au, err := conn.Auth(u); au != nil || err != nil {
			t.Errorf("Auth(%q) = %v, %v; want none", u, au, err)
		}
	}
	au, err := conn.Auth("https://github.com/org/x.git")
	if b, ok := au.(*githttp.BasicAuth); err != nil || !ok || b.Username != "x-access-token" || b.Password != "tok-1" {
		t.Errorf("Auth(https) = %#v, %v", au, err)
	}
	// The token is got once a run: it stays when the variable goes.
	te.env["HH_TOKEN"] = ""
	au, err = conn.Auth("http://github.com/org/x.git")
	if b, ok := au.(*githttp.BasicAuth); err != nil || !ok || b.Password != "tok-1" {
		t.Errorf("second Auth = %#v, %v", au, err)
	}
	// Only the account's own host gets the token, whatever the case or port.
	for u, want := range map[string]bool{
		"https://github.com/org/x.git":      true,
		"https://GitHub.com:8443/org/x.git": true,
		"http://github.com/org/x.git":       true,
		"https://mirror.example.com/x.git":  false,
		"https://github.com.evil.test/x":    false,
		"https://api.github.com/x":          false,
	} {
		au, err := conn.Auth(u)
		if err != nil || (au != nil) != want {
			t.Errorf("Auth(%q) = %v, %v; want token %v", u, au, err, want)
		}
	}
	// An account with no token, and a name that is no account.
	for _, name := range []string{"b", "nobody"} {
		if au, err := a.connFor(c, config.StoreEntry{Account: name}).Auth("https://github.com/org/x.git"); au != nil || err != nil {
			t.Errorf("account %q: Auth = %v, %v; want none", name, au, err)
		}
	}
	// A token that cannot be got is an error, and is asked for again.
	a2 := newApp()
	for range 2 {
		if _, err := a2.connFor(c, config.StoreEntry{Account: "a"}).Auth("https://github.com/org/x.git"); err == nil || !strings.Contains(err.Error(), "HH_TOKEN is not set") {
			t.Errorf("a missing token = %v", err)
		}
	}
}
