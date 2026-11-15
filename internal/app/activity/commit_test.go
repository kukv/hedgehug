package activity

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/automation"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const s1 port.StoreID = "S1"

var s1Info = port.StoreInfo{ID: s1, Name: "work", Color: "sky"}

// doneRule は、「進行中なら完了」のルール(Sample の Web のワークスペース)。
var doneRule = automation.Rule{
	ID: "01ACTRULE00000000000000001", Name: "進行中なら完了", Trigger: automation.TriggerStatus,
	TriggerStatus: apptest.SampleStatusInProgress, Action: automation.ActionStatus, Value: string(apptest.SampleStatusDone), Enabled: true,
}

type fixture struct {
	clock  *apptest.Clock
	stores *apptest.Stores
	log    *apptest.CommitLog
}

// newFixture は、Sample(taro・hanako、Web の HH-P1、HH-1 とその下の HH-2)を書き、taro の表示名を「田中 太郎」にする。
func newFixture(t *testing.T) fixture {
	t.Helper()
	clock := apptest.NewClock(time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC))
	s := apptest.NewStores(clock, apptest.NewIDs())
	s.AddStore(s1)
	f := fixture{clock: clock, stores: s, log: apptest.NewCommitLog(s, clock)}
	if err := apptest.Sample(t.Context(), f.log, s1); err != nil {
		t.Fatal(err)
	}
	f.write(t, change.User("taro"), "メンバー taro の表示名を変更", func(tx port.Tx) error {
		return tx.WriteMembers().Append(member.Rename("taro", "taro", "田中 太郎"))
	})
	return f
}

func (f fixture) write(t *testing.T, actor change.Actor, summary string, fn func(tx port.Tx) error) {
	t.Helper()
	if err := f.log.Write(t.Context(), s1, port.Operation{Actor: actor, Summary: summary}, fn); err != nil {
		t.Fatalf("write %q: %v", summary, err)
	}
}

func (f fixture) setItem(t *testing.T, summary string, id item.ID, drafts ...change.Draft) {
	t.Helper()
	f.write(t, change.User("taro"), summary, func(tx port.Tx) error { return tx.WriteItems().Append(id, drafts...) })
}

// commit は、件名が summary の、いちばん新しいコミット。
func (f fixture) commit(t *testing.T, summary string) port.Commit {
	t.Helper()
	got, err := f.log.Commits(t.Context(), s1)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range got {
		if c.Summary == summary {
			return c
		}
	}
	t.Fatalf("no commit %q", summary)
	return port.Commit{}
}

func (f fixture) rowsOf(t *testing.T, c port.Commit) []Row {
	t.Helper()
	var rows []Row
	err := f.stores.Read(t.Context(), s1, func(r port.Reader) error {
		var err error
		rows, err = commitRows(r, s1Info, c)
		return err
	})
	if err != nil {
		t.Fatalf("commitRows: %v", err)
	}
	return rows
}

func (f fixture) rows(t *testing.T, summary string) []Row {
	t.Helper()
	return f.rowsOf(t, f.commit(t, summary))
}

func assertTarget(t *testing.T, name string, got *Target, want *Target) {
	t.Helper()
	switch {
	case want == nil && got != nil:
		t.Errorf("%s = %+v, want nil", name, *got)
	case want != nil && got == nil:
		t.Errorf("%s = nil, want %+v", name, *want)
	case want != nil && *got != *want:
		t.Errorf("%s = %+v, want %+v", name, *got, *want)
	}
}

func TestRowsUserSave(t *testing.T) {
	f := newFixture(t)
	f.setItem(t, "HH-1 のタイトルを変更", apptest.SampleEpic, change.NewSet("title", "認証", "二段階認証"))
	c := f.commit(t, "HH-1 のタイトルを変更")
	rows := f.rowsOf(t, c)
	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want 1", rows)
	}
	r := rows[0]
	if r.Actor != ActorUser || r.Who != "田中 太郎" || r.Avatar != "田中 太郎" || r.Text != "HH-1 のタイトルを変更" {
		t.Errorf("actor %q who %q avatar %q text %q", r.Actor, r.Who, r.Avatar, r.Text)
	}
	if r.Key != c.SHA+":0" || r.SHA != c.SHA[:7] || len(r.SHA) != 7 || !r.At.Equal(c.At) || r.Store != s1Info || r.dedup != "" {
		t.Errorf("key %q sha %q at %v store %+v dedup %q", r.Key, r.SHA, r.At, r.Store, r.dedup)
	}
	if !slices.Equal(r.Workspaces, []workspace.ID{apptest.SampleWorkspace}) || r.Workspace != "Web" || r.Storewide {
		t.Errorf("workspaces %v name %q storewide %v", r.Workspaces, r.Workspace, r.Storewide)
	}
	assertTarget(t, "Item", r.Item, &Target{Display: "HH-1", Link: "current"})
	assertTarget(t, "Project", r.Project, &Target{Display: "HH-P1", Title: "ログイン", Link: "current"})
}

