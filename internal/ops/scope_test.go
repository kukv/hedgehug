package ops

import (
	"testing"

	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/schema"
)

func TestTaskViewScope(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	add := func(in AddInput) *TaskView {
		t.Helper()
		r, err := ts.Add(in)
		if err != nil {
			t.Fatal(err)
		}
		v, err := r.View()
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	proj := add(AddInput{Title: "P", Kind: "project"})
	epic := add(AddInput{Title: "E", Kind: "epic", Parent: proj.ULID})
	underEpic := add(AddInput{Title: "under epic", Parent: epic.ULID})
	underProj := add(AddInput{Title: "under project", Parent: proj.ULID})
	epic2 := add(AddInput{Title: "E2", Kind: "epic", Parent: proj.ULID})
	underEpic2 := add(AddInput{Title: "under epic2", Parent: epic2.ULID})
	loose := add(AddInput{Title: "loose"})

	statuses := []string{"backlog", "todo", "in_progress", "done", "canceled"}
	set := func(ulid string) {
		t.Helper()
		if _, err := ts.ScopedSet(ulid, schema.ScopedChange{Statuses: &statuses}); err != nil {
			t.Fatal(err)
		}
	}
	set(proj.ULID)
	set(epic.ULID)

	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	scopeOf := func(v *TaskView) string {
		t.Helper()
		e, ok := ix.Task(mustULID(t, v.ULID))
		if !ok {
			t.Fatalf("%s: not in the index", v.Title)
		}
		return TaskViewOf(ix, e).Scope
	}
	for _, c := range []struct {
		name string
		v    *TaskView
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
	ts := newTestSession(t)
	add := func(in AddInput) *TaskView {
		t.Helper()
		r, err := ts.Add(in)
		if err != nil {
			t.Fatal(err)
		}
		v, err := r.View()
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	proj := add(AddInput{Title: "P", Kind: "project"})
	epic := add(AddInput{Title: "E", Kind: "epic", Parent: proj.ULID})
	child := add(AddInput{Title: "under epic", Parent: epic.ULID})
	statuses := []string{"backlog", "todo", "in_progress", "done", "canceled"}
	for _, u := range []string{proj.ULID, epic.ULID} {
		if _, err := ts.ScopedSet(u, schema.ScopedChange{Statuses: &statuses}); err != nil {
			t.Fatal(err)
		}
	}
	ts.commitRawFile(model.TaskPath(mustULID(t, epic.ULID)), "not a task\n")

	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	e, ok := ix.Task(mustULID(t, child.ULID))
	if !ok {
		t.Fatal("the child is not in the index")
	}
	// A write walks no further than a task it cannot read, so no override applies.
	if got := TaskViewOf(ix, e).Scope; got != "" {
		t.Errorf("scope = %q; want none past a broken ancestor", got)
	}
}
