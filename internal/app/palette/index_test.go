package palette_test

import (
	"cmp"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/palette"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

var movedAt = time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)

func index(t *testing.T, f fixture) palette.Index {
	t.Helper()
	got, err := f.svc.Index(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func findItem(t *testing.T, idx palette.Index, store port.StoreID, display string) palette.IndexItem {
	t.Helper()
	for _, it := range idx.Items {
		if it.Store == store && it.Display == display {
			return it
		}
	}
	t.Fatalf("アイテム %s/%s がない: %+v", store, display, idx.Items)
	return palette.IndexItem{}
}

func findProject(t *testing.T, idx palette.Index, store port.StoreID, display string) palette.IndexProject {
	t.Helper()
	for _, p := range idx.Projects {
		if p.Store == store && p.Display == display {
			return p
		}
	}
	t.Fatalf("プロジェクト %s/%s がない: %+v", store, display, idx.Projects)
	return palette.IndexProject{}
}

// 表示中の 2 つの Store(1 つは閲覧のみ)のアイテム・プロジェクトが入る。削除済みのアイテム、削除済みのプロジェクトの
// 下のアイテム、削除済みのワークスペースのプロジェクト、StoreList に出ない Store のものは入らない。
func TestIndexListsLiveItemsAndProjectsOfShownStores(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	must(t, apptest.Sample(ctx, f.stores, store1))
	addItem(t, f.stores, store1, task(item12, apptest.SampleWorkspace, apptest.SampleProject, 12, "パスワードを再設定する"))
	addItem(t, f.stores, store1, task(gone, apptest.SampleWorkspace, apptest.SampleProject, 13, "削除したもの"))
	must(t, apptest.DeleteItems(ctx, f.stores, store1, gone))
	addProject(t, f.stores, store1, project.Project{ID: project2, Place: project.Place{Workspace: apptest.SampleWorkspace, Number: 2}, Rank: "W", Title: "消したプロジェクト", Status: project.Todo})
	addItem(t, f.stores, store1, task(under, apptest.SampleWorkspace, project2, 14, "消したプロジェクトの下"))
	must(t, apptest.DeleteProject(ctx, f.stores, store1, project2))
	must(t, apptest.SampleMobile(ctx, f.stores, store1))
	must(t, apptest.DeleteWorkspace(ctx, f.stores, store1, apptest.SampleMobileWorkspace))
	must(t, apptest.SampleTeam(ctx, f.stores, store2))
	must(t, apptest.SampleTeam(ctx, f.stores, store3))

	idx := index(t, f)
	type row struct {
		store                        port.StoreID
		display, title, projectTitle string
	}
	var items []row
	for _, it := range idx.Items {
		items = append(items, row{it.Store, it.Display, it.Title, it.ProjectTitle})
	}
	wantItems := []row{
		{store1, "HH-1", "認証", "ログイン"},
		{store1, "HH-12", "パスワードを再設定する", "ログイン"},
		{store1, "HH-2", "ログイン画面を作る", "ログイン"},
		{store2, "TM-1", "チーム運営", "運営"},
	}
	sortRows := func(rs []row) {
		slices.SortFunc(rs, func(a, b row) int {
			if a.store != b.store {
				return cmp.Compare(a.store, b.store)
			}
			return cmp.Compare(a.display, b.display)
		})
	}
	sortRows(items)
	if !reflect.DeepEqual(items, wantItems) {
		t.Errorf("items = %+v, want %+v", items, wantItems)
	}
	var projects []row
	for _, p := range idx.Projects {
		projects = append(projects, row{p.Store, p.Display, p.Title, p.WorkspaceName})
	}
	sortRows(projects)
	wantProjects := []row{{store1, "HH-P1", "ログイン", "Web"}, {store2, "TM-P1", "運営", "Team"}}
	if !reflect.DeepEqual(projects, wantProjects) {
		t.Errorf("projects = %+v, want %+v", projects, wantProjects)
	}

	got := findItem(t, idx, store1, "HH-12")
	want := palette.IndexItem{
		Store: store1, Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Item: item12, Number: 12,
		Display: "HH-12", Title: "パスワードを再設定する", ProjectTitle: "ログイン",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("HH-12 = %+v, want %+v", got, want)
	}
	gotP := findProject(t, idx, store2, "TM-P1")
	wantP := palette.IndexProject{
		Store: store2, Workspace: apptest.SampleTeamWorkspace, Project: apptest.SampleTeamProject,
		Display: "TM-P1", Title: "運営", WorkspaceName: "Team",
	}
	if !reflect.DeepEqual(gotP, wantP) {
		t.Errorf("TM-P1 = %+v, want %+v", gotP, wantP)
	}
}

// Store A の HH-12 を Store B の MB-4 へ移した記録(Store A の転送記録、Target はアイテムの内部 ID)→ B のアイテムの
// Aliases に HH-12(At つき)。プロジェクトも同じ(HH-P3 → MB-P1)。
func TestIndexAliasesFollowTransfersAcrossStores(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	must(t, apptest.Sample(ctx, f.stores, store1))
	must(t, apptest.SampleMobile(ctx, f.stores, store2))
	addItem(t, f.stores, store2, task(moved, apptest.SampleMobileWorkspace, apptest.SampleMobileProject, 4, "移したもの"))
	write(t, f.stores, store1, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteTransfers().Append(transfer.Record{At: movedAt, Entity: transfer.EntityItem, Target: moved, Old: "HH-12", New: "MB-4", ToStore: string(store2), ToStoreName: "team"}),
			tx.WriteTransfers().Append(transfer.Record{At: movedAt, Entity: transfer.EntityProject, Target: apptest.SampleMobileProject, Old: "HH-P3", New: "MB-P1", ToStore: string(store2), ToStoreName: "team"}),
		)
	})

	idx := index(t, f)
	if got, want := findItem(t, idx, store2, "MB-4").Aliases, []transfer.Alias{{ID: "HH-12", At: &movedAt}}; !reflect.DeepEqual(got, want) {
		t.Errorf("MB-4 aliases = %+v, want %+v", got, want)
	}
	if got, want := findProject(t, idx, store2, "MB-P1").Aliases, []transfer.Alias{{ID: "HH-P3", At: &movedAt}}; !reflect.DeepEqual(got, want) {
		t.Errorf("MB-P1 aliases = %+v, want %+v", got, want)
	}
	if got := findItem(t, idx, store1, "HH-2").Aliases; len(got) != 0 {
		t.Errorf("HH-2 aliases = %+v, want none", got)
	}
}

