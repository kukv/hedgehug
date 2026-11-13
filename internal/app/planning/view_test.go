package planning_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/planning"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	dp "github.com/kukv/hedgehug/internal/domain/planning"
)

// viewItem は、テストで足すアイテム。points が負なら未設定。
type viewItem struct {
	id      item.ID
	number  int
	kind    item.Kind
	rank    string
	status  change.ID
	sprint  change.ID
	release change.ID
	points  float64
}

func addViewItems(t *testing.T, svc planning.Service, vs ...viewItem) {
	t.Helper()
	op := port.Operation{Actor: change.User("taro"), Summary: "アイテムを作成"}
	err := svc.Stores.Write(t.Context(), store, op, func(tx port.Tx) error {
		for _, v := range vs {
			it := item.Item{
				ID:    v.id,
				Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Number: v.number},
				Kind:  v.kind, Rank: v.rank, Title: string(v.id), Status: v.status, Sprint: v.sprint, Release: v.release,
				Assignee: "hanako",
			}
			if v.points >= 0 {
				p := v.points
				it.Points = &p
			}
			if err := tx.WriteWorkspaces().Append(apptest.SampleWorkspace, numbering.Draft(numbering.Items, v.number)); err != nil {
				return err
			}
			if err := tx.WriteItems().Append(v.id, item.Create(it)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func rowIDs(rows []planning.Row) []item.ID {
	var out []item.ID
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out
}

func sameIDs(got []item.ID, want ...item.ID) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestSprintCards(t *testing.T) {
	svc, _ := setup(t)
	ctx := t.Context()
	s1, s2 := activeSprint(t, svc)
	const (
		todo, doing, done, cancel = apptest.SampleStatusTodo, apptest.SampleStatusInProgress, apptest.SampleStatusDone, apptest.SampleStatusCancel
	)
	addViewItems(t, svc,
		viewItem{itemA, 3, item.Task, "C", done, s1.ID, "", 3},
		viewItem{itemB, 4, item.Story, "B", doing, s1.ID, "", 2},
		viewItem{itemC, 5, item.Bug, "D", todo, s1.ID, "", -1},
		viewItem{itemD, 6, item.Task, "E", cancel, s1.ID, "", 1},
		viewItem{itemE, 7, item.Task, "F", todo, s1.ID, "", 9}, // 削除済み
		viewItem{itemX, 8, item.Epic, "A", todo, s1.ID, "", 7}, // あり得ない値
	)
	if err := apptest.DeleteItems(ctx, svc.Stores, store, itemE); err != nil {
		t.Fatal(err)
	}
	cards, err := svc.SprintCards(ctx, store, apptest.SampleWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 || cards[0].Sprint.ID != s1.ID || cards[1].Sprint.ID != s2.ID {
		t.Fatalf("cards = %+v", cards)
	}
	c := cards[0]
	if c.Total != (dp.Totals{Points: 5, Count: 3}) || c.Done != (dp.Totals{Points: 3, Count: 1}) || c.Undone != 2 || c.Canceled != 1 {
		t.Errorf("counts = %+v %+v %d %d", c.Total, c.Done, c.Undone, c.Canceled)
	}
	if got := rowIDs(c.Items); !sameIDs(got, itemB, itemA, itemC, itemD) {
		t.Fatalf("Items = %v", got)
	}
	r := c.Items[0]
	if r.Display != "HH-4" || r.Kind != item.Story || r.Title != string(itemB) || r.Points == nil || *r.Points != 2 || r.Status != doing || r.Assignee != "hanako" {
		t.Errorf("Row = %+v", r)
	}
	if c.Items[2].Points != nil {
		t.Errorf("ポイントなしは nil: %+v", c.Items[2])
	}
	if len(cards[1].Items) != 0 || cards[1].Total != (dp.Totals{}) {
		t.Errorf("空のカード = %+v", cards[1])
	}
}

func TestSprintCardsClosedAndUnusable(t *testing.T) {
	svc, _ := setup(t)
	ctx := t.Context()
	s1, s2 := activeSprint(t, svc)
	addViewItems(t, svc,
		viewItem{itemA, 3, item.Task, "A", apptest.SampleStatusDone, s1.ID, "", 3},
		viewItem{itemB, 4, item.Task, "B", apptest.SampleStatusTodo, s2.ID, "", 2},
		viewItem{itemC, 5, item.Task, "C", apptest.SampleStatusTodo, "01NOSUCHSPRINT0000000000", "", 2},
	)
	if err := svc.CloseSprint(ctx, store, apptest.SampleWorkspace, s1.ID, nil); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteSprint(ctx, store, apptest.SampleWorkspace, s2.ID); err != nil {
		t.Fatal(err)
	}
	cards, err := svc.SprintCards(ctx, store, apptest.SampleWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 || cards[0].Sprint.ID != s1.ID || cards[0].Sprint.Confirmed == nil {
		t.Fatalf("cards = %+v", cards)
	}
	if got := rowIDs(cards[0].Items); !sameIDs(got, itemA) {
		t.Errorf("削除済み・ないスプリントのアイテムは入らない: %v", got)
	}
}

func TestReleaseCards(t *testing.T) {
	svc, _ := setup(t)
	ctx := t.Context()
	r1, err := svc.CreateRelease(ctx, store, apptest.SampleWorkspace, planning.ReleaseInput{Name: "v1", Date: "2026-12-01"})
	if err != nil {
		t.Fatal(err)
	}
	r2, err := svc.CreateRelease(ctx, store, apptest.SampleWorkspace, planning.ReleaseInput{Name: "v2"})
	if err != nil {
		t.Fatal(err)
	}
	r3, err := svc.CreateRelease(ctx, store, apptest.SampleWorkspace, planning.ReleaseInput{Name: "v3"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteRelease(ctx, store, apptest.SampleWorkspace, r3.ID); err != nil {
		t.Fatal(err)
	}
	addViewItems(t, svc,
		viewItem{itemA, 3, item.Epic, "A", apptest.SampleStatusTodo, "", r1.ID, -1},
		viewItem{itemB, 4, item.Story, "B", apptest.SampleStatusDone, "", r1.ID, 3},
		viewItem{itemC, 5, item.Task, "C", apptest.SampleStatusCancel, "", r1.ID, 1},
		viewItem{itemD, 6, item.Step, "D", apptest.SampleStatusTodo, "", r1.ID, -1}, // あり得ない値
		viewItem{itemE, 7, item.Bug, "E", apptest.SampleStatusInProgress, "", r3.ID, -1},
		viewItem{itemX, 8, item.Task, "F", apptest.SampleStatusInProgress, "", r1.ID, -1},
	)
	cards, err := svc.ReleaseCards(ctx, store, apptest.SampleWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 || cards[0].Release.ID != r1.ID || cards[1].Release.ID != r2.ID {
		t.Fatalf("cards = %+v", cards)
	}
	c := cards[0]
	if c.Target != 2 || c.Done != 1 || c.Undone != 1 {
		t.Errorf("counts = %d %d %d", c.Target, c.Done, c.Undone)
	}
	if got := rowIDs(c.Items); !sameIDs(got, itemA, itemB, itemC, itemX) {
		t.Errorf("Items = %v", got)
	}
	if cards[1].Target != 0 || len(cards[1].Items) != 0 {
		t.Errorf("空のカード = %+v", cards[1])
	}
}

func TestOverview(t *testing.T) {
	svc, _ := setup(t)
	ctx := t.Context()
	s1, s2 := activeSprint(t, svc)
	s3, err := svc.CreateSprint(ctx, store, apptest.SampleWorkspace, planning.SprintInput{Name: "Sprint 3"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteSprint(ctx, store, apptest.SampleWorkspace, s3.ID); err != nil {
		t.Fatal(err)
	}
	r1, _ := svc.CreateRelease(ctx, store, apptest.SampleWorkspace, planning.ReleaseInput{Name: "v1"})
	r2, _ := svc.CreateRelease(ctx, store, apptest.SampleWorkspace, planning.ReleaseInput{Name: "v2"})
	if err := svc.DeleteRelease(ctx, store, apptest.SampleWorkspace, r2.ID); err != nil {
		t.Fatal(err)
	}
	ov, err := svc.Overview(ctx, store, apptest.SampleWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	if len(ov.Sprints) != 2 || ov.Sprints[0].ID != s1.ID || ov.Sprints[1].ID != s2.ID {
		t.Errorf("Sprints = %+v", ov.Sprints)
	}
	if len(ov.Releases) != 1 || ov.Releases[0].ID != r1.ID {
		t.Errorf("Releases = %+v", ov.Releases)
	}
	if ov.Active == nil || ov.Active.ID != s1.ID {
		t.Errorf("Active = %+v", ov.Active)
	}
	today := dp.Today(svc.Clock.Now())
	name, start, end := dp.SprintDefaults(ov.Sprints, today)
	if ov.Defaults != (planning.SprintInput{Name: name, Start: start, End: end}) || ov.Today != today {
		t.Errorf("Defaults = %+v, Today = %s", ov.Defaults, ov.Today)
	}
	if name != "Sprint 3" {
		t.Errorf("既定の名前 = %q(削除済みは数えない)", name)
	}
}

func TestOverviewNoActive(t *testing.T) {
	svc, _ := setup(t)
	ov, err := svc.Overview(t.Context(), store, apptest.SampleWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	if ov.Active != nil || len(ov.Sprints) != 0 || ov.Defaults.Name != "Sprint 1" || ov.Defaults.Start != ov.Today {
		t.Errorf("Overview = %+v", ov)
	}
}

// 別のワークスペースへ移ったアイテムは、参照がこのワークスペースのスプリント・リリースを指したままでも、カードに入らない。
func TestCardsExcludeMovedItems(t *testing.T) {
	svc, _ := setup(t)
	ctx := t.Context()
	s1, _ := activeSprint(t, svc)
	r1, err := svc.CreateRelease(ctx, store, apptest.SampleWorkspace, planning.ReleaseInput{Name: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	otherWorkspace(t, svc)
	addViewItems(t, svc, viewItem{itemA, 3, item.Task, "A", apptest.SampleStatusTodo, s1.ID, r1.ID, 1})
	moved := item.Item{
		ID:    itemB,
		Place: item.Place{Workspace: other, Project: apptest.SampleProject, Number: 1},
		Kind:  item.Task, Rank: "B", Title: "moved", Status: apptest.SampleStatusTodo, Sprint: s1.ID, Release: r1.ID,
	}
	op := port.Operation{Actor: change.User("taro"), Summary: "アイテムを作成"}
	if err := svc.Stores.Write(ctx, store, op, func(tx port.Tx) error {
		return tx.WriteItems().Append(itemB, item.Create(moved))
	}); err != nil {
		t.Fatal(err)
	}
	sc, err := svc.SprintCards(ctx, store, apptest.SampleWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	if got := rowIDs(sc[0].Items); !sameIDs(got, itemA) || sc[0].Total.Count != 1 {
		t.Errorf("スプリントのカード = %v, %+v", got, sc[0].Total)
	}
	rc, err := svc.ReleaseCards(ctx, store, apptest.SampleWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	if got := rowIDs(rc[0].Items); !sameIDs(got, itemA) || rc[0].Target != 1 {
		t.Errorf("リリースのカード = %v, %d", got, rc[0].Target)
	}
}
