package ops

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
	"github.com/kukv/hedgehug/internal/store"
)

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

func TestStoreInitListRemove(t *testing.T) {
	ts := newTestSession(t)
	res, err := ts.StoreInit(StoreInitInput{Name: "team", Prefix: "TM"})
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
	list, err := ts.StoreList()
	if err != nil || len(list) != 2 || list[0].Name != "me" || list[1].Name != "team" || list[1].Path != res.Path {
		t.Fatalf("StoreList = %+v, %v; want me and team", list, err)
	}
	if list[1].Remotes == nil || len(list[1].Remotes) != 0 || list[1].LastSuccess != nil || list[1].Account != "" {
		t.Errorf("the row of team = %+v; want empty remotes (not nil), no sync, no account", list[1])
	}
	rm, err := ts.StoreRemove("team", false)
	if err != nil {
		t.Fatal(err)
	}
	if rm != (StoreRemoveResult{Name: "team", Path: res.Path}) || !exists(res.Path) {
		t.Errorf("StoreRemove = %+v; want the directory left", rm)
	}
	if list, _ := ts.StoreList(); len(list) != 1 || list[0].Name != "me" {
		t.Errorf("StoreList after the removal = %+v", list)
	}
}

func TestStoreInitErrors(t *testing.T) {
	ts := newTestSession(t)
	tests := []struct {
		name string
		in   StoreInitInput
		kind Kind
		code string
		arg  any
	}{
		{"bad store name", StoreInitInput{Name: "bad/name", Prefix: "BN"}, KindUsage, "store.bad_name", "bad/name"},
		{"bad prefix", StoreInitInput{Name: "x", Prefix: "b d"}, KindUsage, "workspace.bad_prefix", "b d"},
		{"bad workspace name", StoreInitInput{Name: "x", Prefix: "XX", Workspace: " "}, KindUsage, "workspace.bad_name", " "},
		{"already set up", StoreInitInput{Name: "me", Prefix: "XX"}, KindConflict, "store.exists", "me"},
		{"prefix of another store", StoreInitInput{Name: "x", Prefix: "ME"}, KindConflict, "workspace.prefix_taken", "ME"},
	}
	for _, tc := range tests {
		_, err := ts.StoreInit(tc.in)
		wantErr(t, err, tc.kind, tc.code)
		var e *Error
		if asError(err, &e) {
			for _, v := range e.Args {
				if v != tc.arg {
					t.Errorf("%s: args = %v; want %v", tc.name, e.Args, tc.arg)
				}
			}
		}
	}
	_, err := ts.StoreInit(StoreInitInput{Name: "me", Prefix: "XX"})
	if want := `a store named "me" is already set up`; err == nil || err.Error() != want {
		t.Errorf("message = %v; want %q", err, want)
	}
	if list, _ := ts.StoreList(); len(list) != 1 {
		t.Errorf("a refused init set up a store: %+v", list)
	}
}

func TestStoreInitWorkspaceNameAndSchema(t *testing.T) {
	ts := newTestSession(t)
	res, err := ts.StoreInit(StoreInitInput{Name: "team", Prefix: "TM", Workspace: "platform", Schema: schema.Dev()})
	if err != nil {
		t.Fatal(err)
	}
	if res.Workspace.Name != "platform" {
		t.Errorf("workspace name = %q; want platform", res.Workspace.Name)
	}
	sub := &Session{Getenv: ts.Getenv, Now: ts.Now, StoreName: "team"}
	s, err := sub.OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Schema().Status("review"); !ok {
		t.Error("the dev template has no review status")
	}
}

