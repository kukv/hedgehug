package search

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/comment"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/query"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

const (
	sprintActive  change.ID = "01TESTSP000000000000000001"
	sprintPlanned change.ID = "01TESTSP000000000000000002"
	sprintLater   change.ID = "01TESTSP000000000000000003"
	release2      change.ID = "01TESTRL000000000000000001"
)

// basicStore は、サンプルに次を足す。
//   - スプリント: 実行中「Sprint 3」、計画中「Sprint 4」(開始 12/01)、計画中「Sprint 5」(開始日なし)
//   - リリース「v2.0.0」。hanako の表示名を「花子」にする
//   - HH-2: hanako の担当、Sprint 3、v2.0.0、PR のリンク、HH-4 をブロック、削除済みの HH-5 と関連。あとでタイトルと順位を変える
//   - HH-3: HH-2 の下の Step、Sprint 4、Issue のリンクだけ、HH-2 と重複
//   - プロジェクト HH-P1 の旧 ID「OLD-P3」(転送記録)
func basicStore(t *testing.T, f fixture) {
	t.Helper()
	must(t, apptest.Sample(t.Context(), f.stores, store1))
	addItem(t, f.stores, task(item3, 3, item.Step, ptr(apptest.SampleTask), "手順"))
	addItem(t, f.stores, task(item4, 4, item.Task, nil, "ブロックされるもの"))
	addItem(t, f.stores, task(item5, 5, item.Task, nil, "消したもの"))
	must(t, apptest.DeleteItems(t.Context(), f.stores, store1, item5))
	pr := itemlink.Ref{Kind: itemlink.PR, Repo: "kukv/web", Number: 7}
	issue := itemlink.Ref{Kind: itemlink.Issue, Repo: "kukv/web", Number: 8}
	v := itemlink.Values{Title: "t", State: itemlink.Open, Review: itemlink.NoReview}
	write(t, f.stores, func(tx port.Tx) error {
		var ds []change.Draft
		ds = append(ds, planning.SprintDrafts(planning.Sprint{}, planning.Sprint{ID: sprintActive, Name: "Sprint 3", State: planning.Active, Start: "2026-11-10"})...)
		ds = append(ds, planning.SprintDrafts(planning.Sprint{}, planning.Sprint{ID: sprintPlanned, Name: "Sprint 4", State: planning.Planned, Start: "2026-12-01"})...)
		ds = append(ds, planning.SprintDrafts(planning.Sprint{}, planning.Sprint{ID: sprintLater, Name: "Sprint 5", State: planning.Planned})...)
		ds = append(ds, planning.ReleaseDrafts(planning.Release{}, planning.Release{ID: release2, Name: "v2.0.0", State: planning.Unreleased})...)
		task2 := append([]change.Draft{
			change.NewSet(item.SprintField, nil, sprintActive), change.NewSet(item.ReleaseField, nil, release2),
			change.NewSet("assignee", nil, "hanako"),
			change.NewAdd("relations", item.Relation{Type: "blocks", To: item4}),
			change.NewAdd("relations", item.Relation{Type: "relates", To: item5}),
		}, itemlink.Set{}.Attach(pr, v)...)
		item3Drafts := append([]change.Draft{
			change.NewSet(item.SprintField, nil, sprintPlanned),
			change.NewAdd("relations", item.Relation{Type: "duplicates", To: apptest.SampleTask}),
		}, itemlink.Set{}.Attach(issue, v)...)
		return errors.Join(
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace, ds...),
			tx.WriteMembers().Append(member.Rename("hanako", "hanako", "花子")),
			tx.WriteItems().Append(apptest.SampleTask, task2...),
			tx.WriteItems().Append(item3, item3Drafts...),
			tx.WriteTransfers().Append(transfer.Record{
				Entity: transfer.EntityProject, Target: apptest.SampleProject, Old: "OLD-P3", New: "HH-P1", ToStore: string(store1),
			}),
		)
	})
	write(t, f.stores, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("title", "ログイン画面を作る", "ログイン画面"))
	})
	write(t, f.stores, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("rank", "a0", "a1"))
	})
}

