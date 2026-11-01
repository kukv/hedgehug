package ops

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/fakegh"
	"github.com/kukv/hedgehug/internal/gitrepo/gitrepotest"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

// makeReadOnly writes access.json of the test store as a read-only store.
func makeReadOnly(t *testing.T, ts *testSession, reason string) {
	t.Helper()
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if err := sync.WriteAccess(s, sync.Access{ReadOnly: true, Reason: reason, Checked: ts.Now()}); err != nil {
		t.Fatal(err)
	}
}

// wantReadOnly checks err is the refusal of a write to the store me.
func wantReadOnly(t *testing.T, what string, err error) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: no error; want store.read_only", what)
		return
	}
	e := AsError(err)
	if e.Kind != KindConflict || e.Code != "store.read_only" {
		t.Errorf("%s: %v is %v %q; want KindConflict store.read_only", what, err, e.Kind, e.Code)
		return
	}
	if e.Args["store"] != "me" || e.Args["reason"] != sync.ReasonGitHub {
		t.Errorf("%s: Args = %v; want store me, reason %s", what, e.Args, sync.ReasonGitHub)
	}
	if err.Error() != "me is read-only (the GitHub permission is read)" {
		t.Errorf("%s: message = %q", what, err)
	}
	var ro *ReadOnlyError
	if !errors.As(err, &ro) || ro.Store != "me" || !ro.Access.ReadOnly {
		t.Errorf("%s: %v is not a *ReadOnlyError of me", what, err)
	}
}

// Every write is refused when the store is opened, before an id is resolved
// or anything else is checked: ME-99 does not exist, and the refusal is not
// "not found".
func TestReadOnlyRefusesEveryWrite(t *testing.T) {
	ts := newTestSession(t)
	live, err := ts.Add(AddInput{Title: "one"})
	if err != nil {
		t.Fatal(err)
	}
	liveID := RefName(live.Index, live.ULID)
	makeReadOnly(t, ts, sync.ReasonGitHub)
	ops := []struct {
		name string
		do   func() error
	}{
		{"Add", func() error { _, err := ts.Add(AddInput{Title: "two"}); return err }},
		{"Set", func() error { _, err := ts.Set("ME-99", nil); return err }},
		{"SetStatus", func() error { _, err := ts.SetStatus("ME-99", "done"); return err }},
		{"Dep", func() error { _, err := ts.Dep("ME-99", "ME-98", true); return err }},
		{"Watch", func() error { _, err := ts.Watch("ME-99", true); return err }},
		{"BeginEdit", func() error { _, err := ts.BeginEdit("ME-99"); return err }},
		{"Edit", func() error { _, err := ts.Edit("ME-99", []byte("x")); return err }},
		{"BeginComment", func() error { _, err := ts.BeginComment("ME-99"); return err }},
		{"CommentAdd", func() error { _, err := ts.CommentAdd("ME-99", "hi"); return err }},
		{"OpenComment", func() error { _, err := ts.OpenComment("nope"); return err }},
		{"CommentEdit", func() error { _, err := ts.CommentEdit("nope", "hi"); return err }},
		{"CommentRemove", func() error { _, err := ts.CommentRemove("nope"); return err }},
		{"Move", func() error { _, err := ts.Move(MoveInput{Arg: "ME-99", Root: true}); return err }},
		{"WorkspaceAdd", func() error { _, err := ts.WorkspaceAdd("w", "WW"); return err }},
		{"WorkspaceRename", func() error { _, err := ts.WorkspaceRename("nope", "x"); return err }},
		{"WorkspacePrefix", func() error { _, err := ts.WorkspacePrefix("nope", "XX"); return err }},
		{"WorkspaceRemove", func() error { _, err := ts.WorkspaceRemove("nope"); return err }},
		{"Undo", func() error { _, err := ts.Undo(); return err }},
		{"Remove", func() error { _, err := ts.Remove("ME-99"); return err }},
		{"Restore", func() error { _, err := ts.Restore("ME-99"); return err }},
		{"RestoreAt", func() error { _, err := ts.RestoreAt("ME-99", "2026-10-01"); return err }},
		// A task that is not deleted: purge says "not deleted" once it is
		// let through, and must not get that far.
		{"PurgeTargets", func() error { _, _, _, err := ts.PurgeTargets(liveID); return err }},
		{"Purge", func() error { _, err := ts.Purge(liveID, []model.ULID{live.ULID}); return err }},
		{"PurgeUnchecked", func() error { _, err := ts.PurgeUnchecked(liveID); return err }},
		{"Migrate", func() error {
			_, _, err := ts.Migrate(schema.Migration{Target: schema.TargetStatus, Old: "todo", New: "ready"}, false)
			return err
		}},
		{"Doctor --fix", func() error { _, err := ts.Doctor(true); return err }},
		{"ViewSave", func() error { _, err := ts.ViewSave("mine", ViewChange{}); return err }},
		// Views that do not exist: the refusal comes before "not found".
		{"ViewRemove", func() error { _, err := ts.ViewRemove("nope"); return err }},
		{"ViewRename", func() error { _, err := ts.ViewRename("nope", "other"); return err }},
		{"UpdateCommit", func() error {
			s, err := ts.OpenStore()
			if err != nil {
				return err
			}
			_, err = ts.UpdateCommit(s, store.Op{Name: "add"}, func(*store.Tx) error {
				t.Error("fn ran in a read-only store")
				return nil
			})
			return err
		}},
		{"Update", func() error {
			s, err := ts.OpenStore()
			if err != nil {
				return err
			}
			_, err = ts.Update(s, store.Op{Name: "add"}, func(*store.Tx) error { return nil })
			return err
		}},
		{"OpenForWrite", func() error { _, _, err := ts.OpenForWrite(); return err }},
	}
	for _, o := range ops {
		wantReadOnly(t, o.name, o.do())
	}
}

