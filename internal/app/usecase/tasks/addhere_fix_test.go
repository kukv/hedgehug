package tasks_test

import (
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/current"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestAddHereWithoutRepoStillSetsRecent(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	p, err := tasks.Add(ts.ForStore("org-a"), tasks.AddInput{Title: "Web release", Kind: "project", Repos: []string{"github.com/org-a/web"}})
	if err != nil {
		t.Fatal(err)
	}
	dir := repoDir(t, "", "https://github.com/org-a/web")
	r, _, _, err := tasks.AddHere(ts.Session, tasks.AddHereInput{Context: usecase.Input{Dir: dir}, Task: tasks.AddInput{Title: "x"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(reposOf(t, ts, "org-a", r.ULID)) != 0 {
		t.Errorf("an add without Repo put a repo on the task")
	}
	if got := usecasetest.LoadContext(t, ts).Recent["github.com/org-a/web"]; got.Project != p.ULID {
		t.Errorf("recent = %+v; want the project %s", got, p.ULID)
	}
	// stage 1 decides: recent is not touched
	q, err := tasks.Add(ts.ForStore("org-a"), tasks.AddInput{Title: "Other", Kind: "project"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := tasks.AddHere(ts.Session, tasks.AddHereInput{Context: usecase.Input{Dir: dir, Parent: q.ULID.String()}, Task: tasks.AddInput{Title: "y"}}); err != nil {
		t.Fatal(err)
	}
	if got := usecasetest.LoadContext(t, ts).Recent["github.com/org-a/web"]; got.Project != p.ULID {
		t.Errorf("recent = %+v after stage 1; want it kept as %s", got, p.ULID)
	}
}

func TestAddHereWarnsOnceAcrossQuestions(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
	_, p, _ := ts.Config()
	if err := current.Save(p.DataDir, current.State{Use: &current.Ref{Store: model.NewULID(), Project: model.NewULID()}}); err != nil {
		t.Fatal(err)
	}
	dir := repoDir(t, "hh/WEB-1-login", "")
	in := tasks.AddHereInput{Context: usecase.Input{Dir: dir, Interactive: true}, Task: tasks.AddInput{Title: "x"}}
	if _, cf, _, err := tasks.AddHere(ts.Session, in); err != nil || cf == nil {
		t.Fatalf("first call = %v, %v", cf, err)
	}
	in.Context.BranchParent = usecase.Yes
	if _, _, _, err := tasks.AddHere(ts.Session, in); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, w := range ts.Warns {
		if w == usecasetest.UseGoneWarning {
			n++
		}
	}
	if n != 1 {
		t.Errorf("the hedgehug use warning came %d times (%q); want once", n, ts.Warns)
	}
}

func TestAddHereExplainsAnAmbiguityWithinTheNamedStore(t *testing.T) {
	t.Parallel()
	ts := usecasetest.NewTwoStore(t)
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
		task := &model.Task{ULID: model.NewULID(), Title: title, Kind: "project", Status: "todo", Workspace: ws, FormerIDs: []string{"OLD-1"}}
		if _, err := ts.ForStore("org-a").Update(s, store.Op{Name: "add", Task: task.ULID}, func(tx *store.Tx) error { return tx.PutTask(task) }); err != nil {
			t.Fatal(err)
		}
	}
	for _, in := range []usecase.Input{{Store: "org-a", Parent: "OLD-1"}, {Parent: "org-a:OLD-1"}} {
		_, _, _, err = tasks.AddHere(ts.Session, tasks.AddHereInput{Context: in, Task: tasks.AddInput{Title: "x"}})
		usecasetest.WantCode(t, "ambiguous former id", err, usecase.KindAmbiguous, "task.ambiguous")
		if err == nil || !strings.Contains(err.Error(), "one") || !strings.Contains(err.Error(), "two") {
			t.Errorf("error = %v; want the candidates spelled out", err)
		}
	}
}
