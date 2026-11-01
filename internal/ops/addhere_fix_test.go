package ops

import (
	"strings"
	"testing"

	hhctx "github.com/kukv/hedgehug/internal/context"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
)

func TestAddHereNoRepoStillSetsRecent(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSession(t)
	p, err := ts.ForStore("org-a").Add(AddInput{Title: "Web release", Kind: "project", Repos: []string{"github.com/org-a/web"}})
	if err != nil {
		t.Fatal(err)
	}
	dir := repoDir(t, "", "https://github.com/org-a/web")
	r, _, _, err := ts.AddHere(AddHereInput{Context: hhctx.Input{Dir: dir}, NoRepo: true, Task: AddInput{Title: "x"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(reposOf(t, ts, "org-a", r.ULID)) != 0 {
		t.Errorf("--no-repo put a repo on the task")
	}
	if got := loadContext(t, ts).Recent["github.com/org-a/web"]; got.Project != p.ULID {
		t.Errorf("recent = %+v; want the project %s", got, p.ULID)
	}
	// stage 1 decides: recent is not touched
	q, err := ts.ForStore("org-a").Add(AddInput{Title: "Other", Kind: "project"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := ts.AddHere(AddHereInput{Context: hhctx.Input{Dir: dir, Parent: q.ULID.String()}, Task: AddInput{Title: "y"}}); err != nil {
		t.Fatal(err)
	}
	if got := loadContext(t, ts).Recent["github.com/org-a/web"]; got.Project != p.ULID {
		t.Errorf("recent = %+v after stage 1; want it kept as %s", got, p.ULID)
	}
}

func TestAddHereWarnsOnceAcrossQuestions(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSession(t)
	_, p, _ := ts.Config()
	if err := hhctx.Save(p.DataDir, hhctx.State{Use: &hhctx.Ref{Store: model.NewULID(), Project: model.NewULID()}}); err != nil {
		t.Fatal(err)
	}
	dir := repoDir(t, "hh/WEB-1-login", "")
	in := AddHereInput{Context: hhctx.Input{Dir: dir, Interactive: true}, Task: AddInput{Title: "x"}}
	if _, cf, _, err := ts.AddHere(in); err != nil || cf == nil {
		t.Fatalf("first call = %v, %v", cf, err)
	}
	in.Context.BranchParent = hhctx.Yes
	if _, _, _, err := ts.AddHere(in); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, w := range ts.warns {
		if w == useGoneWarning {
			n++
		}
	}
	if n != 1 {
		t.Errorf("the hedgehug use warning came %d times (%q); want once", n, ts.warns)
	}
}

func TestAddHereExplainsAnAmbiguityWithinTheNamedStore(t *testing.T) {
	t.Parallel()
	ts := newTwoStoreSession(t)
	s, err := ts.ForStore("org-a").OpenStore()
	if err != nil {
		t.Fatal(err)
	}
	_, ix, err := ts.ForStore("org-a").Open()
	if err != nil {
		t.Fatal(err)
	}
	ws := ix.Workspaces()[0].ULID
	for _, title := range []string{"one", "two"} {
		task := &model.Task{ULID: model.NewULID(), Title: title, Kind: "task", Status: "todo", Workspace: ws, FormerIDs: []string{"OLD-1"}}
		if _, err := ts.ForStore("org-a").Update(s, store.Op{Name: "add", Task: task.ULID}, func(tx *store.Tx) error { return tx.PutTask(task) }); err != nil {
			t.Fatal(err)
		}
	}
	for _, in := range []hhctx.Input{{Store: "org-a", Parent: "OLD-1"}, {Parent: "org-a:OLD-1"}} {
		_, _, _, err = ts.AddHere(AddHereInput{Context: in, Task: AddInput{Title: "x"}})
		wantCode(t, "ambiguous former id", err, KindAmbiguous, "task.ambiguous")
		if err == nil || !strings.Contains(err.Error(), "one") || !strings.Contains(err.Error(), "two") {
			t.Errorf("error = %v; want the candidates spelled out", err)
		}
	}
}
