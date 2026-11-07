package usecase_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/remote/replicate"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestResolveAmbiguousIsKindAmbiguous(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	a := ts.AddTask("first", nil)
	b := ts.AddTask("second", func(x *model.Task) { x.ID = "ME-1" })
	if a.ID != "ME-1" || b.ID != "ME-1" {
		t.Fatalf("ids = %q, %q; want both ME-1", a.ID, b.ID)
	}
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	_, err = ts.Resolve(ix, "ME-1")
	if usecase.KindOf(err) != usecase.KindAmbiguous {
		t.Fatalf("KindOf(%v) = %v; want KindAmbiguous", err, usecase.KindOf(err))
	}
	want := `"ME-1" matches 2 tasks:` +
		"\n  ME-1 (" + a.ULID.String() + ") first" +
		"\n  ME-1 (" + b.ULID.String() + ") second"
	if err.Error() != want {
		t.Errorf("Error() = %q; want %q", err.Error(), want)
	}
}

func TestResolveULID(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	a := ts.AddTask("first", nil)
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ts.ResolveULID(ix)(a.ID)
	if err != nil || got != a.ULID {
		t.Errorf("ResolveULID(%s) = %v, %v; want %v", a.ID, got, err, a.ULID)
	}
	if _, err := ts.ResolveULID(ix)("ME-99"); usecase.KindOf(err) != usecase.KindNotFound {
		t.Errorf("ResolveULID(ME-99) error %v; want KindNotFound", err)
	}
}

func TestWriterIsReadOnce(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AppendConfig(usecasetest.AliceAccount)
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
	if err := os.WriteFile(p.ConfigFile, []byte(strings.Replace(string(b), usecasetest.AliceAccount, "\n", 1)), 0o600); err != nil {
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
	t.Parallel()
	ts := usecasetest.New(t)
	ts.DropDefaultAccount()
	if id, err := ts.AccountID(); err != nil || id != "" {
		t.Errorf("AccountID = %q, %v; want empty, nil", id, err)
	}
}

func TestMarkWrittenWarnsWhenPendingFails(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	s, _, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	// a directory where sync.json should be makes MarkPending fail
	if err := os.MkdirAll(filepath.Join(s.Dir(), filepath.FromSlash(store.LocalDir), "sync.json"), 0o750); err != nil {
		t.Fatal(err)
	}
	var after int
	ts.AfterWrite = func(string) { after++ }
	ts.MarkWritten(s)
	if len(ts.Warns) != 1 || !strings.HasPrefix(ts.Warns[0], "the write is not marked for sync: ") {
		t.Errorf("warns = %q", ts.Warns)
	}
	if after != 0 {
		t.Errorf("AfterWrite called %d times after a failed mark; want 0", after)
	}
}

func TestMarkWrittenWithoutAfterWriteMarksOnly(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	s, _, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	ts.MarkWritten(s)
	st, err := replicate.ReadState(s)
	if err != nil || st.PendingSince.IsZero() {
		t.Errorf("state = %+v, %v; want pending", st, err)
	}
	if len(ts.Warns) != 0 {
		t.Errorf("warns = %q", ts.Warns)
	}
}

func TestWarnNilIsNoop(t *testing.T) {
	t.Parallel()
	(&usecase.Session{}).AddWarning("x")
}

func TestQueryOptions(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AppendConfig(usecasetest.AliceAccount)
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
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AppendConfig(usecasetest.AliceAccount)
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
	t.Parallel()
	ts := usecasetest.New(t)
	ts.StoreName = "nope"
	_, _, err := ts.Entry()
	usecasetest.WantErr(t, err, usecase.KindNotFound, "store.not_found")
	if err != nil && !strings.HasPrefix(err.Error(), `no store is named "nope"`) {
		t.Errorf("message = %q", err)
	}
	_, err = ts.OpenStore()
	usecasetest.WantErr(t, err, usecase.KindNotFound, "store.not_found")

	// Several stores and none named: KindInvalid, not KindUsage (exit 1).
	ts.StoreName = ""
	c, p, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddStore("two", filepath.Join(p.StoresDir, "two"), nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	_, _, err = ts.Entry()
	usecasetest.WantErr(t, err, usecase.KindInvalid, "store.choose")
	if err != nil && !strings.HasPrefix(err.Error(), "several stores are set up; pick one with --store") {
		t.Errorf("message = %q", err)
	}
}

// A store that was open when a rebuild stopped in its apply phase is not
// synced, and the sync leaves no mark in sync.json.
func TestRunSyncRefusesAStoppedApply(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
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
	if err := s.WriteLocal(store.RebuildFile, []byte(`{"phase":"apply"}`+"\n")); err != nil {
		t.Fatal(err)
	}
	_, runErr := ts.RunSync(s, opt, true)
	var left *store.LeftoverError
	if !errors.As(runErr, &left) {
		t.Fatalf("RunSync = %v, want a LeftoverError", runErr)
	}
	st, err := replicate.ReadState(s)
	if err != nil {
		t.Fatal(err)
	}
	if !st.LastAttempt.IsZero() {
		t.Errorf("RunSync recorded an attempt: %v", st.LastAttempt)
	}
}

func TestRunSyncWithoutRemotes(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
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

	st, err := replicate.ReadState(s)
	if err != nil {
		t.Fatal(err)
	}
	if st.LastAttempt.IsZero() {
		t.Error("RunSync did not record the attempt")
	}
	// With no remote there is nothing to sync: it succeeds and says so.
	if runErr != nil {
		t.Errorf("RunSync without remotes = %v; want nil", runErr)
	}
	if st.LastError != "" || st.ErrorReported {
		t.Errorf("state after RunSync without remotes: LastError %q, ErrorReported %v; want none", st.LastError, st.ErrorReported)
	}
	if !st.LastSuccess.Equal(ts.Now()) {
		t.Errorf("LastSuccess = %v; want %v", st.LastSuccess, ts.Now())
	}
}