// atOf は、ログのうち項目 field の最後の set の時刻。
func atOf(log []change.Record, field string) (at change.Record) {
	for _, r := range log {
		if r.T == change.Set && r.F == field {
			at = r
		}
	}
	return at
}

func TestFactsBasic(t *testing.T) {
	f := newFixture(t)
	basicStore(t, f)
	es := entriesOf(t, f, "status:todo")
	if got, want := displays(es), []string{"S1/HH-1", "S1/HH-2", "S1/HH-3", "S1/HH-4"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("displays = %v, want %v", got, want)
	}
	checkTask2(t, factsOf(t, es, store1, "HH-2"), logOf(t, f.stores, store1, apptest.SampleTask))
	checkOthers(t, es)
}

var hh1, hh2 = query.Ref{Display: "HH-1"}, query.Ref{Display: "HH-2"}

// checkTask2 は、HH-2 の材料を確かめる。log は HH-2 のログ。
func checkTask2(t *testing.T, got query.Facts, log []change.Record) {
	t.Helper()
	if got.StoreName != "main" || got.StoreOrder != 0 || got.WorkspaceOrder != 0 || got.Workspace.Prefix != "HH" {
		t.Errorf("Store・ワークスペース = %q %d %d %q", got.StoreName, got.StoreOrder, got.WorkspaceOrder, got.Workspace.Prefix)
	}
	if got.Item.Status != apptest.SampleStatusTodo || len(got.Settings.Statuses) == 0 || got.Item.Assignee != "hanako" {
		t.Errorf("ステータス・担当者 = %q %q", got.Item.Status, got.Item.Assignee)
	}
	if want := map[string]string{"taro": "taro", "hanako": "花子"}; !reflect.DeepEqual(got.Names, want) {
		t.Errorf("Names = %v, want %v", got.Names, want)
	}
	if want := (query.Ref{Display: "HH-P1", Aliases: []string{"OLD-P3"}}); !reflect.DeepEqual(got.Project, want) || got.ProjectTitle != "ログイン" {
		t.Errorf("Project = %+v %q", got.Project, got.ProjectTitle)
	}
	if !reflect.DeepEqual(got.Parent, &hh1) || !reflect.DeepEqual(got.Ancestors, []query.Ref{hh1}) || got.ParentTitle != "認証" {
		t.Errorf("Parent = %+v, Ancestors = %+v", got.Parent, got.Ancestors)
	}
	wantRel := map[string][]query.Ref{"blocks": {{Display: "HH-4"}}, "duplicated_by": {{Display: "HH-3"}}}
	if !reflect.DeepEqual(got.Relations, wantRel) {
		t.Errorf("Relations = %+v, want %+v", got.Relations, wantRel)
	}
	if got.SprintName != "Sprint 3" || !got.SprintCurrent || got.SprintNext || got.ReleaseName != "v2.0.0" {
		t.Errorf("スプリント・リリース = %q %v %v %q", got.SprintName, got.SprintCurrent, got.SprintNext, got.ReleaseName)
	}
	if !got.HasPR || got.Me != "taro" || got.Item.Blocked {
		t.Errorf("HasPR = %v, Me = %q, Blocked = %v", got.HasPR, got.Me, got.Item.Blocked)
	}
	checkTimes(t, got, log)
}

// checkTimes は、HH-2 の作成と最後に変わった時刻(順位の変更は数えない)を確かめる。
func checkTimes(t *testing.T, got query.Facts, log []change.Record) {
	t.Helper()
	if !got.Created.Equal(log[0].At) || !got.Updated.Equal(atOf(log, "title").At) || got.Updated.Equal(atOf(log, "rank").At) {
		t.Errorf("Created = %v, Updated = %v; log = %+v", got.Created, got.Updated, log)
	}
}

