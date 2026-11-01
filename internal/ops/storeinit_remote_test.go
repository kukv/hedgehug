package ops

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/fakegh"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/gitrepo/gitrepotest"
)

const workAccount = "accounts:\n  - {name: work, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n"

// fakeSession is a session whose github.com is a fakegh; accounts is added to
// config.yaml. HH_TOKEN is the token of alice.
func fakeSession(t *testing.T, st fakegh.State, accounts string) (*testSession, *fakegh.Server) {
	t.Helper()
	zero := 0
	st.DeviceInterval = &zero
	if st.Users == nil {
		st.Users = []string{"alice", "bob"}
	}
	fake := fakegh.New(st)
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	ts := newTestSession(t)
	ts.env["HH_TOKEN"] = "t-alice"
	ts.appendConfig("hosts:\n  - {host: github.com, api: \"" + srv.URL + "\", web: \"" + srv.URL + "\"}\n" + accounts)
	return ts, fake
}

func configText(t *testing.T, ts *testSession) string {
	t.Helper()
	_, p, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func notExist(t *testing.T, path, what string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("%s is left: %v", what, err)
	}
}

func storeDirOf(t *testing.T, ts *testSession, name string) string {
	t.Helper()
	_, p, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(p.StoresDir, name)
}

func TestStoreInitMakesLocalBare(t *testing.T) {
	ts := newTestSession(t)
	remotes := filepath.Join(t.TempDir(), "remotes")
	ts.appendConfig("url_rewrites:\n  - {from: \"https://github.com/\", to: \"" + filepath.ToSlash(remotes) + "/\"}\n")
	if _, err := ts.StoreInit(StoreInitInput{Name: "t", Prefix: "TT", Remotes: []string{"https://github.com/kukv/hh-tasks.git"}}); err != nil {
		t.Fatal(err)
	}
	if ok, err := gitrepo.HeadHasFile(filepath.Join(remotes, "kukv", "hh-tasks.git"), ".hedgehug/store.yaml"); err != nil || !ok {
		t.Errorf("the bare was not made and pushed: %v %v", ok, err)
	}
}