func TestRowsUnknownUser(t *testing.T) {
	f := newFixture(t)
	f.write(t, change.User("jiro"), "HH-1 のタイトルを変更", func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleEpic, change.NewSet("title", "認証", "二段階認証"))
	})
	rows := f.rows(t, "HH-1 のタイトルを変更")
	if len(rows) != 1 || rows[0].Actor != ActorUser || rows[0].Who != "不明なユーザー" || rows[0].Avatar != "" {
		t.Errorf("rows = %+v", rows)
	}
}

func (f fixture) addRule(t *testing.T) {
	t.Helper()
	f.write(t, change.User("taro"), "自動化を作成", func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, automation.Drafts(automation.Rule{}, doneRule)...)
	})
}

// setStatusByRule は、taro が HH-2 を進行中にし、同じ保存で doneRule が Done にする。
func (f fixture) setStatusByRule(t *testing.T, summary string) {
	t.Helper()
	f.setItem(t, summary, apptest.SampleTask,
		change.NewSet("status", apptest.SampleStatusTodo, apptest.SampleStatusInProgress),
		change.NewSet("status", apptest.SampleStatusInProgress, apptest.SampleStatusDone).ByRule(doneRule.ID))
}

func TestRowsAutomation(t *testing.T) {
	f := newFixture(t)
	f.addRule(t)
	f.setStatusByRule(t, "HH-2 のステータスを変更")
	c := f.commit(t, "HH-2 のステータスを変更")
	rows := f.rowsOf(t, c)
	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want 2", rows)
	}
	if rows[0].Actor != ActorUser || rows[0].Who != "田中 太郎" || rows[0].dedup != "" {
		t.Errorf("main = %+v", rows[0])
	}
	a := rows[1]
	want := automation.ActivityText("進行中なら完了", automation.ChangeText("ステータス", "Done"))
	if a.Actor != ActorAutomation || a.Who != "自動化" || a.Avatar != "" || a.Text != want || a.Text != "「進行中なら完了」でステータスを Done に変更" {
		t.Errorf("automation actor %q who %q avatar %q text %q", a.Actor, a.Who, a.Avatar, a.Text)
	}
	if a.SHA != rows[0].SHA || !a.At.Equal(rows[0].At) || a.Key != c.SHA+":1" || a.dedup == "" {
		t.Errorf("automation sha %q at %v key %q dedup %q", a.SHA, a.At, a.Key, a.dedup)
	}
	if !slices.Equal(a.Workspaces, []workspace.ID{apptest.SampleWorkspace}) || a.Workspace != "Web" || a.Storewide {
		t.Errorf("automation workspaces %v %q %v", a.Workspaces, a.Workspace, a.Storewide)
	}
	assertTarget(t, "automation Item", a.Item, &Target{Display: "HH-2", Link: "current"})
	assertTarget(t, "automation Project", a.Project, &Target{Display: "HH-P1", Title: "ログイン", Link: "current"})
}

// 同じ変更の自動化の行は、別のコミットでも同じ重なりの鍵を持つ。変更が違えば鍵も違う。
func TestRowsAutomationDedupKey(t *testing.T) {
	f := newFixture(t)
	f.addRule(t)
	f.setStatusByRule(t, "1 回目")
	f.setItem(t, "戻す", apptest.SampleTask, change.NewSet("status", apptest.SampleStatusDone, apptest.SampleStatusTodo))
	f.setStatusByRule(t, "2 回目")
	first, second := f.rows(t, "1 回目"), f.rows(t, "2 回目")
	if first[1].dedup == "" || first[1].dedup != second[1].dedup {
		t.Errorf("dedup = %q, %q; want the same key", first[1].dedup, second[1].dedup)
	}
	if first[1].dedup == first[0].dedup {
		t.Errorf("automation key %q equals the main key", first[1].dedup)
	}
}

