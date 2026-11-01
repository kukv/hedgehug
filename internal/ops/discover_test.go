package ops

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/fakegh"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/gitrepo/gitrepotest"
)

const bothAccounts = "accounts:\n" +
	"  - {name: personal, host: github.com, login: alice, token: \"env:T_ALICE\"}\n" +
	"  - {name: work, host: github.com, login: alice-corp, token: \"env:T_CORP\"}\n"

func rewriteTo(remotes string) string {
	return "url_rewrites:\n  - {from: \"https://github.com/\", to: \"" + filepath.ToSlash(remotes) + "/\"}\n"
}

// storesUnder makes the stores at the URLs (under https://github.com/ that
// url_rewrites turns into remotes), in a session of their own.
func storesUnder(t *testing.T, remotes string, urls ...string) {
	t.Helper()
	maker := newTestSession(t)
	maker.appendConfig(rewriteTo(remotes))
	for i, u := range urls {
		name := strings.TrimSuffix(filepath.Base(u), ".git")
		if _, err := maker.StoreInit(StoreInitInput{Name: name, Prefix: "S" + strings.Repeat("X", i+1), Remotes: []string{u}}); err != nil {
			t.Fatal(err)
		}
	}
}

func candidateNames(cands []Candidate) []string {
	out := make([]string, len(cands))
	for i, x := range cands {
		out[i] = x.Name
	}
	return out
}

func TestDiscoverFrom(t *testing.T) {
	t.Parallel()
	remotes := filepath.Join(t.TempDir(), "remotes")
	storesUnder(t, remotes, "https://github.com/org-a/hh-shared.git", "https://github.com/org-b/hh-ops.git")
	// A bare that is not a store is not offered.
	if err := gitrepo.InitBare(filepath.Join(remotes, "org-a", "web.git")); err != nil {
		t.Fatal(err)
	}
	ts := newTestSession(t)
	ts.appendConfig(rewriteTo(remotes))
	cands, err := ts.StoreDiscover(DiscoverInput{From: remotes})
	if err != nil {
		t.Fatal(err)
	}
	want := []Candidate{
		{Name: "hh-shared", URL: "https://github.com/org-a/hh-shared.git", Permission: "write"},
		{Name: "hh-ops", URL: "https://github.com/org-b/hh-ops.git", Permission: "write"},
	}
	// The walk is in the order of the names: org-a before org-b.
	if len(cands) != 2 {
		t.Fatalf("candidates = %+v", cands)
	}
	for i := range want {
		got := cands[i]
		got.key = ""
		if got != want[i] {
			t.Errorf("candidate %d = %+v, want %+v", i, got, want[i])
		}
	}
	res, err := ts.StoreMount(cands)
	if err != nil || !reflect.DeepEqual(res.Mounted, []string{"hh-shared", "hh-ops"}) {
		t.Fatalf("mount = %+v, %v", res, err)
	}
	// The mounted ones are not offered again.
	again, err := ts.StoreDiscover(DiscoverInput{From: remotes})
	if err != nil || len(again) != 0 {
		t.Errorf("again = %+v, %v", again, err)
	}
}

func TestDiscoverFromAccounts(t *testing.T) {
	t.Parallel()
	remotes := filepath.Join(t.TempDir(), "remotes")
	storesUnder(t, remotes, "https://github.com/org-a/hh-shared.git")

	// No account: the candidate has none.
	ts := newTestSession(t)
	ts.appendConfig(rewriteTo(remotes))
	cands, err := ts.StoreDiscover(DiscoverInput{From: remotes})
	if err != nil || len(cands) != 1 || cands[0].Account != "" {
		t.Fatalf("no account: %+v, %v", cands, err)
	}

	// One account: it is the one, without asking.
	one := newTestSession(t)
	one.appendConfig(rewriteTo(remotes) + "accounts:\n  - {name: personal, host: github.com, login: alice}\n")
	cands, err = one.StoreDiscover(DiscoverInput{From: remotes})
	if err != nil || len(cands) != 1 || cands[0].Account != "personal" {
		t.Fatalf("one account: %+v, %v", cands, err)
	}

	// Two accounts and none named: a choice, with the names.
	two := newTestSession(t)
	two.appendConfig(rewriteTo(remotes) + bothAccounts)
	_, err = two.StoreDiscover(DiscoverInput{From: remotes})
	wantErr(t, err, KindUsage, "choice.required")
	e := AsError(err)
	if e.Args["what"] != "--account" || !reflect.DeepEqual(e.Args["candidates"], []string{"personal", "work"}) {
		t.Errorf("args = %+v", e.Args)
	}
	// Named: that one.
	cands, err = two.StoreDiscover(DiscoverInput{From: remotes, Account: "work"})
	if err != nil || len(cands) != 1 || cands[0].Account != "work" {
		t.Fatalf("named: %+v, %v", cands, err)
	}
	_, err = two.StoreDiscover(DiscoverInput{From: remotes, Account: "nope"})
	wantErr(t, err, KindNotFound, "account.not_found")
}

