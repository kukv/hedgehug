package tasks_test

import (
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

// A sibling without a Parent decides the parent and the workspace.
func TestAddNextToSiblingTakesItsPlace(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.AddWorkspace("api", "API")
	me := ts.WorkspaceULID("me")
	api := ts.WorkspaceULID("api")
	for _, in := range []tasks.AddInput{{Title: "p", Workspace: "me"}, {Title: "k1", Parent: "ME-1"}, {Title: "k2", Parent: "ME-1"}, {Title: "x", Workspace: "api"}} {
		if _, err := tasks.Add(ts.Session, in); err != nil {
			t.Fatal(err)
		}
	}
	// two workspaces: no Workspace needed, the sibling's is taken
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "mid", After: "ME-2"}); err != nil {
		t.Fatal(err)
	}
	p := usecasetest.UlidOf(t, ts, "ME-1")
	if got, want := strings.Join(ts.Order(me, &p), " "), "ME-2 ME-4 ME-3"; got != want {
		t.Errorf("order under the parent = %s; want %s", got, want)
	}
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "root", Before: "API-1"}); err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(ts.Order(api, nil), " "), "API-2 API-1"; got != want {
		t.Errorf("order in the sibling's workspace = %s; want %s", got, want)
	}
	// a Workspace or a Parent that does not fit the sibling is refused
	_, err := tasks.Add(ts.Session, tasks.AddInput{Title: "t", Workspace: "me", After: "API-1"})
	usecasetest.WantErr(t, err, usecase.KindInvalid, "add.position")
	_, err = tasks.Add(ts.Session, tasks.AddInput{Title: "t", Workspace: "api", After: "ME-2"})
	usecasetest.WantErr(t, err, usecase.KindInvalid, "add.position")
	_, err = tasks.Add(ts.Session, tasks.AddInput{Title: "t", Parent: "ME-1", After: "ME-1"})
	usecasetest.WantErr(t, err, usecase.KindInvalid, "add.position")
}
