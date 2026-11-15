package projects_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/projects"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
)

// setupMove は、同じワークスペースにプロジェクト A(サンプルの「ログイン」)・B・C をこの順で並べる。
func setupMove(t *testing.T) (projects.Service, *recorder, [3]project.Project) {
	t.Helper()
	svc, rec := setupEdit(t)
	ps := [3]project.Project{}
	ps[0], _ = svc.Get(t.Context(), store, apptest.SampleProject)
	for i, title := range []string{"B", "C"} {
		p, err := svc.Create(t.Context(), projects.CreateInput{Store: store, Workspace: apptest.SampleWorkspace, Title: title})
		if err != nil {
			t.Fatal(err)
		}
		ps[i+1] = p
	}
	return svc, rec, ps
}

func order(t *testing.T, svc projects.Service) []project.ID {
	t.Helper()
	list, err := svc.List(t.Context(), store, apptest.SampleWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	var ids []project.ID
	for _, p := range list {
		ids = append(ids, p.ID)
	}
	return ids
}

func logLen(t *testing.T, svc projects.Service, id project.ID) int {
	t.Helper()
	var n int
	err := svc.Stores.Read(t.Context(), store, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindProject, ID: id})
		n = len(log)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestMoveProjectUp(t *testing.T) {
	svc, rec, ps := setupMove(t)
	if err := svc.MoveProject(t.Context(), store, ps[1].ID, true); err != nil {
		t.Fatal(err)
	}
	if got, want := order(t, svc), []project.ID{ps[1].ID, ps[0].ID, ps[2].ID}; !slices.Equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
	if got, want := rec.last(), "B の並びを変更"; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
}

func TestMoveProjectEdgesWriteNothing(t *testing.T) {
	svc, _, ps := setupMove(t)
	before := [2]int{logLen(t, svc, ps[0].ID), logLen(t, svc, ps[2].ID)}
	if err := svc.MoveProject(t.Context(), store, ps[0].ID, true); err != nil {
		t.Errorf("先頭で上へ = %v", err)
	}
	if err := svc.MoveProject(t.Context(), store, ps[2].ID, false); err != nil {
		t.Errorf("末尾で下へ = %v", err)
	}
	if after := [2]int{logLen(t, svc, ps[0].ID), logLen(t, svc, ps[2].ID)}; after != before {
		t.Errorf("log lengths = %v, want %v", after, before)
	}
}

func TestMoveProjectOnlyMovedRankChanges(t *testing.T) {
	svc, _, ps := setupMove(t)
	if err := svc.MoveProject(t.Context(), store, ps[1].ID, false); err != nil {
		t.Fatal(err)
	}
	for _, i := range []int{0, 2} {
		got, err := svc.Get(t.Context(), store, ps[i].ID)
		if err != nil || got.Rank != ps[i].Rank {
			t.Errorf("project %d rank = %q, %v, want %q", i, got.Rank, err, ps[i].Rank)
		}
	}
	if got, want := order(t, svc), []project.ID{ps[0].ID, ps[2].ID, ps[1].ID}; !slices.Equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

// rank のないプロジェクトは先頭に並ぶ。動かすと、同じ保存で、rank のないものに表示の順で rank を振ってから動かす。
func TestMoveProjectWithoutRanks(t *testing.T) {
	svc, _ := setupEdit(t)
	old := []project.Project{
		{ID: "01OLD0000000000000000000A1", Place: project.Place{Workspace: apptest.SampleWorkspace, Number: 2}, Title: "旧A", Status: project.Todo},
		{ID: "01OLD0000000000000000000B1", Place: project.Place{Workspace: apptest.SampleWorkspace, Number: 3}, Title: "旧B", Status: project.Todo},
	}
	op := port.Operation{Actor: change.User("taro"), Summary: "準備"}
	if err := svc.Stores.Write(t.Context(), store, op, func(tx port.Tx) error {
		for _, p := range old {
			if err := tx.WriteProjects().Append(p.ID, project.Create(p)); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got, want := order(t, svc), []project.ID{old[0].ID, old[1].ID, apptest.SampleProject}; !slices.Equal(got, want) {
		t.Fatalf("before move = %v, want %v", got, want)
	}
	if err := svc.MoveProject(t.Context(), store, old[1].ID, true); err != nil {
		t.Fatal(err)
	}
	if got, want := order(t, svc), []project.ID{old[1].ID, old[0].ID, apptest.SampleProject}; !slices.Equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
	// rank を持つサンプルのプロジェクトは書き換えない。
	if n := logLen(t, svc, apptest.SampleProject); n != 1 {
		t.Errorf("ranked project was written: %d rows", n)
	}
}

// 同じ rank("V")の 2 件は内部 ID の順に並ぶ。動かすと、後ろの同じ rank を振り直してから動かす。
func TestMoveProjectEqualRanks(t *testing.T) {
	svc, _ := setupEdit(t)
	twin := project.Project{ID: "01ZZZZ00000000000000000001", Place: project.Place{Workspace: apptest.SampleWorkspace, Number: 2}, Rank: "V", Title: "双子", Status: project.Todo}
	op := port.Operation{Actor: change.User("taro"), Summary: "準備"}
	if err := svc.Stores.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteProjects().Append(twin.ID, project.Create(twin))
	}); err != nil {
		t.Fatal(err)
	}
	if got, want := order(t, svc), []project.ID{apptest.SampleProject, twin.ID}; !slices.Equal(got, want) {
		t.Fatalf("before move = %v, want %v", got, want)
	}
	if err := svc.MoveProject(t.Context(), store, apptest.SampleProject, false); err != nil {
		t.Fatal(err)
	}
	if got, want := order(t, svc), []project.ID{twin.ID, apptest.SampleProject}; !slices.Equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
	moved, err1 := svc.Get(t.Context(), store, apptest.SampleProject)
	refilled, err2 := svc.Get(t.Context(), store, twin.ID)
	if err := errors.Join(err1, err2); err != nil {
		t.Fatal(err)
	}
	// 双子は前のもの(V)より大きい値に振り直し、動かしたものはその後ろの値にする。
	if refilled.Rank != "W" || moved.Rank != "X" {
		t.Errorf("ranks = %q, %q, want W, X", refilled.Rank, moved.Rank)
	}
}

func TestMoveProjectOtherWorkspaceUntouched(t *testing.T) {
	svc, _, ps := setupMove(t)
	if err := apptest.SampleMobile(t.Context(), svc.Stores, store); err != nil {
		t.Fatal(err)
	}
	before := logLen(t, svc, apptest.SampleMobileProject)
	if err := svc.MoveProject(t.Context(), store, ps[2].ID, true); err != nil {
		t.Fatal(err)
	}
	if got := logLen(t, svc, apptest.SampleMobileProject); got != before {
		t.Errorf("other workspace log = %d, want %d", got, before)
	}
	list, err := svc.List(t.Context(), store, apptest.SampleMobileWorkspace)
	if err != nil || len(list) != 1 || list[0].ID != apptest.SampleMobileProject {
		t.Errorf("other workspace = %+v, %v", list, err)
	}
}

// 削除済みのプロジェクトの扱いは Rename と同じ。root: true は取り消してから動かし、そうでなければ ErrNotFound。
func TestMoveProjectDeleted(t *testing.T) {
	svc, _, ps := setupMove(t)
	if err := apptest.DeleteProject(t.Context(), svc.Stores, store, ps[2].ID); err != nil {
		t.Fatal(err)
	}
	ctx := port.WithRevived(t.Context())
	// 削除済みを除くと A・B の並び。C は(削除を取り消すと)末尾なので、下へは動かず断る。
	if err := svc.MoveProject(ctx, store, ps[2].ID, false); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("端の削除済み = %v, want port.ErrNotFound", err)
	}
	if got := order(t, svc); len(got) != 2 {
		t.Errorf("order = %v, 断った保存は取り消される", got)
	}
	if err := svc.MoveProject(ctx, store, ps[2].ID, true); err != nil {
		t.Fatal(err)
	}
	if got, want := order(t, svc), []project.ID{ps[0].ID, ps[2].ID, ps[1].ID}; !slices.Equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

func TestMoveProjectDeletedWithoutRootIsNotFound(t *testing.T) {
	svc := setup(t)
	deleteWithItems(t, svc, false)
	if err := svc.MoveProject(t.Context(), store, apptest.SampleProject, true); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("MoveProject = %v, want port.ErrNotFound", err)
	}
}