func TestStoreInitPushesToRemotes(t *testing.T) {
	ts := newTestSession(t)
	remote, mirror := filepath.Join(t.TempDir(), "one.git"), filepath.Join(t.TempDir(), "two.git")
	for _, r := range []string{remote, mirror} {
		if err := gitrepo.InitBare(r); err != nil {
			t.Fatal(err)
		}
	}
	res, err := ts.StoreInit(StoreInitInput{Name: "org", Prefix: "ORG", Remotes: []string{remote, mirror}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(res.Remotes, []string{remote, mirror}) || !slices.Equal(res.Pushed, []string{"origin", "mirror1"}) {
		t.Errorf("Remotes %v, Pushed %v", res.Remotes, res.Pushed)
	}
	list, _ := ts.StoreList()
	if len(list) != 2 || !slices.Equal(list[1].Remotes, []string{remote, mirror}) || list[1].LastSuccess == nil || !list[1].LastSuccess.Equal(ts.Now()) {
		t.Errorf("StoreList = %+v; want org with its remotes and the time of its sync", list)
	}
}

func TestStoreInitIntoANonEmptyRemoteLeavesNothing(t *testing.T) {
	ts := newTestSession(t)
	remote := addBareRemote(t, ts)
	if _, err := ts.Sync(false); err != nil { // the remote now holds the store me
		t.Fatal(err)
	}
	_, err := ts.StoreInit(StoreInitInput{Name: "org", Prefix: "ORG", Remotes: []string{remote}})
	wantErr(t, err, KindConflict, "store.remote_not_empty")
	if want := "the remote is not empty; to use the store there, run hedgehug store add org --remote " + remote; err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}
	_, p, _ := ts.Config()
	if exists(filepath.Join(p.StoresDir, "org")) {
		t.Error("the directory of the failed store is left")
	}
	if list, _ := ts.StoreList(); len(list) != 1 {
		t.Errorf("StoreList = %+v; want only me", list)
	}
}

func TestStoreAddErrors(t *testing.T) {
	ts := newTestSession(t)
	_, err := ts.StoreAdd("bad/name", []string{"x"}, "")
	wantErr(t, err, KindUsage, "store.bad_name")
	_, err = ts.StoreAdd("me", []string{"x"}, "")
	wantErr(t, err, KindConflict, "store.exists")
	if want := `a store named "me" is already set up`; err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}
	// A remote that is not there cannot be cloned; a remote without a store is no store.
	other := otherSession(t, ts, ts.Now())
	_, p, _ := other.Config()
	_, err = other.StoreAdd("gone", []string{filepath.Join(t.TempDir(), "missing.git")}, "")
	wantErr(t, err, KindNotFound, "store.not_found")
	if exists(filepath.Join(p.StoresDir, "gone")) {
		t.Error("the failed clone left its directory")
	}
	empty := filepath.Join(t.TempDir(), "empty.git")
	if err := gitrepo.InitBare(empty); err != nil {
		t.Fatal(err)
	}
	_, err = other.StoreAdd("empty", []string{empty}, "")
	wantErr(t, err, KindNotFound, "store.not_found")
	if exists(filepath.Join(p.StoresDir, "empty")) {
		t.Error("the clone of a remote without a store left its directory")
	}
	if list, _ := other.StoreList(); len(list) != 0 {
		t.Errorf("a failed add set up %+v", list)
	}
}

func TestStoreAddClonesToAPath(t *testing.T) {
	ts := newTestSession(t)
	remote := addBareRemote(t, ts)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Sync(false); err != nil {
		t.Fatal(err)
	}
	other := otherSession(t, ts, ts.Now())
	dir := filepath.Join(t.TempDir(), "here")
	res, err := other.StoreAdd("copy", []string{remote}, dir)
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
	list, _ := other.StoreList()
	if len(list) != 1 || list[0].Path != dir || !slices.Equal(list[0].Remotes, []string{remote}) {
		t.Errorf("StoreList = %+v", list)
	}
	if _, err := other.StoreAdd("again", []string{remote}, dir); err == nil {
		t.Error("a clone into a directory that holds a store succeeded")
	}
	if !exists(filepath.Join(dir, "schema.yaml")) {
		t.Error("the failed second add cleared the directory of the first")
	}
}

