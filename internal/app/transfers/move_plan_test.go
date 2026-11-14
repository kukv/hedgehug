package transfers_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/history"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	sprint1  change.ID = "01SPRINT000000000000000001"
	release1 change.ID = "01RELEASE00000000000000001"
)

// withPlan は、Web に実行中のスプリント「Sprint 1」とリリース「v1」を書き、HH-2 をその両方に入れる。
func withPlan(t *testing.T, f fixture) {
	t.Helper()
	sp := planning.Sprint{ID: sprint1, Name: "Sprint 1", State: planning.Active}
	rel := planning.Release{ID: release1, Name: "v1", State: planning.Unreleased}
	write(t, f.rec, store1, func(tx port.Tx) error {
		ds := append(planning.SprintDrafts(planning.Sprint{}, sp), planning.ReleaseDrafts(planning.Release{}, rel)...)
		return errors.Join(
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace, ds...),
			tx.WriteItems().Append(apptest.SampleTask, change.NewSet(item.SprintField, nil, sprint1), change.NewSet(item.ReleaseField, nil, release1)),
		)
	})
}

// inSprint1 は、Web のアイテムのうち、Sprint 1 に入っているもの(Web のスプリントのカードが数えるもの)の数。
func inSprint1(t *testing.T, f fixture) int {
	t.Helper()
	n := 0
	err := f.rec.Read(t.Context(), store1, func(r port.Reader) error {
		ps, err := r.Projects().List(apptest.SampleWorkspace)
		for _, p := range ps {
			its, err := r.Items().List(p.ID)
			if err != nil {
				return err
			}
			n += len(slices.DeleteFunc(its, func(it item.Item) bool { return it.Sprint != sprint1 }))
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// 別のワークスペース(同じ Store)と別の Store へ移すと、スプリントとリリースを外す(02「移動先で変わること」)。
// 移動先の履歴に「スプリントを変更」「リリースを変更」が残り、移動元のスプリントは数えなくなる。
func TestMoveToAnotherWorkspaceUnsetsSprintAndRelease(t *testing.T) {
	t.Run("same store", func(t *testing.T) { checkMoveUnsetsPlan(t, toMobile(), store1, mobileWS) })
	t.Run("other store", func(t *testing.T) { checkMoveUnsetsPlan(t, toTeam(), store2, teamWS) })
}

// checkMoveUnsetsPlan は、スプリントとリリースに入った HH-2 を in で移し、Store store のワークスペース ws に
// 外して移ったことを確かめる。
func checkMoveUnsetsPlan(t *testing.T, in transfers.MoveInput, store port.StoreID, ws workspace.ID) {
	t.Helper()
	f := newFixture(t)
	withPlan(t, f)
	if n := inSprint1(t, f); n != 1 {
		t.Fatalf("移す前に Sprint 1 に入っている数 = %d, want 1", n)
	}
	if _, err := f.svc.Move(t.Context(), store1, apptest.SampleTask, in); err != nil {
		t.Fatal(err)
	}
	if it := itemOf(t, f.rec, store, apptest.SampleTask); it.Sprint != "" || it.Release != "" {
		t.Errorf("移したアイテムのスプリント・リリース = %q, %q, want 空", it.Sprint, it.Release)
	}
	if n := inSprint1(t, f); n != 0 {
		t.Errorf("移したあとに Sprint 1 に入っている数 = %d, want 0", n)
	}
	got := historyOf(t, f, store, ws, apptest.SampleTask)
	for _, w := range []string{"hanako: スプリントを変更", "hanako: リリースを変更"} {
		if !slices.Contains(got, w) {
			t.Errorf("移動先の履歴に %s がない: %q", w, got)
		}
	}
}

// historyOf は、Store store のアイテム id の履歴の行を「実行者: 文」で返す(いまの設定は ws)。
func historyOf(t *testing.T, f fixture, store port.StoreID, ws workspace.ID, id item.ID) []string {
	t.Helper()
	log := logOf(t, f.rec, store, change.Ref{Kind: change.KindItem, ID: id})
	c := history.Context{Names: func(login string) string { return login }}
	if err := f.rec.Read(t.Context(), store, func(r port.Reader) (err error) { c.Settings, err = r.Workspaces().Settings(ws); return }); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, row := range history.ItemRows(log, c) {
		got = append(got, row.Who+": "+row.Text)
	}
	return got
}
