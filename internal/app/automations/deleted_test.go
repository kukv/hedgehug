package automations_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/automations"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
)

// 親と一緒に削除された(root: false)アイテムには動かない。通知の「制約を無視して {ステータス名} にする」は、
// 利用者が選んで削除していないアイテムなら、ないものと同じ(06「通知」のボタンを出さない条件。A4b-5)。
func TestApplyIgnoringWorkflowRefusesADeletedItem(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	r, err := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule())
	if err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteItems(ctx, rec.Stores, store, apptest.SampleEpic, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyIgnoringWorkflow(ctx, store, apptest.SampleTask, r.ID, apptest.SampleStatusDone); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("err = %v, want port.ErrNotFound", err)
	}
	err = rec.Read(ctx, store, func(rd port.Reader) error {
		log, err := rd.Changes().Of(change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
		if err != nil {
			return err
		}
		if last := log[len(log)-1]; last.T != change.Delete {
			t.Errorf("a refused apply wrote %+v", last)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// 最近の実行は、動いたときの記録なので、そのアイテムが削除されても残す(A4b-5。削除済みを除かない)。
func TestRecentRunsKeepADeletedItemsRun(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	r, err := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule())
	if err != nil {
		t.Fatal(err)
	}
	writeRuleRow(t, rec.Stores, r.ID, nil, "taro")
	if err := apptest.DeleteItems(ctx, rec.Stores, store, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	v, err := svc.List(ctx, store, apptest.SampleWorkspace)
	if err != nil || len(v.Recent) != 1 || v.Recent[0].Display != "HH-2" {
		t.Errorf("Recent = %+v, %v", v.Recent, err)
	}
}

// 削除済みのワークスペースの自動化の画面は、ワークスペースがないときと同じ(B5b-4)。
func TestViewOfDeletedWorkspace(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	if err := apptest.DeleteWorkspace(ctx, rec.Stores, store, apptest.SampleWorkspace); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.List(ctx, store, apptest.SampleWorkspace); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("List = %v, want port.ErrNotFound", err)
	}
}

// 削除済み(root: true)のアイテムへの「制約を無視して適用」は、変更を残して削除を取り消す(06「他の書き手との重なり」)。
func TestApplyIgnoringWorkflowRevivesADeletedItem(t *testing.T) {
	svc, rec := setup(t)
	ctx := port.WithRevived(t.Context())
	r, err := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule())
	if err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteItems(ctx, rec.Stores, store, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyIgnoringWorkflow(ctx, store, apptest.SampleTask, r.ID, apptest.SampleStatusDone); err != nil {
		t.Fatal(err)
	}
	err = rec.Read(ctx, store, func(rd port.Reader) error {
		it, err := rd.Items().Get(apptest.SampleTask)
		if err != nil || it.Deleted != nil || it.Status != apptest.SampleStatusDone {
			t.Errorf("item = %+v, %v", it, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := port.Revived(ctx); !slices.Equal(got, []port.Revival{{Kind: port.RevivedID, Text: "HH-2"}}) {
		t.Errorf("Revived = %v", got)
	}
}

// 同じステータスなら何も保存せず、削除済みのまま 404 になる(決定 S4)。
func TestApplyIgnoringWorkflowSameStatusOnADeletedItemIsNotFound(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	r, err := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule())
	if err != nil {
		t.Fatal(err)
	}
	var status change.ID
	_ = rec.Read(ctx, store, func(rd port.Reader) error {
		it, _ := rd.Items().Get(apptest.SampleTask)
		status = it.Status
		return nil
	})
	if err := apptest.DeleteItems(ctx, rec.Stores, store, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyIgnoringWorkflow(ctx, store, apptest.SampleTask, r.ID, status); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("err = %v, want port.ErrNotFound", err)
	}
}

// deletedRule は、プロジェクト別のルールの「動かさないルール」に入れたルールを作り、削除する(同じ op で外した行も入る)。
func deletedRule(t *testing.T) (automations.Service, *recorder, automation.Rule) {
	t.Helper()
	svc, rec := setup(t)
	ctx := t.Context()
	r, err := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule())
	if err != nil {
		t.Fatal(err)
	}
	op := port.Operation{Actor: change.User("taro"), Summary: "外す"}
	if err := rec.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewAdd(projectrule.DisabledField(apptest.SampleProject), r.ID))
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, store, apptest.SampleWorkspace, r.ID); err != nil {
		t.Fatal(err)
	}
	return svc, rec, r
}

// 取り消したあとのルールと、プロジェクトの動かさないルール。
func revivedState(t *testing.T, rec *recorder, id change.ID) (automation.Rule, []change.ID, bool) {
	t.Helper()
	var rule automation.Rule
	var disabled []change.ID
	var found bool
	err := rec.Read(t.Context(), store, func(rd port.Reader) error {
		log, err := rd.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace})
		if err != nil {
			return err
		}
		for _, r := range automation.FromLog(log) {
			if r.ID == id {
				rule, found = r, true
			}
		}
		disabled = projectrule.FromLog(log).Of(apptest.SampleProject).Disabled
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return rule, disabled, found
}

// 別の画面で削除されたルールの編集は、変更を残して削除を取り消す(06「他の書き手との重なり」)。プロジェクト別のルールの
// 「動かさないルール」から外した行も同じ op なので戻る。
func TestUpdateDeletedRuleRevives(t *testing.T) {
	svc, rec, r := deletedRule(t)
	ctx := port.WithRevived(t.Context())
	edit := doneRule()
	edit.Name = "完了で taro に"
	edit.Value = "taro"
	if err := svc.Update(ctx, store, apptest.SampleWorkspace, r.ID, edit); err != nil {
		t.Fatal(err)
	}
	got, disabled, found := revivedState(t, rec, r.ID)
	if !found || got.Name != "完了で taro に" || got.Value != "taro" || !slices.Equal(disabled, []change.ID{r.ID}) {
		t.Errorf("rule = %+v (found %v), disabled = %v", got, found, disabled)
	}
	if want := []port.Revival{{Kind: port.RevivedName, Text: "完了で外す"}}; !slices.Equal(port.Revived(ctx), want) {
		t.Errorf("Revived = %v, want %v", port.Revived(ctx), want)
	}
}

func TestSetEnabledDeletedRuleRevives(t *testing.T) {
	svc, rec, r := deletedRule(t)
	ctx := port.WithRevived(t.Context())
	if err := svc.SetEnabled(ctx, store, apptest.SampleWorkspace, r.ID, false); err != nil {
		t.Fatal(err)
	}
	got, disabled, found := revivedState(t, rec, r.ID)
	if !found || got.Enabled || !slices.Equal(disabled, []change.ID{r.ID}) {
		t.Errorf("rule = %+v (found %v), disabled = %v", got, found, disabled)
	}
	if want := []port.Revival{{Kind: port.RevivedName, Text: "完了で外す"}}; !slices.Equal(port.Revived(ctx), want) {
		t.Errorf("Revived = %v, want %v", port.Revived(ctx), want)
	}
}

// 削除の前と同じ有効・無効なら何も保存せず、削除済みのまま 404 になる(決定 R3)。
func TestSetEnabledDeletedRuleSameIsNotFound(t *testing.T) {
	svc, rec, r := deletedRule(t)
	if err := svc.SetEnabled(t.Context(), store, apptest.SampleWorkspace, r.ID, true); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("err = %v, want port.ErrNotFound", err)
	}
	if _, _, found := revivedState(t, rec, r.ID); found {
		t.Error("the rule was revived")
	}
}

// 削除のあとに同じ名前のルールができていたら、古い方の有効・無効の変更は断り、何も残さない(決定 R4)。
func TestSetEnabledDeletedRuleNameTaken(t *testing.T) {
	svc, rec, r := deletedRule(t)
	if _, err := svc.Create(t.Context(), store, apptest.SampleWorkspace, doneRule()); err != nil {
		t.Fatal(err)
	}
	err := svc.SetEnabled(t.Context(), store, apptest.SampleWorkspace, r.ID, false)
	if message(err) != "同じ名前のルールがあります" {
		t.Errorf("err = %v", err)
	}
	if _, _, found := revivedState(t, rec, r.ID); found {
		t.Error("the rule was revived")
	}
}

// 削除のあとに同じ名前のルールができていたら、古い方の編集(名前を変えない)は断り、何も残さない(決定 R4)。
func TestUpdateDeletedRuleNameTaken(t *testing.T) {
	svc, rec, r := deletedRule(t)
	if _, err := svc.Create(t.Context(), store, apptest.SampleWorkspace, doneRule()); err != nil {
		t.Fatal(err)
	}
	edit := doneRule()
	edit.Value = "taro"
	err := svc.Update(t.Context(), store, apptest.SampleWorkspace, r.ID, edit)
	if message(err) != "同じ名前のルールがあります" {
		t.Errorf("err = %v", err)
	}
	if _, _, found := revivedState(t, rec, r.ID); found {
		t.Error("the rule was revived")
	}
}

// 削除の前と同じ内容なら何も保存せず、削除済みのまま 404 になる(決定 R3)。
func TestUpdateDeletedRuleSameIsNotFound(t *testing.T) {
	svc, rec, r := deletedRule(t)
	if err := svc.Update(t.Context(), store, apptest.SampleWorkspace, r.ID, doneRule()); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("err = %v, want port.ErrNotFound", err)
	}
	if _, _, found := revivedState(t, rec, r.ID); found {
		t.Error("the rule was revived")
	}
}
