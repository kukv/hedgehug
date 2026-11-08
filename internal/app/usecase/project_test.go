package usecase_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/index"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestTaskViewProject(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	add := func(in tasks.AddInput) *usecase.TaskView {
		t.Helper()
		r, err := tasks.Add(ts.Session, in)
		if err != nil {
			t.Fatal(err)
		}
		v, err := r.View()
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	proj := add(tasks.AddInput{Title: "P", Kind: "project"})
	child := add(tasks.AddInput{Title: "child", Parent: proj.ULID})
	grand := add(tasks.AddInput{Title: "grand", Parent: child.ULID})
	epic := add(tasks.AddInput{Title: "E", Kind: "epic", Parent: proj.ULID})
	underEpic := add(tasks.AddInput{Title: "under epic", Parent: epic.ULID})

	// read after all the adds, as a reader gets it
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	check := func(name string, v, want *usecase.TaskView) {
		t.Helper()
		e, ok := ix.Task(usecasetest.MustULID(t, v.ULID))
		if !ok {
			t.Fatalf("%s: not in the index", name)
		}
		got := usecase.TaskViewOf(ix, e).Project
		switch {
		case want == nil && got != nil:
			t.Errorf("%s: project = %+v; want none", name, got)
		case want != nil && (got == nil || got.ULID != want.ULID || got.ID != want.ID || got.Title != want.Title):
			t.Errorf("%s: project = %+v; want %s %s", name, got, want.ID, want.Title)
		}
	}
	check("a task under the project", child, proj)
	check("a grandchild", grand, proj)
	check("a task under an epic: the project above the epic", underEpic, proj)
	check("the epic under the project", epic, proj)
	check("the project itself: not itself", proj, nil)
}

// A parent the index does not have, and a task that is its own parent, end
// the walk with no project instead of failing or looping.
func TestProjectOfStopsAtUnknownParentAndCycle(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	missing := model.NewULID()
	if got := usecase.ProjectOf(ix, &index.Entry{ULID: model.NewULID(), Parent: &missing}); got != nil {
		t.Errorf("unknown parent: project = %+v; want none", got)
	}
	self := model.NewULID()
	if got := usecase.ProjectOf(ix, &index.Entry{ULID: self, Parent: &self}); got != nil {
		t.Errorf("own parent: project = %+v; want none", got)
	}
}