func githubDiscoverSession(t *testing.T) (*testSession, string) {
	t.Helper()
	ts, _ := fakeSession(t, fakegh.State{
		Tokens: map[string]string{"t-alice": "alice", "t-corp": "alice-corp"},
		Repos: []*fakegh.Repo{
			{Owner: "org-a", Name: "hh-shared", Topics: []string{"hedgehug-store"}, Perms: map[string]string{"alice-corp": "write", "alice": "read"}},
			{Owner: "org-b", Name: "hh-shared", Topics: []string{"hedgehug-store"}, Perms: map[string]string{"alice-corp": "read"}},
			{Owner: "org-a", Name: "web", Perms: map[string]string{"alice-corp": "write"}},
		},
	}, bothAccounts)
	ts.env["T_ALICE"], ts.env["T_CORP"] = "t-alice", "t-corp"
	return ts, t.TempDir()
}

func TestDiscoverGitHub(t *testing.T) {
	ts, d := githubDiscoverSession(t)
	cands, err := ts.StoreDiscover(DiscoverInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 2 {
		t.Fatalf("candidates = %+v", cands)
	}
	a, b := cands[0], cands[1]
	// org-a/hh-shared is seen by both accounts; work can push, so work takes it.
	if a.Name != "hh-shared" || a.Account != "work" || a.Permission != "write" || a.URL != "https://github.com/org-a/hh-shared.git" {
		t.Errorf("first = %+v", a)
	}
	// The same repo name in another org gets the owner in front.
	if b.Name != "org-b-hh-shared" || b.Permission != "read" || b.Account != "work" {
		t.Errorf("second = %+v", b)
	}
	// Named: only that account looks. alice sees org-a/hh-shared only (read).
	only, err := ts.StoreDiscover(DiscoverInput{Account: "personal"})
	if err != nil || len(only) != 1 || only[0].Account != "personal" || only[0].Permission != "read" {
		t.Errorf("personal = %+v, %v", only, err)
	}

	// Mounting one goes through StoreAdd with its account.
	storesUnder(t, d, "https://github.com/org-a/hh-shared.git")
	gitrepotest.InstallHTTPS(t, gitrepotest.Under("https://github.com/", d), nil, nil)
	res, err := ts.StoreMount([]Candidate{a})
	if err != nil || !reflect.DeepEqual(res.Mounted, []string{"hh-shared"}) {
		t.Fatalf("mount = %+v, %v", res, err)
	}
	if got := namedStores(t, ts, "hh-shared"); len(got) != 1 || got[0].Account != "work" {
		t.Errorf("stores = %+v", got)
	}
	// A mounted repo is not offered again.
	again, err := ts.StoreDiscover(DiscoverInput{})
	if err != nil || !reflect.DeepEqual(candidateNames(again), []string{"org-b-hh-shared"}) {
		t.Errorf("again = %+v, %v", again, err)
	}
}

func TestDiscoverGitHubNoToken(t *testing.T) {
	t.Parallel()
	ts, _ := fakeSession(t, fakegh.State{}, "accounts:\n  - {name: bare, host: github.com, login: alice}\n")
	_, err := ts.StoreDiscover(DiscoverInput{})
	wantErr(t, err, KindConflict, "account.none")
	if !strings.Contains(err.Error(), "hedgehug auth login") {
		t.Errorf("err = %v", err)
	}
	// Named, an account with no token is told so.
	_, err = ts.StoreDiscover(DiscoverInput{Account: "bare"})
	wantErr(t, err, KindInvalid, "auth.token")
	_, err = ts.StoreDiscover(DiscoverInput{Account: "nope"})
	wantErr(t, err, KindNotFound, "account.not_found")
}

func TestDiscoverGitHubUnauthorized(t *testing.T) {
	t.Parallel()
	ts, _ := fakeSession(t, fakegh.State{Tokens: map[string]string{"t-alice": "alice"}}, workAccount)
	ts.env["HH_TOKEN"] = "revoked"
	_, err := ts.StoreDiscover(DiscoverInput{})
	wantErr(t, err, KindInvalid, "auth.unauthorized")
}

// The account of a host looks on that host's API and sends its token nowhere
// else: github.com's API gets no request from an account of ghe.example.com.
func TestDiscoverGitHubUsesTheHostOfTheAccount(t *testing.T) {
	t.Parallel()
	var asked int
	other := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { asked++ }))
	t.Cleanup(other.Close)
	ghe := httptest.NewServer(fakegh.New(fakegh.State{
		Tokens: map[string]string{"t-alice": "alice"},
		Repos:  []*fakegh.Repo{{Owner: "o", Name: "r", Topics: []string{"hedgehug-store"}, Perms: map[string]string{"alice": "write"}}},
	}))
	t.Cleanup(ghe.Close)
	ts := newTestSession(t)
	ts.env["HH_TOKEN"] = "t-alice"
	ts.appendConfig("hosts:\n  - {host: github.com, api: \"" + other.URL + "\", web: \"" + other.URL + "\"}\n" +
		"  - {host: ghe.example.com, api: \"" + ghe.URL + "\", web: \"" + ghe.URL + "\"}\n" +
		"accounts:\n  - {name: ghe, host: ghe.example.com, login: alice, token: \"env:HH_TOKEN\"}\n")
	cands, err := ts.StoreDiscover(DiscoverInput{})
	if err != nil || len(cands) != 1 {
		t.Fatalf("cands = %+v, %v", cands, err)
	}
	if asked != 0 {
		t.Errorf("github.com was asked %d times", asked)
	}
}

