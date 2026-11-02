package ops

import "testing"

func TestTaskViewProject(t *testing.T) {
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
	child := add(AddInput{Title: "child", Parent: proj.ULID})
	grand := add(AddInput{Title: "grand", Parent: child.ULID})
	epic := add(AddInput{Title: "E", Kind: "epic", Parent: proj.ULID})
	underEpic := add(AddInput{Title: "under epic", Parent: epic.ULID})
	loose := add(AddInput{Title: "loose"})
	rootEpic := add(AddInput{Title: "RE", Kind: "epic"})
	rootEpicChild := add(AddInput{Title: "under a root epic", Parent: rootEpic.ULID})

	// read after all the adds, as a reader gets it
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	check := func(name string, v, want *TaskView) {
		t.Helper()
		e, ok := ix.Task(mustULID(t, v.ULID))
		if !ok {
			t.Fatalf("%s: not in the index", name)
		}
		got := TaskViewOf(ix, e).Project
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
	check("an epic at the root: no project", rootEpicChild, nil)
	check("the project itself: not itself", proj, nil)
	check("a task with no parent", loose, nil)
}
