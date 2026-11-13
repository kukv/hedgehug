package projects_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/projects"
)

// 削除済みのプロジェクトは、一覧にも Get にも出ず、変更もできない(A4b-4)。
func TestDeletedProjectIsHiddenAndCannotBeChanged(t *testing.T) {
	svc := setup(t)
	ctx := t.Context()
	if err := apptest.DeleteProject(ctx, svc.Stores, store, apptest.SampleProject); err != nil {
		t.Fatal(err)
	}
	if list, err := svc.List(ctx, store, apptest.SampleWorkspace); err != nil || len(list) != 0 {
		t.Errorf("List = %+v, %v", list, err)
	}
	_, getErr := svc.Get(ctx, store, apptest.SampleProject)
	_, renameErr := svc.Rename(ctx, store, apptest.SampleProject, "新しい")
	_, ownerErr := svc.SetOwner(ctx, store, apptest.SampleProject, "hanako")
	for name, err := range map[string]error{"Get": getErr, "Rename": renameErr, "SetOwner": ownerErr} {
		if !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s of a deleted project: %v, want port.ErrNotFound", name, err)
		}
	}
}

// 名前の重複と、並びの末尾の決め方は、削除済みのプロジェクトを数えない。番号は削除済みも数える(A4b-5)。
func TestDeletedProjectsAreNotComparedForNamesAndRanks(t *testing.T) {
	svc := setup(t)
	ctx := t.Context()
	in := projects.CreateInput{Store: store, Workspace: apptest.SampleWorkspace, Title: "決済"}
	gone, err := svc.Create(ctx, in) // HH-P2(W)
	if err != nil || gone.Rank != "W" {
		t.Fatalf("created %+v, %v", gone, err)
	}
	if err := apptest.DeleteProject(ctx, svc.Stores, store, gone.ID); err != nil {
		t.Fatal(err)
	}
	// 削除済みの「決済」と同じ名前を作れ、並びは削除済みの W を見ずに末尾(W)になり、番号は再利用しない。
	again, err := svc.Create(ctx, in)
	if err != nil || again.Rank != "W" || again.Place.Number != 3 {
		t.Errorf("created %+v, %v; want rank W and number 3", again, err)
	}
	// 名前の変更でも、削除済みの名前は重複にならない。
	if err := apptest.DeleteProject(ctx, svc.Stores, store, again.ID); err != nil {
		t.Fatal(err)
	}
	renamed, err := svc.Rename(ctx, store, apptest.SampleProject, "決済")
	if err != nil || renamed.Title != "決済" {
		t.Errorf("Rename = %+v, %v", renamed, err)
	}
}

// 削除済みのワークスペースには、プロジェクトを作れず、一覧も読めない(B5b-4)。
func TestProjectsOfDeletedWorkspace(t *testing.T) {
	svc := setup(t)
	ctx := t.Context()
	if err := apptest.DeleteWorkspace(ctx, svc.Stores, store, apptest.SampleWorkspace); err != nil {
		t.Fatal(err)
	}
	_, createErr := svc.Create(ctx, projects.CreateInput{Store: store, Workspace: apptest.SampleWorkspace, Title: "新しい"})
	_, listErr := svc.List(ctx, store, apptest.SampleWorkspace)
	for name, err := range map[string]error{"Create": createErr, "List": listErr} {
		if !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s in a deleted workspace: %v, want port.ErrNotFound", name, err)
		}
	}
}