func TestReadOnlyAllowsReading(t *testing.T) {
	ts := newTestSession(t)
	ts.addTask("one", nil)
	makeReadOnly(t, ts, sync.ReasonGitHub)
	if _, _, err := ts.Open(); err != nil {
		t.Errorf("Open: %v", err)
	}
	if _, _, err := ts.WorkspaceList(); err != nil {
		t.Errorf("WorkspaceList: %v", err)
	}
	if _, _, err := ts.Migrate(schema.Migration{Target: schema.TargetStatus, Old: "todo", New: "ready"}, true); err != nil {
		t.Errorf("Migrate --dry-run: %v", err)
	}
	if _, err := ts.Doctor(false); err != nil {
		t.Errorf("Doctor: %v", err)
	}
	if _, err := ts.ViewList(); err != nil {
		t.Errorf("ViewList: %v", err)
	}
}

// The words follow the reason.
func TestReadOnlyReasonsInTheMessage(t *testing.T) {
	for reason, want := range map[string]string{
		sync.ReasonNotWritable: "me is read-only (the remote cannot be written)",
		sync.ReasonPushDenied:  "me is read-only (the remote denied the push)",
	} {
		ts := newTestSession(t)
		makeReadOnly(t, ts, reason)
		_, err := ts.Add(AddInput{Title: "x"})
		if err == nil || err.Error() != want {
			t.Errorf("%s: err = %v; want %q", reason, err, want)
		}
		if got := AsError(err).Args["reason"]; got != reason {
			t.Errorf("%s: Args reason = %v", reason, got)
		}
	}
}