func TestStoreAddWarnsOfPrefixDuplicates(t *testing.T) {
	ts := newTestSession(t)
	remote := addBareRemote(t, ts)
	if _, err := ts.Sync(false); err != nil {
		t.Fatal(err)
	}
	other := otherSession(t, ts, ts.Now())
	if _, err := other.StoreInit(StoreInitInput{Name: "mine", Prefix: "ME"}); err != nil {
		t.Fatal(err)
	}
	if _, err := other.StoreAdd("me", []string{remote}, ""); err != nil {
		t.Fatal(err)
	}
	// The workspace of mine was made at the zero time of its prefix, the shared one too:
	// on a tie the larger ULID is later. Only the shape of the warning is asserted here.
	if len(other.warns) != 1 {
		t.Fatalf("warns = %q; want one", other.warns)
	}
	w := other.warns[0]
	if !strings.HasPrefix(w, "ME is also the id prefix of workspace ") || !strings.Contains(w, "; the one set later (workspace ") || !strings.HasSuffix(w, ") should change it with hedgehug workspace prefix") {
		t.Errorf("warning = %q", w)
	}
}

func TestWarnPrefixDuplicatesNamesTheLaterWorkspace(t *testing.T) {
	ts := newTestSession(t)
	c, p, _ := ts.Config()
	// Two stores that share the prefix ME, the second set later (the store stamps a zero time with now).
	later := &model.Workspace{ULID: model.NewULID(), Name: "late", IDPrefix: "ME", PrefixSet: time.Now().Add(time.Hour)}
	author, _ := config.CommitAuthor(config.Account{}, false, ts.Getenv)
	dir := filepath.Join(p.StoresDir, "late")
	s, err := store.Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: "late"}, schema.Default(), later, author, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddStore("late", dir, nil); err != nil {
		t.Fatal(err)
	}
	ts.warnPrefixDuplicates(c, "late", s)
	want := "ME is also the id prefix of workspace me in store me; the one set later (workspace late in store late) should change it with hedgehug workspace prefix"
	if len(ts.warns) != 1 || ts.warns[0] != want {
		t.Errorf("warns = %q; want %q", ts.warns, want)
	}
}

func TestSetLater(t *testing.T) {
	t0 := time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)
	older, newer := &model.Workspace{PrefixSet: t0}, &model.Workspace{PrefixSet: t0.Add(time.Second)}
	if !setLater(newer, older) || setLater(older, newer) {
		t.Error("the prefix set at the later time is not the later one")
	}
	// Set at the same time, the larger ULID is the later one.
	small := &model.Workspace{ULID: model.NewULID(), PrefixSet: t0}
	large := &model.Workspace{ULID: model.NewULID(), PrefixSet: t0}
	if small.ULID.String() > large.ULID.String() {
		small, large = large, small
	}
	if !setLater(large, small) || setLater(small, large) {
		t.Error("the larger ULID is not the later one on a tie")
	}
	if setLater(small, small) {
		t.Error("a workspace is later than itself")
	}
}

