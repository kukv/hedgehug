package integration_test

import (
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/kukv/hedgehug/internal/app/domain/automation"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/integration"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/fakegh"
	"github.com/kukv/hedgehug/internal/lib/gitrepo/gitrepotest"
)

const originURL = "https://github.com/org-a/web.git"

// prEnv is a store on a fakegh and a code repository whose origin
// url_rewrites sends to a local bare repository.
type prEnv struct {
	ts   *usecasetest.Session
	fake *fakegh.Server
	dir  string // the work tree
	bare string // origin
}

func newPREnv(t *testing.T, repo fakegh.Repo) *prEnv {
	t.Helper()
	repo.Owner, repo.Name = "org-a", "web"
	repo.Perms = map[string]string{"alice": "write"}
	ts, fake := usecasetest.FakeSession(t, fakegh.State{
		Tokens: map[string]string{"t-alice": "alice"},
		Repos:  []*fakegh.Repo{&repo},
	}, "")
	ts.AppendConfig("accounts:\n  - {name: me, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n")
	ts.ForgetWriter()
	fake.SetNow(func() time.Time { return ts.Now() })
	remotes := t.TempDir()
	bare := filepath.Join(remotes, "org-a", "web.git")
	gitrepotest.Bare(t, bare)
	ts.AppendConfig("url_rewrites:\n  - {from: \"https://github.com/\", to: \"" + filepath.ToSlash(remotes) + "/\"}\n")
	dir := codeRepo(t)
	setOrigin(t, dir, originURL)
	return &prEnv{ts: ts, fake: fake, dir: dir, bare: bare}
}

func setOrigin(t *testing.T, dir, u string) {
	t.Helper()
	r, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{u}}); err != nil {
		t.Fatal(err)
	}
}