// The only account is the one that writes the store, so it is the one written
// to stores[].account, with or without Account.
func TestStoreInitWritesTheAccount(t *testing.T) {
	ts := newTestSession(t)
	ts.appendConfig("accounts:\n  - {name: alice, host: github.com, login: alice}\n")
	if _, err := ts.StoreInit(StoreInitInput{Name: "one", Prefix: "ON"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.StoreInit(StoreInitInput{Name: "two", Prefix: "TW", Account: "alice"}); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(configText(t, ts), "account: alice"); n != 2 {
		t.Errorf("account: alice written %d times, want 2:\n%s", n, configText(t, ts))
	}
}

// With two accounts and none named, a store with no GitHub remote has no
// account, and nothing is written for it.
func TestStoreInitNoAccountPicked(t *testing.T) {
	ts := newTestSession(t)
	ts.appendConfig("accounts:\n  - {name: a, host: github.com, login: alice}\n  - {name: b, host: github.com, login: bob}\n")
	if _, err := ts.StoreInit(StoreInitInput{Name: "x", Prefix: "XX"}); err != nil {
		t.Fatal(err)
	}
	got := namedStores(t, ts, "x")
	if len(got) != 1 || got[0].Account != "" {
		t.Errorf("stores named x = %+v, want one with no account", got)
	}
}

func TestStoreInitOnGitHub(t *testing.T) {
	ts, fake := fakeSession(t, fakegh.State{Orgs: map[string][]string{"org-a": {"alice"}}, Tokens: map[string]string{"t-alice": "alice"}}, workAccount)
	d := t.TempDir()
	// The fake creates no git repository; the test serves the push from a bare.
	if err := gitrepo.InitBare(filepath.Join(d, "org-a", "hh-new.git")); err != nil {
		t.Fatal(err)
	}
	gitrepotest.InstallHTTPS(t, gitrepotest.Under("https://github.com/", d), nil, nil)
	clone := filepath.Join(d, "clone")
	r, err := ts.StoreInit(StoreInitInput{Name: "org", Prefix: "ORG", Path: clone, Remotes: []string{"https://github.com/org-a/hh-new.git"}})
	if err != nil {
		t.Fatal(err)
	}
	if r.Path != clone {
		t.Errorf("Path = %q, want %q", r.Path, clone)
	}
	repo, ok := fake.Repo("org-a/hh-new")
	if !ok || !repo.Private || len(repo.Topics) != 1 || repo.Topics[0] != "hedgehug-store" {
		t.Errorf("repo = %+v %v", repo, ok)
	}
	if b := configText(t, ts); !strings.Contains(b, "account: work") || !strings.Contains(b, filepath.ToSlash(clone)) {
		t.Errorf("config.yaml = %s", b)
	}
	// Again: the repository exists, and now it has the store in it.
	_, err = ts.StoreInit(StoreInitInput{Name: "org2", Prefix: "ORGB", Remotes: []string{"https://github.com/org-a/hh-new.git"}})
	wantErr(t, err, KindConflict, "store.remote_not_empty")
	if !strings.Contains(err.Error(), "not empty") || !strings.Contains(err.Error(), "hedgehug store add org2") {
		t.Errorf("err = %v", err)
	}
	notExist(t, storeDirOf(t, ts, "org2"), "the store directory of the failed init")
}

// A repository on GitHub with no ref (an empty one that a person made by
// hand, say) is used as it is: the creation is skipped, the topic is set, and
// the store is pushed. One with a commit is not touched.
func TestStoreInitOnAnExistingRepo(t *testing.T) {
	ts, fake := fakeSession(t, fakegh.State{
		Orgs:   map[string][]string{"org-a": {"alice"}},
		Tokens: map[string]string{"t-alice": "alice"},
		Repos: []*fakegh.Repo{
			{Owner: "org-a", Name: "hh-empty", Private: true, Perms: map[string]string{"alice": "admin"}},
			{Owner: "org-a", Name: "hh-full", Private: true, Perms: map[string]string{"alice": "admin"}},
		},
	}, workAccount)
	d := t.TempDir()
	empty := filepath.Join(d, "org-a", "hh-empty.git")
	if err := gitrepo.InitBare(empty); err != nil {
		t.Fatal(err)
	}
	maker := otherSession(t, ts, ts.Now())
	if _, err := maker.StoreInit(StoreInitInput{Name: "full", Prefix: "FULL", Remotes: []string{filepath.Join(d, "org-a", "hh-full.git")}}); err != nil {
		t.Fatal(err)
	}
	gitrepotest.InstallHTTPS(t, gitrepotest.Under("https://github.com/", d), nil, nil)

	if _, err := ts.StoreInit(StoreInitInput{Name: "org", Prefix: "ORG", Remotes: []string{"https://github.com/org-a/hh-empty.git"}}); err != nil {
		t.Fatal(err)
	}
	if r, _ := fake.Repo("org-a/hh-empty"); len(r.Topics) != 1 || r.Topics[0] != "hedgehug-store" {
		t.Errorf("the topic of the empty repository = %v", r.Topics)
	}
	if ok, err := gitrepo.HeadHasFile(empty, ".hedgehug/store.yaml"); err != nil || !ok {
		t.Errorf("the store was not pushed to the empty repository: %v %v", ok, err)
	}
	if b := configText(t, ts); !strings.Contains(b, "account: work") {
		t.Errorf("config.yaml = %s", b)
	}

	full := "https://github.com/org-a/hh-full.git"
	_, err := ts.StoreInit(StoreInitInput{Name: "org2", Prefix: "ORGB", Remotes: []string{full}})
	wantErr(t, err, KindConflict, "store.remote_not_empty")
	if !strings.Contains(err.Error(), "not empty") || !strings.Contains(err.Error(), "hedgehug store add org2 --remote "+full) {
		t.Errorf("a repository with a commit: %v", err)
	}
	if r, _ := fake.Repo("org-a/hh-full"); len(r.Topics) != 0 {
		t.Errorf("the topic was set on a repository that was not used: %v", r.Topics)
	}
	notExist(t, storeDirOf(t, ts, "org2"), "the store directory of the failed init")
}

// An ssh remote is on a GitHub host by its host, like an https one, and a
// host written in capitals in hosts[] is the same host: the repository is
// made from the owner and the name in the ssh URL.
func TestStoreInitSSHRemoteIsOnGitHub(t *testing.T) {
	fake := fakegh.New(fakegh.State{Users: []string{"alice"}, Orgs: map[string][]string{"org-a": {"alice"}}, Tokens: map[string]string{"t-alice": "alice"}})
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	ts := newTestSession(t)
	ts.env["HH_TOKEN"] = "t-alice"
	api := strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)
	ts.appendConfig("accounts:\n  - {name: work, host: localhost, login: alice, token: \"env:HH_TOKEN\"}\n" +
		"hosts:\n  - {host: LOCALHOST, api: \"" + api + "\", web: \"" + api + "\"}\n")
	// Nothing listens on port 1, so the push fails; the repository is made
	// first, and is left there with the way to use it.
	_, err := ts.StoreInit(StoreInitInput{Name: "org", Prefix: "ORG", Remotes: []string{"ssh://git@localhost:1/org-a/hh-ssh.git"}})
	if err == nil || !strings.Contains(err.Error(), "hedgehug store add org") {
		t.Fatalf("err = %v", err)
	}
	if r, ok := fake.Repo("org-a/hh-ssh"); !ok || len(r.Topics) != 1 || r.Topics[0] != "hedgehug-store" {
		t.Errorf("repo = %+v %v", r, ok)
	}
	notExist(t, storeDirOf(t, ts, "org"), "the store directory")
}

func TestStoreInitGitHubFailureKeepsRepo(t *testing.T) {
	ts, fake := fakeSession(t, fakegh.State{Orgs: map[string][]string{"org-a": {"alice"}}, Tokens: map[string]string{"t-alice": "alice"}}, workAccount)
	// No transport serves the push: it fails after the repository is made.
	gitrepotest.InstallHTTPS(t, func(string) (string, bool) { return "", false }, nil, nil)
	_, err := ts.StoreInit(StoreInitInput{Name: "org", Prefix: "ORG", Remotes: []string{"https://github.com/org-a/hh-x.git"}})
	if err == nil || !strings.Contains(err.Error(), "https://github.com/org-a/hh-x.git") || !strings.Contains(err.Error(), "hedgehug store add org") {
		t.Errorf("err = %v", err)
	}
	if _, ok := fake.Repo("org-a/hh-x"); !ok {
		t.Error("the repository made on GitHub was removed")
	}
	notExist(t, storeDirOf(t, ts, "org"), "the store directory")
}

// A 403 on creating (alice is not a member of org-a) tells how to go on with
// a repository made by hand; a 401 names the account and the command.
func TestStoreInitGitHubErrors(t *testing.T) {
	ts, fake := fakeSession(t, fakegh.State{Orgs: map[string][]string{"org-a": {"bob"}}, Tokens: map[string]string{"t-alice": "alice"}}, workAccount)
	in := StoreInitInput{Name: "org", Prefix: "ORG", Remotes: []string{"https://github.com/org-a/x.git"}}
	_, err := ts.StoreInit(in)
	wantErr(t, err, KindInvalid, "github.forbidden")
	if !strings.Contains(err.Error(), "github.com/alice cannot create org-a/x") || !strings.Contains(err.Error(), "make an empty repository on GitHub") {
		t.Errorf("403: %v", err)
	}
	if _, ok := fake.Repo("org-a/x"); ok {
		t.Error("a repository was made for a token that may not")
	}
	// The token of ts is cached; a session of its own gets the stale one.
	ts2, _ := fakeSession(t, fakegh.State{Orgs: map[string][]string{"org-a": {"bob"}}, Tokens: map[string]string{"t-alice": "alice"}}, workAccount)
	ts2.env["HH_TOKEN"] = "stale"
	_, err = ts2.StoreInit(in)
	wantErr(t, err, KindInvalid, "auth.unauthorized")
	if !strings.Contains(err.Error(), "github.com/alice cannot create org-a/x") || !strings.Contains(err.Error(), "run hedgehug auth login work") {
		t.Errorf("401: %v", err)
	}
}

func TestStoreInitAccountChoice(t *testing.T) {
	ts, _ := fakeSession(t, fakegh.State{}, "accounts:\n  - {name: a, host: github.com, login: alice, token: gh}\n  - {name: b, host: github.com, login: bob, token: gh}\n")
	in := StoreInitInput{Name: "org", Prefix: "ORG", Remotes: []string{"https://github.com/org-a/x.git"}}
	_, err := ts.StoreInit(in)
	wantErr(t, err, KindUsage, "choice.required")
	e := AsError(err)
	if got, _ := e.Args["candidates"].([]string); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("candidates = %v", e.Args["candidates"])
	}
	if !strings.Contains(err.Error(), "--account") || !strings.Contains(err.Error(), "a, b") {
		t.Errorf("err = %v", err)
	}
	notExist(t, storeDirOf(t, ts, "org"), "the store directory")
	in.Account = "nope"
	_, err = ts.StoreInit(in)
	wantErr(t, err, KindNotFound, "account.not_found")
}

