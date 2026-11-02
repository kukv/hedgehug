package merge

import (
	"slices"
	"testing"
	"time"
)

func TestTaskChanges(t *testing.T) {
	t.Parallel()
	a := decoded(t, task("2026-10-31T10:00:00Z", "title: a", "status: todo", "props:\n  priority: low\n  estimate: 1"))
	b := decoded(t, task("2026-10-31T11:00:00Z", "title: b", "status: todo", "props:\n  priority: high\n  estimate: 1\n  area: web"))
	b.ID, b.FormerIDs = "WEB-2", []string{"WEB-1"}
	got := TaskChanges(a, b)
	want := []string{"title", "props.priority", "props.area"}
	if !slices.Equal(got, want) {
		t.Errorf("changes = %v, want %v", got, want)
	}
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	b.DeletedAt = &now
	if got := TaskChanges(a, b); got[len(got)-1] != "deleted_at" {
		t.Errorf("changes = %v, want deleted_at last", got)
	}
}