func (x *prEnv) pr(t *testing.T) integration.PRResult {
	t.Helper()
	r, err := integration.PullRequest(x.ts.Session, "ME-1", integration.PRInput{Dir: x.dir})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func (x *prEnv) pulls(t *testing.T) []*fakegh.Pull {
	t.Helper()
	repo, ok := x.fake.Repo("org-a/web")
	if !ok {
		t.Fatal("no repository")
	}
	return repo.Pulls
}

func (x *prEnv) devStore(t *testing.T) {
	t.Helper()
	b, err := automation.DevRules().Encode()
	if err != nil {
		t.Fatal(err)
	}
	x.ts.MustEdit("me", "edit", usecasetest.TAuthor, func(tx *usecasetest.Tx) error {
		if err := tx.PutSchema(schema.Dev()); err != nil {
			return err
		}
		return tx.PutFile(model.AutomationsPath, b)
	})
}

func onBareBranch(t *testing.T, bare, name string) bool {
	t.Helper()
	r, err := git.PlainOpen(bare)
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Reference(plumbing.NewBranchReferenceName(name), false)
	return err == nil
}

func TestPullRequestOpensAndLinks(t *testing.T) {
	t.Parallel()
	x := newPREnv(t, fakegh.Repo{DefaultBranch: "trunk"})
	task := x.ts.AddTask("Fix the login", func(y *model.Task) { y.Body = "Why\n\nBecause.\n" })
	addBranch(t, x.dir, "hh/ME-1-fix-the-login")
	r := x.pr(t)
	if !r.Created || r.Branch != "hh/ME-1-fix-the-login" || r.Ref.String() != "github.com/org-a/web/pull/1" || !r.Committed || len(r.Warnings) != 0 {
		t.Fatalf("result = %+v", r)
	}
	if !onBareBranch(t, x.bare, "hh/ME-1-fix-the-login") {
		t.Error("the branch is not on origin")
	}
	ps := x.pulls(t)
	want := "Why\n\nBecause.\n\nhedgehug:" + task.ULID.String()
	if len(ps) != 1 || ps[0].Title != "Fix the login" || ps[0].Body != want || ps[0].Base != "trunk" || ps[0].Head != "hh/ME-1-fix-the-login" || ps[0].Draft {
		t.Fatalf("pulls = %+v", ps[0])
	}
	y := taskOf(t, x.ts.Session)
	g, err := y.GitHub()
	if err != nil {
		t.Fatal(err)
	}
	if len(y.Links) != 1 || y.Links[0].Ref != "github.com/org-a/web/pull/1" || y.Links[0].Sync != model.SyncMirror {
		t.Errorf("links = %+v", y.Links)
	}
	if len(g.PRs) != 1 || g.PRs[0].State != "open" {
		t.Errorf("gh = %+v", g)
	}
}

func TestPullRequestOfAnEmptyBodyIsTheMark(t *testing.T) {
	t.Parallel()
	x := newPREnv(t, fakegh.Repo{})
	task := x.ts.AddTask("a", nil)
	addBranch(t, x.dir, "hh/ME-1-a")
	x.pr(t)
	if ps := x.pulls(t); ps[0].Body != "hedgehug:"+task.ULID.String() || ps[0].Base != "main" {
		t.Errorf("pull = %+v", ps[0])
	}
}

func TestPullRequestDraft(t *testing.T) {
	t.Parallel()
	x := newPREnv(t, fakegh.Repo{})
	x.ts.AddTask("a", nil)
	addBranch(t, x.dir, "hh/ME-1-a")
	if _, err := integration.PullRequest(x.ts.Session, "ME-1", integration.PRInput{Dir: x.dir, Draft: true}); err != nil {
		t.Fatal(err)
	}
	if !x.pulls(t)[0].Draft {
		t.Error("not a draft")
	}
}

func TestPullRequestFiresTheRule(t *testing.T) {
	t.Parallel()
	x := newPREnv(t, fakegh.Repo{})
	x.devStore(t)
	x.ts.AddTask("a", nil)
	addBranch(t, x.dir, "hh/ME-1-a")
	x.pr(t)
	if y := taskOf(t, x.ts.Session); y.Status != "in_progress" {
		t.Errorf("status = %s", y.Status)
	}
}

func TestPullRequestAgainAddsNothing(t *testing.T) {
	t.Parallel()
	x := newPREnv(t, fakegh.Repo{})
	x.ts.AddTask("a", nil)
	addBranch(t, x.dir, "hh/ME-1-a")
	x.pr(t)
	head := x.ts.Head()
	r := x.pr(t)
	if r.Created || r.Committed || r.Ref.Number != 1 {
		t.Errorf("result = %+v", r)
	}
	if n := len(x.pulls(t)); n != 1 {
		t.Errorf("%d pulls", n)
	}
	if n := len(taskOf(t, x.ts.Session).Links); n != 1 {
		t.Errorf("%d links", n)
	}
	if x.ts.Head() != head {
		t.Error("the second run committed")
	}
}

// The run that pushed and opened the pull request stopped there: the next one
// links what is open.
func TestPullRequestLinksTheOneThatIsOpen(t *testing.T) {
	t.Parallel()
	x := newPREnv(t, fakegh.Repo{})
	x.ts.AddTask("a", nil)
	addBranch(t, x.dir, "hh/ME-1-a")
	if _, err := x.fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	r := x.pr(t)
	if r.Created || !r.Committed || r.Ref.Number != 1 {
		t.Errorf("result = %+v", r)
	}
	if n := len(x.pulls(t)); n != 1 {
		t.Errorf("%d pulls", n)
	}
}

// hideOpenPulls answers the first look for the open pull requests with none,
// as if the pull request was opened right after it.
type hideOpenPulls struct{ looked *atomic.Bool }

func (h hideOpenPulls) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/pulls") && !h.looked.Swap(true) {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("[]")), Request: r}, nil
	}
	return http.DefaultTransport.RoundTrip(r)
}

func TestPullRequestOpenedMeanwhileIsLinked(t *testing.T) {
	t.Parallel()
	x := newPREnv(t, fakegh.Repo{})
	x.ts.AddTask("a", nil)
	addBranch(t, x.dir, "hh/ME-1-a")
	if _, err := x.fake.OpenPull("org-a/web", "hh/ME-1-a", "A", "", false); err != nil {
		t.Fatal(err)
	}
	x.ts.HTTPClient = &http.Client{Transport: hideOpenPulls{&atomic.Bool{}}}
	r := x.pr(t)
	if r.Created || !r.Committed || r.Ref.Number != 1 {
		t.Errorf("result = %+v", r)
	}
	if n := len(x.pulls(t)); n != 1 {
		t.Errorf("%d pulls", n)
	}
}

