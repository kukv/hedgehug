package stores_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/schema"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/schemas"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// The priority and the labels of a task that moves are checked against the
// lists of the destination, without regard to case; one it lacks is 422
// move.invalid, and a match is written as the destination spells it.
func TestMoveStoreChecksThePrioritiesAndLabelsOfTheDestination(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t)
	a, b := f.TS.ForStore("org-a"), f.TS.ForStore("org-b")
	if _, err := tasks.Set(a, "API-3", []usecase.Assign{
		{Key: "priority", Op: "=", Value: "high"}, {Key: "labels", Op: "+=", Value: "bug"}, {Key: "labels", Op: "+=", Value: "ui"},
	}); err != nil {
		t.Fatal(err)
	}
	// org-b has no high priority and no ui label
	if _, err := schemas.EditPriorities(b, []schema.ValueEdit{{Name: "low", From: []string{"low"}}, {Name: "mid", From: []string{"mid"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := schemas.EditLabels(b, []schema.ValueEdit{{Name: "BUG", From: []string{"bug"}}}); err != nil {
		t.Fatal(err)
	}
	ha, hb := f.Head(t, "org-a"), f.Head(t, "org-b")

	_, err := stores.MoveStore(f.TS.Session, f.In())
	usecasetest.MustCode(t, err, usecase.KindInvalid, "move.invalid")
	lines, _ := argsOf(err)["violations"].([]string)
	if len(lines) != 2 || !strings.Contains(lines[0], "API-3") || !strings.Contains(lines[0], "priority") || !strings.Contains(lines[1], "API-3") || !strings.Contains(lines[1], "ui") {
		t.Fatalf("violations = %q; want the priority and the label of API-3", lines)
	}
	if f.Head(t, "org-a") != ha || f.Head(t, "org-b") != hb {
		t.Error("a refused move committed")
	}

	// put them on the list of org-b, in the case it likes: the task is written so
	if _, err := schemas.EditPriorities(b, []schema.ValueEdit{{Name: "low", From: []string{"low"}}, {Name: "HIGH"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := schemas.AddLabels(b, []string{"UI"}); err != nil {
		t.Fatal(err)
	}
	if _, err := stores.MoveStore(f.TS.Session, f.In()); err != nil {
		t.Fatal(err)
	}
	e, ok := usecasetest.MustIndex(t, f.TS, "org-b").Task(f.C)
	if !ok {
		t.Fatal("API-3 did not arrive in org-b")
	}
	if e.Priority != "HIGH" || !slices.Equal(e.Labels, []string{"BUG", "UI"}) {
		t.Errorf("priority = %q, labels = %v; want HIGH and [BUG UI], as org-b spells them", e.Priority, e.Labels)
	}
}