// checkOthers は、HH-1・HH-3・HH-4 の親子・関連・スプリント・ブロック中を確かめる。
func checkOthers(t *testing.T, es []entry) {
	t.Helper()
	step := factsOf(t, es, store1, "HH-3")
	if !reflect.DeepEqual(step.Parent, &hh2) || !reflect.DeepEqual(step.Ancestors, []query.Ref{hh2, hh1}) {
		t.Errorf("HH-3 Parent = %+v, Ancestors = %+v", step.Parent, step.Ancestors)
	}
	if !reflect.DeepEqual(step.Relations, map[string][]query.Ref{"duplicates": {hh2}}) {
		t.Errorf("HH-3 Relations = %+v", step.Relations)
	}
	if step.SprintName != "Sprint 4" || step.SprintCurrent || !step.SprintNext || step.HasPR {
		t.Errorf("HH-3 = %q %v %v HasPR %v", step.SprintName, step.SprintCurrent, step.SprintNext, step.HasPR)
	}

	blocked := factsOf(t, es, store1, "HH-4")
	if !blocked.Item.Blocked || !reflect.DeepEqual(blocked.Relations, map[string][]query.Ref{"blocked_by": {hh2}}) {
		t.Errorf("HH-4 Blocked = %v, Relations = %+v", blocked.Item.Blocked, blocked.Relations)
	}
	if epic := factsOf(t, es, store1, "HH-1"); epic.Parent != nil || epic.Ancestors != nil || len(epic.Relations) != 0 {
		t.Errorf("HH-1 = %+v %+v %+v", epic.Parent, epic.Ancestors, epic.Relations)
	}
}

func TestFactsTextOnlyWhenNeeded(t *testing.T) {
	f := newFixture(t)
	must(t, apptest.Sample(t.Context(), f.stores, store1))
	write(t, f.stores, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Describe(apptest.SampleTask, "説明の本文"),
			tx.WriteItems().Append(apptest.SampleTask, change.NewDesc(),
				change.NewComment("01TESTCM000000000000000001", nil, comment.Value("残すコメント")),
				change.NewComment("01TESTCM000000000000000002", nil, comment.Value("消すコメント"))),
		)
	})
	write(t, f.stores, func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask,
			change.NewComment("01TESTCM000000000000000002", comment.Value("消すコメント"), nil))
	})

	no := factsOf(t, entriesOf(t, f, "status:todo"), store1, "HH-2")
	if no.Description != "" || no.Comments != nil {
		t.Errorf("NeedsText が偽: %q %v", no.Description, no.Comments)
	}
	yes := factsOf(t, entriesOf(t, f, "ログイン"), store1, "HH-2")
	if yes.Description != "説明の本文" || !reflect.DeepEqual(yes.Comments, []string{"残すコメント"}) {
		t.Errorf("NeedsText が真: %q %v", yes.Description, yes.Comments)
	}
}

func TestFactsInvalid(t *testing.T) {
	f := newFixture(t)
	must(t, apptest.Sample(t.Context(), f.stores, store1))
	bad := task(item3, 3, item.Task, ptr(apptest.SampleEpic), "日付が逆")
	bad.Start, bad.End = "2026-11-10", "2026-11-01"
	addItem(t, f.stores, bad)
	good := task(item4, 4, item.Task, ptr(apptest.SampleEpic), "日付が順")
	good.Start, good.End = "2026-11-01", "2026-11-10"
	addItem(t, f.stores, good)
	// HH-5・HH-6: 同じ Issue をリンクする(Issue の二重リンク。Issues はこのパッケージがログから作る)。
	issue := itemlink.Ref{Kind: itemlink.Issue, Repo: "kukv/web", Number: 9}
	v := itemlink.Values{Title: "t", State: itemlink.Open, Review: itemlink.NoReview}
	for i, id := range []item.ID{item5, item6} {
		addItem(t, f.stores, task(id, 5+i, item.Task, ptr(apptest.SampleEpic), "同じ Issue"))
		write(t, f.stores, func(tx port.Tx) error { return tx.WriteItems().Append(id, itemlink.Set{}.Attach(issue, v)...) })
	}

	var invalid []string
	for _, e := range entriesOf(t, f, "is:invalid") {
		if e.facts.Invalid {
			invalid = append(invalid, e.facts.Display)
		}
	}
	if want := []string{"HH-3", "HH-5", "HH-6"}; !reflect.DeepEqual(invalid, want) {
		t.Errorf("is:invalid の Invalid = %v, want %v", invalid, want)
	}
	for _, e := range entriesOf(t, f, "status:todo") {
		if e.facts.Invalid {
			t.Errorf("NeedsInvalid が偽なのに %s が Invalid", e.facts.Display)
		}
	}
}

