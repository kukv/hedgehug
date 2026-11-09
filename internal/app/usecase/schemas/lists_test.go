package schemas_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/schemas"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestEditPrioritiesRenamesMergesAndRefusesInUse(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	u := ts.AddTask("urgent", func(t *model.Task) { t.Priority = "urgent" })
	m := ts.AddTask("mid", func(t *model.Task) { t.Priority = "mid" })
	ts.AddTask("none", nil)

	// urgent goes into high, mid is renamed, low goes (nothing uses it)
	before := ts.Head()
	w, err := schemas.EditPriorities(ts.Session, []schema.ValueEdit{
		{Name: "high", From: []string{"high", "urgent"}},
		{Name: "medium", From: []string{"mid"}},
	})
	if err != nil || w.Commit == nil {
		t.Fatalf("EditPriorities = %+v, %v", w, err)
	}
	if !slices.Equal(w.Schema.Priorities, []string{"high", "medium"}) {
		t.Errorf("priorities = %v", w.Schema.Priorities)
	}
	if got := taskNow(t, ts, u.ULID).Priority; got != "high" {
		t.Errorf("urgent task: priority = %q, want high", got)
	}
	if got := taskNow(t, ts, m.ULID).Priority; got != "medium" {
		t.Errorf("mid task: priority = %q, want medium", got)
	}
	if usecasetest.LastOp(t, ts) != "schema" || parentOfHead(t, ts) != before {
		t.Errorf("want one commit of op schema on top of %s", before)
	}

	// medium is in use, so taking it off is refused, naming the value
	before = ts.Head()
	_, err = schemas.EditPriorities(ts.Session, []schema.ValueEdit{{Name: "high", From: []string{"high"}}})
	var ue *usecase.Error
	if usecase.ProblemCode(err) != "schema.in_use" || !errors.As(err, &ue) || ue.Args["value"] != "medium" || ue.Args["count"] != 1 {
		t.Fatalf("err = %v", err)
	}
	if ts.Head() != before {
		t.Error("a refused edit committed")
	}
}

func TestEditPrioritiesRefusesTheLastOneAndCaseTwins(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	_, err := schemas.EditPriorities(ts.Session, nil)
	usecasetest.WantErr(t, err, usecase.KindInvalid, "schema.value_invalid")
	_, err = schemas.EditPriorities(ts.Session, []schema.ValueEdit{{Name: "high", From: []string{"high"}}, {Name: "HIGH"}})
	usecasetest.WantErr(t, err, usecase.KindInvalid, "schema.value_invalid")
}

func TestEditLabels(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	// the store starts with the labels of usecasetest.TestLabels: bug, ui and api among them
	a := ts.AddTask("a", func(t *model.Task) { t.Labels = []string{"bug", "ui"} })
	b := ts.AddTask("b", func(t *model.Task) { t.Labels = []string{"api"} })
	// A deleted task keeps its labels, and they count.
	gone := time.Now()
	ts.AddTask("gone", func(t *model.Task) { t.Labels = []string{"ui"}; t.DeletedAt = &gone })

	// api becomes bug (a merge), ui is renamed, in the order given
	w, err := schemas.EditLabels(ts.Session, []schema.ValueEdit{
		{Name: "ui", From: []string{"ui"}},
		{Name: "defect", From: []string{"bug", "api"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(w.Schema.Labels, []string{"ui", "defect"}) {
		t.Errorf("labels = %v", w.Schema.Labels)
	}
	if got := taskNow(t, ts, a.ULID).Labels; !slices.Equal(got, []string{"defect", "ui"}) {
		t.Errorf("a: labels = %v", got)
	}
	if got := taskNow(t, ts, b.ULID).Labels; !slices.Equal(got, []string{"defect"}) {
		t.Errorf("b: labels = %v", got)
	}

	// ui is used by a task that is deleted: it cannot go
	_, err = schemas.RemoveLabels(ts.Session, []string{"ui"})
	var ue *usecase.Error
	if usecase.ProblemCode(err) != "schema.in_use" || !errors.As(err, &ue) || ue.Args["count"] != 2 || ue.Args["deleted"] != 1 {
		t.Fatalf("RemoveLabels of a label in use: %v", err)
	}

	// the list may end up empty
	if _, err := schemas.EditLabels(ts.Session, []schema.ValueEdit{{Name: "defect", From: []string{"defect"}}, {Name: "ui", From: []string{"ui"}}}); err != nil {
		t.Fatal(err)
	}
}

func TestAddAndRemoveLabels(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	// the store starts with the labels of usecasetest.TestLabels
	w, err := schemas.AddLabels(ts.Session, []string{"fresh", "Other"})
	if err != nil || !slices.Equal(w.Schema.Labels[len(usecasetest.TestLabels):], []string{"fresh", "Other"}) {
		t.Fatalf("AddLabels = %+v, %v", w, err)
	}
	// the same label again, written in another case, is a twin
	_, err = schemas.AddLabels(ts.Session, []string{"BUG"})
	usecasetest.WantErr(t, err, usecase.KindInvalid, "schema.value_invalid")
	_, err = schemas.AddLabels(ts.Session, []string{"x", "X"})
	usecasetest.WantErr(t, err, usecase.KindInvalid, "schema.value_invalid")

	used := ts.AddTask("t", func(t *model.Task) { t.Labels = []string{"ui"} })
	before := ts.Head()
	_, err = schemas.RemoveLabels(ts.Session, []string{"ui"})
	usecasetest.WantErr(t, err, usecase.KindInvalid, "schema.in_use")
	_, err = schemas.RemoveLabels(ts.Session, []string{"nope"})
	usecasetest.WantErr(t, err, usecase.KindNotFound, "schema.not_defined")
	if ts.Head() != before {
		t.Error("a refused removal committed")
	}

	// a label is found without regard to case
	w, err = schemas.RemoveLabels(ts.Session, []string{"BUG", "FRESH"})
	if err != nil || slices.Contains(w.Schema.Labels, "bug") || slices.Contains(w.Schema.Labels, "fresh") || !slices.Contains(w.Schema.Labels, "ui") || !slices.Contains(w.Schema.Labels, "Other") {
		t.Fatalf("RemoveLabels = %+v, %v", w, err)
	}
	if got := taskNow(t, ts, used.ULID).Labels; !slices.Equal(got, []string{"ui"}) {
		t.Errorf("the task lost its label: %v", got)
	}
}
