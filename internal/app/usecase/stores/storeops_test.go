package stores_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/automation"
	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/replication"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func TestStoreInitListRemove(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	res, err := stores.StoreInit(ts.Session, stores.StoreInitInput{Name: "team", Prefix: "TM"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Workspace.IDPrefix != "TM" || res.Workspace.Name != "team" || res.Name != "team" || len(res.Pushed) != 0 {
		t.Errorf("result = %+v (%+v)", res, res.Workspace)
	}
	if !strings.HasSuffix(res.Path, filepath.Join("stores", "team")) || !exists(res.Path) {
		t.Errorf("Path = %q; want the directory of the store under stores", res.Path)
	}
	if ts.StoreName != "" {
		t.Errorf("StoreInit assigned StoreName = %q to the session", ts.StoreName)
	}
	list, err := stores.StoreList(ts.Session)
	if err != nil || len(list) != 2 || list[0].Name != "me" || list[1].Name != "team" || list[1].Path != res.Path {
		t.Fatalf("StoreList = %+v, %v; want me and team", list, err)
	}
	if list[1].Remotes == nil || len(list[1].Remotes) != 0 || list[1].LastSuccess != nil || list[1].Account != "github.com/alice" {
		t.Errorf("the row of team = %+v; want empty remotes (not nil), no sync, the account github.com/alice", list[1])
	}
	rm, err := stores.StoreRemove(ts.Session, "team", true)
	if err != nil {
		t.Fatal(err)
	}
	if rm != (stores.StoreRemoveResult{Name: "team", Path: res.Path}) || !exists(res.Path) {
		t.Errorf("StoreRemove --keep = %+v; want the directory left", rm)
	}
	if list, _ := stores.StoreList(ts.Session); len(list) != 1 || list[0].Name != "me" {
		t.Errorf("StoreList after the removal = %+v", list)
	}
}

func TestStoreInitErrors(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	tests := []struct {
		name string
		in   stores.StoreInitInput
		kind usecase.Kind
		code string
		key  string
		arg  any
	}{
		{"bad store name", stores.StoreInitInput{Name: "bad/name", Prefix: "BN"}, usecase.KindUsage, "store.bad_name", "value", "bad/name"},
		{"bad prefix", stores.StoreInitInput{Name: "x", Prefix: "b d"}, usecase.KindUsage, "workspace.bad_prefix", "value", "b d"},
		{"bad workspace name", stores.StoreInitInput{Name: "x", Prefix: "XX", Workspace: " "}, usecase.KindUsage, "workspace.bad_name", "value", " "},
		{"already set up", stores.StoreInitInput{Name: "me", Prefix: "XX"}, usecase.KindConflict, "store.exists", "name", "me"},
		{"prefix of another store", stores.StoreInitInput{Name: "x", Prefix: "ME"}, usecase.KindConflict, "workspace.prefix_taken", "prefix", "ME"},
	}
	for _, tc := range tests {
		_, err := stores.StoreInit(ts.Session, tc.in)
		usecasetest.WantErr(t, err, tc.kind, tc.code)
		var e *usecase.Error
		if !usecasetest.AsError(err, &e) || e.Args[tc.key] != tc.arg {
			t.Errorf("%s: args = %v; want %s = %q", tc.name, e, tc.key, tc.arg)
		}
	}
	_, err := stores.StoreInit(ts.Session, stores.StoreInitInput{Name: "me", Prefix: "XX"})
	if want := `a store named "me" is already set up`; err == nil || err.Error() != want {
		t.Errorf("message = %v; want %q", err, want)
	}
	if list, _ := stores.StoreList(ts.Session); len(list) != 1 {
		t.Errorf("a refused init set up a store: %+v", list)
	}
}