// ワークスペースの接頭辞を WEB → W に変えた → W-3 の Aliases に WEB-3(At なし)。いまの ID と同じものは入らない
// (W-3 から移して戻した記録の Old「W-3」)。プロジェクトも W-P1 の Aliases に WEB-P1。
func TestIndexAliasesIncludeOldPrefixes(t *testing.T) {
	f := newFixture(t)
	ws := workspace.Workspace{ID: webWS, Name: "Web", Prefix: "WEB"}
	d, err := workspace.CreateWithSettings(ws, workspace.DefaultSettings(apptest.NewIDs().New))
	must(t, err)
	write(t, f.stores, store1, func(tx port.Tx) error { return tx.WriteWorkspaces().Append(ws.ID, d) })
	addProject(t, f.stores, store1, project.Project{ID: webProj, Place: project.Place{Workspace: webWS, Number: 1}, Rank: "V", Title: "サイト", Status: project.Todo})
	addItem(t, f.stores, store1, task(webTask, webWS, webProj, 3, "トップページ"))
	write(t, f.stores, store1, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteWorkspaces().Append(webWS, change.NewSet("prefix", "WEB", "W")),
			tx.WriteTransfers().Append(transfer.Record{At: movedAt, Entity: transfer.EntityItem, Target: webTask, Old: "W-3", New: "W-9", ToStore: string(store1)}),
		)
	})

	idx := index(t, f)
	if got, want := findItem(t, idx, store1, "W-3").Aliases, []transfer.Alias{{ID: "WEB-3"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("W-3 aliases = %+v, want %+v", got, want)
	}
	if got, want := findProject(t, idx, store1, "W-P1").Aliases, []transfer.Alias{{ID: "WEB-P1"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("W-P1 aliases = %+v, want %+v", got, want)
	}
}
