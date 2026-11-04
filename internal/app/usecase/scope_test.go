package usecase_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/schemas"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestTaskViewScope(t *testing.T) {
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
	epic := add(tasks.AddInput{Title: "E", Kind: "epic", Parent: proj.ULID})
	underEpic := add(tasks.AddInput{Title: "under epic", Parent: epic.ULID})
	underProj := add(tasks.AddInput{Title: "under project", Parent: proj.ULID})
	epic2 := add(tasks.AddInput{Title: "E2", Kind: "epic", Parent: proj.ULID})
	underEpic2 := add(tasks.AddInput{Title: "under epic2", Parent: epic2.ULID})
	loose := add(tasks.AddInput{Title: "loose"})

	statuses := []string{"backlog", "todo", "in_progress", "done", "canceled"}
	set := func(ulid string) {
		t.Helper()
		if _, err := schemas.ScopedSet(ts.Session, ulid, schema.ScopedChange{Statuses: &statuses}); err != nil {
			t.Fatal(err)
		}
	}
	set(proj.ULID)
	set(epic.ULID)

	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	scopeOf := func(v *usecase.TaskView) string {
		t.Helper()
		e, ok := ix.Task(usecasetest.MustULID(t, v.ULID))
		if !ok {
			t.Fatalf("%s: not in the index", v.Title)
		}
		return usecase.TaskViewOf(ix, e).Scope
	}
	for _, c := range []struct {
		name string
		v    *usecase.TaskView
		want string
	}{
		{"a task under an epic with an override: the epic, nearer than the project", underEpic, epic.ULID},
		{"an epic with its own override: itself", epic, epic.ULID},
		{"a task under the project", underProj, proj.ULID},
		{"an epic with none: the project above it", epic2, proj.ULID},
		{"under an epic with none: the project", underEpic2, proj.ULID},
		{"the project itself", proj, proj.ULID},
		{"no ancestor: none", loose, ""},
	} {
		if got := scopeOf(c.v); got != c.want {
			t.Errorf("%s: scope = %q; want %q", c.name, got, c.want)
		}
	}
}

func TestTaskViewScopeStopsAtABrokenAncestor(t *testing.T) {
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
	epic := add(tasks.AddInput{Title: "E", Kind: "epic", Parent: proj.ULID})
	child := add(tasks.AddInput{Title: "under epic", Parent: epic.ULID})
	statuses := []string{"backlog", "todo", "in_progress", "done", "canceled"}
	for _, u := range []string{proj.ULID, epic.ULID} {
		if _, err := schemas.ScopedSet(ts.Session, u, schema.ScopedChange{Statuses: &statuses}); err != nil {
			t.Fatal(err)
		}
	}
	ts.CommitRawFile(model.TaskPath(usecasetest.MustULID(t, epic.ULID)), "not a task\n")

	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	e, ok := ix.Task(usecasetest.MustULID(t, child.ULID))
	if !ok {
		t.Fatal("the child is not in the index")
	}
	// A write walks no further than a task it cannot read, so no override applies.
	if got := usecase.TaskViewOf(ix, e).Scope; got != "" {
		t.Errorf("scope = %q; want none past a broken ancestor", got)
	}
}
