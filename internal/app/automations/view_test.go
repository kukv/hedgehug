package automations_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/automations"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// writeRuleRow は、ルール rule が HH-2 の担当者を to にした保存を 1 回作る(実行の操作は Task 9 なので、行を直接書く)。
func writeRuleRow(t *testing.T, s port.Stores, rule change.ID, from, to any) {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "HH-2 のステータスを Done に変更"}
	err := s.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("assignee", from, to).ByRule(rule))
	})
	if err != nil {
		t.Fatal(err)
	}
}

// twinStores は、読むログの最後の自動化の行に、同じ prev を持つ双子の行を足す。
// 偽物の Store は書き込みを順に並べるので、複数のマシンが同じ変更を重ねて保存し、
// 同期で両方が残った場面は、読むところで作る。
type twinStores struct{ port.Stores }

func (s twinStores) Read(ctx context.Context, id port.StoreID, fn func(port.Reader) error) error {
	return s.Stores.Read(ctx, id, func(r port.Reader) error { return fn(twinReader{r}) })
}

type twinReader struct{ port.Reader }

func (r twinReader) Changes() port.ChangeReader { return twinChanges{r.Reader.Changes()} }

type twinChanges struct{ port.ChangeReader }

func (c twinChanges) Of(ref change.Ref) ([]change.Record, error) {
	log, err := c.ChangeReader.Of(ref)
	if err != nil || ref.Kind != change.KindItem {
		return log, err
	}
	for _, rec := range slices.Backward(log) {
		if rec.Actor.Kind == change.ActorAutomation {
			rec.ID, rec.At = "01TWIN0000000000000000000A", rec.At.Add(time.Second)
			return append(log, rec), nil // 同じ prev のまま、別の行として残る
		}
	}
	return log, nil
}

func TestListShowsRecentRuns(t *testing.T) {
	svc, rec := setup(t)
	ctx := t.Context()
	r, _ := svc.Create(ctx, store, apptest.SampleWorkspace, doneRule())
	writeRuleRow(t, rec.Stores, r.ID, nil, "taro")
	_ = svc.Delete(ctx, store, apptest.SampleWorkspace, r.ID) // 消したルールの名前も引く(B4-16)
	svc.Stores = twinStores{svc.Stores}                       // 同じ変更の重なり: 1 行だけ(B4-15)
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
		writeRuleRow(t, rec.Stores, r.ID, nil, fmt.Sprintf("user%d", i))
	}
	v, _ := svc.List(t.Context(), store, apptest.SampleWorkspace)
	if len(v.Recent) != 8 || v.Recent[0].Text != "「完了で外す」で担当者を 不明なユーザー に変更" {
		t.Errorf("Recent = %+v", v.Recent)
	}
}

// 同じルールが順に 2 回動いたとき(外す → 人が担当者を戻す → また外す)は、2 行とも出す(B4-15)。
func TestListShowsEachFiringOfTheSameChange(t *testing.T) {
	svc, rec := setup(t)
	r, _ := svc.Create(t.Context(), store, apptest.SampleWorkspace, doneRule())
	writeRuleRow(t, rec.Stores, r.ID, "taro", nil)
	op := port.Operation{Actor: change.User("taro"), Summary: "HH-2 の担当者を taro に変更"}
	err := rec.Write(t.Context(), store, op, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("assignee", nil, "taro"))
	})
	if err != nil {
		t.Fatal(err)
	}
	writeRuleRow(t, rec.Stores, r.ID, "taro", nil)
	v, err := svc.List(t.Context(), store, apptest.SampleWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Recent) != 2 {
		t.Errorf("Recent = %+v", v.Recent)
	}
}

// 専用ルールの持ち主のプロジェクトは、表示 ID と削除済みかで返す(03「ルールの一覧」の「HH-P1」「HH-P1(削除済み)」)。
func TestListShowsTheScopesProject(t *testing.T) {
	svc, rec := setup(t)
	seedScoped(t, rec)
	if _, err := svc.Create(t.Context(), store, apptest.SampleWorkspace, doneRule()); err != nil {
		t.Fatal(err)
	}
	v, err := svc.List(t.Context(), store, apptest.SampleWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	if got := v.Scopes[apptest.SampleProject]; got != (automations.Scope{Label: "HH-P1"}) || len(v.Scopes) != 1 {
		t.Errorf("Scopes = %+v", v.Scopes)
	}
	if err := apptest.DeleteProject(t.Context(), rec.Stores, store, apptest.SampleProject); err != nil {
		t.Fatal(err)
	}
	v, err = svc.List(t.Context(), store, apptest.SampleWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	if got := v.Scopes[apptest.SampleProject]; got != (automations.Scope{Label: "HH-P1", Deleted: true}) {
		t.Errorf("deleted: Scopes = %+v", v.Scopes)
	}
}
