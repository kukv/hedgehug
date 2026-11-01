package ops

import (
	"testing"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"

	hhctx "github.com/kukv/hedgehug/internal/context"
	"github.com/kukv/hedgehug/internal/model"
)

// repoDir makes a git repository on the branch ("" keeps master) whose
// origin is url ("" for none).
func repoDir(t *testing.T, branch, url string) string {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if url != "" {
		if _, err := repo.CreateRemote(&gitconfig.RemoteConfig{Name: "origin", URLs: []string{url}}); err != nil {
			t.Fatal(err)
		}
	}
	if branch != "" {
		if err := repo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.NewBranchReferenceName(branch))); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// reposOf is the repos of the task ulid in the store name.
func reposOf(t *testing.T, ts *testSession, name string, ulid model.ULID) []string {
	t.Helper()
	s, err := ts.ForStore(name).OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	task, err := s.Task(ulid)
	if err != nil {
		t.Fatal(err)
	}
	return task.Repos
}

func TestAddWithStoreAndRepos(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSession(t)
	r, err := ts.Add(AddInput{Store: "org-b", Title: "Elsewhere", Repos: []string{"github.com/o/r"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := reposOf(t, ts, "org-b", r.ULID); len(got) != 1 || got[0] != "github.com/o/r" {
		t.Errorf("repos = %q", got)
	}
	// a session on --store org-a cannot be told to add to org-b
	_, err = ts.ForStore("org-a").Add(AddInput{Store: "org-b", Title: "x"})
	wantCode(t, "Add(--store org-a, Store org-b)", err, KindUsage, "store.conflict")
	// Parent goes through ResolveRef: a task of another store is refused
	_, err = ts.Add(AddInput{Store: "org-a", Title: "x", Parent: "WEB-1"})
	wantCode(t, "Add(parent in another store)", err, KindInvalid, "ref.cross_store")
}

func TestAddHereExplicitParentInAnotherStore(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSession(t)
	r, cf, ch, err := ts.AddHere(AddHereInput{Context: hhctx.Input{Parent: "WEB-1"}, Task: AddInput{Title: "child"}})
	if err != nil || cf != nil || ch != nil {
		t.Fatalf("AddHere = %v %v %v", cf, ch, err)
	}
	d := r.Destination
	if d.Stage != 1 || d.Store.Name != "org-b" || d.Parent == nil || d.Parent.ID != "WEB-1" {
		t.Errorf("destination = %+v", d)
	}
	v, err := r.View()
	if err != nil || v.Store != "org-b" || v.Parent == nil || v.Title != "child" {
		t.Errorf("view = %+v, %v", v, err)
	}
}

func TestAddHereRefusesAReadOnlyStore(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSession(t)
	makeStoreReadOnly(t, ts, "org-a")
	makeStoreReadOnly(t, ts, "org-b")
	_, _, _, err := ts.AddHere(AddHereInput{Context: hhctx.Input{Store: "org-a"}, Task: AddInput{Title: "x"}})
	wantCode(t, "AddHere(read-only)", err, KindConflict, "store.read_only")
	_, _, _, err = ts.AddHere(AddHereInput{Context: hhctx.Input{Parent: "WEB-1"}, Task: AddInput{Title: "x"}})
	wantCode(t, "AddHere(read-only, by parent)", err, KindConflict, "store.read_only")
}

func TestAddHerePutsTheRepoOnTheTask(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSession(t)
	dir := repoDir(t, "", "https://github.com/Org-A/Web.git")
	r, _, _, err := ts.AddHere(AddHereInput{Context: hhctx.Input{Dir: dir, Store: "org-a"}, Task: AddInput{Title: "linked"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := reposOf(t, ts, "org-a", r.ULID); len(got) != 1 || got[0] != "github.com/org-a/web" {
		t.Errorf("repos = %q; want the repo ID", got)
	}
	r, _, _, err = ts.AddHere(AddHereInput{Context: hhctx.Input{Dir: dir, Store: "org-a"}, NoRepo: true, Task: AddInput{Title: "unlinked"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := reposOf(t, ts, "org-a", r.ULID); len(got) != 0 {
		t.Errorf("--no-repo: repos = %q", got)
	}
	// no repository, no link
	r, _, _, err = ts.AddHere(AddHereInput{Context: hhctx.Input{Dir: t.TempDir(), Store: "org-a"}, Task: AddInput{Title: "bare"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := reposOf(t, ts, "org-a", r.ULID); len(got) != 0 {
		t.Errorf("no repository: repos = %q", got)
	}
}

func TestAddHereStage4SetsRecent(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSession(t)
	p, err := ts.ForStore("org-a").Add(AddInput{Title: "Web release", Kind: "project", Repos: []string{"github.com/org-a/web"}})
	if err != nil {
		t.Fatal(err)
	}
	dir := repoDir(t, "", "https://github.com/org-a/web")
	r, _, _, err := ts.AddHere(AddHereInput{Context: hhctx.Input{Dir: dir}, Task: AddInput{Title: "in the project"}})
	if err != nil {
		t.Fatal(err)
	}
	if r.Destination.Stage != 4 || r.Destination.Parent.ULID != p.ULID {
		t.Fatalf("destination = %+v", r.Destination)
	}
	m, _ := ts.Mount()
	a, _ := m.Get("org-a")
	got := loadContext(t, ts).Recent["github.com/org-a/web"]
	if got.Store != a.ULID || got.Project != p.ULID {
		t.Errorf("recent = %+v; want org-a's %s", got, p.ULID)
	}
	if reposOf(t, ts, "org-a", r.ULID)[0] != "github.com/org-a/web" {
		t.Errorf("the task has no repo")
	}
	// recent is not set by a stage that is not 4
	_, _, _, err = ts.AddHere(AddHereInput{Context: hhctx.Input{Dir: dir, Store: "org-a"}, Task: AddInput{Title: "x"}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAddHereReturnsWhatToAsk(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSession(t)
	setDefaultStore(t, ts, "")
	dir := repoDir(t, "hh/WEB-1-login", "")
	_, cf, _, err := ts.AddHere(AddHereInput{Context: hhctx.Input{Dir: dir, Interactive: true}, Task: AddInput{Title: "x"}})
	if err != nil || cf == nil || cf.Parent.ID != "WEB-1" {
		t.Fatalf("Confirm = %+v, %v", cf, err)
	}
	r, _, _, err := ts.AddHere(AddHereInput{Context: hhctx.Input{Dir: dir, Interactive: true, BranchParent: hhctx.Yes}, Task: AddInput{Title: "x"}})
	if err != nil || r.Destination.Stage != 3 || r.Destination.Store.Name != "org-b" {
		t.Fatalf("Yes = %+v, %v", r.Destination, err)
	}
	_, _, ch, err := ts.AddHere(AddHereInput{Context: hhctx.Input{Interactive: true}, Task: AddInput{Title: "x"}})
	if err != nil || ch == nil || ch.What != "store" {
		t.Fatalf("Choice = %+v, %v", ch, err)
	}
	r, _, _, err = ts.AddHere(AddHereInput{Context: hhctx.Input{Interactive: true, PickStore: "org-a"}, Task: AddInput{Title: "x"}})
	if err != nil || r.Destination.Stage != 5 || r.Destination.Store.Name != "org-a" {
		t.Fatalf("PickStore = %+v, %v", r.Destination, err)
	}
}

func TestAddHereUsesDefaultStoreAndTheGlobalStore(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSession(t)
	setDefaultStore(t, ts, "org-b")
	r, _, _, err := ts.AddHere(AddHereInput{Task: AddInput{Title: "x"}})
	if err != nil || r.Destination.Store.Name != "org-b" || r.Destination.Stage != 5 {
		t.Fatalf("default_store: %+v, %v", r.Destination, err)
	}
	r, _, _, err = ts.ForStore("org-a").AddHere(AddHereInput{Task: AddInput{Title: "x"}})
	if err != nil || r.Destination.Store.Name != "org-a" || r.Destination.Stage != 1 {
		t.Fatalf("--store org-a: %+v, %v", r.Destination, err)
	}
	_, _, _, err = ts.AddHere(AddHereInput{Context: hhctx.Input{Store: "nosuch"}, Task: AddInput{Title: "x"}})
	if err == nil {
		t.Error("--store nosuch: no error")
	}
}

func TestAddHereErrors(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSession(t)
	setDefaultStore(t, ts, "")
	pid := addProjectIn(t, ts, "org-a", "Release")
	cases := []struct {
		name string
		in   hhctx.Input
		kind Kind
		code string
	}{
		{"parent and project", hhctx.Input{Parent: "API-1", Project: pid}, KindUsage, "add.parent_and_project"},
		{"not a project", hhctx.Input{Project: "API-1"}, KindInvalid, "add.not_project"},
		{"no such parent", hhctx.Input{Parent: "API-99"}, KindNotFound, "task.not_found"},
		{"no such workspace", hhctx.Input{Workspace: "nosuch"}, KindNotFound, "workspace.not_found"},
		{"no store chosen", hhctx.Input{}, KindUsage, "choice.required"},
	}
	for _, c := range cases {
		_, _, _, err := ts.AddHere(AddHereInput{Context: c.in, Task: AddInput{Title: "x"}})
		wantCode(t, c.name, err, c.kind, c.code)
	}
	if e := AsError(func() error {
		_, _, _, err := ts.AddHere(AddHereInput{Task: AddInput{Title: "x"}})
		return err
	}()); e == nil || e.Args["what"] != "store" {
		t.Errorf("choice.required Args = %+v", e)
	}
}

// setDefaultStore makes name the default store in config.yaml; "" removes it
// (the first store added is the default).
func setDefaultStore(t *testing.T, ts *testSession, name string) {
	t.Helper()
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.SetDefaultStore(name); err != nil {
		t.Fatal(err)
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
}