func TestFactsOverwrittenOnlyWhenNeeded(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	must(t, apptest.Sample(ctx, f.stores, store1))
	must(t, apptest.SampleTeam(ctx, f.stores, store2))
	over := func(store port.StoreID, id item.ID) port.Notice {
		return port.Notice{Kind: port.NoticeOverwrite, Store: store, Workspace: apptest.SampleWorkspace, Item: id, Text: "上書き"}
	}
	// HH-2 の未読の上書きは、いちばん古い(2 ページ目にある)。
	must(t, f.inbox.Add(ctx, over(store1, apptest.SampleTask)))
	// HH-1: 既読の上書きが inboxPage 件と、未読のウォッチ。別の Store の同じ内部 ID の未読の上書き。
	for range inboxPage {
		must(t, f.inbox.Add(ctx, over(store1, apptest.SampleEpic)))
	}
	rows, err := f.inbox.List(ctx, port.NoticeQuery{Stores: []port.StoreID{store1}})
	must(t, err)
	for _, r := range rows {
		if r.Item == apptest.SampleEpic {
			must(t, f.inbox.SetRead(ctx, r.ID, true))
		}
	}
	must(t, f.inbox.Add(ctx, port.Notice{Kind: port.NoticeWatch, Store: store1, Item: apptest.SampleEpic, Text: "ウォッチ"}))
	must(t, f.inbox.Add(ctx, over(store2, apptest.SampleEpic)))

	es := entriesOf(t, f, "is:overwritten")
	if !factsOf(t, es, store1, "HH-2").Overwritten || factsOf(t, es, store1, "HH-1").Overwritten || factsOf(t, es, store2, "TM-1").Overwritten {
		t.Errorf("Overwritten: HH-2 %v, HH-1 %v, TM-1 %v", factsOf(t, es, store1, "HH-2").Overwritten,
			factsOf(t, es, store1, "HH-1").Overwritten, factsOf(t, es, store2, "TM-1").Overwritten)
	}

	f.svc.Inbox = failingInbox{t: t}
	for _, e := range entriesOf(t, f, "status:todo") {
		if e.facts.Overwritten {
			t.Errorf("NeedsInbox が偽なのに %s が Overwritten", e.facts.Display)
		}
	}
}

// ワークスペース A の接頭辞を HH から WEB に変え、ワークスペース B が HH を使うとき、A のアイテムの旧 ID に HH-1 がある。
func TestSearchOldPrefixReusedByAnother(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	must(t, apptest.Sample(ctx, f.stores, store1))
	write(t, f.stores, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewSet("prefix", "HH", "WEB"))
	})
	must(t, apptest.SampleMobile(ctx, f.stores, store1))
	write(t, f.stores, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleMobileWorkspace, change.NewSet("prefix", "MB", "HH"))
	})

	es := entriesOf(t, f, "")
	if a := factsOf(t, es, store1, "WEB-1"); !slices.Contains(a.Aliases, "HH-1") {
		t.Errorf("WEB-1 Aliases = %v, want HH-1 を含む", a.Aliases)
	}
	if b := factsOf(t, es, store1, "HH-1"); !reflect.DeepEqual(b.Aliases, []string{"MB-1"}) {
		t.Errorf("HH-1 Aliases = %v, want [MB-1]", b.Aliases)
	}
	if p := factsOf(t, es, store1, "WEB-2"); !reflect.DeepEqual(p.Project, query.Ref{Display: "WEB-P1", Aliases: []string{"HH-P1"}}) {
		t.Errorf("WEB-2 Project = %+v", p.Project)
	}
}