// A GitHub remote with no account at all points at auth login.
func TestStoreInitNoAccountAtAll(t *testing.T) {
	ts, _ := fakeSession(t, fakegh.State{}, "")
	_, err := ts.StoreInit(StoreInitInput{Name: "org", Prefix: "ORG", Remotes: []string{"https://github.com/org-a/x.git"}})
	wantErr(t, err, KindConflict, "account.none")
	if !strings.Contains(err.Error(), "hedgehug auth login") {
		t.Errorf("err = %v", err)
	}
}

// The token of an account is not sent to another host: an account of
// github.com does not make a repository on a host of hosts[].
func TestStoreInitAccountOfAnotherHost(t *testing.T) {
	fake := fakegh.New(fakegh.State{Orgs: map[string][]string{"org-a": {"alice"}}, Tokens: map[string]string{"t-alice": "alice"}})
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	ts := newTestSession(t)
	ts.env["HH_TOKEN"] = "t-alice"
	ts.appendConfig("hosts:\n  - {host: github.com, api: \"" + srv.URL + "\"}\n  - {host: ghe.example.com, api: \"" + srv.URL + "\"}\n" + workAccount)
	_, err := ts.StoreInit(StoreInitInput{Name: "org", Prefix: "ORG", Remotes: []string{"https://ghe.example.com/org-a/x.git"}})
	wantErr(t, err, KindInvalid, "account.host_mismatch")
	if _, ok := fake.Repo("org-a/x"); ok {
		t.Error("a repository was made")
	}
}