func TestRowsMemberJoin(t *testing.T) {
	f := newFixture(t)
	f.write(t, change.User("taro"), "HH-1 を担当者つきで変更", func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Append(apptest.SampleEpic, change.NewSet("title", "認証", "二段階認証")),
			tx.WriteMembers().Append(member.Join("hanako"), member.Join("jiro")),
		)
	})
	c := f.commit(t, "HH-1 を担当者つきで変更")
	rows := f.rowsOf(t, c)
	texts := make([]string, 0, len(rows))
	for _, r := range rows {
		texts = append(texts, r.Text)
	}
	if want := []string{"HH-1 を担当者つきで変更", "メンバー hanako を追加", "メンバー jiro を追加"}; !slices.Equal(texts, want) {
		t.Fatalf("texts = %q, want %q", texts, want)
	}
	for i, r := range rows[1:] {
		if r.Actor != ActorUser || r.Who != "田中 太郎" || r.SHA != rows[0].SHA || !r.At.Equal(rows[0].At) || r.Key != c.SHA+":"+string(rune('1'+i)) {
			t.Errorf("member row %d = %+v", i, r)
		}
		if len(r.Workspaces) != 0 || !r.Storewide || r.Item != nil || r.Project != nil {
			t.Errorf("member row %d: workspaces %v storewide %v item %v project %v", i, r.Workspaces, r.Storewide, r.Item, r.Project)
		}
	}
	f.write(t, change.User("taro"), "メンバー saburo を追加", func(tx port.Tx) error {
		return tx.WriteMembers().Append(member.Join("saburo"))
	})
	only := f.rows(t, "メンバー saburo を追加")
	if len(only) != 1 || only[0].Text != "メンバー saburo を追加" || !only[0].Storewide || len(only[0].Workspaces) != 0 {
		t.Errorf("members only = %+v", only)
	}
}

// newItem は、Web のプロジェクト p のアイテム。parent があればその下。
func newItem(id item.ID, p project.ID, number int, kind item.Kind, parent *item.ID) item.Item {
	return item.Item{
		ID: id, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: p, Parent: parent, Number: number},
		Kind: kind, Rank: rank.First, Title: "新しいアイテム", Status: apptest.SampleStatusTodo,
	}
}

func TestRowsTargetRoot(t *testing.T) {
	f := newFixture(t)
	parent := item.ID("01ACTITEM00000000000000003")
	f.write(t, change.User("taro"), "テンプレートから作成", func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Append(parent, item.Create(newItem(parent, apptest.SampleProject, 3, item.Epic, nil))),
			tx.WriteItems().Append("01ACTITEM00000000000000004", item.Create(newItem("01ACTITEM00000000000000004", apptest.SampleProject, 4, item.Task, &parent))),
			tx.WriteItems().Append("01ACTITEM00000000000000005", item.Create(newItem("01ACTITEM00000000000000005", apptest.SampleProject, 5, item.Task, &parent))),
		)
	})
	rows := f.rows(t, "テンプレートから作成")
	assertTarget(t, "template Item", rows[0].Item, &Target{Display: "HH-3", Link: "current"})
	assertTarget(t, "template Project", rows[0].Project, &Target{Display: "HH-P1", Title: "ログイン", Link: "current"})

	p2 := project.Project{ID: "01ACTPROJ00000000000000002", Place: project.Place{Workspace: apptest.SampleWorkspace, Number: 2}, Rank: "W", Title: "決済", Status: project.Todo}
	other := item.ID("01ACTITEM00000000000000006")
	f.write(t, change.User("taro"), "HH-P2 を作成", func(tx port.Tx) error {
		return errors.Join(
			tx.WriteProjects().Append(p2.ID, project.Create(p2)),
			tx.WriteItems().Append(other, item.Create(newItem(other, p2.ID, 6, item.Epic, nil))),
		)
	})
	f.write(t, change.User("taro"), "2 件を一括変更", func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Append(apptest.SampleEpic, change.NewSet("title", "認証", "二段階認証")),
			tx.WriteItems().Append(other, change.NewSet("title", "新しいアイテム", "決済画面")),
		)
	})
	bulk := f.rows(t, "2 件を一括変更")
	assertTarget(t, "bulk Item", bulk[0].Item, nil)
	assertTarget(t, "bulk Project", bulk[0].Project, nil)
}

