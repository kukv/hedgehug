package tasks_test

import (
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
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
func reposOf(t *testing.T, ts *usecasetest.Session, name string, ulid model.ULID) []string {
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
	ts := usecasetest.NewTwoStore(t)
	r, err := tasks.Add(ts.Session, tasks.AddInput{Store: "org-b", Title: "Elsewhere", Repos: []string{"github.com/o/r"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := reposOf(t, ts, "org-b", r.ULID); len(got) != 1 || got[0] != "github.com/o/r" {
		t.Errorf("repos = %q", got)
	}
	// a session on --store org-a cannot be told to add to org-b
	_, err = tasks.Add(ts.ForStore("org-a"), tasks.AddInput{Store: "org-b", Title: "x"})
	usecasetest.WantCode(t, "Add(--store org-a, Store org-b)", err, usecase.KindUsage, "store.conflict")
	// Parent goes through ResolveRef: a task of another store is refused
	_, err = tasks.Add(ts.Session, tasks.AddInput{Store: "org-a", Title: "x", Parent: "WEB-1"})
	usecasetest.WantCode(t, "Add(parent in another store)", err, usecase.KindInvalid, "ref.cross_store")
}

func TestAddHereExplicitParentInAnotherStore(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	r, cf, ch, err := tasks.AddHere(ts.Session, tasks.AddHereInput{Context: usecase.Input{Parent: "WEB-1"}, Task: tasks.AddInput{Title: "child"}})
	if err != nil || cf != nil || ch != nil {
		t.Fatalf("AddHere = %v %v %v", cf, ch, err)
	}
	v, err := r.View()
	if err != nil || v.Store != "org-b" || v.Parent == nil || v.Title != "child" || v.ParentRef == nil || v.ParentRef.ID != "WEB-1" {
		t.Fatalf("view = %+v, %v", v, err)
	}
	if want := "org-b / " + v.ParentRef.Title; r.Destination != want || tasks.StageOf(r) != 1 {
		t.Errorf("destination = %q (stage %d), want %q (stage 1)", r.Destination, tasks.StageOf(r), want)
	}
}

func TestAddHereRefusesAReadOnlyStore(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	usecasetest.MakeStoreReadOnly(t, ts, "org-a")
	usecasetest.MakeStoreReadOnly(t, ts, "org-b")
	_, _, _, err := tasks.AddHere(ts.Session, tasks.AddHereInput{Context: usecase.Input{Store: "org-a"}, Task: tasks.AddInput{Title: "x"}})
	usecasetest.WantCode(t, "AddHere(read-only)", err, usecase.KindConflict, "store.read_only")
	_, _, _, err = tasks.AddHere(ts.Session, tasks.AddHereInput{Context: usecase.Input{Parent: "WEB-1"}, Task: tasks.AddInput{Title: "x"}})
	usecasetest.WantCode(t, "AddHere(read-only, by parent)", err, usecase.KindConflict, "store.read_only")
}

func TestAddHereRepoPutsTheRepoOnTheTask(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	dir := repoDir(t, "", "https://github.com/Org-A/Web.git")
	// without Repo the repository is not put on the task
	r, _, _, err := tasks.AddHere(ts.Session, tasks.AddHereInput{Context: usecase.Input{Dir: dir, Store: "org-a"}, Task: tasks.AddInput{Title: "unlinked"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := reposOf(t, ts, "org-a", r.ULID); len(got) != 0 {
		t.Errorf("without Repo: repos = %q", got)
	}
	r, _, _, err = tasks.AddHere(ts.Session, tasks.AddHereInput{Context: usecase.Input{Dir: dir, Store: "org-a"}, Repo: true, Task: tasks.AddInput{Title: "linked"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := reposOf(t, ts, "org-a", r.ULID); len(got) != 1 || got[0] != "github.com/org-a/web" {
		t.Errorf("Repo: repos = %q; want the repo ID", got)
	}
	// the repo ID goes first, and is not repeated when the task has it
	r, _, _, err = tasks.AddHere(ts.Session, tasks.AddHereInput{
		Context: usecase.Input{Dir: dir, Store: "org-a"}, Repo: true,
		Task: tasks.AddInput{Title: "both", Repos: []string{"github.com/o/other", "github.com/org-a/web"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := reposOf(t, ts, "org-a", r.ULID); len(got) != 2 || got[0] != "github.com/o/other" || got[1] != "github.com/org-a/web" {
		t.Errorf("Repo with repos given: repos = %q", got)
	}
	// Repo in a repository without a remote is a usage error
	_, _, _, err = tasks.AddHere(ts.Session, tasks.AddHereInput{Context: usecase.Input{Dir: repoDir(t, "", ""), Store: "org-a"}, Repo: true, Task: tasks.AddInput{Title: "bare"}})
	usecasetest.WantCode(t, "Repo in a repository without a remote", err, usecase.KindUsage, "add.repo_unavailable")
}

func TestAddHereStage4SetsRecent(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	p, err := tasks.Add(ts.ForStore("org-a"), tasks.AddInput{Title: "Web release", Kind: "project", Repos: []string{"github.com/org-a/web"}})
	if err != nil {
		t.Fatal(err)
	}
	dir := repoDir(t, "", "https://github.com/org-a/web")
	r, _, _, err := tasks.AddHere(ts.Session, tasks.AddHereInput{Context: usecase.Input{Dir: dir}, Repo: true, Task: tasks.AddInput{Title: "in the project"}})
	if err != nil {
		t.Fatal(err)
	}
	if v, err := r.View(); err != nil || v.Store != "org-a" || v.Parent == nil || *v.Parent != p.ULID.String() || r.Destination != "org-a / Web release" || tasks.StageOf(r) != 4 {
		t.Fatalf("destination = %q, view %+v, %v", r.Destination, v, err)
	}
	m, _ := ts.Mount()
	a, _ := m.Get("org-a")
	got := usecasetest.LoadContext(t, ts).Recent["github.com/org-a/web"]
	if got.Store != a.ULID || got.Project != p.ULID {
		t.Errorf("recent = %+v; want org-a's %s", got, p.ULID)
	}
	if reposOf(t, ts, "org-a", r.ULID)[0] != "github.com/org-a/web" {
		t.Errorf("the task has no repo")
	}
	// recent is not set by a stage that is not 4
	_, _, _, err = tasks.AddHere(ts.Session, tasks.AddHereInput{Context: usecase.Input{Dir: dir, Store: "org-a"}, Task: tasks.AddInput{Title: "x"}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAddHereReturnsWhatToAsk(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	setDefaultStore(t, ts, "")
	dir := repoDir(t, "hh/WEB-1-login", "")
	_, cf, _, err := tasks.AddHere(ts.Session, tasks.AddHereInput{Context: usecase.Input{Dir: dir, Interactive: true}, Task: tasks.AddInput{Title: "x"}})
	if err != nil || cf == nil || cf.ParentID != "WEB-1" || cf.ParentTitle == "" {
		t.Fatalf("BranchQuestion = %+v, %v", cf, err)
	}
	r, _, _, err := tasks.AddHere(ts.Session, tasks.AddHereInput{Context: usecase.Input{Dir: dir, Interactive: true, BranchParent: usecase.Yes}, Task: tasks.AddInput{Title: "x"}})
	if v, verr := r.View(); err != nil || verr != nil || v.Store != "org-b" || v.ParentRef == nil || v.ParentRef.ID != "WEB-1" || r.Destination != "org-b / "+v.ParentRef.Title || tasks.StageOf(r) != 3 {
		t.Fatalf("Yes = %q (stage %d), %v", r.Destination, tasks.StageOf(r), err)
	}
	_, _, ch, err := tasks.AddHere(ts.Session, tasks.AddHereInput{Context: usecase.Input{Interactive: true}, Task: tasks.AddInput{Title: "x"}})
	if err != nil || ch == nil || ch.What != "store" {
		t.Fatalf("Choice = %+v, %v", ch, err)
	}
	r, _, _, err = tasks.AddHere(ts.Session, tasks.AddHereInput{Context: usecase.Input{Interactive: true, PickStore: "org-a"}, Task: tasks.AddInput{Title: "x"}})
	if v, verr := r.View(); err != nil || verr != nil || v.Store != "org-a" || v.Parent != nil || !strings.HasPrefix(r.Destination, "org-a / ") || tasks.StageOf(r) != 5 {
		t.Fatalf("PickStore = %q (stage %d), %v", r.Destination, tasks.StageOf(r), err)
	}
}

func TestAddHereUsesDefaultStoreAndTheGlobalStore(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	setDefaultStore(t, ts, "org-b")
	r, _, _, err := tasks.AddHere(ts.Session, tasks.AddHereInput{Task: tasks.AddInput{Title: "x"}})
	if v, verr := r.View(); err != nil || verr != nil || v.Store != "org-b" || v.Parent != nil || tasks.StageOf(r) != 5 {
		t.Fatalf("default_store: %q (stage %d), %v", r.Destination, tasks.StageOf(r), err)
	}
	r, _, _, err = tasks.AddHere(ts.ForStore("org-a"), tasks.AddHereInput{Task: tasks.AddInput{Title: "x"}})
	if v, verr := r.View(); err != nil || verr != nil || v.Store != "org-a" || tasks.StageOf(r) != 1 {
		t.Fatalf("--store org-a: %q (stage %d), %v", r.Destination, tasks.StageOf(r), err)
	}
	_, _, _, err = tasks.AddHere(ts.Session, tasks.AddHereInput{Context: usecase.Input{Store: "nosuch"}, Task: tasks.AddInput{Title: "x"}})
	if err == nil {
		t.Error("--store nosuch: no error")
	}
}

func TestAddHereErrors(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	setDefaultStore(t, ts, "")
	pid := usecasetest.AddProjectIn(t, ts, "org-a", "Release")
	cases := []struct {
		name string
		in   usecase.Input
		kind usecase.Kind
		code string
	}{
		{"parent and project", usecase.Input{Parent: "API-1", Project: pid}, usecase.KindUsage, "add.parent_and_project"},
		{"not a project", usecase.Input{Project: "API-1"}, usecase.KindInvalid, "add.not_project"},
		{"no such parent", usecase.Input{Parent: "API-99"}, usecase.KindNotFound, "task.not_found"},
		{"no such workspace", usecase.Input{Workspace: "nosuch"}, usecase.KindNotFound, "workspace.not_found"},
		{"no store chosen", usecase.Input{}, usecase.KindUsage, "choice.required"},
	}
	for _, c := range cases {
		_, _, _, err := tasks.AddHere(ts.Session, tasks.AddHereInput{Context: c.in, Task: tasks.AddInput{Title: "x"}})
		usecasetest.WantCode(t, c.name, err, c.kind, c.code)
	}
	if e := usecase.AsError(func() error {
		_, _, _, err := tasks.AddHere(ts.Session, tasks.AddHereInput{Task: tasks.AddInput{Title: "x"}})
		return err
	}()); e == nil || e.Args["what"] != "store" {
		t.Errorf("choice.required Args = %+v", e)
	}
}

// setDefaultStore makes name the default store in config.yaml; "" removes it
// (the first store added is the default).
func setDefaultStore(t *testing.T, ts *usecasetest.Session, name string) {
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

// An unknown store given to add is said as the other commands say it.
func TestAddHereUnknownStoreIsStoreNotFound(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	_, _, _, err := tasks.AddHere(ts.Session, tasks.AddHereInput{Context: usecase.Input{Store: "nope"}, Task: tasks.AddInput{Title: "x"}})
	usecasetest.WantCode(t, "AddHere in nope", err, usecase.KindNotFound, "store.not_found")
}
