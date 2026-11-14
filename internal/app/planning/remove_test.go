package planning_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/planning"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/history"
	"github.com/kukv/hedgehug/internal/domain/item"
	dp "github.com/kukv/hedgehug/internal/domain/planning"
)

const unknownID change.ID = "01NOSUCHPLAN000000000000"

// setRef は、空のアイテム id の項目 f(sprint・release)を to にする(別の操作の形)。
func setRef(t *testing.T, svc planning.Service, id item.ID, f string, to change.ID) {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "アイテムを編集"}
	err := svc.Stores.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteItems().Append(id, change.NewSet(f, nil, to))
	})
	if err != nil {
		t.Fatal(err)
	}
}

// plannedWorks は、計画中の Sprint 1 に A(未着手)・B(完了)・C(削除済み)を入れる。
func plannedWorks(t *testing.T, svc planning.Service) dp.Sprint {
	t.Helper()
	s1, err := svc.CreateSprint(t.Context(), store, apptest.SampleWorkspace, planning.SprintInput{Name: "Sprint 1"})
	if err != nil {
		t.Fatal(err)
	}
	addWorks(t, svc, []work{
		{itemA, 3, "B", apptest.SampleStatusTodo, s1.ID, 1},
		{itemB, 4, "C", apptest.SampleStatusDone, s1.ID, 2},
		{itemC, 5, "D", apptest.SampleStatusTodo, s1.ID, 3},
	})
	if err := apptest.DeleteItems(t.Context(), svc.Stores, store, itemC); err != nil {
		t.Fatal(err)
	}
	return s1
}

func isLive(t *testing.T, svc planning.Service, id change.ID) bool {
	t.Helper()
	for _, sp := range sprints(t, svc) {
		if sp.ID == id {
			return true
		}
	}
	return false
}

func TestDeleteSprint(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	s1 := plannedWorks(t, svc)
	_, cLog := getItem(t, svc, itemC)

	if err := svc.DeleteSprint(ctx, store, apptest.SampleWorkspace, s1.ID); err != nil {
		t.Fatal(err)
	}
	if got := lastSummary(rec); got != "スプリント「Sprint 1」を削除" {
		t.Errorf("summary = %q", got)
	}
	if isLive(t, svc, s1.ID) {
		t.Error("S1 が削除済みでない")
	}
	a, aLog := getItem(t, svc, itemA)
	b, bLog := getItem(t, svc, itemB)
	if a.Sprint != "" || b.Sprint != "" || a.Rank != "B" || b.Rank != "C" {
		t.Errorf("A=%s/%s B=%s/%s", a.Sprint, a.Rank, b.Sprint, b.Rank)
	}
	if sprintChanges(aLog) != 1 || sprintChanges(bLog) != 1 {
		t.Errorf("履歴: A=%d B=%d", sprintChanges(aLog), sprintChanges(bLog))
	}
	if c, log := getItem(t, svc, itemC); c.Sprint != s1.ID || len(log) != len(cLog) {
		t.Errorf("C に触れた: %s %d→%d", c.Sprint, len(cLog), len(log))
	}
	// 削除済みのスプリントは、もう一度の削除も編集も開始もできない。
	if err := svc.DeleteSprint(ctx, store, apptest.SampleWorkspace, s1.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("削除済みの削除: %v", err)
	}
	if err := svc.UpdateSprint(ctx, store, apptest.SampleWorkspace, s1.ID, planning.SprintInput{Name: "x"}); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("削除済みの編集: %v", err)
	}
	if err := svc.StartSprint(ctx, store, apptest.SampleWorkspace, s1.ID, "2026-11-13", "2026-11-27"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("削除済みの開始: %v", err)
	}
	if err := svc.DeleteSprint(ctx, store, apptest.SampleWorkspace, unknownID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("ないスプリント: %v", err)
	}
}

func TestDeleteSprintByState(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	s1, _ := activeSprint(t, svc)
	n := len(rec.summaries)
	if err := svc.DeleteSprint(ctx, store, apptest.SampleWorkspace, s1.ID); message(err) != "実行中は削除できません" {
		t.Errorf("実行中: %v", err)
	}
	if len(rec.summaries) != n || !isLive(t, svc, s1.ID) {
		t.Error("断った削除が保存された")
	}
	if err := svc.CloseSprint(ctx, store, apptest.SampleWorkspace, s1.ID, nil); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteSprint(ctx, store, apptest.SampleWorkspace, s1.ID); err != nil {
		t.Errorf("完了: %v", err)
	}
}

