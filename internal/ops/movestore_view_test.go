package ops

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// The confirmation leaves ops as a MoveView: ulid, id and title of each
// task, the references cut and who loses sight; the comments, worklogs and
// bodies of the tasks are not in it.
func TestMoveConfirmationHoldsOnlyTheView(t *testing.T) {
	f := newMoveFx(t) // API-3 has a comment "first" and a worklog "work"
	_, err := f.ts.MoveStore(MoveStoreInput{Arg: "API-2", To: "org-b"})
	mustCode(t, err, KindUsage, "confirm.required")
	view, ok := argsOf(err)["plan"].(MoveView)
	if !ok {
		t.Fatalf("plan = %T, want a MoveView", argsOf(err)["plan"])
	}
	if view.From != "org-a" || view.To != "org-b" || len(view.Tasks) != 3 {
		t.Fatalf("view = %+v", view)
	}
	root := view.Tasks[0]
	if root.ULID != f.p || root.ID != "API-2" || root.Title != "P" {
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

// PrepareMoveStore is the entry for a caller that asks in its own way: it
// returns the same view.
func TestPrepareMoveStoreReturnsTheView(t *testing.T) {
	f := newMoveFx(t)
	v, err := f.ts.PrepareMoveStore(f.in())
	if err != nil {
		t.Fatal(err)
	}
	if v.From != "org-a" || v.To != "org-b" || len(v.Tasks) != 3 || v.Tasks[0].ULID != f.p {
		t.Errorf("view = %+v", v)
	}
}

// The words for what a scoped schema loses are ops's, for every client.
func TestScopedPlanLines(t *testing.T) {
	var none *ScopedPlan
	if len(none.Lines("org-b")) != 0 {
		t.Error("a nil plan has lines")
	}
	sp := &ScopedPlan{Dropped: []string{"status review", "transitions backlog", "required todo", "prop gone", "template nope", "kind alien"}}
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