// Of several candidates, the one that fails is undone alone, the rest are
// added, and the result comes with an error.
func TestStoreMountOneFails(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	bare := filepath.Join(t.TempDir(), "good.git")
	if _, err := newTestSession(t).StoreInit(StoreInitInput{Name: "src", Prefix: "SRC", Remotes: []string{bare}}); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(t.TempDir(), "nope.git")
	res, err := ts.StoreMount([]Candidate{
		{Name: "bad", URL: missing},
		{Name: "good", URL: bare},
	})
	wantErr(t, err, KindInvalid, "store.mount_failed")
	if !reflect.DeepEqual(res.Mounted, []string{"good"}) || len(res.Failed) != 1 || res.Failed[0].Name != "bad" || res.Failed[0].Err == nil {
		t.Fatalf("result = %+v", res)
	}
	if !strings.Contains(err.Error(), "1 of 2") || !strings.Contains(err.Error(), "bad") {
		t.Errorf("err = %v", err)
	}
	notExist(t, storeDirOf(t, ts, "bad"), "the directory of the failed one")
	if got := namedStores(t, ts, "bad"); len(got) != 0 {
		t.Errorf("bad is in config.yaml: %+v", got)
	}
	if got := namedStores(t, ts, "good"); len(got) != 1 {
		t.Errorf("good is not added: %+v", got)
	}
}

// The names offered: the repo name; <owner>-<repo> when that is used; then
// <owner>-<repo>-2, -3...; and store-1, store-2... where no valid name can be
// made of the owner and the repo.
func TestNameCandidates(t *testing.T) {
	t.Parallel()
	c := &config.Config{Stores: []config.StoreEntry{{Name: "hh-shared"}, {Name: "org-a-hh-shared"}}}
	// In order: both names used; lower case; the repo name used; no valid
	// name (twice).
	cands := []Candidate{
		{URL: "https://github.com/org-a/hh-shared.git"},
		{URL: "https://github.com/org-b/Hh_Shared.git"},
		{URL: "https://github.com/org-b/hh-shared.git"},
		{URL: "/srv/日本/日本.git"},
		{URL: "/srv/語/語.git"},
	}
	nameCandidates(c, cands)
	want := []string{"org-a-hh-shared-2", "hh_shared", "org-b-hh-shared", "store-1", "store-2"}
	for i, x := range cands {
		if x.Name != want[i] {
			t.Errorf("name of %s = %q, want %q", x.URL, x.Name, want[i])
		}
	}
}

// A directory under --from that cannot be read is warned of and skipped; the
// stores elsewhere are still found.
func TestDiscoverFromSkipsAnUnreadableDirectory(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("a directory cannot be made unreadable here")
	}
	remotes := filepath.Join(t.TempDir(), "remotes")
	storesUnder(t, remotes, "https://github.com/org-a/hh-shared.git")
	locked := filepath.Join(remotes, "locked")
	if err := os.Mkdir(locked, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o750) })
	ts := newTestSession(t)
	ts.appendConfig(rewriteTo(remotes))
	cands, err := ts.StoreDiscover(DiscoverInput{From: remotes})
	if err != nil || !reflect.DeepEqual(candidateNames(cands), []string{"hh-shared"}) {
		t.Fatalf("cands = %+v, %v", cands, err)
	}
	if len(ts.warns) != 1 || !strings.Contains(ts.warns[0], "cannot read "+locked) {
		t.Errorf("warnings = %q", ts.warns)
	}
	// The root itself that cannot be read is an error.
	if _, err = ts.StoreDiscover(DiscoverInput{From: locked}); err == nil {
		t.Error("an unreadable root is not an error")
	}
}

// The URL of a candidate goes back through the rule whose to is the longest
// that matches.
func TestDiscoverFromUsesTheLongestRewrite(t *testing.T) {
	t.Parallel()
	remotes := filepath.Join(t.TempDir(), "remotes")
	storesUnder(t, remotes, "https://github.com/org-a/hh-shared.git")
	ts := newTestSession(t)
	to := filepath.ToSlash(remotes)
	ts.appendConfig("url_rewrites:\n" +
		"  - {from: \"https://github.com/\", to: \"" + to + "/\"}\n" +
		"  - {from: \"https://mirror.example/team/\", to: \"" + to + "/org-a/\"}\n")
	cands, err := ts.StoreDiscover(DiscoverInput{From: remotes})
	if err != nil || len(cands) != 1 || cands[0].URL != "https://mirror.example/team/hh-shared.git" {
		t.Fatalf("cands = %+v, %v", cands, err)
	}
}