func TestStoreInitWorkspaceNameAndSchema(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	res, err := stores.StoreInit(ts.Session, stores.StoreInitInput{Name: "team", Prefix: "TM", Workspace: "platform", Schema: schema.Dev()})
	if err != nil {
		t.Fatal(err)
	}
	if res.Workspace.Name != "platform" {
		t.Errorf("workspace name = %q; want platform", res.Workspace.Name)
	}
	sub := &usecase.Session{Getenv: ts.Getenv, Now: ts.Now, StoreName: "team"}
	s, err := sub.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Schema().Status("review"); !ok {
		t.Error("the dev template has no review status")
	}
}

func TestStoreInitWritesTheRulesInACommitOfOpInit(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	if _, err := stores.StoreInit(ts.Session, stores.StoreInitInput{Name: "team", Prefix: "TM", Schema: schema.Dev(), Automations: automation.DevRules()}); err != nil {
		t.Fatal(err)
	}
	sub := &usecase.Session{Getenv: ts.Getenv, Now: ts.Now, StoreName: "team"}
	s, err := sub.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	f, err := s.Automations()
	if err != nil || len(f.Rules) != 4 {
		t.Fatalf("Automations = %+v, %v", f, err)
	}
	head, err := s.Repo().Head()
	if err != nil {
		t.Fatal(err)
	}
	log, err := s.Repo().LogFrom(head, 1)
	if err != nil || len(log) != 1 {
		t.Fatalf("log = %+v, %v", log, err)
	}
	if op, _ := log[0].Trailer("Hedgehug-Op"); op != "init" {
		t.Errorf("newest commit op = %q, want init", op)
	}
}

func TestStoreInitPushesTheRulesToTheRemote(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	bare := filepath.Join(t.TempDir(), "rules.git")
	if err := gitrepo.InitBare(bare); err != nil {
		t.Fatal(err)
	}
	in := stores.StoreInitInput{Name: "team", Prefix: "TM", Schema: schema.Dev(), Automations: automation.DevRules(), Remotes: []string{bare}}
	if _, err := stores.StoreInit(ts.Session, in); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "clone")
	r, err := gitrepo.Clone(bare, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	b, err := os.ReadFile(filepath.Join(dir, model.AutomationsPath)) //nolint:gosec // a file of the test clone
	if err != nil {
		t.Fatalf("the remote has no %s: %v", model.AutomationsPath, err)
	}
	f, err := automation.Decode(b)
	if err != nil || len(f.Rules) != 4 {
		t.Errorf("rules at the remote = %+v, %v", f, err)
	}
}

