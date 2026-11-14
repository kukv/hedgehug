package planning_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/planning"
	"github.com/kukv/hedgehug/internal/app/port"
	dp "github.com/kukv/hedgehug/internal/domain/planning"
)

// 削除済みのワークスペースのスプリント・リリースは、読みと、削除・割り当て・できない完了・同じ状態への変更がワークスペースがないときと
// 同じ(B5b-4)。作成と変更はワークスペースの削除を取り消す(決定 R12。revive_test.go)。
func TestDeletedWorkspacePlanning(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	ws := apptest.SampleWorkspace
	sp, err := svc.CreateSprint(ctx, store, ws, planning.SprintInput{Name: "Sprint 1"})
	if err != nil {
		t.Fatal(err)
	}
	r, err := svc.CreateRelease(ctx, store, ws, planning.ReleaseInput{Name: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteWorkspace(ctx, svc.Stores, store, ws); err != nil {
		t.Fatal(err)
	}
	n := len(rec.summaries)
	setSprint := svc.SetSprint(ctx, store, apptest.SampleTask, &sp.ID)
	setRelease := svc.SetRelease(ctx, store, apptest.SampleTask, &r.ID)
	_, overview := svc.Overview(ctx, store, ws)
	_, sprintCards := svc.SprintCards(ctx, store, ws)
	_, releaseCards := svc.ReleaseCards(ctx, store, ws)
	errs := map[string]error{
		"Overview":        overview,
		"SprintCards":     sprintCards,
		"ReleaseCards":    releaseCards,
		"CloseSprint":     svc.CloseSprint(ctx, store, ws, sp.ID, nil),
		"DeleteSprint":    svc.DeleteSprint(ctx, store, ws, sp.ID),
		"DeleteRelease":   svc.DeleteRelease(ctx, store, ws, r.ID),
		"SetReleaseState": svc.SetReleaseState(ctx, store, ws, r.ID, dp.Unreleased),
		"SetSprint":       setSprint,
		"SetRelease":      setRelease,
	}
	for name, err := range errs {
		if !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s = %v, want port.ErrNotFound", name, err)
		}
	}
	if len(rec.summaries) != n {
		t.Errorf("保存した: %v", rec.summaries[n:])
	}
}

// 削除済みのワークスペースのスプリント・リリースの復元は、名前の重なりより先に断る(08 356 行)。
func TestRestoreUnderDeletedWorkspace(t *testing.T) {
	const want = "ワークスペースも削除されています。先にワークスペースを復元してください"
	svc, rec := setup(t)
	ctx := t.Context()
	ws := apptest.SampleWorkspace
	sp, err := svc.CreateSprint(ctx, store, ws, planning.SprintInput{Name: "Sprint 1"})
	if err != nil {
		t.Fatal(err)
	}
	r, err := svc.CreateRelease(ctx, store, ws, planning.ReleaseInput{Name: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	for _, err := range []error{
		svc.DeleteSprint(ctx, store, ws, sp.ID),
		svc.DeleteRelease(ctx, store, ws, r.ID),
		func() error {
			_, err := svc.CreateSprint(ctx, store, ws, planning.SprintInput{Name: "Sprint 1"})
			return err
		}(),
		func() error {
			_, err := svc.CreateRelease(ctx, store, ws, planning.ReleaseInput{Name: "v1"})
			return err
		}(),
		apptest.DeleteWorkspace(ctx, svc.Stores, store, ws),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	n := len(rec.summaries)
	if err := svc.RestoreSprint(ctx, store, ws, sp.ID); message(err) != want {
		t.Errorf("RestoreSprint = %v", err)
	}
	if err := svc.RestoreRelease(ctx, store, ws, r.ID); message(err) != want {
		t.Errorf("RestoreRelease = %v", err)
	}
	if err := svc.RestoreSprint(ctx, store, "01NOSUCHWS0000000000000000", sp.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("RestoreSprint(ないワークスペース) = %v", err)
	}
	if len(rec.summaries) != n {
		t.Errorf("保存した: %v", rec.summaries[n:])
	}
}

// 別のタブで削除された(root: true)アイテムのスプリント・リリースを変えると、削除を取り消して保存する
// (C3a-screen S1)。値が変わらないなら断り、削除済みのまま。
func TestAssignDeletedItemRevives(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	ws := apptest.SampleWorkspace
	sp, err := svc.CreateSprint(ctx, store, ws, planning.SprintInput{Name: "Sprint 1"})
	if err != nil {
		t.Fatal(err)
	}
	r, err := svc.CreateRelease(ctx, store, ws, planning.ReleaseInput{Name: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteItems(ctx, svc.Stores, store, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	n := len(rec.summaries)
	if err := svc.SetSprint(ctx, store, apptest.SampleTask, nil); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("SetSprint(同じ) = %v, want port.ErrNotFound", err)
	}
	if err := svc.SetRelease(ctx, store, apptest.SampleTask, nil); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("SetRelease(同じ) = %v, want port.ErrNotFound", err)
	}
	if it, _ := getItem(t, svc, apptest.SampleTask); it.Deleted == nil || len(rec.summaries) != n {
		t.Fatalf("同じ値で取り消した: %+v %v", it.Deleted, rec.summaries[n:])
	}
	if err := svc.SetSprint(ctx, store, apptest.SampleTask, &sp.ID); err != nil {
		t.Fatal(err)
	}
	if it, _ := getItem(t, svc, apptest.SampleTask); it.Deleted != nil || it.Sprint != sp.ID {
		t.Errorf("SetSprint: deleted %+v sprint %q", it.Deleted, it.Sprint)
	}
	if err := apptest.DeleteItems(ctx, svc.Stores, store, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetRelease(ctx, store, apptest.SampleTask, &r.ID); err != nil {
		t.Fatal(err)
	}
	if it, _ := getItem(t, svc, apptest.SampleTask); it.Deleted != nil || it.Release != r.ID {
		t.Errorf("SetRelease: deleted %+v release %q", it.Deleted, it.Release)
	}
}

// 親と一緒に削除された(root: false)アイテムは、取り消さずに断る(決定 S9)。
func TestAssignItemDeletedWithParent(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	sp, err := svc.CreateSprint(ctx, store, apptest.SampleWorkspace, planning.SprintInput{Name: "Sprint 1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteItems(ctx, svc.Stores, store, apptest.SampleEpic, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	n := len(rec.summaries)
	if err := svc.SetSprint(ctx, store, apptest.SampleTask, &sp.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("SetSprint = %v, want port.ErrNotFound", err)
	}
	if err := svc.SetRelease(ctx, store, apptest.SampleTask, nil); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("SetRelease = %v, want port.ErrNotFound", err)
	}
	if len(rec.summaries) != n {
		t.Errorf("保存した: %v", rec.summaries[n:])
	}
}
