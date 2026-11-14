package planning_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/planning"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/item"
	dp "github.com/kukv/hedgehug/internal/domain/planning"
)

func backlogIDs(rows []planning.BacklogRow) []item.ID {
	var out []item.ID
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out
}

// backlogRows は、バックログの画面の最後の枠(バックログ)の行の ID。
func backlogRows(t *testing.T, svc planning.Service) []item.ID {
	t.Helper()
	frames, err := svc.Backlog(t.Context(), store, apptest.SampleWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	return backlogIDs(frames[len(frames)-1].Rows)
}

func TestBacklog(t *testing.T) {
	svc, _ := setup(t)
	ctx := t.Context()
	s1, s2 := activeSprint(t, svc)
	s3, err := svc.CreateSprint(ctx, store, apptest.SampleWorkspace, planning.SprintInput{Name: "Sprint 3"})
	if err != nil {
		t.Fatal(err)
	}
	const todo, done, cancel = apptest.SampleStatusTodo, apptest.SampleStatusDone, apptest.SampleStatusCancel
	addViewItems(t, svc,
		viewItem{itemA, 3, item.Task, "C", done, s1.ID, "", 3},
		viewItem{itemB, 4, item.Story, "B", cancel, s1.ID, "", 5},
		viewItem{itemC, 5, item.Bug, "A", todo, s1.ID, "", -1},
		viewItem{itemD, 6, item.Task, "W", done, "", "", 1}, // 完了はバックログに入らない
		viewItem{itemE, 7, item.Task, "X", todo, "", "", 2},
	)
	frames, err := svc.Backlog(ctx, store, apptest.SampleWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 4 || frames[0].Sprint.ID != s1.ID || frames[1].Sprint.ID != s2.ID || frames[2].Sprint.ID != s3.ID || frames[3].Sprint != nil {
		t.Fatalf("frames = %+v", frames)
	}
	if got := backlogIDs(frames[0].Rows); !sameIDs(got, itemC, itemB, itemA) {
		t.Errorf("S1 = %v", got)
	}
	if frames[0].Total != (dp.Totals{Points: 3, Count: 2}) {
		t.Errorf("S1 の Total = %+v", frames[0].Total)
	}
	if len(frames[1].Rows) != 0 {
		t.Errorf("S2 = %v", backlogIDs(frames[1].Rows))
	}
	if got := backlogIDs(frames[3].Rows); !sameIDs(got, apptest.SampleTask, itemE) {
		t.Errorf("バックログ = %v", got)
	}
	r := frames[3].Rows[0]
	if r.Display != "HH-2" || r.Kind != item.Task || r.Priority != apptest.SamplePriorityMedium || r.Project != apptest.SampleProject || r.Status != todo {
		t.Errorf("Row = %+v", r)
	}
}

func TestBacklogWorkspaceNotFound(t *testing.T) {
	svc, _ := setup(t)
	ctx := t.Context()
	if _, err := svc.Backlog(ctx, store, "01NOSUCHWS0000000000000000"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("ないワークスペース: %v", err)
	}
	if err := apptest.DeleteWorkspace(ctx, svc.Stores, store, apptest.SampleWorkspace); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Backlog(ctx, store, apptest.SampleWorkspace); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("削除済みのワークスペース: %v", err)
	}
}
