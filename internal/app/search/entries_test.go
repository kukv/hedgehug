package search

import (
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/query"
)

const project2 project.ID = "01TESTPR000000000000000002"

// is:deleted なら、削除済みのアイテムのうちワークスペースが削除されていないものだけ。残り日数は Store の保持日数で求める。
// プロジェクトごと削除したアイテムも、そのプロジェクトの記録で材料を作る。
func TestEntriesDeleted(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	must(t, apptest.Sample(ctx, f.stores, store1))
	addItem(t, f.stores, task(item3, 3, item.Task, nil, "消したもの"))
	must(t, apptest.DeleteItems(ctx, f.stores, store1, item3))
	write(t, f.stores, func(tx port.Tx) error {
		return tx.WriteProjects().Append(project2, project.Create(project.Project{
			ID: project2, Place: project.Place{Workspace: apptest.SampleWorkspace, Number: 2}, Rank: "W", Title: "消したプロジェクト", Status: project.Todo,
		}))
	})
	under := task(item4, 4, item.Task, nil, "消したプロジェクトの下")
	under.Place.Project = project2
	addItem(t, f.stores, under)
	must(t, apptest.DeleteItems(ctx, f.stores, store1, item4))
	must(t, apptest.DeleteProject(ctx, f.stores, store1, project2))
	must(t, apptest.SampleMobile(ctx, f.stores, store1))
	must(t, apptest.DeleteItems(ctx, f.stores, store1, apptest.SampleMobileEpic))
	must(t, apptest.DeleteWorkspace(ctx, f.stores, store1, apptest.SampleMobileWorkspace))
	f.stores.SetRetentionDays(store1, 10)

	es := entriesOf(t, f, "is:deleted")
	if got, want := displays(es), []string{"S1/HH-3", "S1/HH-4"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("displays = %v, want %v", got, want)
	}
	today := planning.Today(clockStart)
	for _, e := range es {
		want := e.facts.Item.Deleted.DaysLeft(10, today)
		if e.daysLeft == nil || *e.daysLeft != want {
			t.Errorf("%s daysLeft = %v, want %d", e.facts.Display, e.daysLeft, want)
		}
	}
	if p := factsOf(t, es, store1, "HH-4"); p.ProjectTitle != "消したプロジェクト" || p.Project.Display != "HH-P2" {
		t.Errorf("HH-4 Project = %+v %q", p.Project, p.ProjectTitle)
	}

	live := entriesOf(t, f, "")
	if got, want := displays(live), []string{"S1/HH-1", "S1/HH-2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("削除済みでない displays = %v, want %v", got, want)
	}
	for _, e := range live {
		if e.daysLeft != nil {
			t.Errorf("%s に daysLeft がある", e.facts.Display)
		}
	}
}

func TestEntriesHiddenStoreLeftOut(t *testing.T) {
	f := newFixture(t)
	ctx := t.Context()
	must(t, apptest.Sample(ctx, f.stores, store1))
	must(t, apptest.SampleTeam(ctx, f.stores, store2))
	must(t, apptest.SampleTeam(ctx, f.stores, store3))

	es := entriesOf(t, f, "")
	if got, want := displays(es), []string{"S1/HH-1", "S1/HH-2", "S2/TM-1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("displays = %v, want %v", got, want)
	}
	if tm := factsOf(t, es, store2, "TM-1"); tm.StoreName != "team" || tm.StoreOrder != 1 {
		t.Errorf("TM-1 = %q %d", tm.StoreName, tm.StoreOrder)
	}
}

// 親が循環しているアイテムでも、祖先をたどるのは一度通ったところで止まる。
func TestFactsParentCycleStops(t *testing.T) {
	f := newFixture(t)
	must(t, apptest.Sample(t.Context(), f.stores, store1))
	addItem(t, f.stores, task(item5, 5, item.Task, ptr(item6), "循環 A"))
	addItem(t, f.stores, task(item6, 6, item.Task, ptr(item5), "循環 B"))

	a := factsOf(t, entriesOf(t, f, "is:invalid"), store1, "HH-5")
	if want := []query.Ref{{Display: "HH-6"}}; !reflect.DeepEqual(a.Ancestors, want) || !a.Invalid {
		t.Errorf("HH-5 Ancestors = %+v, Invalid = %v", a.Ancestors, a.Invalid)
	}
}

// ワークスペースの並びはサイドバーと同じ(rank の順、同じなら内部 ID。rank のないものは先頭。E1-10)。
func TestEntriesWorkspaceOrder(t *testing.T) {
	f := newFixture(t)
	must(t, apptest.Sample(t.Context(), f.stores, store1))
	must(t, apptest.SampleMobile(t.Context(), f.stores, store1))
	write(t, f.stores, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewSet("rank", nil, "V"))
	})

	es := entriesOf(t, f, "")
	if got, want := displays(es), []string{"S1/MB-1", "S1/HH-1", "S1/HH-2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("displays = %v, want %v", got, want)
	}
	if mb, hh := factsOf(t, es, store1, "MB-1"), factsOf(t, es, store1, "HH-1"); mb.WorkspaceOrder != 0 || hh.WorkspaceOrder != 1 {
		t.Errorf("WorkspaceOrder: MB-1 %d, HH-1 %d", mb.WorkspaceOrder, hh.WorkspaceOrder)
	}
}