// hh pr names the pull request itself, so a link that hh unlink turned off
// comes back.
func TestPullRequestBringsAnUnlinkedLinkBack(t *testing.T) {
	t.Parallel()
	x := newPREnv(t, fakegh.Repo{})
	x.ts.AddTask("a", nil)
	addBranch(t, x.dir, "hh/ME-1-a")
	x.pr(t)
	if _, err := integration.Unlink(x.ts.Session, "ME-1", integration.LinkInput{PR: "org-a/web#1"}); err != nil {
		t.Fatal(err)
	}
	x.pr(t)
	if l := taskOf(t, x.ts.Session).Links; len(l) != 1 || !l[0].Live() {
		t.Errorf("links = %+v", l)
	}
}

func TestPullRequestUsesTheBranchItIsOn(t *testing.T) {
	t.Parallel()
	x := newPREnv(t, fakegh.Repo{})
	x.ts.AddTask("a", nil)
	addBranch(t, x.dir, "hh/ME-1-a")
	addBranch(t, x.dir, "hh/ME-1-b")
	r, err := git.PlainOpen(x.dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.NewBranchReferenceName("hh/ME-1-a"))); err != nil {
		t.Fatal(err)
	}
	if got := x.pr(t); got.Branch != "hh/ME-1-a" {
		t.Errorf("branch = %s", got.Branch)
	}
}

func TestPullRequestTakesTheBareBranchName(t *testing.T) {
	t.Parallel()
	x := newPREnv(t, fakegh.Repo{})
	x.ts.AddTask("ログインを直す", nil)
	addBranch(t, x.dir, "hh/ME-1")
	if r := x.pr(t); r.Branch != "hh/ME-1" {
		t.Errorf("branch = %s", r.Branch)
	}
}

func TestPullRequestWithSeveralBranchesAsks(t *testing.T) {
	t.Parallel()
	x := newPREnv(t, fakegh.Repo{})
	x.ts.AddTask("a", nil)
	addBranch(t, x.dir, "hh/ME-1-a")
	addBranch(t, x.dir, "hh/ME-1-b")
	_, err := integration.PullRequest(x.ts.Session, "ME-1", integration.PRInput{Dir: x.dir})
	usecasetest.MustCode(t, err, usecase.KindUsage, "choice.required")
	if !strings.Contains(err.Error(), "hh/ME-1-a, hh/ME-1-b") {
		t.Errorf("err = %v", err)
	}
}

func TestPullRequestWithNoBranchSaysStart(t *testing.T) {
	t.Parallel()
	x := newPREnv(t, fakegh.Repo{})
	x.ts.AddTask("a", nil)
	addBranch(t, x.dir, "hh/ME-12-a")
	_, err := integration.PullRequest(x.ts.Session, "ME-1", integration.PRInput{Dir: x.dir})
	usecasetest.MustCode(t, err, usecase.KindInvalid, "pr.no_branch")
	if !strings.Contains(err.Error(), "hedgehug start ME-1") {
		t.Errorf("err = %v", err)
	}
}

func TestPullRequestOutsideARepository(t *testing.T) {
	t.Parallel()
	x := newPREnv(t, fakegh.Repo{})
	x.ts.AddTask("a", nil)
	_, err := integration.PullRequest(x.ts.Session, "ME-1", integration.PRInput{Dir: t.TempDir()})
	usecasetest.MustCode(t, err, usecase.KindInvalid, "code.no_repository")
}

func TestPullRequestWithoutAnOrigin(t *testing.T) {
	t.Parallel()
	x := newPREnv(t, fakegh.Repo{})
	x.ts.AddTask("a", nil)
	dir := codeRepo(t)
	addBranch(t, dir, "hh/ME-1-a")
	_, err := integration.PullRequest(x.ts.Session, "ME-1", integration.PRInput{Dir: dir})
	usecasetest.MustCode(t, err, usecase.KindInvalid, "pr.no_origin")
	setOrigin(t, dir, x.bare)
	_, err = integration.PullRequest(x.ts.Session, "ME-1", integration.PRInput{Dir: dir})
	usecasetest.MustCode(t, err, usecase.KindInvalid, "pr.no_origin")
}

