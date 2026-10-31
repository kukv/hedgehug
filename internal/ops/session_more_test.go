package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

const aliceAccount = "\naccounts:\n  - name: alice\n    host: github.com\n    login: alice\n"

func TestResolveAmbiguousIsKindAmbiguous(t *testing.T) {
	ts := newTestSession(t)
	a := ts.addTask("first", nil)
	b := ts.addTask("second", func(x *model.Task) { x.ID = "ME-1" })
	if a.ID != "ME-1" || b.ID != "ME-1" {
		t.Fatalf("ids = %q, %q; want both ME-1", a.ID, b.ID)
	}
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	_, err = ts.Resolve(ix, "ME-1")
	if KindOf(err) != KindAmbiguous {
		t.Fatalf("KindOf(%v) = %v; want KindAmbiguous", err, KindOf(err))
	}
	want := `"ME-1" matches 2 tasks:` +
		"\n  ME-1 (" + a.ULID.String() + ") first" +
		"\n  ME-1 (" + b.ULID.String() + ") second"
	if err.Error() != want {
		t.Errorf("Error() = %q; want %q", err.Error(), want)
	}
}

func TestResolveULID(t *testing.T) {
	ts := newTestSession(t)
	a := ts.addTask("first", nil)
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ts.ResolveULID(ix)(a.ID)
	if err != nil || got != a.ULID {
		t.Errorf("ResolveULID(%s) = %v, %v; want %v", a.ID, got, err, a.ULID)
	}
	if _, err := ts.ResolveULID(ix)("ME-99"); KindOf(err) != KindNotFound {
		t.Errorf("ResolveULID(ME-99) error %v; want KindNotFound", err)
	}
}

func TestWriterIsReadOnce(t *testing.T) {
	ts := newTestSession(t)
	ts.appendConfig(aliceAccount)
	acct, ok, err := ts.Writer()
	if err != nil || !ok || acct.ID() != "github.com/alice" {
		t.Fatalf("Writer = %v, %v, %v; want github.com/alice", acct, ok, err)
	}
	_, p, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.ConfigFile, []byte(strings.Replace(string(b), aliceAccount, "\n", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if c, _, err := ts.Config(); err != nil || len(c.Accounts) != 0 {
		t.Fatalf("config after the edit: %v accounts, %v", c.Accounts, err)
	}
	acct, ok, err = ts.Writer()
	if err != nil || !ok || acct.ID() != "github.com/alice" {
		t.Errorf("Writer after the edit = %v, %v, %v; want the cached account", acct, ok, err)
	}
	if id, err := ts.AccountID(); err != nil || id != "github.com/alice" {
		t.Errorf("AccountID = %q, %v", id, err)
	}
}

func TestAccountIDWithoutAccountIsEmpty(t *testing.T) {
	ts := newTestSession(t)
	if id, err := ts.AccountID(); err != nil || id != "" {
		t.Errorf("AccountID = %q, %v; want empty, nil", id, err)
	}
}

func TestMarkWrittenWarnsWhenPendingFails(t *testing.T) {
	ts := newTestSession(t)
	s, _, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	// a directory where sync.json should be makes MarkPending fail
	if err := os.MkdirAll(filepath.Join(s.Dir(), filepath.FromSlash(store.LocalDir), "sync.json"), 0o750); err != nil {
		t.Fatal(err)
	}
	var after int
	ts.AfterWrite = func(*store.Store, config.StoreEntry) { after++ }
	ts.markWritten(s)
	if len(ts.warns) != 1 || !strings.HasPrefix(ts.warns[0], "the write is not marked for sync: ") {
		t.Errorf("warns = %q", ts.warns)
	}
	if after != 0 {
		t.Errorf("AfterWrite called %d times after a failed mark; want 0", after)
	}
}

func TestMarkWrittenWithoutAfterWriteMarksOnly(t *testing.T) {
	ts := newTestSession(t)
	s, _, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	ts.markWritten(s)
	st, err := sync.ReadState(s)
	if err != nil || st.PendingSince.IsZero() {
		t.Errorf("state = %+v, %v; want pending", st, err)
	}
	if len(ts.warns) != 0 {
		t.Errorf("warns = %q", ts.warns)
	}
}

func TestWarnNilIsNoop(t *testing.T) {
	(&Session{}).warn("x")
}

func TestQueryOptions(t *testing.T) {
	ts := newTestSession(t)
	ts.appendConfig(aliceAccount)
	s, _, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	opt, err := ts.QueryOptions(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(opt.Me) != 1 || opt.Me[0] != "github.com/alice" {
		t.Errorf("Me = %v", opt.Me)
	}
	if opt.Overwritten == nil || len(opt.Overwritten) != 0 {
		t.Errorf("Overwritten = %v; want empty, not nil", opt.Overwritten)
	}
}

func TestSyncOptions(t *testing.T) {
	ts := newTestSession(t)
	ts.appendConfig(aliceAccount)
	c, p, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(p.StoresDir, "other")
	if err := c.AddStore("other", other, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	ts.StoreName = "me" // two stores now
	c, e, err := ts.Entry()
	if err != nil {
		t.Fatal(err)
	}
	e.Remotes = []string{"/some/remote"}
	opt, err := ts.SyncOptions(c, e)
	if err != nil {
		t.Fatal(err)
	}
	if opt.Account != "github.com/alice" {
		t.Errorf("Account = %q", opt.Account)
	}
	if opt.Author.Name != "alice" {
		t.Errorf("Author = %+v; want the account's login", opt.Author)
	}
	if len(opt.Remotes) != 1 || opt.Remotes[0] != "/some/remote" {
		t.Errorf("Remotes = %v", opt.Remotes)
	}
	if len(opt.Me) != 1 || opt.Me[0] != "github.com/alice" {
		t.Errorf("Me = %v", opt.Me)
	}
	if len(opt.Others) != 1 || opt.Others[0].Name != "other" || opt.Others[0].Path != other {
		t.Errorf("Others = %+v; want only the other store", opt.Others)
	}
	if opt.Now == nil || !opt.Now().Equal(ts.Now()) {
		t.Error("Now is not the session's")
	}
}

func TestEntryErrors(t *testing.T) {
	ts := newTestSession(t)
	ts.StoreName = "nope"
	if _, _, err := ts.Entry(); err == nil {
		t.Error("Entry with an unknown store did not fail")
	}
	if _, err := ts.OpenStore(); err == nil {
		t.Error("OpenStore with an unknown store did not fail")
	}
}

func TestUnpushedWithoutRemote(t *testing.T) {
	ts := newTestSession(t)
	s, _, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	n, err := Unpushed(s)
	if err != nil || n != 1 {
		t.Errorf("Unpushed = %d, %v; want 1 (the first commit), nil", n, err)
	}
}

func TestRunSyncWithoutRemotes(t *testing.T) {
	ts := newTestSession(t)
	s, _, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	c, e, err := ts.Entry()
	if err != nil {
		t.Fatal(err)
	}
	opt, err := ts.SyncOptions(c, e)
	if err != nil {
		t.Fatal(err)
	}
	_, runErr := ts.RunSync(s, opt, true)
	st, err := sync.ReadState(s)
	if err != nil {
		t.Fatal(err)
	}
	if st.LastAttempt.IsZero() {
		t.Error("RunSync did not record the attempt")
	}
	if (runErr != nil) != (st.LastError != "") {
		t.Errorf("run error %v, LastError %q: they must agree", runErr, st.LastError)
	}
	if runErr != nil && !st.ErrorReported {
		t.Error("ErrorReported is false for reported = true")
	}
}