func TestClearDir(t *testing.T) {
	d := t.TempDir()
	keep := filepath.Join(d, "keep")
	if err := os.MkdirAll(filepath.Join(keep, "sub"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(keep, "sub", "f"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := clearDir(keep, true); err != nil {
		t.Fatal(err)
	}
	if es, _ := os.ReadDir(keep); !exists(keep) || len(es) != 0 {
		t.Errorf("a directory that existed must stay, emptied: %v", es)
	}
	if err := clearDir(keep, false); err != nil || exists(keep) {
		t.Errorf("a directory the command made must go: %v", err)
	}
}

func TestStoreListShowsAccountAndLastSync(t *testing.T) {
	ts := newTestSession(t)
	ts.useAccount()
	addBareRemote(t, ts)
	if _, err := ts.Sync(false); err != nil {
		t.Fatal(err)
	}
	list, err := ts.StoreList()
	if err != nil || len(list) != 1 {
		t.Fatalf("StoreList = %+v, %v", list, err)
	}
	if list[0].Account != "github.com/alice" || list[0].LastSuccess == nil || !list[0].LastSuccess.Equal(ts.Now()) || len(list[0].Remotes) != 1 {
		t.Errorf("row = %+v", list[0])
	}
}

func TestStoreRemoveNotFound(t *testing.T) {
	ts := newTestSession(t)
	_, err := ts.StoreRemove("nope", false)
	wantErr(t, err, KindNotFound, "store.not_found")
	if !strings.Contains(err.Error(), `no store is named "nope"`) {
		t.Errorf("message = %q; want it unchanged", err)
	}
}

func TestStoreRemovePurgeRefusals(t *testing.T) {
	ts := newTestSession(t)
	e := func() config.StoreEntry { _, e, _ := ts.Entry(); return e }()

	// No remote: the commits are only here.
	_, err := ts.StoreRemove("me", true)
	wantErr(t, err, KindConflict, "store.unpushed")
	if want := "store me has no remote, so its commits are only in " + e.Path + "; delete the directory by hand if you mean to lose them"; err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}

	remote := addBareRemote(t, ts)
	_ = remote
	// A remote the store never pushed to: all its commits are unpushed.
	_, err = ts.StoreRemove("me", true)
	wantErr(t, err, KindConflict, "store.unpushed")
	if !strings.Contains(err.Error(), "store me has ") || !strings.HasSuffix(err.Error(), " commits not pushed; run hedgehug sync --store me first") {
		t.Errorf("message = %q", err)
	}

	if _, err := ts.Sync(false); err != nil {
		t.Fatal(err)
	}
	// Changes that are not committed.
	if err := os.WriteFile(filepath.Join(e.Path, "stray.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = ts.StoreRemove("me", true)
	wantErr(t, err, KindConflict, "store.unpushed")
	if !strings.HasPrefix(err.Error(), "store me has changes that are not committed (") || !strings.Contains(err.Error(), "stray.txt") {
		t.Errorf("message = %q", err)
	}
	if err := os.Remove(filepath.Join(e.Path, "stray.txt")); err != nil {
		t.Fatal(err)
	}

	// A commit after the push.
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	_, err = ts.StoreRemove("me", true)
	wantErr(t, err, KindConflict, "store.unpushed")
	if want := "store me has 1 commits not pushed; run hedgehug sync --store me first"; err.Error() != want {
		t.Errorf("message = %q; want %q", err, want)
	}
	if list, _ := ts.StoreList(); len(list) != 1 || !exists(e.Path) {
		t.Error("a refused purge removed the store")
	}
}

func TestStoreRemovePurgeOfAStoreThatDoesNotOpen(t *testing.T) {
	ts := newTestSession(t)
	addBareRemote(t, ts)
	_, e, _ := ts.Entry()
	if err := os.WriteFile(filepath.Join(e.Path, model.StoreMetaPath), []byte("not: [yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ts.StoreRemove("me", true)
	wantErr(t, err, KindConflict, "store.unpushed")
	if !strings.HasPrefix(err.Error(), "store me: ") || !strings.HasSuffix(err.Error(), "; delete the directory by hand if you mean to") {
		t.Errorf("message = %q", err)
	}
}

func TestStoreRemovePurgeDeletesAPushedStore(t *testing.T) {
	ts := newTestSession(t)
	addBareRemote(t, ts)
	if _, err := ts.Add(AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Sync(false); err != nil {
		t.Fatal(err)
	}
	_, e, _ := ts.Entry()
	res, err := ts.StoreRemove("me", true)
	if err != nil {
		t.Fatal(err)
	}
	if res != (StoreRemoveResult{Name: "me", Path: e.Path, Purged: true}) || exists(e.Path) {
		t.Errorf("StoreRemove = %+v; want the directory deleted (%v)", res, exists(e.Path))
	}
	if list, _ := ts.StoreList(); len(list) != 0 {
		t.Errorf("StoreList = %+v", list)
	}
}