// When config.yaml cannot take the store (another writer added one of the
// name), the store directory and the bare are removed, and the repository
// made on GitHub is left, with the way to use it.
func TestStoreInitConfigFailureUndoes(t *testing.T) {
	ts, fake := fakeSession(t, fakegh.State{Orgs: map[string][]string{"org-a": {"alice"}}, Tokens: map[string]string{"t-alice": "alice"}}, workAccount)
	d := t.TempDir()
	bare := filepath.Join(d, "org-a", "hh-race.git")
	if err := gitrepo.InitBare(bare); err != nil {
		t.Fatal(err)
	}
	gitrepotest.InstallHTTPS(t, gitrepotest.Under("https://github.com/", d), nil, nil)
	theirs := filepath.Join(t.TempDir(), "theirs")
	hookConfigWrite(t, ts, func(c *config.Config) error { return c.AddStore("race", theirs, nil) })
	_, err := ts.StoreInit(StoreInitInput{Name: "race", Prefix: "RC", Remotes: []string{"https://github.com/org-a/hh-race.git"}})
	wantErr(t, err, KindConflict, "store.exists")
	if !strings.Contains(err.Error(), "left there") || !strings.Contains(err.Error(), "hedgehug store add race --remote https://github.com/org-a/hh-race.git") {
		t.Errorf("err = %v", err)
	}
	notExist(t, storeDirOf(t, ts, "race"), "the store directory")
	if _, ok := fake.Repo("org-a/hh-race"); !ok {
		t.Error("the repository made on GitHub was removed")
	}
	if got := namedStores(t, ts, "race"); len(got) != 1 || got[0].Path != theirs {
		t.Errorf("stores = %+v", got)
	}
}

