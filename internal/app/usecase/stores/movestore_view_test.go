package stores_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/stores"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// The confirmation leaves usecase as a MoveView: ulid, id and title of each
// task, the references cut and who loses sight; the comments, worklogs and
// bodies of the tasks are not in it.
func TestMoveConfirmationHoldsOnlyTheView(t *testing.T) {
	t.Parallel()
	f := usecasetest.NewMoveFx(t) // API-3 has a comment "first" and a worklog "work"
	_, err := stores.MoveStore(f.TS.Session, stores.MoveStoreInput{Arg: "API-2", To: "org-b"})
	usecasetest.MustCode(t, err, usecase.KindUsage, "confirm.required")
	view, ok := argsOf(err)["plan"].(stores.MoveView)
	if !ok {
		t.Fatalf("plan = %T, want a MoveView", argsOf(err)["plan"])
	}
	if view.From != "org-a" || view.To != "org-b" || len(view.Tasks) != 3 {
		t.Fatalf("view = %+v", view)
	}
	root := view.Tasks[0]
	if root.ULID != f.P || root.ID != "API-2" || root.Title != "P" {
		t.Errorf("root = %+v", root)
	}
	if !slices.Contains(view.Unlinked, "unlink: API-4 blocked_by API-1") {
		t.Errorf("unlinked = %q", view.Unlinked)
	}
	b, err := json.Marshal(argsOf(err))
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"first", "work", "Comments", "Worklogs", "Text"} {
		if strings.Contains(string(b), leak) {
			t.Errorf("the args hold %q: %s", leak, b)
		}
	}
	if len(b) > 2000 {
		t.Errorf("the args are %d bytes: %s", len(b), b)
	}
}

// The words for what a scoped schema loses are usecase's, for every client.
func TestScopedPlanLines(t *testing.T) {
	t.Parallel()
	var none *stores.ScopedPlan
	if len(none.Lines("org-b")) != 0 {
		t.Error("a nil plan has lines")
	}
	sp := &stores.ScopedPlan{Dropped: []string{"status review", "transitions backlog", "required todo", "prop gone", "template nope", "kind alien"}}
	want := []string{
		`scoped: dropped status "review" (not in org-b)`,
		`scoped: dropped transitions from "backlog" (no target in org-b)`,
		`scoped: dropped the required fields of "todo" (none of them in org-b)`,
		`scoped: dropped prop "gone" (not in org-b)`,
		`scoped: dropped template "nope" (not in org-b)`,
		`scoped: dropped kind "alien" (not in org-b)`,
	}
	if got := sp.Lines("org-b"); !slices.Equal(got, want) {
		t.Errorf("lines = %q, want %q", got, want)
	}
}
