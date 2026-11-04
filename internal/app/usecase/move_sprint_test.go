package usecase

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
)

func TestMoveToWorkspaceDetachesSprintAndReleases(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	ts.addWorkspace("api", "API")
	me := ts.workspaceULID("me")
	mySprint, myRelease := ts.putSprintRelease(me, "S3", "v1")
	gone := model.NewULID()
	ts.writeAndCommit(model.ReleasePath(gone), "this is not a release\n") // a file that cannot be read
	ts.addTask("parent", func(t *model.Task) { t.Workspace, t.Sprint, t.Releases = me, &mySprint, []model.ULID{myRelease} })
	ts.addTask("dangling", func(t *model.Task) { t.Workspace, t.Releases = me, []model.ULID{gone} })

	// The sprint S3 and the release v1 are of "me", not of "api", so they go.
	res, err := ts.Move(MoveInput{Arg: "ME-1", Workspace: "api"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Detached) != 1 {
		t.Fatalf("Detached = %+v; want one task", res.Detached)
	}
	d := res.Detached[0]
	if d.Task != res.Tasks[0].ULID || d.ID != "API-1" || d.Sprint != "S3" || !slices.Equal(d.Releases, []string{"v1"}) {
		t.Errorf("Detached[0] = %+v; want API-1 without sprint S3 and release v1", d)
	}
	s, _, _ := ts.Open()
	got, err := s.Task(res.Tasks[0].ULID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Sprint != nil || len(got.Releases) != 0 {
		t.Errorf("task keeps sprint %v releases %v; want none", got.Sprint, got.Releases)
	}

	// A release whose file cannot be read is told by its ULID, and goes too.
	res, err = ts.Move(MoveInput{Arg: "ME-2", Workspace: "api"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Detached) != 1 || !slices.Equal(res.Detached[0].Releases, []string{gone.String()}) {
		t.Errorf("Detached = %+v; want the missing release by its ULID", res.Detached)
	}
}

func TestMoveToWorkspaceDetachesFromSubtree(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	ts.addWorkspace("api", "API")
	me := ts.workspaceULID("me")
	sp, _ := ts.putSprintRelease(me, "S3", "v1")
	parent := ts.addTask("parent", func(t *model.Task) { t.Workspace = me })
	ts.addTask("kid", func(t *model.Task) { t.Workspace, t.Parent, t.Sprint = me, &parent.ULID, &sp })
	res, err := ts.Move(MoveInput{Arg: "ME-1", Workspace: "api"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Detached) != 1 || res.Detached[0].ID != "API-2" || res.Detached[0].Sprint != "S3" {
		t.Errorf("Detached = %+v; want only the child, API-2, without S3", res.Detached)
	}
}

func TestMoveWithinWorkspaceKeepsSprint(t *testing.T) {
	t.Parallel()
	ts := newTestSession(t)
	me := ts.workspaceULID("me")
	sp, rel := ts.putSprintRelease(me, "S3", "v1")
	ts.addTask("a", func(t *model.Task) { t.Workspace, t.Sprint, t.Releases = me, &sp, []model.ULID{rel} })
	ts.addTask("b", func(t *model.Task) { t.Workspace = me })
	res, err := ts.Move(MoveInput{Arg: "ME-1", After: "ME-2"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Moved || len(res.Detached) != 0 {
		t.Errorf("moved %v detached %+v; want neither", res.Moved, res.Detached)
	}
	s, _, _ := ts.Open()
	got, _ := s.Task(res.Tasks[0].ULID)
	if got.Sprint == nil || len(got.Releases) != 1 {
		t.Errorf("sprint %v releases %v; want both kept", got.Sprint, got.Releases)
	}
}
