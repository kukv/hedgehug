package context

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/mount"
	"github.com/kukv/hedgehug/internal/store"
)

var testAuthor = gitrepo.Signature{Name: "Test", Email: "test@example.com"}

// newStoreAt makes a store called name with the workspaces wss (name, prefix)
// and returns its entry.
func newStoreAt(t *testing.T, name string, wss ...string) config.StoreEntry {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	ws := &model.Workspace{ULID: model.NewULID(), Name: wss[0], IDPrefix: wss[1]}
	s, err := store.Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: name}, nil, ws, testAuthor, "", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	for i := 2; i+1 < len(wss); i += 2 {
		w := &model.Workspace{ULID: model.NewULID(), Name: wss[i], IDPrefix: wss[i+1]}
		if _, err := s.Update(store.Op{Name: "workspace", Author: testAuthor}, func(tx *store.Tx) error { return tx.PutWorkspace(w) }); err != nil {
			t.Fatal(err)
		}
	}
	return config.StoreEntry{Name: name, Path: dir}
}

// put writes one task into the first workspace of e, after edit changed it.
func put(t *testing.T, e config.StoreEntry, title string, edit func(*model.Task)) *model.Task {
	t.Helper()
	s, err := store.Open(e.Path)
	if err != nil {
		t.Fatal(err)
	}
	var ws model.ULID
	for w, err := range s.Workspaces() {
		if err != nil {
			t.Fatal(err)
		}
		if ws.IsZero() || w.ULID.String() < ws.String() {
			ws = w.ULID
		}
	}
	task := &model.Task{ULID: model.NewULID(), Title: title, Kind: "task", Status: "todo", Workspace: ws}
	if edit != nil {
		edit(task)
	}
	if _, err := s.Update(store.Op{Name: "add", Author: testAuthor}, func(tx *store.Tx) error { return tx.PutTask(task) }); err != nil {
		t.Fatal(err)
	}
	return task
}

func project(repos ...string) func(*model.Task) {
	return func(t *model.Task) { t.Kind, t.Repos = "project", repos }
}

