package server

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestMoveTaskToWorkspaceTellsWhatItTookOff(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	id := ts.add("one", "")
	ts.call("POST", "/api/workspaces", map[string]any{"name": "web", "prefix": "WEB"}, 201, nil)
	u := usecasetest.UlidOf(t, ts.uc, id)
	ws := ts.uc.TaskByULID("me", u).Workspace
	sp := &model.Sprint{ULID: model.NewULID(), Workspace: ws, Name: "S3"}
	rel := &model.Release{ULID: model.NewULID(), Workspace: ws, Name: "v1"}
	if err := ts.uc.EditTask("me", "update", u, usecasetest.AccountAuthor, func(tx *usecasetest.Tx) error {
		task, err := tx.Task(u)
		if err != nil {
			return err
		}
		task.Sprint, task.Releases = &sp.ULID, []model.ULID{rel.ULID}
		return errors.Join(tx.PutSprint(sp), tx.PutRelease(rel), tx.PutTask(task))
	}); err != nil {
		t.Fatal(err)
	}

	var out struct {
		Task     *usecase.TaskView  `json:"task"`
		Detached []usecase.Detached `json:"detached"`
	}
	ts.call("POST", "/api/tasks/"+id+"/move", map[string]any{"workspace": "web"}, 200, &out)
	if len(out.Detached) != 1 {
		t.Fatalf("detached = %+v; want one task", out.Detached)
	}
	d := out.Detached[0]
	if d.Task.String() != out.Task.ULID || d.ID != out.Task.ID || d.Sprint != "S3" || !slices.Equal(d.Releases, []string{"v1"}) {
		t.Errorf("detached = %+v for task %s %s", d, out.Task.ID, out.Task.ULID)
	}

	// A move that takes nothing off says nothing.
	var again struct {
		Detached []usecase.Detached `json:"detached"`
	}
	ts.call("POST", "/api/tasks/"+out.Task.ID+"/move", map[string]any{"root": true}, 200, &again)
	if len(again.Detached) != 0 {
		t.Errorf("detached = %+v; want none", again.Detached)
	}
}