func TestPullRequestStopsOnANonFastForwardPush(t *testing.T) {
	t.Parallel()
	x := newPREnv(t, fakegh.Repo{})
	x.ts.AddTask("a", nil)
	if _, err := integration.Start(x.ts.Session, "ME-1", x.dir); err != nil {
		t.Fatal(err)
	}
	r, err := git.PlainOpen(x.dir)
	if err != nil {
		t.Fatal(err)
	}
	head, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	// A commit on the remote branch that the local one does not have.
	base, err := r.CommitObject(head.Hash())
	if err != nil {
		t.Fatal(err)
	}
	sig := object.Signature{Name: "t", Email: "t@example.com", When: time.Unix(1_700_000_100, 0)}
	o := r.Storer.NewEncodedObject()
	if err := (&object.Commit{Author: sig, Committer: sig, Message: "theirs", TreeHash: base.TreeHash, ParentHashes: []plumbing.Hash{head.Hash()}}).Encode(o); err != nil {
		t.Fatal(err)
	}
	theirs, err := r.Storer.SetEncodedObject(o)
	if err != nil {
		t.Fatal(err)
	}
	refs := gitconfig.RefSpec(theirs.String() + ":refs/heads/hh/ME-1-a")
	if err := r.Push(&git.PushOptions{RemoteURL: x.bare, RefSpecs: []gitconfig.RefSpec{refs}}); err != nil {
		t.Fatal(err)
	}
	_, err = integration.PullRequest(x.ts.Session, "ME-1", integration.PRInput{Dir: x.dir})
	usecasetest.MustCode(t, err, usecase.KindConflict, "pr.rejected")
	if n := len(x.pulls(t)); n != 0 {
		t.Errorf("%d pulls", n)
	}
}

// failGraphQL lets everything through but the reads of the fetch.
type failGraphQL struct{}

func (failGraphQL) RoundTrip(r *http.Request) (*http.Response, error) {
	if strings.HasSuffix(r.URL.Path, "/graphql") {
		return nil, errors.New("no graphql today")
	}
	return http.DefaultTransport.RoundTrip(r)
}

func TestPullRequestKeepsThePullWhenTheFetchFails(t *testing.T) {
	t.Parallel()
	x := newPREnv(t, fakegh.Repo{})
	x.ts.AddTask("a", nil)
	addBranch(t, x.dir, "hh/ME-1-a")
	x.ts.HTTPClient = &http.Client{Transport: failGraphQL{}}
	r := x.pr(t)
	if !r.Created || len(r.Warnings) == 0 || !strings.Contains(r.Warnings[0], "no graphql today") {
		t.Fatalf("result = %+v", r)
	}
	if l := taskOf(t, x.ts.Session).Links; len(l) != 1 {
		t.Errorf("links = %+v", l)
	}
}

func TestPullRequestOfAReadOnlyStore(t *testing.T) {
	t.Parallel()
	x := newPREnv(t, fakegh.Repo{})
	x.ts.AddTask("a", nil)
	addBranch(t, x.dir, "hh/ME-1-a")
	usecasetest.MakeReadOnly(t, x.ts, "no push access")
	_, err := integration.PullRequest(x.ts.Session, "ME-1", integration.PRInput{Dir: x.dir})
	var ro *usecase.ReadOnlyError
	if !errors.As(err, &ro) {
		t.Fatalf("err = %v", err)
	}
	if onBareBranch(t, x.bare, "hh/ME-1-a") {
		t.Error("the branch was pushed")
	}
}

// A task named with its store goes through the session of that store.
func TestPullRequestOfATaskNamedWithItsStore(t *testing.T) {
	t.Parallel()
	x := newPREnv(t, fakegh.Repo{})
	x.ts.AddTask("a", nil)
	addBranch(t, x.dir, "hh/ME-1-a")
	r, err := integration.PullRequest(x.ts.Session, "me:ME-1", integration.PRInput{Dir: x.dir})
	if err != nil || !r.Created {
		t.Fatalf("result = %+v, %v", r, err)
	}
}