func TestStoreInitPushesToRemotes(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	remote, mirror := filepath.Join(t.TempDir(), "one.git"), filepath.Join(t.TempDir(), "two.git")
	for _, r := range []string{remote, mirror} {
		if err := gitrepo.InitBare(r); err != nil {
			t.Fatal(err)
		}
	}
	res, err := stores.StoreInit(ts.Session, stores.StoreInitInput{Name: "org", Prefix: "ORG", Remotes: []string{remote, mirror}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(res.Remotes, []string{remote, mirror}) || !slices.Equal(res.Pushed, []string{"origin", "mirror1"}) {
		t.Errorf("Remotes %v, Pushed %v", res.Remotes, res.Pushed)
	}
	list, _ := stores.StoreList(ts.Session)
	if len(list) != 2 || !slices.Equal(list[1].Remotes, []string{remote, mirror}) || list[1].LastSuccess == nil || !list[1].LastSuccess.Equal(ts.Now()) {
		t.Errorf("StoreList = %+v; want org with its remotes and the time of its sync", list)
	}
}

func TestStoreInitIntoANonEmptyRemoteLeavesNothing(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	remote := usecasetest.AddBareRemote(t, ts)
	if _, err := replication.Sync(ts.Session, false, true); err != nil { // the remote now holds the store me
		t.Fatal(err)
	}
	_, err := stores.StoreInit(ts.Session, stores.StoreInitInput{Name: "org", Prefix: "ORG", Remotes: []string{remote}})
	usecasetest.WantErr(t, err, usecase.KindConflict, "store.remote_not_empty")
	if want := "the remote is not empty; to use the store there, run hedgehug store add org --remote " + remote; err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}
	_, p, _ := ts.Config()
	if exists(filepath.Join(p.StoresDir, "org")) {
		t.Error("the directory of the failed store is left")
	}
	if list, _ := stores.StoreList(ts.Session); len(list) != 1 {
		t.Errorf("StoreList = %+v; want only me", list)
	}
}

func TestStoreAddErrors(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	_, err := stores.StoreAdd(ts.Session, "bad/name", []string{"x"}, "", "")
	usecasetest.WantErr(t, err, usecase.KindUsage, "store.bad_name")
	_, err = stores.StoreAdd(ts.Session, "me", []string{"x"}, "", "")
	usecasetest.WantErr(t, err, usecase.KindConflict, "store.exists")
	if want := `a store named "me" is already set up`; err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}
	_, err = stores.StoreAdd(ts.Session, "x", nil, "", "")
	usecasetest.WantErr(t, err, usecase.KindUsage, "store.remote_required")
	if err.Error() != "a remote is needed" {
		t.Errorf("message = %q", err)
	}
	// A remote that is not there cannot be cloned; a remote without a store is no store.
	other := usecasetest.Other(t, ts, ts.Now())
	_, p, _ := other.Config()
	_, err = stores.StoreAdd(other.Session, "gone", []string{filepath.Join(t.TempDir(), "missing.git")}, "", "")
	usecasetest.WantErr(t, err, usecase.KindInvalid, "store.clone_failed")
	if exists(filepath.Join(p.StoresDir, "gone")) {
		t.Error("the failed clone left its directory")
	}
	empty := filepath.Join(t.TempDir(), "empty.git")
	if err := gitrepo.InitBare(empty); err != nil {
		t.Fatal(err)
	}
	_, err = stores.StoreAdd(other.Session, "empty", []string{empty}, "", "")
	if err == nil {
		t.Fatal("the clone of a remote without a store was added")
	}
	usecasetest.WantErr(t, err, usecase.KindInvalid, "store.clone_failed")
	// A remote with commits but no store.
	plain := filepath.Join(t.TempDir(), "plain.git")
	if err := gitrepo.InitBare(plain); err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	r, err := gitrepo.Init(work)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "x.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	author := config.CommitAuthor(usecasetest.TestAccount)
	if _, err := r.Commit(gitrepo.Commit{Paths: []string{"x.txt"}, Subject: "x", Author: author, When: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := r.SetRemotes([]string{plain}); err != nil {
		t.Fatal(err)
	}
	if err := r.Push("origin"); err != nil {
		t.Fatal(err)
	}
	_, err = stores.StoreAdd(other.Session, "plain", []string{plain}, "", "")
	usecasetest.WantErr(t, err, usecase.KindInvalid, "store.not_hedgehug")
	if !strings.HasPrefix(err.Error(), plain+" is not a hedgehug store: ") {
		t.Errorf("message = %q", err)
	}
	if exists(filepath.Join(p.StoresDir, "plain")) {
		t.Error("the add of a remote without a store left its directory")
	}
	if exists(filepath.Join(p.StoresDir, "empty")) {
		t.Error("the clone of a remote without a store left its directory")
	}
	if list, _ := stores.StoreList(other.Session); len(list) != 0 {
		t.Errorf("a failed add set up %+v", list)
	}
}

func TestStoreAddClonesToAPath(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	remote := usecasetest.AddBareRemote(t, ts)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := replication.Sync(ts.Session, false, true); err != nil {
		t.Fatal(err)
	}
	other := usecasetest.Other(t, ts, ts.Now())
	dir := filepath.Join(t.TempDir(), "here")
	res, err := stores.StoreAdd(other.Session, "copy", []string{remote}, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Name != "copy" || res.Path != dir || !slices.Equal(res.Remotes, []string{remote}) {
		t.Errorf("StoreAdd = %+v", res)
	}
	if _, ix, err := other.Open(); err != nil {
		t.Fatal(err)
	} else if e, err := other.Resolve(ix, "ME-1"); err != nil || e.Title != "a" {
		t.Errorf("ME-1 in the clone = %v, %v", e, err)
	}
	list, _ := stores.StoreList(other.Session)
	if len(list) != 1 || list[0].Path != dir || !slices.Equal(list[0].Remotes, []string{remote}) {
		t.Errorf("StoreList = %+v", list)
	}
	if _, err := stores.StoreAdd(other.Session, "again", []string{remote}, dir, ""); err == nil {
		t.Error("a clone into a directory that holds a store succeeded")
	}
	if !exists(filepath.Join(dir, "schema.yaml")) {
		t.Error("the failed second add cleared the directory of the first")
	}
}

// With two accounts a store cannot do without one: store add asks for it.
func TestStoreAddNeedsTheAccountWhenThereAreTwo(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	remote := usecasetest.AddBareRemote(t, ts)
	if _, err := replication.Sync(ts.Session, false, true); err != nil {
		t.Fatal(err)
	}
	other := usecasetest.Other(t, ts, ts.Now())
	other.AppendConfig("accounts:\n  - {name: a, host: github.com, login: alice}\n  - {name: b, host: github.com, login: bob}\n")
	_, err := stores.StoreAdd(other.Session, "copy", []string{remote}, "", "")
	usecasetest.WantErr(t, err, usecase.KindUsage, "choice.required")
	if _, err := stores.StoreAdd(other.Session, "copy", []string{remote}, "", "b"); err != nil {
		t.Fatal(err)
	}
	if list, _ := stores.StoreList(other.Session); len(list) != 1 || list[0].Account != "github.com/bob" {
		t.Errorf("StoreList = %+v; want the account github.com/bob", list)
	}
}

func TestWriteWithoutAnAccountIsRefused(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.DropDefaultAccount()
	_, err := tasks.Add(ts.Session, tasks.AddInput{Title: "x"})
	usecasetest.WantErr(t, err, usecase.KindConflict, "account.none")
}

func TestStoreAddWarnsOfPrefixDuplicates(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	remote := usecasetest.AddBareRemote(t, ts)
	if _, err := replication.Sync(ts.Session, false, true); err != nil {
		t.Fatal(err)
	}
	other := usecasetest.Other(t, ts, ts.Now())
	if _, err := stores.StoreInit(other.Session, stores.StoreInitInput{Name: "mine", Prefix: "ME"}); err != nil {
		t.Fatal(err)
	}
	if _, err := stores.StoreAdd(other.Session, "me", []string{remote}, "", ""); err != nil {
		t.Fatal(err)
	}
	// The workspace of mine was made at the zero time of its prefix, the shared one too:
	// on a tie the larger ULID is later. Only the shape of the warning is asserted here.
	if len(other.Warns) != 1 {
		t.Fatalf("warns = %q; want one", other.Warns)
	}
	w := other.Warns[0]
	if !strings.HasPrefix(w, "ME is also the id prefix of workspace ") || !strings.Contains(w, "; the one set later (workspace ") || !strings.HasSuffix(w, ") should change it with hedgehug workspace set-prefix") {
		t.Errorf("warning = %q", w)
	}
}

func TestWarnPrefixDuplicatesNamesTheLaterWorkspace(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	c, p, _ := ts.Config()
	// Two stores that share the prefix ME, the second set later (the store stamps a zero time with now).
	later := &model.Workspace{ULID: model.NewULID(), Name: "late", IDPrefix: "ME", PrefixSet: ts.Now().Add(time.Hour)}
	author := config.CommitAuthor(usecasetest.TestAccount)
	dir := filepath.Join(p.StoresDir, "late")
	s, err := store.Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: "late"}, schema.Default(), later, author, "", ts.Now)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddStore("late", dir, nil); err != nil {
		t.Fatal(err)
	}
	stores.WarnPrefixDuplicates(ts.Session, c, "late", s)
	want := "ME is also the id prefix of workspace me in store me; the one set later (workspace late in store late) should change it with hedgehug workspace set-prefix"
	if len(ts.Warns) != 1 || ts.Warns[0] != want {
		t.Errorf("warns = %q; want %q", ts.Warns, want)
	}
}

func TestSetLater(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)
	older, newer := &model.Workspace{PrefixSet: t0}, &model.Workspace{PrefixSet: t0.Add(time.Second)}
	if !stores.SetLater(newer, older) || stores.SetLater(older, newer) {
		t.Error("the prefix set at the later time is not the later one")
	}
	// Set at the same time, the larger ULID is the later one.
	small := &model.Workspace{ULID: model.NewULID(), PrefixSet: t0}
	large := &model.Workspace{ULID: model.NewULID(), PrefixSet: t0}
	if small.ULID.String() > large.ULID.String() {
		small, large = large, small
	}
	if !stores.SetLater(large, small) || stores.SetLater(small, large) {
		t.Error("the larger ULID is not the later one on a tie")
	}
	if stores.SetLater(small, small) {
		t.Error("a workspace is later than itself")
	}
}

