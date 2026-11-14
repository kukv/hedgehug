package planning_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/planning"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	dp "github.com/kukv/hedgehug/internal/domain/planning"
)

// wsLog は、サンプルのワークスペースのログ。
func wsLog(t *testing.T, svc planning.Service) []change.Record {
	t.Helper()
	var log []change.Record
	err := svc.Stores.Read(t.Context(), store, func(r port.Reader) error {
		var err error
		log, err = r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return log
}

// hasRestore は、ログに取り消しの行があるか。
func hasRestore(log []change.Record) bool {
	return slices.ContainsFunc(log, func(r change.Record) bool { return r.T == change.Restore })
}

// revivedName は、ctx に積んだ取り消しが、名前 name の 1 つだけかを確かめる。
func revivedName(ctx context.Context, t *testing.T, name string) {
	t.Helper()
	if got, want := port.Revived(ctx), []port.Revival{{Kind: port.RevivedName, Text: name}}; !slices.Equal(got, want) {
		t.Errorf("Revived = %v, want %v", got, want)
	}
}

// notRevived は、n 回目の保存のあと何も保存せず、スプリント id が削除済みのままかを確かめる。
func notRevived(t *testing.T, svc planning.Service, rec *recorder, n int, id change.ID) {
	t.Helper()
	if len(rec.summaries) != n || isLive(t, svc, id) || hasRestore(wsLog(t, svc)) {
		t.Errorf("取り消した: %v", rec.summaries[n:])
	}
}

func workspaceLive(t *testing.T, svc planning.Service) bool {
	t.Helper()
	var live bool
	err := svc.Stores.Read(t.Context(), store, func(r port.Reader) error {
		_, err := port.LiveWorkspace(r, apptest.SampleWorkspace)
		live = err == nil
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return live
}

// 別のタブで削除されたスプリントを編集すると、削除を取り消して保存し、削除で外したアイテムも戻る(決定 R12)。
// 削除のあとに別のスプリントに入れたアイテムは戻さない。
func TestUpdateDeletedSprintRevives(t *testing.T) {
	svc, rec := setup(t)
	ctx := port.WithRevived(t.Context())
	ws := apptest.SampleWorkspace
	s1 := plannedWorks(t, svc)
	if err := svc.DeleteSprint(t.Context(), store, ws, s1.ID); err != nil {
		t.Fatal(err)
	}
	s2, err := svc.CreateSprint(t.Context(), store, ws, planning.SprintInput{Name: "Sprint 2"})
	if err != nil {
		t.Fatal(err)
	}
	setRef(t, svc, itemA, item.SprintField, s2.ID)
	if err := svc.UpdateSprint(ctx, store, ws, s1.ID, planning.SprintInput{Name: "Sprint 1b", Goal: "g"}); err != nil {
		t.Fatal(err)
	}
	if got := lastSummary(rec); got != "スプリント「Sprint 1b」を編集" {
		t.Errorf("summary = %q", got)
	}
	if got := sprintOf(t, svc, s1.ID); got.Name != "Sprint 1b" || got.Goal != "g" || got.State != dp.Planned {
		t.Errorf("S1 = %+v", got)
	}
	a, _ := getItem(t, svc, itemA)
	b, _ := getItem(t, svc, itemB)
	if a.Sprint != s2.ID || b.Sprint != s1.ID {
		t.Errorf("A=%s B=%s", a.Sprint, b.Sprint)
	}
	revivedName(ctx, t, "Sprint 1")
}

// 計画中のスプリントを削除したあと開始すると、削除を取り消して実行中にする(決定 R12)。
func TestStartDeletedSprintRevives(t *testing.T) {
	svc, _ := setup(t)
	ctx := port.WithRevived(t.Context())
	ws := apptest.SampleWorkspace
	s1 := plannedWorks(t, svc)
	if err := svc.DeleteSprint(t.Context(), store, ws, s1.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.StartSprint(ctx, store, ws, s1.ID, "2026-11-13", "2026-11-27"); err != nil {
		t.Fatal(err)
	}
	if got := sprintOf(t, svc, s1.ID); got.State != dp.Active || got.Start != "2026-11-13" {
		t.Errorf("S1 = %+v", got)
	}
	revivedName(ctx, t, "Sprint 1")
}

func TestUpdateDeletedReleaseRevives(t *testing.T) {
	svc, _ := setup(t)
	ctx := port.WithRevived(t.Context())
	ws := apptest.SampleWorkspace
	r1 := releasedWorks(t, svc)
	if err := svc.DeleteRelease(t.Context(), store, ws, r1.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.UpdateRelease(ctx, store, ws, r1.ID, planning.ReleaseInput{Name: "v1", Description: "d"}); err != nil {
		t.Fatal(err)
	}
	if got, ok := releaseOf(t, svc, r1.ID); !ok || got.Description != "d" || got.State != dp.Released {
		t.Errorf("v1 = %+v %v", got, ok)
	}
	if a, _ := getItem(t, svc, itemA); a.Release != r1.ID {
		t.Errorf("A = %s", a.Release)
	}
	revivedName(ctx, t, "v1")
}

func TestSetStateDeletedReleaseRevives(t *testing.T) {
	svc, _ := setup(t)
	ctx := port.WithRevived(t.Context())
	ws := apptest.SampleWorkspace
	r1 := releasedWorks(t, svc)
	if err := svc.DeleteRelease(t.Context(), store, ws, r1.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetReleaseState(ctx, store, ws, r1.ID, dp.Archived); err != nil {
		t.Fatal(err)
	}
	if got, ok := releaseOf(t, svc, r1.ID); !ok || got.State != dp.Archived {
		t.Errorf("v1 = %+v %v", got, ok)
	}
	revivedName(ctx, t, "v1")
}

// 削除の前と同じ内容の保存は、行を書かないので 404 で、削除済みのまま(決定 R3)。
func TestDeletedSprintUnchangedIsNotFound(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	ws := apptest.SampleWorkspace
	s1 := plannedWorks(t, svc)
	r1, err := svc.CreateRelease(ctx, store, ws, planning.ReleaseInput{Name: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteSprint(ctx, store, ws, s1.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteRelease(ctx, store, ws, r1.ID); err != nil {
		t.Fatal(err)
	}
	n := len(rec.summaries)
	for name, err := range map[string]error{
		"UpdateSprint":    svc.UpdateSprint(ctx, store, ws, s1.ID, planning.SprintInput{Name: "Sprint 1"}),
		"UpdateRelease":   svc.UpdateRelease(ctx, store, ws, r1.ID, planning.ReleaseInput{Name: "v1"}),
		"SetReleaseState": svc.SetReleaseState(ctx, store, ws, r1.ID, dp.Unreleased),
	} {
		if !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s = %v, want port.ErrNotFound", name, err)
		}
	}
	notRevived(t, svc, rec, n, s1.ID)
	if _, ok := releaseOf(t, svc, r1.ID); ok {
		t.Error("v1 が戻った")
	}
}

// 完了・削除・アイテムへの割り当ては、削除済みのスプリントを取り消さずに 404(決定 R12)。
func TestCloseDeletedSprintIsNotFound(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	ws := apptest.SampleWorkspace
	s1 := plannedWorks(t, svc)
	if err := svc.DeleteSprint(ctx, store, ws, s1.ID); err != nil {
		t.Fatal(err)
	}
	n := len(rec.summaries)
	if err := svc.CloseSprint(ctx, store, ws, s1.ID, nil); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("CloseSprint = %v", err)
	}
	notRevived(t, svc, rec, n, s1.ID)
}

func TestDeleteDeletedSprintIsNotFound(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	ws := apptest.SampleWorkspace
	s1 := plannedWorks(t, svc)
	if err := svc.DeleteSprint(ctx, store, ws, s1.ID); err != nil {
		t.Fatal(err)
	}
	n := len(rec.summaries)
	if err := svc.DeleteSprint(ctx, store, ws, s1.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("DeleteSprint = %v", err)
	}
	notRevived(t, svc, rec, n, s1.ID)
}

func TestSetSprintToDeletedIsNotFound(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	ws := apptest.SampleWorkspace
	s1 := plannedWorks(t, svc)
	if err := svc.DeleteSprint(ctx, store, ws, s1.ID); err != nil {
		t.Fatal(err)
	}
	n := len(rec.summaries)
	if err := svc.SetSprint(ctx, store, apptest.SampleTask, &s1.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("SetSprint = %v", err)
	}
	notRevived(t, svc, rec, n, s1.ID)
}

// 取り消すと名前が重なるなら、08「復元」の文言で断る。名前を変える保存は、変えたあとの名前で判定する(決定 R12)。
func TestUpdateDeletedSprintNameTaken(t *testing.T) {
	const want = "同じ名前のスプリントがあります。先にそのスプリントの名前を変えてください"
	svc, rec := setup(t)
	ctx := t.Context()
	ws := apptest.SampleWorkspace
	s1 := plannedWorks(t, svc)
	if err := svc.DeleteSprint(ctx, store, ws, s1.ID); err != nil {
		t.Fatal(err)
	}
	s2, err := svc.CreateSprint(ctx, store, ws, planning.SprintInput{Name: "Sprint 2"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateSprint(ctx, store, ws, planning.SprintInput{Name: "Sprint 1"}); err != nil {
		t.Fatal(err)
	}
	n := len(rec.summaries)
	if err := svc.UpdateSprint(ctx, store, ws, s1.ID, planning.SprintInput{Name: "Sprint 1", Goal: "g"}); message(err) != want {
		t.Errorf("目標の変更 = %v", err)
	}
	if err := svc.UpdateSprint(ctx, store, ws, s1.ID, planning.SprintInput{Name: s2.Name}); message(err) != want {
		t.Errorf("重なる名前への変更 = %v", err)
	}
	notRevived(t, svc, rec, n, s1.ID)
	if err := svc.UpdateSprint(ctx, store, ws, s1.ID, planning.SprintInput{Name: "Sprint 1 old", Goal: "g"}); err != nil {
		t.Fatal(err)
	}
	if got := sprintOf(t, svc, s1.ID); got.Name != "Sprint 1 old" {
		t.Errorf("S1 = %+v", got)
	}
}

func TestUpdateDeletedReleaseNameTaken(t *testing.T) {
	const want = "同じ名前のリリースがあります。先にそのリリースの名前を変えてください"
	svc, rec := setup(t)
	ctx := t.Context()
	ws := apptest.SampleWorkspace
	r1 := releasedWorks(t, svc)
	if err := svc.DeleteRelease(ctx, store, ws, r1.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateRelease(ctx, store, ws, planning.ReleaseInput{Name: "v1"}); err != nil {
		t.Fatal(err)
	}
	n := len(rec.summaries)
	if err := svc.UpdateRelease(ctx, store, ws, r1.ID, planning.ReleaseInput{Name: "v1", Description: "d"}); message(err) != want {
		t.Errorf("説明の変更 = %v", err)
	}
	if err := svc.SetReleaseState(ctx, store, ws, r1.ID, dp.Archived); message(err) != want {
		t.Errorf("状態の変更 = %v", err)
	}
	if _, ok := releaseOf(t, svc, r1.ID); ok || len(rec.summaries) != n || hasRestore(wsLog(t, svc)) {
		t.Errorf("取り消した: %v", rec.summaries[n:])
	}
	if err := svc.UpdateRelease(ctx, store, ws, r1.ID, planning.ReleaseInput{Name: "v1 old"}); err != nil {
		t.Fatal(err)
	}
	if got, ok := releaseOf(t, svc, r1.ID); !ok || got.Name != "v1 old" {
		t.Errorf("v1 = %+v %v", got, ok)
	}
}

// ワークスペースが削除済みなら、スプリントの作成・編集でワークスペースの削除も取り消す(決定 R12・R11)。
func TestCreateSprintInDeletedWorkspaceRevives(t *testing.T) {
	svc, _ := setup(t)
	ctx := port.WithRevived(t.Context())
	ws := apptest.SampleWorkspace
	if err := apptest.DeleteWorkspace(t.Context(), svc.Stores, store, ws); err != nil {
		t.Fatal(err)
	}
	sp, err := svc.CreateSprint(ctx, store, ws, planning.SprintInput{Name: "Sprint 1"})
	if err != nil {
		t.Fatal(err)
	}
	if !isLive(t, svc, sp.ID) || !workspaceLive(t, svc) {
		t.Error("戻っていない")
	}
	if len(port.Revived(ctx)) != 1 {
		t.Errorf("Revived = %v", port.Revived(ctx))
	}
}

func TestUpdateSprintInDeletedWorkspaceRevives(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	ws := apptest.SampleWorkspace
	sp, err := svc.CreateSprint(ctx, store, ws, planning.SprintInput{Name: "Sprint 1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteWorkspace(ctx, svc.Stores, store, ws); err != nil {
		t.Fatal(err)
	}
	n := len(rec.summaries)
	if err := svc.UpdateSprint(ctx, store, ws, sp.ID, planning.SprintInput{Name: "Sprint 1"}); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("同じ内容 = %v", err)
	}
	if len(rec.summaries) != n || workspaceLive(t, svc) {
		t.Error("同じ内容で取り消した")
	}
	if err := svc.UpdateSprint(ctx, store, ws, sp.ID, planning.SprintInput{Name: "Sprint 9"}); err != nil {
		t.Fatal(err)
	}
	if !workspaceLive(t, svc) || sprintOf(t, svc, sp.ID).Name != "Sprint 9" {
		t.Error("戻っていない")
	}
}

// planFixture は、ワークスペースを削除する前に作った、計画中のスプリントとリリース(と、あれば実行中のスプリント)。
type planFixture struct {
	planned, active dp.Sprint
	release         dp.Release
}

// 削除済みのワークスペースでも、スプリント・リリースの作成と変更、実行中のスプリントの完了は、ワークスペースの削除を
// 取り消して保存する(決定 R12・R11)。
func TestDeletedWorkspaceRevivesOnPlanChange(t *testing.T) {
	ws := apptest.SampleWorkspace
	for _, c := range []struct {
		name   string
		active bool // 実行中のスプリントを作っておく
		run    func(ctx context.Context, svc planning.Service, f planFixture) error
	}{
		{"CreateRelease", false, func(ctx context.Context, svc planning.Service, _ planFixture) error {
			_, err := svc.CreateRelease(ctx, store, ws, planning.ReleaseInput{Name: "v2"})
			return err
		}},
		{"UpdateRelease", false, func(ctx context.Context, svc planning.Service, f planFixture) error {
			return svc.UpdateRelease(ctx, store, ws, f.release.ID, planning.ReleaseInput{Name: "v9"})
		}},
		{"SetReleaseState", false, func(ctx context.Context, svc planning.Service, f planFixture) error {
			return svc.SetReleaseState(ctx, store, ws, f.release.ID, dp.Released)
		}},
		{"StartSprint", false, func(ctx context.Context, svc planning.Service, f planFixture) error {
			return svc.StartSprint(ctx, store, ws, f.planned.ID, "2026-11-13", "2026-11-26")
		}},
		{"CloseSprint", true, func(ctx context.Context, svc planning.Service, f planFixture) error {
			return svc.CloseSprint(ctx, store, ws, f.active.ID, nil)
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			svc, _ := setup(t)
			f := deletedWorkspaceFixture(t, svc, c.active)
			ctx := port.WithRevived(t.Context())
			if err := c.run(ctx, svc, f); err != nil {
				t.Fatal(err)
			}
			if !workspaceLive(t, svc) {
				t.Error("ワークスペースが戻っていない")
			}
			revivedName(ctx, t, "Web")
		})
	}
}

// deletedWorkspaceFixture は、計画中のスプリント・リリース(active なら実行中のスプリントも)を作ってから、
// ワークスペースを削除する。
func deletedWorkspaceFixture(t *testing.T, svc planning.Service, active bool) planFixture {
	t.Helper()
	ctx := t.Context()
	ws := apptest.SampleWorkspace
	var f planFixture
	var err error
	if f.planned, err = svc.CreateSprint(ctx, store, ws, planning.SprintInput{Name: "Sprint 1"}); err != nil {
		t.Fatal(err)
	}
	if f.release, err = svc.CreateRelease(ctx, store, ws, planning.ReleaseInput{Name: "v1"}); err != nil {
		t.Fatal(err)
	}
	if active {
		if f.active, err = svc.CreateSprint(ctx, store, ws, planning.SprintInput{Name: "Sprint A"}); err != nil {
			t.Fatal(err)
		}
		if err := svc.StartSprint(ctx, store, ws, f.active.ID, "2026-11-13", "2026-11-26"); err != nil {
			t.Fatal(err)
		}
	}
	if err := apptest.DeleteWorkspace(ctx, svc.Stores, store, ws); err != nil {
		t.Fatal(err)
	}
	return f
}

// 削除済みのスプリント・リリースの編集で名前が空なら、08 の判定より先に名前の入力を求める。
func TestUpdateDeletedEmptyName(t *testing.T) {
	const want = "名前を入力してください"
	svc, rec := setup(t)
	ctx := t.Context()
	ws := apptest.SampleWorkspace
	s1 := plannedWorks(t, svc)
	r1, err := svc.CreateRelease(ctx, store, ws, planning.ReleaseInput{Name: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteSprint(ctx, store, ws, s1.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteRelease(ctx, store, ws, r1.ID); err != nil {
		t.Fatal(err)
	}
	// 削除の前と同じ名前のものを作る(空の名前をいまの名前と取り違えると、08 の文言になる)。
	if _, err := svc.CreateSprint(ctx, store, ws, planning.SprintInput{Name: "Sprint 1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateRelease(ctx, store, ws, planning.ReleaseInput{Name: "v1"}); err != nil {
		t.Fatal(err)
	}
	n := len(rec.summaries)
	if err := svc.UpdateSprint(ctx, store, ws, s1.ID, planning.SprintInput{Name: " "}); message(err) != want {
		t.Errorf("UpdateSprint = %v", err)
	}
	if err := svc.UpdateRelease(ctx, store, ws, r1.ID, planning.ReleaseInput{Name: ""}); message(err) != want {
		t.Errorf("UpdateRelease = %v", err)
	}
	notRevived(t, svc, rec, n, s1.ID)
}
