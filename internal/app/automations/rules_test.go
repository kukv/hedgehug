package automations_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/automations"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

const store port.StoreID = "S1"

// recorder は、保存のアクティビティの文を覚える。
type recorder struct {
	port.Stores
	summaries []string
}

func (r *recorder) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(port.Tx) error) error {
	r.summaries = append(r.summaries, op.Summary)
	return r.Stores.Write(ctx, id, op, fn)
}

func setup(t *testing.T) (automations.Service, *recorder) {
	t.Helper()
	ids := apptest.NewIDs()
	clock := apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))
	s := apptest.NewStores(clock, ids)
	s.AddStore(store)
	if err := apptest.Sample(context.Background(), s, store); err != nil {
		t.Fatal(err)
	}
	rec := &recorder{Stores: s}
	return automations.Service{Stores: rec, IDs: ids, Identity: apptest.Identity{Login: "hanako"}, Clock: clock}, rec
}

func message(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	return ""
}

func doneRule() automation.Rule {
	return automation.Rule{
		Name: " 完了で外す ", Trigger: automation.TriggerStatus, TriggerStatus: apptest.SampleStatusDone,
		AllMerged: true, Action: automation.ActionAssignee, Value: automation.Unassigned,
	}
}

func TestCreateUpdateDelete(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	r, err := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule())
	if err != nil {
		t.Fatal(err)
	}
	if r.Name != "完了で外す" || !r.Enabled || r.AllMerged || r.ID == "" {
		t.Errorf("Create = %+v", r)
	}
	if _, err := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule()); message(err) != "同じ名前のルールがあります" {
		t.Errorf("duplicate: %v", err)
	}
	if err := svc.SetEnabled(ctx, store, apptest.SampleWorkspace, r.ID, false); err != nil {
		t.Fatal(err)
	}
	edit := doneRule()
	edit.Name = "完了で taro に"
	edit.Value = "taro"
	if err := svc.Update(ctx, store, apptest.SampleWorkspace, r.ID, edit); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, store, apptest.SampleWorkspace, r.ID); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"自動化「完了で外す」を作成",
		"自動化「完了で外す」を作成", // 断った保存も Write は呼ばれる(行は残らない)
		"自動化「完了で外す」を無効にする",
		"自動化「完了で taro に」を編集",
		"自動化「完了で taro に」を削除",
	}
	if len(rec.summaries) != len(want) {
		t.Fatalf("summaries = %q", rec.summaries)
	}
	for i := range want {
		if rec.summaries[i] != want[i] {
			t.Errorf("%d: %q, want %q", i, rec.summaries[i], want[i])
		}
	}
}

// 編集では有効・無効を変えない(B4-5)。
func TestUpdateKeepsEnabled(t *testing.T) {
	svc, _ := setup(t)
	ctx := t.Context()
	r, _ := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule())
	_ = svc.SetEnabled(ctx, store, apptest.SampleWorkspace, r.ID, false)
	edit := doneRule()
	edit.Name = "別名"
	_ = svc.Update(ctx, store, apptest.SampleWorkspace, r.ID, edit)
	v, err := svc.List(ctx, store, apptest.SampleWorkspace)
	if err != nil || len(v.Rules) != 1 || v.Rules[0].Enabled || v.Rules[0].Name != "別名" {
		t.Errorf("List = %+v, %v", v, err)
	}
}

func TestSetEnabledRefusesTheSameStatus(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	r, _ := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule())
	_ = svc.SetEnabled(ctx, store, apptest.SampleWorkspace, r.ID, false)
	// 同期で届いた形を作る: きっかけとすることが同じステータスで無効のルール
	same := automation.Rule{
		ID: "01RULESAME000000000000001", Name: "同じ", Trigger: automation.TriggerStatus,
		TriggerStatus: apptest.SampleStatusDone, Action: automation.ActionStatus, Value: string(apptest.SampleStatusDone),
	}
	err := rec.Stores.Write(ctx, store, port.Operation{Actor: change.User("taro"), Summary: "同期"}, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, automation.Drafts(automation.Rule{}, same)...)
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetEnabled(ctx, store, apptest.SampleWorkspace, same.ID, true); message(err) != automation.SwitchRefusal {
		t.Errorf("SetEnabled = %v", err)
	}
	if err := svc.SetEnabled(ctx, store, apptest.SampleWorkspace, same.ID, false); err != nil { // OFF は通す
		t.Errorf("SetEnabled(false) = %v", err)
	}
}

func TestBadReferencesAreNotFound(t *testing.T) {
	svc, _ := setup(t)
	ctx := t.Context()
	bad := doneRule()
	bad.Value = "jiro" // メンバーにいない
	if _, err := svc.Create(ctx, store, apptest.SampleWorkspace, bad); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Create = %v", err)
	}
	if err := svc.Delete(ctx, store, apptest.SampleWorkspace, "01NORULE"); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Delete = %v", err)
	}
	if err := svc.SetEnabled(ctx, store, apptest.SampleWorkspace, "01NORULE", true); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("SetEnabled = %v", err)
	}
}

// 担当者の値は、メンバーの保存どおりの綴りで入る(実行は大文字小文字を区別して比べる)。
func TestCreateStoresTheMembersLogin(t *testing.T) {
	svc, _ := setup(t)
	in := doneRule()
	in.Value = "TARO"
	r, err := svc.Create(t.Context(), store, apptest.SampleWorkspace, in)
	if err != nil || r.Value != "taro" {
		t.Fatalf("Create = %+v, %v", r, err)
	}
	v, _ := svc.List(t.Context(), store, apptest.SampleWorkspace)
	if len(v.Rules) != 1 || v.Rules[0].Value != "taro" {
		t.Errorf("List = %+v", v.Rules)
	}
}
