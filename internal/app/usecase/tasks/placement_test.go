package tasks_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// wantPlacement checks that err refuses a write with a placement violation.
func wantPlacement(t *testing.T, err error) {
	t.Helper()
	vs, ok := usecase.ViolationsOf(err)
	if !ok {
		t.Fatalf("err = %v; want a refusal by the schema", err)
	}
	for _, v := range vs {
		if v.Code == schema.CodePlacement {
			return
		}
	}
	t.Errorf("violations = %v; want a placement one", vs)
}

func TestAddWithoutAParentOnlyMakesProjects(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	// the kind is task when none is given, and a task is not at the top
	_, err := tasks.Add(ts.Session, tasks.AddInput{Title: "loose"})
	wantPlacement(t, err)
	for _, kind := range []string{"epic", "story", "bug", "step"} {
		_, err := tasks.Add(ts.Session, tasks.AddInput{Title: "loose " + kind, Kind: kind})
		wantPlacement(t, err)
	}
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "P", Kind: "project"}); err != nil {
		t.Fatalf("a project at the top: %v", err)
	}
}

func TestAddDefaultsToATask(t *testing.T) {
	if schema.DefaultKind != "task" {
		t.Fatalf("DefaultKind = %q", schema.DefaultKind)
	}
	t.Parallel()
	ts := usecasetest.New(t)
	p := ts.AddProject("P")
	r, err := tasks.Add(ts.Session, tasks.AddInput{Title: "t", Parent: p.String()})
	if err != nil {
		t.Fatal(err)
	}
	if v, err := r.View(); err != nil || v.Kind != "task" {
		t.Errorf("kind = %+v, %v; want task", v, err)
	}
}

func TestAddRefusesWhereAKindDoesNotGo(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	p := ts.AddProject("P")
	e, err := tasks.Add(ts.Session, tasks.AddInput{Title: "E", Kind: "epic", Parent: p.String()})
	if err != nil {
		t.Fatal(err)
	}
	task, err := tasks.Add(ts.Session, tasks.AddInput{Title: "T", Parent: e.ULID.String()})
	if err != nil {
		t.Fatal(err)
	}
	for name, in := range map[string]tasks.AddInput{
		"a project under a project": {Title: "x", Kind: "project", Parent: p.String()},
		"an epic under an epic":     {Title: "x", Kind: "epic", Parent: e.ULID.String()},
		"a task under a task":       {Title: "x", Parent: task.ULID.String()},
		"a step under an epic":      {Title: "x", Kind: "step", Parent: e.ULID.String()},
		"a step under a project":    {Title: "x", Kind: "step", Parent: p.String()},
	} {
		if _, err := tasks.Add(ts.Session, in); err == nil {
			t.Errorf("%s: no error", name)
		} else {
			wantPlacement(t, err)
		}
	}
	for name, in := range map[string]tasks.AddInput{
		"an epic under a project": {Title: "x", Kind: "epic", Parent: p.String()},
		"a bug under an epic":     {Title: "x", Kind: "bug", Parent: e.ULID.String()},
		"a story under a project": {Title: "x", Kind: "story", Parent: p.String()},
		"a step under a task":     {Title: "x", Kind: "step", Parent: task.ULID.String()},
	} {
		if _, err := tasks.Add(ts.Session, in); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestMoveRefusesWhereAKindDoesNotGo(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	p := ts.AddProject("P")
	e, err := tasks.Add(ts.Session, tasks.AddInput{Title: "E", Kind: "epic", Parent: p.String()})
	if err != nil {
		t.Fatal(err)
	}
	task, err := tasks.Add(ts.Session, tasks.AddInput{Title: "T", Parent: p.String()})
	if err != nil {
		t.Fatal(err)
	}
	step, err := tasks.Add(ts.Session, tasks.AddInput{Title: "S", Kind: "step", Parent: task.ULID.String()})
	if err != nil {
		t.Fatal(err)
	}
	// to the top of the workspace
	_, err = tasks.Move(ts.Session, tasks.MoveInput{Arg: task.ULID.String(), Root: true})
	wantPlacement(t, err)
	_, err = tasks.Move(ts.Session, tasks.MoveInput{Arg: e.ULID.String(), Root: true})
	wantPlacement(t, err)
	// a step goes under a story, task or bug only
	_, err = tasks.Move(ts.Session, tasks.MoveInput{Arg: step.ULID.String(), Parent: e.ULID.String()})
	wantPlacement(t, err)
	// a task under a task
	_, err = tasks.Move(ts.Session, tasks.MoveInput{Arg: task.ULID.String(), Parent: task.ULID.String()})
	if err == nil {
		t.Error("a task under itself is moved")
	}
	// what fits goes
	if _, err := tasks.Move(ts.Session, tasks.MoveInput{Arg: task.ULID.String(), Parent: e.ULID.String()}); err != nil {
		t.Errorf("a task under an epic: %v", err)
	}
}