func openSet(t *testing.T, es ...config.StoreEntry) *mount.Set {
	t.Helper()
	set, err := mount.Open(&config.Config{Stores: es}, mount.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return set
}

// world is org-a (workspace api, API) and org-b (workspace web, WEB): API-1 is a
// project and API-2 a task in org-a, WEB-1 a task in org-b.
type world struct {
	a, b       config.StoreEntry
	api1, api2 *model.Task
	web1       *model.Task
}

func newWorld(t *testing.T) (*world, *mount.Set) {
	t.Helper()
	w := &world{a: newStoreAt(t, "org-a", "api", "API"), b: newStoreAt(t, "org-b", "web", "WEB")}
	w.api1 = put(t, w.a, "API project", project())
	w.api2 = put(t, w.a, "API task", nil)
	w.web1 = put(t, w.b, "Web task", nil)
	return w, openSet(t, w.a, w.b)
}

// gitDir makes a repository whose remotes are remotes (name, url), and whose
// HEAD is on branch ("" keeps master).
func gitDir(t *testing.T, branch string, remotes ...string) string {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i+1 < len(remotes); i += 2 {
		if _, err := repo.CreateRemote(&gitconfig.RemoteConfig{Name: remotes[i], URLs: []string{remotes[i+1]}}); err != nil {
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

func resolve(t *testing.T, set *mount.Set, st State, in Input) (*Destination, *Confirm, *Choice, []string, error) {
	t.Helper()
	var warns []string
	d, cf, ch, err := Resolve(set, st, in, func(m string) { warns = append(warns, m) })
	return d, cf, ch, warns, err
}

func mustDest(t *testing.T, set *mount.Set, st State, in Input) *Destination {
	t.Helper()
	d, cf, ch, _, err := resolve(t, set, st, in)
	if err != nil || cf != nil || ch != nil || d == nil {
		t.Fatalf("Resolve = %v, %v, %v, %v; want a destination", d, cf, ch, err)
	}
	return d
}

func wantKind(t *testing.T, err error, kind Kind, code string) {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) || e.Kind != kind || e.Code != code {
		t.Errorf("error = %v; want kind %d %q", err, kind, code)
	}
}

func TestExplicitStoreAloneIsTheRootOfItsOnlyWorkspace(t *testing.T) {
	_, set := newWorld(t)
	d := mustDest(t, set, State{}, Input{Store: "org-b"})
	if d.Store.Name != "org-b" || d.Parent != nil || d.Workspace.Name != "web" || d.Stage != 1 {
		t.Errorf("destination = %s %v %v stage %d", d.Store.Name, d.Parent, d.Workspace, d.Stage)
	}
}

func TestExplicitParentIsLookedUpInEveryStoreOrInTheStore(t *testing.T) {
	w, set := newWorld(t)
	d := mustDest(t, set, State{}, Input{Parent: "WEB-1"})
	if d.Store.Name != "org-b" || d.Parent == nil || d.Parent.ULID != w.web1.ULID || d.Workspace.Name != "web" {
		t.Errorf("--parent WEB-1 = %s %v", d.Store.Name, d.Parent)
	}
	d = mustDest(t, set, State{}, Input{Parent: "WEB-1", Store: "org-b"})
	if d.Parent == nil || d.Parent.ULID != w.web1.ULID {
		t.Errorf("--parent WEB-1 --store org-b = %v", d.Parent)
	}
	// WEB-1 is not in org-a
	_, _, _, _, err := resolve(t, set, State{}, Input{Parent: "WEB-1", Store: "org-a"})
	var le *LookupError
	if !errors.As(err, &le) || !errors.Is(err, store.ErrNotFound) {
		t.Errorf("--parent WEB-1 --store org-a: %v; want a lookup error, not found", err)
	}
	// a "<store>:" that is not --store
	_, _, _, _, err = resolve(t, set, State{}, Input{Parent: "org-b:WEB-1", Store: "org-a"})
	wantKind(t, err, KindUsage, "store.conflict")
}

func TestExplicitProject(t *testing.T) {
	w, set := newWorld(t)
	d := mustDest(t, set, State{}, Input{Project: "API-1"})
	if d.Parent == nil || d.Parent.ULID != w.api1.ULID || d.Store.Name != "org-a" || d.Workspace.Name != "api" {
		t.Errorf("--project API-1 = %v", d.Parent)
	}
	_, _, _, _, err := resolve(t, set, State{}, Input{Project: "API-2"})
	wantKind(t, err, KindInvalid, "add.not_project")
	_, _, _, _, err = resolve(t, set, State{}, Input{Project: "API-1", Parent: "API-2"})
	wantKind(t, err, KindUsage, "add.parent_and_project")
}

func TestExplicitWorkspaceAlone(t *testing.T) {
	a := newStoreAt(t, "org-a", "api", "API", "shared", "SH")
	b := newStoreAt(t, "org-b", "web", "WEB", "shared", "SH2")
	set := openSet(t, a, b)
	d := mustDest(t, set, State{}, Input{Workspace: "web"})
	if d.Store.Name != "org-b" || d.Parent != nil || d.Workspace.Name != "web" {
		t.Errorf("--workspace web = %s %v", d.Store.Name, d.Workspace)
	}
	_, _, _, _, err := resolve(t, set, State{}, Input{Workspace: "shared"})
	wantKind(t, err, KindAmbiguous, "workspace.ambiguous")
	if !strings.Contains(err.Error(), "org-a:shared") || !strings.Contains(err.Error(), "org-b:shared") {
		t.Errorf("error = %v; want the stores named", err)
	}
	_, _, _, _, err = resolve(t, set, State{}, Input{Workspace: "nosuch"})
	var le *LookupError
	if !errors.As(err, &le) || le.What != "workspace" {
		t.Errorf("--workspace nosuch: %v", err)
	}
}

func TestUseStage(t *testing.T) {
	w, set := newWorld(t)
	a, _ := set.Get("org-a")
	st := State{Use: &Ref{Store: a.ULID, Project: w.api1.ULID}}
	d := mustDest(t, set, st, Input{})
	if d.Stage != 2 || d.Parent == nil || d.Parent.ULID != w.api1.ULID {
		t.Errorf("use = stage %d %v", d.Stage, d.Parent)
	}
	// a project that is gone: warned, and the next stage (the only store... two stores, so a choice)
	st = State{Use: &Ref{Store: a.ULID, Project: model.NewULID()}}
	_, _, ch, warns, err := resolve(t, set, st, Input{Interactive: true})
	if err != nil || ch == nil || ch.What != "store" {
		t.Errorf("gone use: choice %v, %v", ch, err)
	}
	if len(warns) != 1 || warns[0] != UseGoneWarning {
		t.Errorf("warns = %q", warns)
	}
	// a task, not a project, is gone as well
	st = State{Use: &Ref{Store: a.ULID, Project: w.api2.ULID}}
	if _, _, _, warns, _ := resolve(t, set, st, Input{Interactive: true}); len(warns) != 1 {
		t.Errorf("a task is not a project: warns = %q", warns)
	}
	// an unmounted store
	st = State{Use: &Ref{Store: model.NewULID(), Project: w.api1.ULID}}
	if _, _, _, warns, _ := resolve(t, set, st, Input{Interactive: true}); len(warns) != 1 {
		t.Errorf("an unmounted store: warns = %q", warns)
	}
}

func TestBranchStage(t *testing.T) {
	w, set := newWorld(t)
	dir := gitDir(t, "hh/WEB-1-login")
	_, cf, _, _, err := resolve(t, set, State{}, Input{Dir: dir, Interactive: true})
	if err != nil || cf == nil || cf.Parent.ULID != w.web1.ULID || cf.Store.Name != "org-b" {
		t.Fatalf("Confirm = %+v, %v", cf, err)
	}
	d := mustDest(t, set, State{}, Input{Dir: dir, Interactive: true, BranchParent: Yes})
	if d.Stage != 3 || d.Parent == nil || d.Parent.ULID != w.web1.ULID {
		t.Errorf("Yes = stage %d %v", d.Stage, d.Parent)
	}
	// No: the next stage, a choice of the store
	_, cf, ch, _, err := resolve(t, set, State{}, Input{Dir: dir, Interactive: true, BranchParent: No})
	if err != nil || cf != nil || ch == nil || ch.What != "store" {
		t.Errorf("No = %v %v %v", cf, ch, err)
	}
	// not a terminal: skipped, the stages after it ask for a store
	_, cf, _, _, err = resolve(t, set, State{}, Input{Dir: dir})
	if cf != nil {
		t.Errorf("not interactive asked: %v", cf)
	}
	wantKind(t, err, KindUsage, "choice.required")
	for _, branch := range []string{"feature/x", "hh/WEB-999-x", "hh/WEB-1x", "hedgehug/web-1"} {
		_, cf, _, _, _ := resolve(t, set, State{}, Input{Dir: gitDir(t, branch), Interactive: true})
		if branch == "hedgehug/web-1" {
			if cf == nil {
				t.Errorf("%s: no Confirm; want one (the id ends the name, in any case)", branch)
			}
			continue
		}
		if cf != nil {
			t.Errorf("%s: asked, want skipped", branch)
		}
	}
}

func TestRepoStage(t *testing.T) {
	a := newStoreAt(t, "org-a", "api", "API")
	b := newStoreAt(t, "org-b", "web", "WEB")
	p1 := put(t, a, "Web release", project("github.com/org-a/web"))
	set := openSet(t, a, b)
	dir := gitDir(t, "", "origin", "https://github.com/Org-A/Web.git")
	d := mustDest(t, set, State{}, Input{Dir: dir})
	if d.Stage != 4 || d.Parent == nil || d.Parent.ULID != p1.ULID || !d.Recent || d.RepoID != "github.com/org-a/web" {
		t.Errorf("one project = stage %d %v recent %v repo %q", d.Stage, d.Parent, d.Recent, d.RepoID)
	}

	// two projects: a choice whose default is recent; not interactive uses recent, or fails
	p2 := put(t, b, "Other web", project("github.com/org-a/web"))
	set = openSet(t, a, b)
	_, _, ch, _, err := resolve(t, set, State{}, Input{Dir: dir, Interactive: true})
	if err != nil || ch == nil || ch.What != "project" || len(ch.Options) != 2 || ch.Default != "" {
		t.Fatalf("Choice = %+v, %v", ch, err)
	}
	if ch.Options[0] != "org-a:API-1" || ch.Options[1] != "org-b:WEB-1" || ch.Labels[0] != "Web release" {
		t.Errorf("options %q labels %q", ch.Options, ch.Labels)
	}
	bm, _ := set.Get("org-b")
	st := State{Recent: map[string]Ref{"github.com/org-a/web": {Store: bm.ULID, Project: p2.ULID}}}
	_, _, ch, _, _ = resolve(t, set, st, Input{Dir: dir, Interactive: true})
	if ch == nil || ch.Default != "org-b:WEB-1" {
		t.Errorf("Default = %+v; want org-b:WEB-1", ch)
	}
	d = mustDest(t, set, State{}, Input{Dir: dir, Interactive: true, PickProject: "org-a:API-1"})
	if d.Parent.ULID != p1.ULID || !d.Recent {
		t.Errorf("picked = %v", d.Parent)
	}
	d = mustDest(t, set, st, Input{Dir: dir})
	if d.Parent.ULID != p2.ULID || d.Stage != 4 {
		t.Errorf("not interactive with recent = %v", d.Parent)
	}
	_, _, _, _, err = resolve(t, set, State{}, Input{Dir: dir})
	wantKind(t, err, KindUsage, "choice.required")
	if !strings.Contains(err.Error(), "org-a:API-1") || !strings.Contains(err.Error(), "org-b:WEB-1") {
		t.Errorf("error = %v; want the candidates", err)
	}
	_, _, _, _, err = resolve(t, set, State{}, Input{Dir: dir, Interactive: true, PickProject: "org-a:API-9"})
	wantKind(t, err, KindUsage, "choice.invalid")

	// a project that is deleted does not count
	s, _ := store.Open(b.Path)
	if _, err := s.Update(store.Op{Name: "rm", Author: testAuthor}, func(tx *store.Tx) error {
		tk, err := tx.Task(p2.ULID)
		if err != nil {
			return err
		}
		now := time.Now()
		tk.DeletedAt = &now
		return tx.PutTask(tk)
	}); err != nil {
		t.Fatal(err)
	}
	if d := mustDest(t, set, State{}, Input{Dir: dir}); d.Parent.ULID != p1.ULID {
		t.Errorf("after the delete = %v", d.Parent)
	}
}

func TestRepoStageReadsRemotesAsWritten(t *testing.T) {
	a := newStoreAt(t, "org-a", "api", "API")
	p := put(t, a, "Web", project("github.com/org-a/web"))
	set := openSet(t, a)
	dir := gitDir(t, "", "origin", "git@github.com:org-a/web.git")
	cfg := filepath.Join(dir, ".git", "config")
	b, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// url_rewrites in the repository's own config must not change what the remote is
	b = append(b, []byte("[url \"/local/mirror/\"]\n\tinsteadOf = git@github.com:\n")...)
	if err := os.WriteFile(cfg, b, 0o600); err != nil {
		t.Fatal(err)
	}
	d := mustDest(t, set, State{}, Input{Dir: dir})
	if d.Stage != 4 || d.Parent.ULID != p.ULID {
		t.Errorf("destination = stage %d %v", d.Stage, d.Parent)
	}
}

func TestRepoIDWithoutOriginTakesTheRemotesByName(t *testing.T) {
	a := newStoreAt(t, "org-a", "api", "API")
	put(t, a, "T", nil)
	set := openSet(t, a)
	dir := gitDir(t, "", "zeta", "https://github.com/o/z.git", "alpha", "https://github.com/o/a.git")
	if d := mustDest(t, set, State{}, Input{Dir: dir}); d.RepoID != "github.com/o/a" {
		t.Errorf("RepoID = %q; want the first by name", d.RepoID)
	}
	// origin alone: origin that is not a repo ID leaves none, however the others read
	dir = gitDir(t, "", "origin", "/local/path", "alpha", "https://github.com/o/a.git")
	if d := mustDest(t, set, State{}, Input{Dir: dir}); d.RepoID != "" {
		t.Errorf("RepoID = %q; want none", d.RepoID)
	}
	if d := mustDest(t, set, State{}, Input{}); d.RepoID != "" {
		t.Errorf("no Dir: RepoID = %q", d.RepoID)
	}
}

func TestRepoIDIsSetWhateverStageDecided(t *testing.T) {
	_, set := newWorld(t)
	dir := gitDir(t, "", "origin", "https://github.com/o/r.git")
	d := mustDest(t, set, State{}, Input{Dir: dir, Parent: "WEB-1"})
	if d.Stage != 1 || d.RepoID != "github.com/o/r" {
		t.Errorf("stage %d RepoID %q", d.Stage, d.RepoID)
	}
}

func TestDefaultStage(t *testing.T) {
	_, set := newWorld(t)
	d := mustDest(t, set, State{}, Input{DefaultStore: "org-b"})
	if d.Stage != 5 || d.Store.Name != "org-b" || d.Parent != nil || d.Workspace.Name != "web" {
		t.Errorf("default_store = stage %d %s", d.Stage, d.Store.Name)
	}
	_, _, ch, _, err := resolve(t, set, State{}, Input{Interactive: true})
	if err != nil || ch == nil || ch.What != "store" || len(ch.Options) != 2 {
		t.Errorf("two stores: %+v, %v", ch, err)
	}
	d = mustDest(t, set, State{}, Input{Interactive: true, PickStore: "org-a"})
	if d.Store.Name != "org-a" {
		t.Errorf("picked store = %s", d.Store.Name)
	}
	_, _, _, _, err = resolve(t, set, State{}, Input{})
	wantKind(t, err, KindUsage, "choice.required")
	if !strings.Contains(err.Error(), "--store") || !strings.Contains(err.Error(), "hedgehug store default") {
		t.Errorf("error = %v; want the flag and the command", err)
	}

	// one store: it; several workspaces: a choice, or an error naming --workspace
	one := newStoreAt(t, "solo", "main", "MN", "extra", "EX")
	set = openSet(t, one)
	_, _, ch, _, err = resolve(t, set, State{}, Input{Interactive: true})
	if err != nil || ch == nil || ch.What != "workspace" || len(ch.Options) != 2 {
		t.Fatalf("two workspaces: %+v, %v", ch, err)
	}
	d = mustDest(t, set, State{}, Input{Interactive: true, PickWorkspace: "extra"})
	if d.Workspace.Name != "extra" || d.Store.Name != "solo" {
		t.Errorf("picked workspace = %v", d.Workspace)
	}
	_, _, _, _, err = resolve(t, set, State{}, Input{})
	wantKind(t, err, KindUsage, "choice.required")
	if !strings.Contains(err.Error(), "--workspace") {
		t.Errorf("error = %v; want --workspace", err)
	}
	// --store alone with several workspaces asks the same
	_, _, ch, _, _ = resolve(t, set, State{}, Input{Store: "solo", Interactive: true})
	if ch == nil || ch.What != "workspace" {
		t.Errorf("--store alone: %+v", ch)
	}
}
