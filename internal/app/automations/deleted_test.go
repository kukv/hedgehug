package automations_test

import (
	"context"
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
	if err := svc.ApplyIgnoringWorkflow(ctx, store, apptest.SampleTask, r.ID, apptest.SampleStatusTodo, apptest.SampleStatusDone); !errors.Is(err, port.ErrNotFound) {
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

// 削除済み(root: true)のアイテムへの「制約を無視して…」は、削除を取り消さず、何も書かずに断る。同じステータスでも
// 断る(C4a-11。06「受信箱」の「アイテムが削除済みのときは、2 つのボタンを出さない」と「押した瞬間に…値が
// 変わっていたときは、保存せず、エラーのトースト」)。
func TestApplyIgnoringWorkflowRefusesARootDeletedItem(t *testing.T) {
	for name, status := range map[string]change.ID{"other": apptest.SampleStatusDone, "same": apptest.SampleStatusTodo} {
		t.Run(name, func(t *testing.T) { refusesRootDeleted(t, status) })
	}
}

// refusesRootDeleted は、削除済み(root: true)のアイテムを status にする「制約を無視して…」が断られ、何も書かず、
// 削除も取り消さないことを確かめる。
func refusesRootDeleted(t *testing.T, status change.ID) {
	t.Helper()
	svc, rec := setup(t)
	ctx := port.WithRevived(t.Context())
	r, err := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule())
	if err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteItems(ctx, rec.Stores, store, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	err = svc.ApplyIgnoringWorkflow(ctx, store, apptest.SampleTask, r.ID, apptest.SampleStatusTodo, status)
	if got := message(err); got != "HH-2 は先に変更されたため、適用しませんでした" {
		t.Errorf("err = %v", err)
	}
	err = rec.Read(ctx, store, func(rd port.Reader) error {
		log, err := rd.Changes().Of(change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
		if err == nil && log[len(log)-1].T != change.Delete {
			t.Errorf("a refused apply wrote %+v", log[len(log)-1])
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := port.Revived(ctx); len(got) != 0 {
		t.Errorf("Revived = %v", got)
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

// deleteWorkspace は、ワークスペースを削除する(B5b と同じく、プロジェクトは root: false で同じ op)。
func deleteWorkspace(t *testing.T, rec *recorder) {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "ワークスペースを削除"}
	if err := rec.Stores.Write(t.Context(), store, op, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewDeleteEntity(true)),
			tx.WriteProjects().Append(apptest.SampleProject, change.NewDeleteEntity(false)),
		)
	}); err != nil {
		t.Fatal(err)
	}
}

// workspaceDeleted は、ワークスペースとプロジェクトが削除済みかどうか。
func workspaceDeleted(t *testing.T, rec *recorder) (ws, p bool) {
	t.Helper()
	if err := rec.Read(t.Context(), store, func(rd port.Reader) error {
		w, err := rd.Workspaces().Get(apptest.SampleWorkspace)
		if err != nil {
			return err
		}
		pr, err := rd.Projects().Get(apptest.SampleProject)
		ws, p = w.Deleted != nil, pr.Deleted != nil
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return ws, p
}

// checkRevived は、保存の結果 err が、revives ならワークスペースの削除を取り消した成功、そうでなければ削除済みのままの
// ErrNotFound であることを確かめる。revived は保存の入れ物に積まれたもの。
func checkRevived(t *testing.T, rec *recorder, err error, revives bool, revived []port.Revival) {
	t.Helper()
	ws, p := workspaceDeleted(t, rec)
	if !revives {
		if !errors.Is(err, port.ErrNotFound) || !ws || !p {
			t.Errorf("err = %v, deleted = %v/%v", err, ws, p)
		}
		return
	}
	if err != nil || ws || p {
		t.Fatalf("err = %v, deleted = %v/%v", err, ws, p)
	}
	if want := []port.Revival{{Kind: port.RevivedName, Text: "Web"}}; !slices.Equal(revived, want) {
		t.Errorf("Revived = %v, want %v", revived, want)
	}
}

// 別の画面で削除されたワークスペースのルールの作成・編集・有効と無効の切り替えは、変更を残してワークスペースの削除を
// 取り消す(決定 R11)。ルールの削除と、行を書かない保存は取り消さず 404。
func TestChangeRuleInDeletedWorkspaceRevives(t *testing.T) {
	scoped := doneRule()
	scoped.Name = "専用"
	scoped.Scope = apptest.SampleProject
	for name, tt := range map[string]struct {
		change  func(svc automations.Service, ctx context.Context, id change.ID) error
		revives bool
	}{
		"create": {func(svc automations.Service, ctx context.Context, _ change.ID) error {
			r := doneRule()
			r.Name = "別のルール"
			_, err := svc.Create(ctx, store, apptest.SampleWorkspace, r)
			return err
		}, true},
		"create scoped": {func(svc automations.Service, ctx context.Context, _ change.ID) error {
			_, err := svc.Create(ctx, store, apptest.SampleWorkspace, scoped)
			return err
		}, true},
		"update": {func(svc automations.Service, ctx context.Context, id change.ID) error {
			r := doneRule()
			r.Value = "taro"
			return svc.Update(ctx, store, apptest.SampleWorkspace, id, r)
		}, true},
		"enable": {func(svc automations.Service, ctx context.Context, id change.ID) error {
			return svc.SetEnabled(ctx, store, apptest.SampleWorkspace, id, false)
		}, true},
		"enable same": {func(svc automations.Service, ctx context.Context, id change.ID) error {
			return svc.SetEnabled(ctx, store, apptest.SampleWorkspace, id, true)
		}, false},
		"update same": {func(svc automations.Service, ctx context.Context, id change.ID) error {
			return svc.Update(ctx, store, apptest.SampleWorkspace, id, doneRule())
		}, false},
		"delete": {func(svc automations.Service, ctx context.Context, id change.ID) error {
			return svc.Delete(ctx, store, apptest.SampleWorkspace, id)
		}, false},
	} {
		t.Run(name, func(t *testing.T) {
			svc, rec := setup(t)
			r, err := svc.Create(t.Context(), store, apptest.SampleWorkspace, doneRule())
			if err != nil {
				t.Fatal(err)
			}
			deleteWorkspace(t, rec)
			ctx := port.WithRevived(t.Context())
			err = tt.change(svc, ctx, r.ID)
			checkRevived(t, rec, err, tt.revives, port.Revived(ctx))
		})
	}
}

// 自分で削除された(root: true)プロジェクトの専用ルールの作成は、ワークスペースが削除済みでも取り消さず 404。
func TestCreateScopedRuleOnDeletedProjectInDeletedWorkspace(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	if err := apptest.DeleteProject(ctx, rec.Stores, store, apptest.SampleProject); err != nil {
		t.Fatal(err)
	}
	if err := apptest.DeleteWorkspace(ctx, rec.Stores, store, apptest.SampleWorkspace); err != nil {
		t.Fatal(err)
	}
	r := doneRule()
	r.Scope = apptest.SampleProject
	if _, err := svc.Create(ctx, store, apptest.SampleWorkspace, r); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("err = %v, want port.ErrNotFound", err)
	}
	if ws, _ := workspaceDeleted(t, rec); !ws {
		t.Error("the workspace was revived")
	}
}

// 取り消すとワークスペースの名前が、削除されていないワークスペースと重なるなら、08「復元」の文言で断り、何も残さない
// (決定 R11)。
func TestCreateRuleInDeletedWorkspaceNameTaken(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	deleteWorkspace(t, rec)
	if err := apptest.SampleMobile(ctx, rec.Stores, store); err != nil {
		t.Fatal(err)
	}
	op := port.Operation{Actor: change.User("taro"), Summary: "名前を変更"}
	if err := rec.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleMobileWorkspace, change.NewSet("name", "Mobile", "Web"))
	}); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule())
	if want := "同じ名前のワークスペースがあります。先にそのワークスペースの名前を変更してください"; message(err) != want {
		t.Errorf("err = %v, want %q", err, want)
	}
	if ws, p := workspaceDeleted(t, rec); !ws || !p {
		t.Errorf("deleted = %v/%v", ws, p)
	}
	if err := rec.Read(ctx, store, func(rd port.Reader) error {
		log, err := rd.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace})
		if rules := automation.FromLog(log); len(rules) != 0 {
			t.Errorf("rules = %+v", rules)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
}
