package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/fakegh"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/gitrepo/gitrepotest"
)

const twoAccounts = "accounts:\n" +
	"  - {name: a, host: github.com, login: alice, token: \"env:HH_TOKEN_A\"}\n" +
	"  - {name: b, host: github.com, login: bob, token: \"env:HH_TOKEN_B\"}\n"

func TestStoreInitPathAndAccountFlags(t *testing.T) {
	te := newTestEnv(t)
	te.writeAccounts("github.com", "alice")
	dir := filepath.Join(t.TempDir(), "mine")
	te.ok("store", "init", "me", "--prefix", "ME", "--account", "alice", "--path", dir)
	b, _ := os.ReadFile(filepath.Join(te.hhHome, "config.yaml"))
	if !strings.Contains(string(b), "account: alice") || !strings.Contains(string(b), filepath.ToSlash(dir)) {
		t.Errorf("config.yaml = %s", b)
	}
	if code, _, errs := te.run("store", "init", "x", "--prefix", "XX", "--account", "nope"); code != 1 || !strings.Contains(errs, "nope") {
		t.Errorf("unknown account: exit %d, %q", code, errs)
	}
}

// Off a terminal the choice of the account is not asked: exit 2 with the
// candidates.
func TestStoreInitAccountChoiceOffTerminal(t *testing.T) {
	te := newTestEnv(t)
	te.withFake(fakegh.State{})
	te.writeConfigYAML(twoAccounts)
	code, _, errs := te.run("store", "init", "org", "--prefix", "ORG", "--remote", "https://github.com/bob/x.git")
	if code != 2 || !strings.Contains(errs, "--account") || !strings.Contains(errs, "a, b") {
		t.Errorf("exit %d, %q", code, errs)
	}
}

// On a terminal the account is asked by number, and the init goes on with it.
func TestStoreInitAccountChoiceOnTerminal(t *testing.T) {
	te := newTestEnv(t)
	d := t.TempDir()
	fake := te.withFake(fakegh.State{Tokens: map[string]string{"t-alice": "alice", "t-bob": "bob"}})
	te.env["HH_TOKEN_A"], te.env["HH_TOKEN_B"] = "t-alice", "t-bob"
	te.writeConfigYAML(twoAccounts)
	if err := gitrepo.InitBare(filepath.Join(d, "bob", "hh.git")); err != nil {
		t.Fatal(err)
	}
	gitrepotest.InstallHTTPS(t, gitrepotest.Under("https://github.com/", d), nil, nil)
	te.terminal, te.stdin = true, "2\n"
	code, _, errs := te.run("store", "init", "org", "--prefix", "ORG", "--remote", "https://github.com/bob/hh.git")
	if code != 0 || !strings.Contains(errs, "1) a") || !strings.Contains(errs, "2) b") {
		t.Fatalf("exit %d, %q", code, errs)
	}
	if _, ok := fake.Repo("bob/hh"); !ok {
		t.Error("the repository was not made as bob")
	}
	if b := te.configText(); !strings.Contains(b, "account: b") {
		t.Errorf("config.yaml = %s", b)
	}
	// A number outside the list is a usage error.
	te.stdin = "9\n"
	if code, _, errs := te.run("store", "init", "org2", "--prefix", "OR2", "--remote", "https://github.com/bob/hh2.git"); code != 2 || !strings.Contains(errs, "not one of") {
		t.Errorf("bad number: exit %d, %q", code, errs)
	}
}

func TestStoreInitGitHubForbiddenExit(t *testing.T) {
	te := newTestEnv(t)
	te.withFake(fakegh.State{Orgs: map[string][]string{"org-a": {"bob"}}, Tokens: map[string]string{"t-alice": "alice"}})
	te.env["HH_TOKEN"] = "t-alice"
	te.writeConfigYAML("accounts:\n  - {name: work, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n")
	code, _, errs := te.run("store", "init", "org", "--prefix", "ORG", "--remote", "https://github.com/org-a/x.git")
	if code != 1 || !strings.Contains(errs, "github.com/alice cannot create org-a/x") || !strings.Contains(errs, "make an empty repository on GitHub") {
		t.Errorf("exit %d, %q", code, errs)
	}
}