// The store is named as config.yaml names it, not as its store.yaml does.
func TestReadOnlyNamesTheStoreOfTheConfig(t *testing.T) {
	ts := newTestSession(t)
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	e, err := c.Select("me")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.RemoveStore("me"); err != nil {
		t.Fatal(err)
	}
	if err := c.AddStore("renamed", e.Path, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	makeReadOnly(t, ts, sync.ReasonGitHub)
	_, err = ts.Add(AddInput{Title: "x"})
	if err == nil || !strings.HasPrefix(err.Error(), "renamed is read-only") {
		t.Errorf("err = %v; want the name of the config", err)
	}
}

// What is written to access.json by a check that finds the store writable
// lets writes through again.
func TestWritableAgain(t *testing.T) {
	ts := newTestSession(t)
	makeReadOnly(t, ts, sync.ReasonGitHub)
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if err := sync.WriteAccess(s, sync.Access{Checked: ts.Now()}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Add(AddInput{Title: "x"}); err != nil {
		t.Errorf("Add after the permission came back: %v", err)
	}
}

// A sync of the background does not take back a push_denied; the one the
// user asks for does, and pushes.
func TestSyncExplicitClearsPushDeniedBackgroundDoesNot(t *testing.T) {
	ts := newTestSession(t)
	remote := addBareRemote(t, ts)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	unlock := gitrepotest.LockObjects(t, remote) // skips where there are no read-only directories
	reps, err := ts.Sync(false, true)
	if err == nil || !strings.Contains(err.Error(), "me is read-only (the remote denied the push)") || len(reps) != 1 {
		t.Fatalf("Sync = %+v, %v; want the push to be denied", reps, err)
	}
	st, err := ts.SyncStatus()
	if err != nil || len(st) != 1 || st[0].ReadOnly != "the remote denied the push" {
		t.Fatalf("SyncStatus = %+v, %v; want ReadOnly set", st, err)
	}
	unlock()
	// The directory is writable again, but a background sync leaves the denial.
	reps, err = ts.Sync(false, false)
	if err != nil || len(reps) != 1 || reps[0].Unpushable != 2 || len(reps[0].Pushed) != 0 {
		t.Fatalf("background Sync = %+v, %v; want 2 unpushable, nothing pushed", reps, err)
	}
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if a, _ := sync.ReadAccess(s); !a.ReadOnly || a.Reason != sync.ReasonPushDenied {
		t.Errorf("access after a background sync = %+v; want push_denied kept", a)
	}
	reps, err = ts.Sync(false, true)
	if err != nil || len(reps) != 1 || reps[0].Unpushable != 0 || len(reps[0].Pushed) == 0 {
		t.Fatalf("explicit Sync = %+v, %v; want a push", reps, err)
	}
	if a, _ := sync.ReadAccess(s); a.ReadOnly {
		t.Errorf("access after an explicit sync = %+v; want writable", a)
	}
	st, _ = ts.SyncStatus()
	if st[0].ReadOnly != "" {
		t.Errorf("SyncStatus ReadOnly = %q; want none", st[0].ReadOnly)
	}
}

// A store that is read-only for its permission reports the commits that
// cannot be pushed.
func TestSyncReportsUnpushable(t *testing.T) {
	ts := newTestSession(t)
	bare := addBareRemote(t, ts)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	// An explicit sync judges a read-only store again, so the bare is
	// really not writable.
	gitrepotest.LockObjects(t, bare)
	makeReadOnly(t, ts, sync.ReasonNotWritable)
	reps, err := ts.Sync(false, true)
	if err != nil || len(reps) != 1 || reps[0].Unpushable != 2 || len(reps[0].Pushed) != 0 { // the first commit of the store, and the task
		t.Fatalf("Sync = %+v, %v", reps, err)
	}
	st, err := ts.SyncStatus()
	if err != nil || st[0].ReadOnly != "the remote cannot be written" || st[0].Unpushed == nil || *st[0].Unpushed != 2 {
		t.Errorf("SyncStatus = %+v, %v", st, err)
	}
}

// Runs everywhere (no read-only directory): a store that access.json says is
// read-only by the GitHub permission, synced in the background (nothing judges
// it again), reports the commits it cannot push and the GitHub reason.
func TestBackgroundSyncReportsUnpushableByGitHubPermission(t *testing.T) {
	ts := newTestSession(t)
	addBareRemote(t, ts)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	makeReadOnly(t, ts, sync.ReasonGitHub)
	reps, err := ts.Sync(false, false)
	if err != nil || len(reps) != 1 || reps[0].Unpushable != 2 || len(reps[0].Pushed) != 0 {
		t.Fatalf("Sync = %+v, %v", reps, err)
	}
	st, err := ts.SyncStatus()
	if err != nil || st[0].ReadOnly != "the GitHub permission is read" || st[0].Unpushed == nil || *st[0].Unpushed != 2 {
		t.Errorf("SyncStatus = %+v, %v", st, err)
	}
}

func TestGitHubRepoOf(t *testing.T) {
	ts := newTestSession(t)
	ts.appendConfig("hosts:\n  - {host: ghe.example.com}\n" +
		"url_rewrites:\n  - {from: \"https://github.com/\", to: \"/srv/remotes/\"}\n" +
		"accounts:\n  - {name: me, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n")
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		remote      string
		owner, name string // empty: not GitHub
	}{
		{"https://github.com/org-a/x.git", "org-a", "x"}, // before url_rewrites: the rewrite makes it a local path
		{"git@github.com:org-a/x.git", "org-a", "x"},
		{"ssh://git@GitHub.com/org-a/x", "org-a", "x"},
		{"https://ghe.example.com/o/r.git", "o", "r"},
		{"https://gitlab.com/o/r.git", "", ""},
		{"/srv/remotes/org-a/x.git", "", ""},
	} {
		g := ts.githubRepoOf(c, withRemotes(c.Stores[0], tc.remote))
		if tc.owner == "" {
			if g != nil {
				t.Errorf("%s: %+v; want nil", tc.remote, g)
			}
			continue
		}
		if g == nil || g.Owner != tc.owner || g.Name != tc.name || g.AccountID != "github.com/alice" || g.AccountName != "me" {
			t.Errorf("%s: %+v; want %s/%s as me", tc.remote, g, tc.owner, tc.name)
		}
	}
	// No remote, and no account.
	e := c.Stores[0]
	if g := ts.githubRepoOf(c, e); g != nil {
		t.Errorf("no remote: %+v", g)
	}
	c.Accounts = nil
	if g := ts.githubRepoOf(c, withRemotes(e, "https://github.com/o/r.git")); g != nil {
		t.Errorf("no account: %+v", g)
	}
}

// The token of an account goes to its own host only.
func TestGitHubRepoOfAnotherHostOfTheAccount(t *testing.T) {
	ts := newTestSession(t)
	ts.env["HH_TOKEN"] = "t"
	ts.appendConfig("hosts:\n  - {host: ghe.example.com}\n" +
		"accounts:\n  - {name: me, host: github.com, login: alice, token: \"env:HH_TOKEN\"}\n")
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	g := ts.githubRepoOf(c, withRemotes(c.Stores[0], "https://ghe.example.com/o/r.git"))
	if g == nil {
		t.Fatal("nil")
	}
	if _, err := g.Client(); err == nil || AsError(err).Code != "account.host_mismatch" {
		t.Errorf("Client err = %v; want account.host_mismatch", err)
	}
}

// aliceStore makes alice's store "org" in the bare repository d/org-a/x.git,
// pushed. With old, the store is as one made before the merge driver is:
// without .gitattributes in HEAD.
func aliceStore(t *testing.T, d string, old bool) {
	t.Helper()
	alice := newTestSession(t)
	if _, err := alice.StoreInit(StoreInitInput{Name: "org", Prefix: "ORG", Remotes: []string{filepath.Join(d, "org-a", "x.git")}}); err != nil {
		t.Fatal(err)
	}
	if !old {
		return
	}
	alice.StoreName = "org"
	s, err := alice.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := alice.UpdateCommit(s, store.Op{Name: "edit"}, func(tx *store.Tx) error { return tx.RemoveFile(store.AttributesPath) }); err != nil {
		t.Fatal(err)
	}
	if err := s.Repo().Push("origin"); err != nil { // not a sync: it would set the store up again
		t.Fatal(err)
	}
}

// bobSession is a session of bob, who may only read org-a/x on GitHub, and
// the directory that serves https://github.com/ with alice's store "org" in
// org-a/x.
func bobSession(t *testing.T, token string, old bool) *testSession {
	t.Helper()
	d := t.TempDir()
	aliceStore(t, d, old)
	gitrepotest.InstallHTTPS(t, gitrepotest.Under("https://github.com/", d), nil, nil)
	bob, _ := fakeSession(t, fakegh.State{
		Tokens: map[string]string{"t-bob": "bob"},
		Repos:  []*fakegh.Repo{{Owner: "org-a", Name: "x", Perms: map[string]string{"bob": "read"}}},
	}, "accounts:\n  - {name: me, host: github.com, login: bob, token: \"env:HH_TOKEN\"}\n")
	bob.env["HH_TOKEN"] = token
	return bob
}

func TestStoreAddChecksAccessOnGitHub(t *testing.T) {
	bob := bobSession(t, "t-bob", false)
	res, err := bob.StoreAdd("org", []string{"https://github.com/org-a/x.git"}, "", "me")
	if err != nil {
		t.Fatal(err)
	}
	if b := configText(t, bob); !strings.Contains(b, "account: me") {
		t.Errorf("config.yaml = %s", b)
	}
	s, err := store.Open(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	if a, err := sync.ReadAccess(s); err != nil || !a.ReadOnly || a.Reason != sync.ReasonGitHub {
		t.Errorf("access = %+v, %v; want read-only by the GitHub permission", a, err)
	}
	bob.StoreName = "org"
	_, err = bob.Add(AddInput{Title: "x"})
	if err == nil || err.Error() != "org is read-only (the GitHub permission is read)" {
		t.Errorf("Add = %v", err)
	}
	wantErr(t, err, KindConflict, "store.read_only")
}

// A check that fails is a warning: the store is added.
func TestStoreAddAccessCheckFailureIsAWarning(t *testing.T) {
	bob := bobSession(t, "t-wrong", false)
	if _, err := bob.StoreAdd("org", []string{"https://github.com/org-a/x.git"}, "", "me"); err != nil {
		t.Fatalf("StoreAdd: %v; want the failed check to be a warning", err)
	}
	if !strings.Contains(strings.Join(bob.warns, "\n"), "org") {
		t.Errorf("warnings = %q; want one about org", bob.warns)
	}
	if !strings.Contains(configText(t, bob), "name: org") {
		t.Errorf("config.yaml lacks the store: %s", configText(t, bob))
	}
}

func TestStoreAddUnknownAccount(t *testing.T) {
	ts := newTestSession(t)
	remote := addBareRemote(t, ts)
	if _, err := ts.Sync(false, true); err != nil {
		t.Fatal(err)
	}
	ts.appendConfig("accounts:\n  - {name: a, host: github.com, login: alice}\n")
	_, err := ts.StoreAdd("copy", []string{remote}, "", "nope")
	wantErr(t, err, KindNotFound, "account.not_found")
	if !strings.Contains(err.Error(), "(accounts: a)") {
		t.Errorf("err = %v; want the accounts", err)
	}
	notExist(t, storeDirOf(t, ts, "copy"), "the clone")
}

// The check of a local remote is of whether it can be written, with or
// without an account.
func TestStoreAddChecksALocalRemote(t *testing.T) {
	ts := newTestSession(t)
	remote := addBareRemote(t, ts)
	if _, err := ts.Sync(false, true); err != nil {
		t.Fatal(err)
	}
	unlock := gitrepotest.LockObjects(t, remote)
	defer unlock()
	other := otherSession(t, ts, ts.Now())
	res, err := other.StoreAdd("copy", []string{remote}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	if a, _ := sync.ReadAccess(s); !a.ReadOnly || a.Reason != sync.ReasonNotWritable {
		t.Errorf("access = %+v; want not_writable", a)
	}
}

// The check runs under sync.lock: when it asks GitHub, another who wants the
// lock does not get it.
func TestStoreAddCheckTakesSyncLock(t *testing.T) {
	bob := bobSession(t, "t-bob", false)
	var lockErr error
	var asked bool
	bob.HTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if !asked && strings.Contains(r.URL.Path, "/repos/org-a/x") {
			asked = true
			s, err := store.Open(storeDirOf(t, bob, "org"))
			if err != nil {
				t.Error(err)
			} else {
				lockErr = s.WithLocalLock("sync.lock", 50*time.Millisecond, func() error { return nil })
			}
		}
		return http.DefaultTransport.RoundTrip(r)
	})}
	if _, err := bob.StoreAdd("org", []string{"https://github.com/org-a/x.git"}, "", "me"); err != nil {
		t.Fatal(err)
	}
	if !asked {
		t.Fatal("GitHub was not asked")
	}
	if !errors.Is(lockErr, store.ErrLocked) {
		t.Errorf("taking sync.lock while the check asked GitHub = %v; want ErrLocked", lockErr)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// A store made before the merge driver is added from a remote this machine
// cannot write to: it is read-only, and the setup commit stays here.
func TestStoreAddOldStoreReadOnlyOnGitHub(t *testing.T) {
	bob := bobSession(t, "t-bob", true)
	res, err := bob.StoreAdd("org", []string{"https://github.com/org-a/x.git"}, "", "me")
	if err != nil {
		t.Fatalf("StoreAdd: %v; want the read-only store to be added", err)
	}
	s, err := store.Open(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	if a, _ := sync.ReadAccess(s); !a.ReadOnly || a.Reason != sync.ReasonGitHub {
		t.Errorf("access = %+v; want read-only by the GitHub permission", a)
	}
	if n, err := Unpushed(s); err != nil || n != 1 {
		t.Errorf("Unpushed = %d, %v; want the setup commit", n, err)
	}
	if need, err := sync.NeedsSetup(s); err != nil || need {
		t.Errorf("NeedsSetup = %v, %v; want the setup done", need, err)
	}
}

func TestStoreAddOldStoreReadOnlyLocalBare(t *testing.T) {
	d := t.TempDir()
	aliceStore(t, d, true)
	unlock := gitrepotest.LockObjects(t, filepath.Join(d, "org-a", "x.git")) // skips where there are no read-only directories
	defer unlock()
	ts := newTestSession(t)
	res, err := ts.StoreAdd("org", []string{filepath.Join(d, "org-a", "x.git")}, "", "")
	if err != nil {
		t.Fatalf("StoreAdd: %v; want the read-only store to be added", err)
	}
	s, err := store.Open(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	if a, _ := sync.ReadAccess(s); !a.ReadOnly || a.Reason != sync.ReasonNotWritable {
		t.Errorf("access = %+v; want not_writable", a)
	}
	if n, err := Unpushed(s); err != nil || n != 1 {
		t.Errorf("Unpushed = %d, %v; want the setup commit", n, err)
	}
}

// A push the remote denies in the setup sync, for a store not known to be
// read-only before, makes it read-only and does not undo the add.
func TestStoreAddOldStorePushDeniedInTheSetup(t *testing.T) {
	d := t.TempDir()
	aliceStore(t, d, true)
	gitrepotest.InstallHTTPS(t, gitrepotest.Under("https://other.example.com/", d), nil, func(string) bool { return true })
	ts := newTestSession(t)
	res, err := ts.StoreAdd("org", []string{"https://other.example.com/org-a/x.git"}, "", "")
	if err != nil {
		t.Fatalf("StoreAdd: %v; want the add to stand", err)
	}
	s, err := store.Open(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	if a, _ := sync.ReadAccess(s); !a.ReadOnly || a.Reason != sync.ReasonPushDenied {
		t.Errorf("access = %+v; want push_denied", a)
	}
	if n, err := Unpushed(s); err != nil || n != 1 {
		t.Errorf("Unpushed = %d, %v; want the setup commit", n, err)
	}
	if !strings.Contains(strings.Join(ts.warns, "\n"), "org is read-only (the remote denied the push)") {
		t.Errorf("warnings = %q; want one that the push was denied", ts.warns)
	}
	if !strings.Contains(configText(t, ts), "name: org") {
		t.Errorf("the store is not in config.yaml: %s", configText(t, ts))
	}
}

// access.json that cannot be read refuses writes, and says what to do.
func TestBrokenAccessFileRefusesWrites(t *testing.T) {
	ts := newTestSession(t)
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(s.Dir(), store.LocalDir, "access.json")
	if err := s.WriteLocal("access.json", []byte("{not json")); err != nil {
		t.Fatal(err)
	}
	_, err = ts.Add(AddInput{Title: "x"})
	if err == nil || !strings.Contains(err.Error(), "delete "+p) {
		t.Errorf("Add = %v; want it to name %s", err, p)
	}
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Add(AddInput{Title: "x"}); err != nil {
		t.Errorf("Add after the file was deleted: %v", err)
	}
}

// A sync judges a broken access.json again, so that deleting it by hand is
// not the only way out; until then writes are refused with the path.
func TestSyncRepairsBrokenAccessFile(t *testing.T) {
	ts := newTestSession(t)
	addBareRemote(t, ts)
	s, err := ts.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(s.Dir(), store.LocalDir, "access.json")
	if err := s.WriteLocal("access.json", []byte("{not json")); err != nil {
		t.Fatal(err)
	}
	_, err = ts.Add(AddInput{Title: "x"})
	if err == nil || !strings.Contains(err.Error(), p) || !strings.Contains(err.Error(), "hedgehug sync") {
		t.Fatalf("Add before the sync = %v; want a refusal that names %s and the sync", err, p)
	}
	reps, err := ts.Sync(false, true)
	if err != nil || len(reps) != 1 {
		t.Fatalf("Sync = %+v, %v", reps, err)
	}
	if !slices.Contains(reps[0].Warnings, p+" is broken; judging access again") {
		t.Errorf("warnings = %q", reps[0].Warnings)
	}
	if a, err := sync.ReadAccess(s); err != nil || a.ReadOnly {
		t.Errorf("access after the sync = %+v, %v", a, err)
	}
	if _, err := ts.Add(AddInput{Title: "x"}); err != nil {
		t.Errorf("Add after the sync: %v", err)
	}
}

// A revert from the inbox writes too, through a session of its own on the
// store of the item.
func TestReadOnlyRefusesInboxRevert(t *testing.T) {
	ts := newTestSession(t)
	item, _ := overwriteME1(t, ts)
	makeReadOnly(t, ts, sync.ReasonGitHub)
	_, err := ts.InboxRevert(item.ULID.String())
	wantReadOnly(t, "InboxRevert", err)
}

// withRemotes is e with the remotes r.
func withRemotes(e config.StoreEntry, r ...string) config.StoreEntry {
	e.Remotes = r
	return e
}
