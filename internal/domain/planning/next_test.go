package planning_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/planning"
)

func TestNextSprint(t *testing.T) {
	sp := func(name string, state planning.SprintState, start string) planning.Sprint {
		return planning.Sprint{ID: change.ID(name), Name: name, State: state, Start: start}
	}
	deleted := sp("del", planning.Planned, "2026-10-01")
	deleted.Deleted = &change.Deletion{At: time.Now()}
	live := []planning.Sprint{
		sp("active", planning.Active, "2026-10-01"),
		sp("closed", planning.Closed, "2026-09-01"),
		deleted,
		sp("dec", planning.Planned, "2026-12-01"),
		sp("none", planning.Planned, ""),
		sp("nov", planning.Planned, "2026-11-20"),
	}
	if got := planning.NextSprint(live); got == nil || got.Name != "nov" {
		t.Fatalf("NextSprint = %+v, want nov", got)
	}

	tie := []planning.Sprint{sp("b", planning.Planned, "2026-11-20"), sp("a", planning.Planned, "2026-11-20")}
	if got := planning.NextSprint(tie); got == nil || got.Name != "a" {
		t.Fatalf("tie = %+v, want a (ScreenOrder)", got)
	}

	if got := planning.NextSprint(live[:3]); got != nil {
		t.Fatalf("no planned = %+v, want nil", got)
	}
}
