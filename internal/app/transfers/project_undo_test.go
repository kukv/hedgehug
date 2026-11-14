package transfers_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/transfers"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

// assertProjectRolledBack は、移動先からフォルダ・足したメンバー・ルールの行・作った専用ルールが消え、移動元が元のままで
// あることを確かめる。
func assertProjectRolledBack(t *testing.T, f fixture) {
	t.Helper()
	assertProjectUntouched(t, f)
	if got := loginsOf(t, f.rec); !slices.Equal(got, []string{"Taro"}) {
		t.Errorf("destination members = %v, want [Taro]", got)
	}
	rules, own := rulesAt(t, f.rec, store2, teamWS)
	if !rules.Empty() || len(own) != 0 {
		t.Errorf("destination rules = %+v / %+v", rules, own)
	}
	if all := automation.FromLog(logOf(t, f.rec, store2, change.Ref{Kind: change.KindWorkspace, ID: teamWS})); len(all) != 0 {
		t.Errorf("destination automation rules = %+v", all)
	}
	if s := f.rec.summaries(store2); s[len(s)-1] != "TM-P2 の移動を取り消し" {
		t.Errorf("destination summary = %q", s[len(s)-1])
	}
}

// withNewcomer は、Task の担当者を jiro(移動先にいない人)にする。
func withNewcomer(t *testing.T, f fixture) {
	t.Helper()
	write(t, f.rec, store1, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("assignee", nil, "jiro"))
	})
}

// Review Focus 2・A5b-14: 移動元の保存が失敗したら、移動先からフォルダ・足したメンバー・ルールの行・作った専用ルールを戻す。
func TestMoveProjectAcrossRollsBack(t *testing.T) {
	f := newFixture(t)
	withRules(t, f)
	withNewcomer(t, f)
	f.svc.Stores = failWrite{Stores: f.rec, store: store1}
	if _, err := f.svc.MoveProject(t.Context(), store1, apptest.SampleProject, toTeamWS()); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v", err)
	}
	assertProjectRolledBack(t, f)
}

// 移動先の保存が送れず捨てられもしなかった(保存されたかもしれない)ときも、移動元を保存せずに戻す。
func TestMoveProjectAcrossRollsBackASaveThatWasNotDiscarded(t *testing.T) {
	f := newFixture(t)
	withRules(t, f)
	withNewcomer(t, f)
	f.svc.Delivery = notDiscarded{inner: f.deliver, done: new(bool)}
	if _, err := f.svc.MoveProject(t.Context(), store1, apptest.SampleProject, toTeamWS()); !errors.Is(err, port.ErrSaveNotDiscarded) {
		t.Fatalf("err = %v", err)
	}
	assertProjectRolledBack(t, f)
}

// 戻しも失敗したら *UndoError(Old はプロジェクトの旧 ID、NewDisplay はプロジェクトの新 ID)。
func TestMoveProjectAcrossReportsAnUndoThatFailed(t *testing.T) {
	for _, d := range []undoFails{{reason: syncstatus.WriteDenied}, {err: errUndo}} {
		f := newFixture(t)
		f.svc.Stores = failWrite{Stores: f.rec, store: store1}
		d.inner, d.calls = f.deliver, new(int)
		f.svc.Delivery = d
		_, err := f.svc.MoveProject(t.Context(), store1, apptest.SampleProject, toTeamWS())
		var ue *transfers.UndoError
		if !errors.As(err, &ue) || ue.Old != "HH-P1" || ue.Store != "team" || ue.NewDisplay != "TM-P2" || !errors.Is(err, errBoom) {
			t.Fatalf("err = %#v", err)
		}
		if _, ok := projectAt(t, f.rec, store2, apptest.SampleProject); !ok {
			t.Error("the destination project was removed")
		}
	}
}

// A5b-12: 移動先の保存のあと、移動元の保存の前に、移動元のプロジェクトにアイテムが足されたら、移動元の保存が失敗して戻す。
func TestMoveProjectAcrossSourceChanged(t *testing.T) {
	f := newFixture(t)
	withRules(t, f)
	const added item.ID = "01TESTIT000000000000000013"
	done := false
	f.deliver.inside = func(port.StoreID) {
		if !done {
			done = true
			addItem(t, f.rec, store1, item.Item{ID: added, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: ptr(apptest.SampleEpic), Number: 3}, Kind: item.Task, Rank: "X", Title: "あとから"})
		}
	}
	if _, err := f.svc.MoveProject(t.Context(), store1, apptest.SampleProject, toTeamWS()); err == nil {
		t.Fatal("the move succeeded")
	}
	assertProjectRolledBack(t, f)
	if !exists(t, f.rec, store1, added) || exists(t, f.rec, store2, added) {
		t.Error("the added item moved")
	}
}

