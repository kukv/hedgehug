package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/fakegh"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/gitrepo/gitrepotest"
)

func rewriteTo(remotes string) string {
	return "url_rewrites:\n  - {from: \"https://github.com/\", to: \"" + filepath.ToSlash(remotes) + "/\"}\n"
}

// storesUnder makes stores named after the repo of each URL (the owner and
// the repo of https://github.com/ that url_rewrites turns into remotes).
func storesUnder(t *testing.T, remotes string, prefixes map[string]string) {
	t.Helper()
	maker := newTestEnv(t)
	maker.writeConfigYAML(rewriteTo(remotes))
	for u, prefix := range prefixes {
		maker.ok("store", "init", strings.TrimSuffix(filepath.Base(u), ".git"), "--prefix", prefix, "--remote", u)
	}
}

func TestDiscoverFrom(t *testing.T) {
	t.Parallel()
	remotes := filepath.Join(t.TempDir(), "remotes")
	storesUnder(t, remotes, map[string]string{
		"https://github.com/org-a/hh-shared.git": "SH",
		"https://github.com/org-b/hh-ops.git":    "OPS",
	})
	// A bare that is not a store is not offered.
	if err := gitrepo.InitBare(filepath.Join(remotes, "org-a", "web.git")); err != nil {
		t.Fatal(err)
	}
	te := newTestEnv(t)
	te.writeConfigYAML(rewriteTo(remotes))
	out := te.ok("store", "discover", "--from", remotes)
	for _, want := range []string{"hh-shared", "https://github.com/org-a/hh-shared.git", "hh-ops", "write", "hedgehug store add"} {
		if !strings.Contains(out, want) {
			t.Errorf("list lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "web") {
		t.Errorf("a bare that is not a store is offered:\n%s", out)
	}
	if out := te.ok("store", "list"); strings.Contains(out, "hh-shared") {
		t.Errorf("not a terminal: nothing is mounted:\n%s", out)
	}
	if out := te.ok("store", "discover", "--from", remotes, "--all"); !strings.Contains(out, "Added store hh-shared") || !strings.Contains(out, "Added store hh-ops") {
		t.Errorf("--all printed:\n%s", out)
	}
	if out := te.ok("store", "list"); !strings.Contains(out, "hh-shared") || !strings.Contains(out, "hh-ops") {
		t.Errorf("--all did not mount them:\n%s", out)
	}
	out = te.ok("store", "discover", "--from", remotes, "--json")
	var got struct {
		Candidates []map[string]string `json:"candidates"`
		Mounted    []string            `json:"mounted"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil || len(got.Candidates) != 0 || got.Mounted == nil {
		t.Errorf("mounted ones are offered again, or the arrays are null: %s", out)
	}
	if out := te.ok("store", "discover", "--from", remotes); !strings.Contains(out, "No store found") {
		t.Errorf("nothing left:\n%s", out)
	}
}

func TestDiscoverGitHub(t *testing.T) {
	te := newTestEnv(t)
	d := t.TempDir()
	te.withFake(fakegh.State{
		Tokens: map[string]string{"t-alice": "alice", "t-corp": "alice-corp"},
		Repos: []*fakegh.Repo{
			{Owner: "org-a", Name: "hh-shared", Topics: []string{"hedgehug-store"}, Perms: map[string]string{"alice-corp": "write", "alice": "read"}},
			{Owner: "org-b", Name: "hh-shared", Topics: []string{"hedgehug-store"}, Perms: map[string]string{"alice-corp": "read"}},
			{Owner: "org-a", Name: "web", Perms: map[string]string{"alice-corp": "write"}},
		},
	})
	te.env["T_ALICE"], te.env["T_CORP"] = "t-alice", "t-corp"
	te.writeConfigYAML("accounts:\n  - {name: personal, host: github.com, login: alice, token: \"env:T_ALICE\"}\n  - {name: work, host: github.com, login: alice-corp, token: \"env:T_CORP\"}\n")
	out := te.ok("store", "discover", "--json")
	var got struct {
		Candidates []struct{ Name, URL, Account, Permission string } `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Candidates) != 2 {
		t.Fatalf("candidates = %+v", got.Candidates)
	}
	a, b := got.Candidates[0], got.Candidates[1]
	if a.Name != "hh-shared" || a.Account != "work" || a.Permission != "write" || a.URL != "https://github.com/org-a/hh-shared.git" {
		t.Errorf("first = %+v", a)
	}
	if b.Name != "org-b-hh-shared" || b.Permission != "read" {
		t.Errorf("second = %+v", b)
	}
	// Picking one on a terminal adds it through store add, with its account.
	maker := newTestEnv(t)
	maker.ok("store", "init", "hh-shared", "--prefix", "SH", "--remote", filepath.Join(d, "org-a", "hh-shared.git"))
	gitrepotest.InstallHTTPS(t, gitrepotest.Under("https://github.com/", d), nil, nil)
	te.terminal, te.stdin = true, "1\n"
	te.ok("store", "discover")
	cfg, _ := os.ReadFile(filepath.Join(te.hhHome, "config.yaml"))
	if !strings.Contains(string(cfg), "name: hh-shared") || !strings.Contains(string(cfg), "account: work") {
		t.Errorf("config.yaml = %s", cfg)
	}
}

func TestDiscoverTerminal(t *testing.T) {
	t.Parallel()
	remotes := filepath.Join(t.TempDir(), "remotes")
	storesUnder(t, remotes, map[string]string{"https://github.com/o/aa.git": "AA", "https://github.com/o/bb.git": "BB"})
	te := newTestEnv(t)
	te.writeConfigYAML(rewriteTo(remotes))
	te.terminal, te.stdin = true, "2\n"
	te.ok("store", "discover", "--from", remotes)
	out := te.ok("store", "list")
	if strings.Contains(out, "aa") || !strings.Contains(out, "bb") {
		t.Errorf("picked 2, got:\n%s", out)
	}
	// A number outside the list is a usage error; an empty line stops.
	te.stdin = "9\n"
	if code, _, errs := te.run("store", "discover", "--from", remotes); code != 2 || !strings.Contains(errs, "not a number from the list") {
		t.Errorf("bad number: exit %d, %q", code, errs)
	}
	te.stdin = "\n"
	te.ok("store", "discover", "--from", remotes)
	if out := te.ok("store", "list"); strings.Contains(out, "aa") {
		t.Errorf("empty line mounted something:\n%s", out)
	}
}

// With --json on a terminal the question is still asked, and the list goes to
// stderr so that stdout is the JSON alone.
func TestDiscoverTerminalJSON(t *testing.T) {
	t.Parallel()
	remotes := filepath.Join(t.TempDir(), "remotes")
	storesUnder(t, remotes, map[string]string{"https://github.com/o/aa.git": "AA"})
	te := newTestEnv(t)
	te.writeConfigYAML(rewriteTo(remotes))
	te.terminal, te.stdin = true, "1\n"
	code, out, errs := te.run("store", "discover", "--from", remotes, "--json")
	if code != 0 || !strings.Contains(errs, "NAME") {
		t.Fatalf("exit %d, stderr %q", code, errs)
	}
	var got struct {
		Mounted []string `json:"mounted"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil || len(got.Mounted) != 1 || got.Mounted[0] != "aa" {
		t.Errorf("stdout = %q, %v", out, err)
	}
}

// --from with two accounts and none named: off a terminal the choice is not
// asked (exit 2, the accounts listed); on one it is asked by number.
func TestDiscoverFromAccountChoice(t *testing.T) {
	t.Parallel()
	remotes := filepath.Join(t.TempDir(), "remotes")
	storesUnder(t, remotes, map[string]string{"https://github.com/o/aa.git": "AA"})
	te := newTestEnv(t)
	te.writeConfigYAML(rewriteTo(remotes) + twoAccounts)
	code, _, errs := te.run("store", "discover", "--from", remotes)
	if code != 2 || !strings.Contains(errs, "--account") || !strings.Contains(errs, "a, b") {
		t.Fatalf("off a terminal: exit %d, %q", code, errs)
	}
	te.terminal, te.stdin = true, "2\n"
	code, _, errs = te.run("store", "discover", "--from", remotes, "--all")
	if code != 0 || !strings.Contains(errs, "1) a") || !strings.Contains(errs, "2) b") {
		t.Fatalf("on a terminal: exit %d, %q", code, errs)
	}
	if b := te.configText(); !strings.Contains(b, "name: aa") || !strings.Contains(b, "account: b") {
		t.Errorf("config.yaml = %s", b)
	}
	// --account decides it without asking.
	te.terminal, te.stdin = false, ""
	if code, out, errs := te.run("store", "discover", "--from", remotes, "--account", "nope"); code != 1 || out != "" || !strings.Contains(errs, "nope") {
		t.Errorf("unknown account: exit %d, %q, %q", code, out, errs)
	}
}

// Of two that are picked, the one whose clone fails is reported, the other is
// still added, and the exit code is 1.
func TestDiscoverOneFails(t *testing.T) {
	t.Parallel()
	remotes := filepath.Join(t.TempDir(), "remotes")
	storesUnder(t, remotes, map[string]string{"https://github.com/o/aa.git": "AA", "https://github.com/o/bb.git": "BB"})
	te := newTestEnv(t)
	te.writeConfigYAML(rewriteTo(remotes))
	// A file where the clone of aa goes.
	if err := os.MkdirAll(filepath.Dir(te.storeDir("aa")), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(te.storeDir("aa"), []byte("in the way"), 0o600); err != nil {
		t.Fatal(err)
	}
	te.terminal, te.stdin = true, "all\n"
	code, out, errs := te.run("store", "discover", "--from", remotes)
	if code != 1 || !strings.Contains(out, "Added store bb") || !strings.Contains(errs, "Could not add store aa") || !strings.Contains(errs, "1 of 2 stores could not be added") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errs)
	}
	if list := te.ok("store", "list"); strings.Contains(list, "aa") || !strings.Contains(list, "bb") {
		t.Errorf("store list:\n%s", list)
	}
	// --json: the failure is in the output too.
	te.terminal, te.stdin = false, ""
	code, out, _ = te.run("store", "discover", "--from", remotes, "--all", "--json")
	var got struct {
		Failed []struct{ Name, Error string } `json:"failed"`
	}
	if code != 1 || json.Unmarshal([]byte(out), &got) != nil || len(got.Failed) != 1 || got.Failed[0].Name != "aa" || got.Failed[0].Error == "" {
		t.Errorf("--json: exit %d, %q", code, out)
	}
}

// The account question and the number question are answered from one input
// of two lines: the second answer is not lost to the first prompt's reader.
func TestDiscoverTwoPromptsFromOneInput(t *testing.T) {
	t.Parallel()
	remotes := filepath.Join(t.TempDir(), "remotes")
	storesUnder(t, remotes, map[string]string{"https://github.com/o/aa.git": "AA", "https://github.com/o/bb.git": "BB"})
	te := newTestEnv(t)
	te.writeConfigYAML(rewriteTo(remotes) + twoAccounts)
	te.terminal, te.stdin = true, "2\n2\n"
	te.ok("store", "discover", "--from", remotes)
	b := te.configText()
	if !strings.Contains(b, "name: bb") || strings.Contains(b, "name: aa") || !strings.Contains(b, "account: b") {
		t.Errorf("config.yaml = %s", b)
	}
}

// A number given twice is added once.
func TestDiscoverDuplicateNumbers(t *testing.T) {
	t.Parallel()
	remotes := filepath.Join(t.TempDir(), "remotes")
	storesUnder(t, remotes, map[string]string{"https://github.com/o/aa.git": "AA", "https://github.com/o/bb.git": "BB"})
	te := newTestEnv(t)
	te.writeConfigYAML(rewriteTo(remotes))
	te.terminal, te.stdin = true, "2,1,2\n"
	out := te.ok("store", "discover", "--from", remotes)
	if strings.Count(out, "Added store bb") != 1 || strings.Count(out, "Added store aa") != 1 || strings.Index(out, "Added store bb") > strings.Index(out, "Added store aa") {
		t.Errorf("out:\n%s", out)
	}
}