func TestClearDir(t *testing.T) {
	t.Parallel()
	d := t.TempDir()
	keep := filepath.Join(d, "keep")
	if err := os.MkdirAll(filepath.Join(keep, "sub"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(keep, "sub", "f"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := stores.ClearDir(keep, true); err != nil {
		t.Fatal(err)
	}
	if es, _ := os.ReadDir(keep); !exists(keep) || len(es) != 0 {
		t.Errorf("a directory that existed must stay, emptied: %v", es)
	}
	if err := stores.ClearDir(keep, false); err != nil || exists(keep) {
		t.Errorf("a directory the command made must go: %v", err)
	}
}

func TestStoreListShowsAccountAndLastSync(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.UseAccount()
	usecasetest.AddBareRemote(t, ts)
	if _, err := replication.Sync(ts.Session, false, true); err != nil {
		t.Fatal(err)
	}
	list, err := stores.StoreList(ts.Session)
	if err != nil || len(list) != 1 {
		t.Fatalf("StoreList = %+v, %v", list, err)
	}
	if list[0].Account != "github.com/alice" || list[0].LastSuccess == nil || !list[0].LastSuccess.Equal(ts.Now()) || len(list[0].Remotes) != 1 {
		t.Errorf("row = %+v", list[0])
	}
}

func TestStoreRemoveNotFound(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	_, err := stores.StoreRemove(ts.Session, "nope", false)
	usecasetest.WantErr(t, err, usecase.KindNotFound, "store.not_found")
	if !strings.Contains(err.Error(), `no store is named "nope"`) {
		t.Errorf("message = %q; want it unchanged", err)
	}
}

func TestStoreRemoveRefusals(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	e := func() config.StoreEntry { _, e, _ := ts.Entry(); return e }()

	// No remote: the commits are only here.
	_, err := stores.StoreRemove(ts.Session, "me", false)
	usecasetest.WantErr(t, err, usecase.KindConflict, "store.unpushed")
	if want := "store me has no remote, so removing " + e.Path + " would lose its tasks for good; use --keep to take the store out of the config and leave the directory, or delete the directory yourself if you do not need it"; err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}

	usecasetest.AddBareRemote(t, ts)
	// A remote the store never pushed to: all its commits are unpushed.
	_, err = stores.StoreRemove(ts.Session, "me", false)
	usecasetest.WantErr(t, err, usecase.KindConflict, "store.unpushed")
	if !strings.Contains(err.Error(), "store me has ") || !strings.Contains(err.Error(), "not pushed; run hedgehug sync --store me first") || !strings.Contains(err.Error(), "--keep") {
		t.Errorf("message = %q", err)
	}

	if _, err := replication.Sync(ts.Session, false, true); err != nil {
		t.Fatal(err)
	}
	// Changes that are not committed.
	if err := os.WriteFile(filepath.Join(e.Path, "stray.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = stores.StoreRemove(ts.Session, "me", false)
	usecasetest.WantErr(t, err, usecase.KindConflict, "store.unpushed")
	if !strings.HasPrefix(err.Error(), "store me has changes that are not committed (") || !strings.Contains(err.Error(), "stray.txt") {
		t.Errorf("message = %q", err)
	}
	if err := os.Remove(filepath.Join(e.Path, "stray.txt")); err != nil {
		t.Fatal(err)
	}

	// A commit after the push.
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	_, err = stores.StoreRemove(ts.Session, "me", false)
	usecasetest.WantErr(t, err, usecase.KindConflict, "store.unpushed")
	if !strings.Contains(err.Error(), "run hedgehug sync --store me first") || !strings.Contains(err.Error(), "--keep") {
		t.Errorf("message = %q", err)
	}
	if list, _ := stores.StoreList(ts.Session); len(list) != 1 || !exists(e.Path) {
		t.Error("a refused removal removed the store")
	}

	// --keep takes the store out all the same, and checks nothing.
	res, err := stores.StoreRemove(ts.Session, "me", true)
	if err != nil || res != (stores.StoreRemoveResult{Name: "me", Path: e.Path}) || !exists(e.Path) {
		t.Errorf("StoreRemove --keep = %+v, %v; want the store out of the config and the directory left", res, err)
	}
}

func TestStoreRemoveOfAStoreThatDoesNotOpen(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	usecasetest.AddBareRemote(t, ts)
	_, e, _ := ts.Entry()
	if err := os.WriteFile(filepath.Join(e.Path, model.StoreMetaPath), []byte("not: [yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := stores.StoreRemove(ts.Session, "me", false)
	usecasetest.WantErr(t, err, usecase.KindConflict, "store.unpushed")
	if !strings.HasPrefix(err.Error(), "store me: ") || !strings.HasSuffix(err.Error(), "; delete the directory by hand if you mean to, or use --keep to take the store out of the config and leave it") {
		t.Errorf("message = %q", err)
	}
}

func TestStoreRemoveDeletesAPushedStore(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	usecasetest.AddBareRemote(t, ts)
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := replication.Sync(ts.Session, false, true); err != nil {
		t.Fatal(err)
	}
	_, e, _ := ts.Entry()
	res, err := stores.StoreRemove(ts.Session, "me", false)
	if err != nil {
		t.Fatal(err)
	}
	if res != (stores.StoreRemoveResult{Name: "me", Path: e.Path, Deleted: true}) || exists(e.Path) {
		t.Errorf("StoreRemove = %+v; want the directory deleted (%v)", res, exists(e.Path))
	}
	if list, _ := stores.StoreList(ts.Session); len(list) != 0 {
		t.Errorf("StoreList = %+v", list)
	}
}

// A directory that is not there has nothing to lose: the store is only taken
// out of the config, with or without a remote.
func TestStoreRemoveOfADirectoryThatIsGone(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	_, e, _ := ts.Entry()
	if err := os.RemoveAll(e.Path); err != nil {
		t.Fatal(err)
	}
	res, err := stores.StoreRemove(ts.Session, "me", false)
	if err != nil || res != (stores.StoreRemoveResult{Name: "me", Path: e.Path}) {
		t.Errorf("StoreRemove = %+v, %v; want the config changed and nothing deleted", res, err)
	}
	if list, _ := stores.StoreList(ts.Session); len(list) != 0 {
		t.Errorf("StoreList = %+v", list)
	}
}

// The store lock is held from the check until the directory is gone, so that
// nothing is written between the two: it is held when the config is written,
// which comes between them.
func TestStoreRemoveHoldsTheStoreLock(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	usecasetest.AddBareRemote(t, ts)
	if _, err := replication.Sync(ts.Session, false, true); err != nil {
		t.Fatal(err)
	}
	_, e, _ := ts.Entry()
	lock := filepath.Join(e.Path, ".hedgehug", "local", "lock")
	if usecasetest.HeldElsewhere(lock) {
		t.Fatal("the store is locked before the removal")
	}
	held := false
	ts.BeforeUpdate = func() { held = usecasetest.HeldElsewhere(lock) }
	if _, err := stores.StoreRemove(ts.Session, "me", false); err != nil {
		t.Fatal(err)
	}
	if !held {
		t.Error("the store lock was not held while the config was written")
	}
}