func TestRestoreSprint(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	s1 := plannedWorks(t, svc)
	_, cLog := getItem(t, svc, itemC)
	if err := svc.DeleteSprint(ctx, store, apptest.SampleWorkspace, s1.ID); err != nil {
		t.Fatal(err)
	}
	s2, err := svc.CreateSprint(ctx, store, apptest.SampleWorkspace, planning.SprintInput{Name: "Sprint 2"})
	if err != nil {
		t.Fatal(err)
	}
	setRef(t, svc, itemA, item.SprintField, s2.ID)

	if err := svc.RestoreSprint(ctx, store, apptest.SampleWorkspace, s1.ID); err != nil {
		t.Fatal(err)
	}
	if got := lastSummary(rec); got != "スプリント「Sprint 1」を復元" {
		t.Errorf("summary = %q", got)
	}
	if got := sprintOf(t, svc, s1.ID); got.State != dp.Planned || got.Name != "Sprint 1" {
		t.Errorf("S1 = %+v", got)
	}
	a, _ := getItem(t, svc, itemA)
	b, _ := getItem(t, svc, itemB)
	if a.Sprint != s2.ID || b.Sprint != s1.ID {
		t.Errorf("A=%s B=%s", a.Sprint, b.Sprint)
	}
	if c, log := getItem(t, svc, itemC); c.Sprint != s1.ID || len(log) != len(cLog) {
		t.Errorf("C に触れた: %s %d→%d", c.Sprint, len(cLog), len(log))
	}
	if err := svc.RestoreSprint(ctx, store, apptest.SampleWorkspace, s1.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("削除済みでない: %v", err)
	}
	if err := svc.RestoreSprint(ctx, store, apptest.SampleWorkspace, unknownID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("ないスプリント: %v", err)
	}
}