func TestRowsRepairWithoutOpRows(t *testing.T) {
	f := newFixture(t)
	const parentOp change.ID = "01ACTPARENTOP0000000000001"
	f.write(t, change.Actor{Kind: change.ActorSync}, "HH-2 を削除", func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("title", "ログイン画面を作る", "ログイン").InOp(parentOp))
	})
	c := f.commit(t, "HH-2 を削除")
	c.Op = "01ACTREPAIROP0000000000001" // 修復 (2): 行の op は親の削除のもので、コミットの op と違う
	if c.Author != change.BotLogin {
		t.Fatalf("author = %q", c.Author)
	}
	rows := f.rowsOf(t, c)
	if len(rows) != 1 || rows[0].Actor != ActorSync || rows[0].Who != "同期" || rows[0].Avatar != "" || rows[0].dedup == "" {
		t.Fatalf("rows = %+v", rows)
	}
	assertTarget(t, "Item", rows[0].Item, &Target{Display: "HH-2", Link: "current"})
}

// 行のない、bot が作者のコミット(のせ直しの修復)は、実行者「同期」で、重なりの鍵は件名で決まる。
func TestRowsBotWithoutRows(t *testing.T) {
	f := newFixture(t)
	c := port.Commit{SHA: "abcdef0123456789", Summary: "スプリント「Sprint 25」を計画中に戻す", Author: change.BotLogin, At: time.Now()}
	d := c
	d.SHA = "0123456789abcdef"
	a, b := f.rowsOf(t, c), f.rowsOf(t, d)
	if len(a) != 1 || a[0].Actor != ActorSync || a[0].Who != "同期" || a[0].dedup == "" || a[0].dedup != b[0].dedup {
		t.Errorf("rows = %+v, %+v", a, b)
	}
}

func TestRowsMovedOut(t *testing.T) {
	f := newFixture(t)
	moved := item.ID("01ACTITEM00000000000000003")
	purged := item.ID("01ACTITEM00000000000000004")
	f.write(t, change.User("taro"), "アイテムを作成", func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Append(moved, item.Create(newItem(moved, apptest.SampleProject, 3, item.Epic, nil))),
			tx.WriteItems().Append(purged, item.Create(newItem(purged, apptest.SampleProject, 4, item.Epic, nil))),
		)
	})
	f.write(t, change.User("taro"), "HH-3 を Team へ移動", func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Remove(moved),
			tx.WriteTransfers().Append(transfer.Record{Entity: transfer.EntityItem, Target: moved, Old: "HH-3", New: "TM-2", ToStore: "S2"}),
		)
	})
	rows := f.rows(t, "HH-3 を Team へ移動")
	assertTarget(t, "moved Item", rows[0].Item, &Target{Display: "HH-3", Link: "moved"})
	assertTarget(t, "moved Project", rows[0].Project, nil)
	if len(rows[0].Workspaces) != 0 || rows[0].Workspace != "" || rows[0].Storewide {
		t.Errorf("moved workspaces %v %q storewide %v", rows[0].Workspaces, rows[0].Workspace, rows[0].Storewide)
	}

	f.write(t, change.User("taro"), "HH-4 を完全に削除", func(tx port.Tx) error { return tx.WriteItems().Remove(purged) })
	f.stores.AddPurged(s1, port.Purged{Entity: "item", Target: purged, Display: "HH-4"})
	assertTarget(t, "purged Item", f.rows(t, "HH-4 を完全に削除")[0].Item, &Target{Display: "HH-4"})
}

