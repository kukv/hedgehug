package convert

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/upgrade"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

// The places that read a task of a past commit read it through
// domain/upgrade, so that a commit from before the conversion shows the task
// as it would be now. These tests read the commit of a store in format 1 after
// the store was converted: the task of the custom kind chore is a task.

// The steps of hh store convert and the steps that read one task file name
// the same formats.
func TestEveryStepHasAStepForOneTask(t *testing.T) {
	t.Parallel()
	from := func(ss []Step) []int {
		var out []int
		for _, s := range ss {
			out = append(out, s.From)
		}
		return out
	}
	var tasksFrom []int
	for _, s := range upgrade.Steps {
		tasksFrom = append(tasksFrom, s.From)
	}
	if !slices.Equal(from(steps), tasksFrom) {
		t.Errorf("steps of the store: formats %v; steps of one task: formats %v", from(steps), tasksFrom)
	}
}

func TestShowAtReadsACommitBeforeTheConversion(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	o := makeOld(t, ts)
	old := headOf(t, ts)
	if _, err := Run(ts.Session, Options{}, false); err != nil {
		t.Fatal(err)
	}
	v, err := tasks.ShowAt(ts.Session, o.sharp.ID, old)
	if err != nil {
		t.Fatal(err)
	}
	if v.Kind != "task" || v.Title != "a chore" {
		t.Errorf("kind %q, title %q; want task, a chore", v.Kind, v.Title)
	}
}

func TestRestoreAtReadsACommitBeforeTheConversion(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	o := makeOld(t, ts)
	old := headOf(t, ts)
	if _, err := Run(ts.Session, Options{}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Set(ts.Session, o.sharp.ID, []usecase.Assign{{Key: "title", Op: "=", Value: "changed"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.RestoreAt(ts.Session, o.sharp.ID, old); err != nil {
		t.Fatal(err)
	}
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	e, ok := ix.Task(o.sharp.ULID)
	if !ok || e.Kind != "task" || e.Title != "a chore" {
		t.Errorf("after restore: %+v; want kind task, title a chore", e)
	}
}

// Undo takes back a commit of format 1 that another writer's conversion came
// after: the version before it and the one it wrote are read as the store is
// now, so the kind that the conversion changed is not taken for a change
// someone made after it.
func TestUndoReadsCommitsBeforeTheConversion(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AppendConfig(usecasetest.AliceAccount)
	o := makeOld(t, ts)
	o.sharp.Title = "edited"
	b, err := o.sharp.Encode()
	if err != nil {
		t.Fatal(err)
	}
	commitRawWith(t, ts, "set ME-6 edited", map[string]string{model.TaskPath(o.sharp.ULID): string(b)}, []gitrepo.Trailer{
		{Key: "Hedgehug-Op", Value: "set"}, {Key: "Hedgehug-Author", Value: "github.com/alice"},
	})
	ts.AsAccount("bob")
	if _, err := Run(ts.Session, Options{}, false); err != nil {
		t.Fatal(err)
	}
	ts.AsAccount("alice")
	res, err := tasks.Undo(ts.Session)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Found || len(res.Kept) != 0 {
		t.Errorf("found %v, kept %+v; want found, nothing kept", res.Found, res.Kept)
	}
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	e, ok := ix.Task(o.sharp.ULID)
	if !ok || e.Kind != "task" || e.Title != "a chore" {
		t.Errorf("after undo: %+v; want kind task, title a chore", e)
	}
}

// The conversion itself is not what undo takes back: right after hh store
// convert, undo takes back the operation of the writer before it.
func TestUndoAfterTheConversionTakesBackTheOperationBeforeIt(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AppendConfig(usecasetest.AliceAccount)
	o := makeOld(t, ts)
	o.sharp.Title = "edited"
	b, err := o.sharp.Encode()
	if err != nil {
		t.Fatal(err)
	}
	commitRawWith(t, ts, "set ME-6 edited", map[string]string{model.TaskPath(o.sharp.ULID): string(b)}, []gitrepo.Trailer{
		{Key: "Hedgehug-Op", Value: "set"}, {Key: "Hedgehug-Author", Value: "github.com/alice"},
	})
	if _, err := Run(ts.Session, Options{}, false); err != nil {
		t.Fatal(err)
	}
	if op := usecasetest.LastOp(t, ts); op != "convert" {
		t.Fatalf("last op = %q, want convert", op)
	}
	res, err := tasks.Undo(ts.Session)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Found || tasks.UndoSubject(res.Undone.Subject) != "set ME-6 edited" {
		t.Errorf("undone = %+v; want the set before the conversion", res.Undone)
	}
	if formatOf(t, ts) != model.StoreFormat {
		t.Errorf("format = %d; the conversion was taken back", formatOf(t, ts))
	}
	_, ix, err := ts.Open()
	if err != nil {
		t.Fatal(err)
	}
	if e, ok := ix.Task(o.sharp.ULID); !ok || e.Title != "a chore" || e.Kind != "task" {
		t.Errorf("after undo: %+v", e)
	}
}