func TestRestoreSprintNameTaken(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	s1 := plannedWorks(t, svc)
	if err := svc.DeleteSprint(ctx, store, apptest.SampleWorkspace, s1.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateSprint(ctx, store, apptest.SampleWorkspace, planning.SprintInput{Name: " Sprint 1 "}); err != nil {
		t.Fatal(err)
	}
	n := len(rec.summaries)
	err := svc.RestoreSprint(ctx, store, apptest.SampleWorkspace, s1.ID)
	if message(err) != "同じ名前のスプリントがあります。先にそのスプリントの名前を変えてください" {
		t.Errorf("同じ名前: %v", err)
	}
	if b, _ := getItem(t, svc, itemB); len(rec.summaries) != n || isLive(t, svc, s1.ID) || b.Sprint != "" {
		t.Error("断った復元が保存された")
	}
}

// 削除と同時に、別の人が D を S1 に入れた(D のログに削除の op の行はない)。参照が残っているので、
// 復元すると D も S1 に入った状態に戻る(05 の 155 行)。
func TestRestoreSprintKeepsConcurrentAdd(t *testing.T) {
	svc, _ := setup(t)
	ctx := t.Context()
	s1 := plannedWorks(t, svc)
	addWorks(t, svc, []work{{itemD, 6, "E", apptest.SampleStatusTodo, "", 1}})
	if err := svc.DeleteSprint(ctx, store, apptest.SampleWorkspace, s1.ID); err != nil {
		t.Fatal(err)
	}
	setRef(t, svc, itemD, item.SprintField, s1.ID)
	_, dLog := getItem(t, svc, itemD)
	if err := svc.RestoreSprint(ctx, store, apptest.SampleWorkspace, s1.ID); err != nil {
		t.Fatal(err)
	}
	d, log := getItem(t, svc, itemD)
	if d.Sprint != s1.ID || len(log) != len(dLog) || !isLive(t, svc, s1.ID) {
		t.Errorf("D=%s %d→%d", d.Sprint, len(dLog), len(log))
	}
	if b, _ := getItem(t, svc, itemB); b.Sprint != s1.ID {
		t.Errorf("B=%s", b.Sprint)
	}
}

// releaseChanges は、アイテムの履歴の「リリースを変更」の行の数。
func releaseChanges(log []change.Record) int {
	n := 0
	for _, r := range history.ItemRows(log, history.Context{Names: history.NamesOf(nil)}) {
		if r.Text == "リリースを変更" {
			n++
		}
	}
	return n
}

func releaseOf(t *testing.T, svc planning.Service, id change.ID) (dp.Release, bool) {
	t.Helper()
	for _, r := range releases(t, svc) {
		if r.ID == id {
			return r, true
		}
	}
	return dp.Release{}, false
}

// releasedWorks は、リリース済みの v1 に A(未着手)・B(完了)・C(削除済み)を入れる。
func releasedWorks(t *testing.T, svc planning.Service) dp.Release {
	t.Helper()
	ctx := t.Context()
	r1, err := svc.CreateRelease(ctx, store, apptest.SampleWorkspace, planning.ReleaseInput{Name: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	addWorks(t, svc, []work{
		{itemA, 3, "B", apptest.SampleStatusTodo, "", 1},
		{itemB, 4, "C", apptest.SampleStatusDone, "", 2},
		{itemC, 5, "D", apptest.SampleStatusTodo, "", 3},
	})
	for _, id := range []item.ID{itemA, itemB, itemC} {
		setRef(t, svc, id, item.ReleaseField, r1.ID)
	}
	if err := apptest.DeleteItems(ctx, svc.Stores, store, itemC); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetReleaseState(ctx, store, apptest.SampleWorkspace, r1.ID, dp.Released); err != nil {
		t.Fatal(err)
	}
	return r1
}

// リリースは、どの状態でも消せる(ここではリリース済み)。
func TestDeleteRelease(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	r1 := releasedWorks(t, svc)
	_, aBefore := getItem(t, svc, itemA)
	_, bBefore := getItem(t, svc, itemB)
	_, cLog := getItem(t, svc, itemC)
	if err := svc.DeleteRelease(ctx, store, apptest.SampleWorkspace, r1.ID); err != nil {
		t.Fatal(err)
	}
	if got := lastSummary(rec); got != "リリース「v1」を削除" {
		t.Errorf("summary = %q", got)
	}
	if _, ok := releaseOf(t, svc, r1.ID); ok {
		t.Error("R1 が削除済みでない")
	}
	a, aLog := getItem(t, svc, itemA)
	b, bLog := getItem(t, svc, itemB)
	if a.Release != "" || b.Release != "" || a.Rank != "B" || b.Rank != "C" {
		t.Errorf("A=%s/%s B=%s/%s", a.Release, a.Rank, b.Release, b.Rank)
	}
	// 入れたときの 1 件に、削除の 1 件が足される。
	if releaseChanges(aLog)-releaseChanges(aBefore) != 1 || releaseChanges(bLog)-releaseChanges(bBefore) != 1 {
		t.Errorf("履歴: A=%d→%d B=%d→%d", releaseChanges(aBefore), releaseChanges(aLog), releaseChanges(bBefore), releaseChanges(bLog))
	}
	if c, log := getItem(t, svc, itemC); c.Release != r1.ID || len(log) != len(cLog) {
		t.Errorf("C に触れた: %s %d→%d", c.Release, len(cLog), len(log))
	}
	for name, err := range map[string]error{
		"削除":   svc.DeleteRelease(ctx, store, apptest.SampleWorkspace, r1.ID),
		"編集":   svc.UpdateRelease(ctx, store, apptest.SampleWorkspace, r1.ID, planning.ReleaseInput{Name: "x"}),
		"状態":   svc.SetReleaseState(ctx, store, apptest.SampleWorkspace, r1.ID, dp.Archived),
		"同じ状態": svc.SetReleaseState(ctx, store, apptest.SampleWorkspace, r1.ID, dp.Released),
		"ない削除": svc.DeleteRelease(ctx, store, apptest.SampleWorkspace, unknownID),
	} {
		if !errors.Is(err, port.ErrNotFound) {
			t.Errorf("削除済みの%s: %v", name, err)
		}
	}
}

func TestRestoreRelease(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	r1 := releasedWorks(t, svc)
	if err := svc.DeleteRelease(ctx, store, apptest.SampleWorkspace, r1.ID); err != nil {
		t.Fatal(err)
	}
	r2, err := svc.CreateRelease(ctx, store, apptest.SampleWorkspace, planning.ReleaseInput{Name: "v2"})
	if err != nil {
		t.Fatal(err)
	}
	setRef(t, svc, itemA, item.ReleaseField, r2.ID)
	if err := svc.RestoreRelease(ctx, store, apptest.SampleWorkspace, r1.ID); err != nil {
		t.Fatal(err)
	}
	if got := lastSummary(rec); got != "リリース「v1」を復元" {
		t.Errorf("summary = %q", got)
	}
	if got, ok := releaseOf(t, svc, r1.ID); !ok || got.State != dp.Released {
		t.Errorf("R1 = %+v %v", got, ok)
	}
	a, _ := getItem(t, svc, itemA)
	b, _ := getItem(t, svc, itemB)
	if a.Release != r2.ID || b.Release != r1.ID {
		t.Errorf("A=%s B=%s", a.Release, b.Release)
	}
	if err := svc.RestoreRelease(ctx, store, apptest.SampleWorkspace, r1.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("削除済みでない: %v", err)
	}
	if err := svc.RestoreRelease(ctx, store, apptest.SampleWorkspace, unknownID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("ないリリース: %v", err)
	}
}

func TestRestoreReleaseNameTaken(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	r1, err := svc.CreateRelease(ctx, store, apptest.SampleWorkspace, planning.ReleaseInput{Name: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteRelease(ctx, store, apptest.SampleWorkspace, r1.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateRelease(ctx, store, apptest.SampleWorkspace, planning.ReleaseInput{Name: " v1 "}); err != nil {
		t.Fatal(err)
	}
	n := len(rec.summaries)
	err = svc.RestoreRelease(ctx, store, apptest.SampleWorkspace, r1.ID)
	if message(err) != "同じ名前のリリースがあります。先にそのリリースの名前を変えてください" {
		t.Errorf("同じ名前: %v", err)
	}
	if _, ok := releaseOf(t, svc, r1.ID); len(rec.summaries) != n || ok {
		t.Error("断った復元が保存された")
	}
}