// A bare that the failed init made is removed with it.
func TestStoreInitConfigFailureRemovesTheBare(t *testing.T) {
	ts := newTestSession(t)
	bare := filepath.Join(t.TempDir(), "r.git")
	theirs := filepath.Join(t.TempDir(), "theirs")
	hookConfigWrite(t, ts, func(c *config.Config) error { return c.AddStore("race", theirs, nil) })
	_, err := ts.StoreInit(StoreInitInput{Name: "race", Prefix: "RC", Remotes: []string{bare}})
	wantErr(t, err, KindConflict, "store.exists")
	notExist(t, bare, "the bare made by the failed init")
}

// A store pushed to a remote that was there before (an existing bare) and
// then refused by config.yaml: the store here is removed, the remote keeps the
// commits, and the error says how to use them.
func TestStoreInitConfigFailureAfterPushTellsOfTheRemote(t *testing.T) {
	ts := newTestSession(t)
	bare := filepath.Join(t.TempDir(), "r.git")
	if err := gitrepo.InitBare(bare); err != nil {
		t.Fatal(err)
	}
	theirs := filepath.Join(t.TempDir(), "theirs")
	hookConfigWrite(t, ts, func(c *config.Config) error { return c.AddStore("race", theirs, nil) })
	_, err := ts.StoreInit(StoreInitInput{Name: "race", Prefix: "RC", Remotes: []string{bare}})
	wantErr(t, err, KindConflict, "store.exists")
	if !strings.Contains(err.Error(), "stay there") || !strings.Contains(err.Error(), "hedgehug store add race --remote "+bare) {
		t.Errorf("err = %v", err)
	}
	if ok, herr := gitrepo.HeadHasFile(bare, ".hedgehug/store.yaml"); herr != nil || !ok {
		t.Errorf("the bare lost the commits: %v %v", ok, herr)
	}
	notExist(t, storeDirOf(t, ts, "race"), "the store directory")
}

// Only the accounts of the host of the remote are offered; with none on that
// host the way to set one up is given.
func TestStoreInitChoiceIsOfTheRemotesHost(t *testing.T) {
	ts, _ := fakeSession(t, fakegh.State{}, "accounts:\n  - {name: a, host: github.com, login: alice, token: gh}\n  - {name: b, host: github.com, login: bob, token: gh}\n  - {name: g, host: ghe.example.com, login: gina, token: gh}\n")
	_, err := ts.StoreInit(StoreInitInput{Name: "org", Prefix: "ORG", Remotes: []string{"https://github.com/org-a/x.git"}})
	wantErr(t, err, KindUsage, "choice.required")
	if got, _ := AsError(err).Args["candidates"].([]string); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("candidates = %v, want a and b", AsError(err).Args["candidates"])
	}
}