// 別の Store へ移したアイテムは、いまの親が分からないので、子ごと移したときはどれも根になり、対象を出さない。
func TestRowsMovedOutWithChild(t *testing.T) {
	f := newFixture(t)
	f.write(t, change.User("taro"), "HH-1 を Team へ移動", func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Remove(apptest.SampleEpic),
			tx.WriteItems().Remove(apptest.SampleTask),
			tx.WriteTransfers().Append(transfer.Record{Entity: transfer.EntityItem, Target: apptest.SampleEpic, Old: "HH-1", New: "TM-2", ToStore: "S2"}),
			tx.WriteTransfers().Append(transfer.Record{Entity: transfer.EntityItem, Target: apptest.SampleTask, Old: "HH-2", New: "TM-3", ToStore: "S2"}),
		)
	})
	assertTarget(t, "Item", f.rows(t, "HH-1 を Team へ移動")[0].Item, nil)
}

func TestRowsMovedWithinStore(t *testing.T) {
	f := newFixture(t)
	if err := apptest.SampleMobile(t.Context(), f.log, s1); err != nil {
		t.Fatal(err)
	}
	f.addRule(t)
	f.setStatusByRule(t, "HH-2 のステータスを変更")
	task := newItem(apptest.SampleTask, apptest.SampleProject, 2, item.Task, nil)
	parent := apptest.SampleEpic
	task.Place.Parent = &parent
	f.setItem(t, "HH-2 を Mobile へ移動", apptest.SampleTask, change.NewSet("place", task.Place,
		item.Place{Workspace: apptest.SampleMobileWorkspace, Project: apptest.SampleMobileProject, Number: 2}))
	rows := f.rows(t, "HH-2 のステータスを変更")
	if len(rows) != 2 {
		t.Fatalf("rows = %+v", rows)
	}
	for i, r := range rows {
		if !slices.Equal(r.Workspaces, []workspace.ID{apptest.SampleMobileWorkspace}) || r.Workspace != "Mobile" {
			t.Errorf("row %d workspaces %v %q", i, r.Workspaces, r.Workspace)
		}
		assertTarget(t, "Item", r.Item, &Target{Display: "MB-2", Link: "current"})
		assertTarget(t, "Project", r.Project, &Target{Display: "MB-P1", Title: "アプリ", Link: "current"})
	}
	if rows[1].Text != "「進行中なら完了」でステータスを Done に変更" {
		t.Errorf("automation text = %q", rows[1].Text)
	}
}

func TestRowsDeletedItem(t *testing.T) {
	f := newFixture(t)
	if err := apptest.DeleteItems(t.Context(), f.log, s1, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
	rows := f.rows(t, "アイテムを削除")
	assertTarget(t, "Item", rows[0].Item, &Target{Display: "HH-2"})
	assertTarget(t, "Project", rows[0].Project, &Target{Display: "HH-P1", Title: "ログイン", Link: "current"})
	if !slices.Equal(rows[0].Workspaces, []workspace.ID{apptest.SampleWorkspace}) {
		t.Errorf("workspaces = %v", rows[0].Workspaces)
	}
}

// retentionStores は、内側の Write のあとで保持日数を変える(Write の中からは、偽物が同じロックを取るので変えられない)。
type retentionStores struct {
	port.Stores
	base *apptest.Stores
	days int
}

func (r retentionStores) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	if err := r.Stores.Write(ctx, id, op, fn); err != nil {
		return err
	}
	r.base.SetRetentionDays(id, r.days)
	return nil
}

func TestRowsRetention(t *testing.T) {
	f := newFixture(t)
	log := apptest.NewCommitLog(retentionStores{Stores: f.stores, base: f.stores, days: 7}, f.clock)
	op := port.Operation{Actor: change.User("taro"), Summary: "削除済みの保持日数を 7 日に変更"}
	if err := log.Write(t.Context(), s1, op, func(port.Tx) error { return nil }); err != nil {
		t.Fatal(err)
	}
	got, err := log.Commits(t.Context(), s1)
	if err != nil || len(got) != 1 || !got[0].StoreLog {
		t.Fatalf("commits = %+v, %v", got, err)
	}
	rows := f.rowsOf(t, got[0])
	if len(rows) != 1 || !rows[0].Storewide || len(rows[0].Workspaces) != 0 || rows[0].Item != nil || rows[0].Project != nil {
		t.Errorf("rows = %+v", rows)
	}
	if rows[0].Actor != ActorUser || rows[0].Who != "田中 太郎" {
		t.Errorf("actor %q who %q", rows[0].Actor, rows[0].Who)
	}
}
