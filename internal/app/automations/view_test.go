package automations_test

import (
	"fmt"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// writeRuleRow は、ルール rule が担当者を to にした保存を 1 回作る(実行の操作は Task 9 なので、行を直接書く)。
func writeRuleRow(t *testing.T, s port.Stores, rule change.ID, it item.ID, from, to any) {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "HH-2 のステータスを Done に変更"}
	err := s.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteItems().Append(it, change.NewSet("assignee", from, to).ByRule(rule))
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestListShowsRecentRuns(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	r, _ := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule())
	writeRuleRow(t, rec.Stores, r.ID, apptest.SampleTask, nil, "taro")
	writeRuleRow(t, rec.Stores, r.ID, apptest.SampleTask, nil, "taro") // 同じ変更: 最も早い 1 行だけ(B4-15)
	_ = svc.Delete(ctx, store, apptest.SampleWorkspace, r.ID)          // 消したルールの名前も引く(B4-16)
	v, err := svc.List(ctx, store, apptest.SampleWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Recent) != 1 {
		t.Fatalf("Recent = %+v", v.Recent)
	}
	got := v.Recent[0]
	if got.Item != apptest.SampleTask || got.Display != "HH-2" || got.Text != "「完了で外す」で担当者を taro に変更" {
		t.Errorf("Recent[0] = %+v", got)
	}
}

func TestListKeepsTheNewestEight(t *testing.T) {
	svc, rec := setup(t)
	r, _ := svc.Create(t.Context(), store, apptest.SampleWorkspace, doneRule())
	for i := range 10 {
		writeRuleRow(t, rec.Stores, r.ID, apptest.SampleTask, nil, fmt.Sprintf("user%d", i))
	}
	v, _ := svc.List(t.Context(), store, apptest.SampleWorkspace)
	if len(v.Recent) != 8 || v.Recent[0].Text != "「完了で外す」で担当者を 不明なユーザー に変更" {
		t.Errorf("Recent = %+v", v.Recent)
	}
}