// With two GitHub remotes, the first made and the second failing, the error
// tells that the first was left on GitHub.
func TestStoreInitSecondRemoteFailsAfterTheFirstWasMade(t *testing.T) {
	ts, fake := fakeSession(t, fakegh.State{Orgs: map[string][]string{"org-a": {"alice"}, "org-b": {"bob"}}, Tokens: map[string]string{"t-alice": "alice"}}, workAccount)
	first := "https://github.com/org-a/hh-a.git"
	_, err := ts.StoreInit(StoreInitInput{Name: "org", Prefix: "ORG", Remotes: []string{first, "https://github.com/org-b/hh-b.git"}})
	wantErr(t, err, KindInvalid, "github.forbidden")
	if !strings.Contains(err.Error(), "left there") || !strings.Contains(err.Error(), "hedgehug store add org --remote "+first) {
		t.Errorf("err = %v", err)
	}
	if _, ok := fake.Repo("org-a/hh-a"); !ok {
		t.Error("the first repository was removed")
	}
	notExist(t, storeDirOf(t, ts, "org"), "the store directory")
}

// A topic that cannot be set fails the init, and the repository made is told.
func TestStoreInitTopicFailureTellsOfTheRepo(t *testing.T) {
	fake := fakegh.New(fakegh.State{Users: []string{"alice"}, Orgs: map[string][]string{"org-a": {"alice"}}, Tokens: map[string]string{"t-alice": "alice"}})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/topics") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		fake.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	ts := newTestSession(t)
	ts.env["HH_TOKEN"] = "t-alice"
	ts.appendConfig("hosts:\n  - {host: github.com, api: \"" + srv.URL + "\", web: \"" + srv.URL + "\"}\n" + workAccount)
	u := "https://github.com/org-a/hh-t.git"
	_, err := ts.StoreInit(StoreInitInput{Name: "org", Prefix: "ORG", Remotes: []string{u}})
	wantErr(t, err, KindInvalid, "github.forbidden")
	if err == nil || !strings.Contains(err.Error(), "set the topic of org-a/hh-t") || !strings.Contains(err.Error(), "hedgehug store add org --remote "+u) {
		t.Errorf("err = %v", err)
	}
	if _, ok := fake.Repo("org-a/hh-t"); !ok {
		t.Error("the repository was removed")
	}
	notExist(t, storeDirOf(t, ts, "org"), "the store directory")
}

// A remote that is neither local nor on a GitHub host is not made: the push
// is what fails, and nothing is left.
func TestStoreInitOtherRemoteIsNotMade(t *testing.T) {
	ts, fake := fakeSession(t, fakegh.State{Orgs: map[string][]string{"org-a": {"alice"}}, Tokens: map[string]string{"t-alice": "alice"}}, workAccount)
	gitrepotest.InstallHTTPS(t, func(string) (string, bool) { return "", false }, nil, nil)
	_, err := ts.StoreInit(StoreInitInput{Name: "org", Prefix: "ORG", Remotes: []string{"https://git.example.com/org-a/x.git"}})
	if err == nil || strings.Contains(err.Error(), "left there") {
		t.Errorf("err = %v", err)
	}
	if _, ok := fake.Repo("org-a/x"); ok {
		t.Error("a repository was made on GitHub")
	}
	notExist(t, storeDirOf(t, ts, "org"), "the store directory")
}

// Accounts that are all of other hosts are no choice: account.none names the
// host of the remote.
func TestStoreInitNoAccountOfTheRemotesHost(t *testing.T) {
	ts, _ := fakeSession(t, fakegh.State{}, "accounts:\n  - {name: g, host: ghe.example.com, login: gina, token: gh}\n  - {name: h, host: ghe.example.com, login: hal, token: gh}\n")
	// (With one account only, it is picked as the only one and the host check
	// answers account.host_mismatch instead.)
	_, err := ts.StoreInit(StoreInitInput{Name: "org", Prefix: "ORG", Remotes: []string{"https://github.com/org-a/x.git"}})
	wantErr(t, err, KindConflict, "account.none")
	if !strings.Contains(err.Error(), "github.com") {
		t.Errorf("err = %v", err)
	}
	notExist(t, storeDirOf(t, ts, "org"), "the store directory")
}