// A5b-12: 移動先の保存のあと、移動元の保存の前に、このプロジェクトの専用ルールが足されたら、移動元の保存が失敗して戻す
// (足されたルールは移動元に残り、移動先にはない)。
func TestMoveProjectAcrossRulesChanged(t *testing.T) {
	f := newFixture(t)
	withRules(t, f)
	later := statusRule("01TESTRULE0000000000000009", "あとから", apptest.SampleProject, apptest.SampleStatusDone)
	done := false
	f.deliver.inside = func(port.StoreID) {
		if !done {
			done = true
			addRule(t, f.rec, apptest.SampleWorkspace, later)
		}
	}
	if _, err := f.svc.MoveProject(t.Context(), store1, apptest.SampleProject, toTeamWS()); err == nil {
		t.Fatal("the move succeeded")
	}
	assertProjectRolledBack(t, f)
	_, own := rulesAt(t, f.rec, store1, apptest.SampleWorkspace)
	if !slices.ContainsFunc(own, func(r automation.Rule) bool { return r.ID == later.ID }) {
		t.Errorf("source own rules = %+v", own)
	}
}

const earlierEpic item.ID = "01TESTIT000000000000000020"

// deleteSampleProject は、別の画面で Web のプロジェクト「ログイン」を削除した状態にする(Epic は同じ操作で削除、Task と、
// 順位が前の Epic HH-3 はその前に別の操作で削除)。
func deleteSampleProject(t *testing.T, f fixture) {
	t.Helper()
	addItem(t, f.rec, store1, item.Item{ID: earlierEpic, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Number: 3}, Kind: item.Epic, Rank: "A", Title: "前"})
	if err := apptest.DeleteItems(t.Context(), f.rec, store1, apptest.SampleTask, earlierEpic); err != nil {
		t.Fatal(err)
	}
	write(t, f.rec, store1, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteProjects().Append(apptest.SampleProject, change.NewDeleteEntity(true)),
			tx.WriteItems().Append(apptest.SampleEpic, change.NewDeleteEntity(false)),
		)
	})
}

// A5b-15: 別の画面で削除されたプロジェクトは、移動先で削除を取り消し(同じ操作で削除された Epic も)、両方の保存のあとで旧 ID
// を積む。番号は、取り消した Epic を削除されていないものとして振る(同じ Store の移動と同じ)。移動元の保存が失敗したら積まない。
func TestMoveProjectAcrossRevives(t *testing.T) {
	f := newFixture(t)
	deleteSampleProject(t, f)
	ctx := port.WithRevived(t.Context())
	if _, err := f.svc.MoveProject(ctx, store1, apptest.SampleProject, toTeamWS()); err != nil {
		t.Fatal(err)
	}
	if p, _ := projectAt(t, f.rec, store2, apptest.SampleProject); p.Deleted != nil {
		t.Errorf("project = %+v", p)
	}
	if e := itemOf(t, f.rec, store2, apptest.SampleEpic); e.Deleted != nil {
		t.Errorf("epic = %+v", e)
	}
	if task := itemOf(t, f.rec, store2, apptest.SampleTask); task.Deleted == nil {
		t.Error("the task deleted by another operation was revived")
	}
	if got := port.Revived(ctx); !slices.Equal(got, []string{"HH-P1"}) {
		t.Errorf("revived = %v", got)
	}
	for id, n := range map[item.ID]int{apptest.SampleEpic: 2, apptest.SampleTask: 3, earlierEpic: 4} {
		if got := itemOf(t, f.rec, store2, id).Place.Number; got != n {
			t.Errorf("%s number = %d, want %d", id, got, n)
		}
	}

	f = newFixture(t)
	deleteSampleProject(t, f)
	f.svc.Stores = failWrite{Stores: f.rec, store: store1}
	ctx = port.WithRevived(t.Context())
	if _, err := f.svc.MoveProject(ctx, store1, apptest.SampleProject, toTeamWS()); !errors.Is(err, errBoom) {
		t.Fatalf("err = %v", err)
	}
	if got := port.Revived(ctx); len(got) != 0 {
		t.Errorf("revived = %v, want none", got)
	}
}

// Review Focus 2(A5a): main の「ログイン」を team へ、team の「アプリ」を main の Web へ同時に移しても止まらない。
// fn は移動元の Store を読まないので、両方とも成功する。
func TestMoveProjectAcrossOppositeDirections(t *testing.T) {
	f := newFixture(t)
	f.deliver.gate = newBarrier()
	f.deliver.inside = newBarrier()
	done := make(chan error, 2)
	go func() {
		_, err := f.svc.MoveProject(t.Context(), store1, apptest.SampleProject, toTeamWS())
		done <- err
	}()
	go func() {
		_, err := f.svc.MoveProject(t.Context(), store2, teamProject, projectTo(store1, apptest.SampleWorkspace))
		done <- err
	}()
	timeout := time.After(10 * time.Second)
	for range 2 {
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("err = %v", err)
			}
		case <-timeout:
			t.Fatal("the two moves stalled")
		}
	}
	if _, ok := projectAt(t, f.rec, store2, apptest.SampleProject); !ok {
		t.Error("ログイン is not in team")
	}
	if p, ok := projectAt(t, f.rec, store1, teamProject); !ok || p.Place.Workspace != apptest.SampleWorkspace {
		t.Errorf("アプリ in main = %+v (%v)", p, ok)
	}
	for _, r := range recordsOf(t, f.rec, store2) {
		if r.ToStoreName != "main" || r.ToRemote != "https://example.invalid/main.git" {
			t.Errorf("team record = %+v", r)
		}
	}
}