// The token of repo_accounts pushes and opens the pull request; the account of
// the store has no access. Serial: InstallHTTPSSessions replaces the transports.
func TestPullRequestUsesTheAccountOfTheRepository(t *testing.T) {
	remotes := t.TempDir()
	gitrepotest.Bare(t, filepath.Join(remotes, "org-a", "web.git"))
	var sessions []gitrepotest.Session
	gitrepotest.InstallHTTPSSessions(t, gitrepotest.Under("https://github.com/", remotes), &sessions, nil)
	ts, fake := usecasetest.FakeSession(t, fakegh.State{
		Tokens: map[string]string{"t-alice": "alice", "t-bob": "bob"},
		Repos:  []*fakegh.Repo{{Owner: "org-a", Name: "web", Perms: map[string]string{"bob": "write"}}},
	}, "")
	ts.Env["HH_TOKEN_BOB"] = "t-bob"
	ts.AppendConfig("accounts:\n" +
		"  - {name: me, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n" +
		"  - {name: work, host: github.com, login: bob, token: \"env:HH_TOKEN_BOB\"}\n" +
		"repo_accounts:\n  - {match: github.com/org-a/*, account: work}\n")
	ts.ForgetWriter()
	ts.AddTask("a", nil)
	dir := codeRepo(t)
	setOrigin(t, dir, originURL)
	addBranch(t, dir, "hh/ME-1-a")
	r, err := integration.PullRequest(ts.Session, "ME-1", integration.PRInput{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Created {
		t.Errorf("result = %+v", r)
	}
	push := slices.IndexFunc(sessions, func(s gitrepotest.Session) bool { return s.Kind == gitrepotest.ReceivePack })
	if push < 0 {
		t.Fatal("no push")
	}
	if b, ok := sessions[push].Auth.(*githttp.BasicAuth); !ok || b.Password != "t-bob" {
		t.Errorf("auth = %#v", sessions[push].Auth)
	}
	if repo, _ := fake.Repo("org-a/web"); len(repo.Pulls) != 1 {
		t.Errorf("pulls = %+v", repo.Pulls)
	}
}

// Serial: InstallHTTPSSessions replaces the transports.
func TestPullRequestPushDenied(t *testing.T) {
	remotes := t.TempDir()
	gitrepotest.Bare(t, filepath.Join(remotes, "org-a", "web.git"))
	var auths []transport.AuthMethod
	gitrepotest.InstallHTTPS(t, gitrepotest.Under("https://github.com/", remotes), &auths, func(string) bool { return true })
	ts, fake := usecasetest.FakeSession(t, fakegh.State{
		Tokens: map[string]string{"t-alice": "alice"},
		Repos:  []*fakegh.Repo{{Owner: "org-a", Name: "web", Perms: map[string]string{"alice": "write"}}},
	}, "")
	ts.AppendConfig("accounts:\n  - {name: me, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n")
	ts.ForgetWriter()
	ts.AddTask("a", nil)
	dir := codeRepo(t)
	setOrigin(t, dir, originURL)
	addBranch(t, dir, "hh/ME-1-a")
	_, err := integration.PullRequest(ts.Session, "ME-1", integration.PRInput{Dir: dir})
	usecasetest.MustCode(t, err, usecase.KindInvalid, "pr.denied")
	if repo, _ := fake.Repo("org-a/web"); len(repo.Pulls) != 0 {
		t.Errorf("pulls = %+v", repo.Pulls)
	}
}

// What the fetch commits is marked for the background sync as well: the
// session of the fetch does not start one. Serial: StubSpawn is global.
func TestPullRequestStartsTheSyncOfTheFetch(t *testing.T) {
	x := newPREnv(t, fakegh.Repo{})
	x.ts.AddTask("a", nil)
	addBranch(t, x.dir, "hh/ME-1-a")
	usecasetest.AddBareRemote(t, x.ts)
	x.ts.Env["HEDGEHUG_AUTO_SYNC"] = "1"
	x.ts.BackgroundSync = true
	started := 0
	defer usecasetest.StubSpawn(func([]string) { started++ })()
	r := x.pr(t)
	if !r.Created {
		t.Fatalf("result = %+v", r)
	}
	if started != 2 {
		t.Errorf("%d background syncs, want one for the link and one for the fetch", started)
	}
}
