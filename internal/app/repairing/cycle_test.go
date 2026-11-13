package repairing_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/repairing"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/rank"
	"github.com/kukv/hedgehug/internal/domain/repair"
)

// reparent は、生の行で id の親を parent にする。
func (e *env) reparent(id, parent item.ID, ds ...change.Draft) {
	e.t.Helper()
	old := e.item(id).Place
	to := old
	to.Parent = &parent
	e.appendItem(taro, id, append(ds, change.NewSet("place", old, to))...)
}

// repairEpicID は、サンプルのプロジェクトに作る「修復で作った Epic」の内部 ID。
func repairEpicID() change.ID {
	return repair.EpicID(apptest.SampleProject, func(change.ID) bool { return false })
}

func wantPlaced(t *testing.T, it item.Item, kind item.Kind, parent *item.ID, rk string) {
	t.Helper()
	if it.Kind != kind || it.Rank != rk || (parent == nil) != (it.Place.Parent == nil) || (parent != nil && *it.Place.Parent != *parent) {
		t.Errorf("%s = kind %s parent %v rank %q, want %s %v %q", it.ID, it.Kind, it.Place.Parent, it.Rank, kind, parent, rk)
	}
}

// HH-1 を Story にして親を HH-2 にすると輪 {HH-1, HH-2} ができ、親のない Epic がなくなる。番号の大きい HH-2 を
// 「修復で作った Epic」HH-3 の下へ動かし、続けて、親が Task になった HH-1(Story)も HH-3 の下へ動かす。
func TestCycleCreatesRepairEpic(t *testing.T) {
	e := newEnv(t)
	e.reparent(apptest.SampleEpic, apptest.SampleTask, change.NewSet("kind", item.Epic, item.Story))
	wantOps(t, e.run(nil, nil), "HH-2 を HH-3 の下へ移動(親の循環)", "HH-1 を HH-3 の下へ移動(親の循環)")
	id := repairEpicID()
	epic := e.item(id)
	if epic.Title != repair.RepairEpicTitle || epic.Place.Number != 3 || epic.Place.Project != apptest.SampleProject ||
		epic.Status != apptest.SampleStatusTodo || epic.Priority != apptest.SamplePriorityMedium {
		t.Errorf("repair epic = %+v", epic)
	}
	wantPlaced(t, epic, item.Epic, nil, rank.First)
	wantPlaced(t, e.item(apptest.SampleTask), item.Task, &id, rank.First)
	wantPlaced(t, e.item(apptest.SampleEpic), item.Story, &id, rank.After(rank.First))
	if n := e.numberRows(numbering.Items, 3); n != 1 {
		t.Errorf("number 3 rows = %d, want 1", n)
	}
}

// 動かすものが Step なら Task にして、プロジェクトの先頭の Epic の下の末尾へ動かす。
func TestCycleMovesStepAsTask(t *testing.T) {
	e := newEnv(t)
	addStep(e)
	e.reparent(apptest.SampleTask, step)
	epic := apptest.SampleEpic
	wantOps(t, e.run(nil, nil), "HH-3 を HH-1 の下へ移動(親の循環)", "HH-2 を HH-1 の下へ移動(親の循環)")
	wantPlaced(t, e.item(step), item.Task, &epic, rank.First)
	wantPlaced(t, e.item(apptest.SampleTask), item.Task, &epic, rank.After(rank.First))
}

// 輪に入った Epic は、プロジェクトの直下へ戻す。行き先の ID はプロジェクトの ID。
func TestCycleMovesEpicToProject(t *testing.T) {
	e := newEnv(t)
	e.reparent(apptest.SampleEpic, apptest.SampleTask)
	wantOps(t, e.run(nil, nil), "HH-2 を HH-3 の下へ移動(親の循環)", "HH-1 を HH-P1 の下へ移動(親の循環)")
	id := repairEpicID()
	wantPlaced(t, e.item(apptest.SampleTask), item.Task, &id, rank.First)
	wantPlaced(t, e.item(apptest.SampleEpic), item.Epic, nil, rank.After(rank.First))
}

// 「修復で作った Epic」の内部 ID がすでに使われていれば(削除済みを含む)、次の文字に替えたものを使う。
func TestCycleRepairEpicIDInUse(t *testing.T) {
	e := newEnv(t)
	used := repairEpicID()
	old := item.Item{ID: used, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Number: 3}, Kind: item.Epic, Rank: "W", Title: "古い", Status: apptest.SampleStatusTodo, Priority: apptest.SamplePriorityMedium}
	e.write(taro, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace, numbering.Draft(numbering.Items, 3)),
			tx.WriteItems().Append(used, item.Create(old), change.NewDeleteEntity(true)),
		)
	})
	e.reparent(apptest.SampleEpic, apptest.SampleTask, change.NewSet("kind", item.Epic, item.Story))
	wantOps(t, e.run(nil, nil), "HH-2 を HH-4 の下へ移動(親の循環)", "HH-1 を HH-4 の下へ移動(親の循環)")
	next := change.ID("S" + string(used[1:]))
	if it := e.item(next); it.Title != repair.RepairEpicTitle || it.Place.Number != 4 {
		t.Errorf("repair epic = %+v", it)
	}
}

// hiddenProject は、Projects().Get で hidden を読めない(完全に削除された)ように見せる。
type hiddenProject struct {
	*apptest.RepairWriter
	hidden project.ID
}

func (w hiddenProject) Read(fn func(r port.Reader) error) error {
	return w.RepairWriter.Read(func(r port.Reader) error { return fn(hidingReader{Reader: r, hidden: w.hidden}) })
}

type hidingReader struct {
	port.Reader
	hidden project.ID
}

func (r hidingReader) Projects() port.ProjectReader {
	return hidingProjects{ProjectReader: r.Reader.Projects(), hidden: r.hidden}
}

type hidingProjects struct {
	port.ProjectReader
	hidden project.ID
}

func (p hidingProjects) Get(id project.ID) (project.Project, error) {
	if id == p.hidden {
		return project.Project{}, port.ErrNotFound
	}
	return p.ProjectReader.Get(id)
}

// 動かすもののプロジェクトが読めない輪は直さず、エラーにもせず、ほかの輪を直す。
func TestCycleSkipsUnreadableProject(t *testing.T) {
	e := newEnv(t)
	a, b := item.ID("01AAAACYCLE000000000000001"), item.ID("01AAAACYCLE000000000000002")
	place := func(n int, parent item.ID) item.Place {
		return item.Place{Workspace: apptest.SampleWorkspace, Project: project2, Parent: &parent, Number: n}
	}
	e.write(taro, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace, numbering.Draft(numbering.Items, 3), numbering.Draft(numbering.Items, 4)),
			tx.WriteItems().Append(a, item.Create(item.Item{ID: a, Place: place(3, b), Kind: item.Story, Rank: "V", Title: "輪", Status: apptest.SampleStatusTodo, Priority: apptest.SamplePriorityMedium})),
			tx.WriteItems().Append(b, item.Create(item.Item{ID: b, Place: place(4, a), Kind: item.Task, Rank: "V", Title: "輪", Status: apptest.SampleStatusTodo, Priority: apptest.SamplePriorityMedium})),
		)
	})
	e.reparent(apptest.SampleEpic, apptest.SampleTask, change.NewSet("kind", item.Epic, item.Story))
	for i, want := range [][]string{{"HH-2 を HH-5 の下へ移動(親の循環)", "HH-1 を HH-5 の下へ移動(親の循環)"}, nil} {
		w := hiddenProject{RepairWriter: &apptest.RepairWriter{Stores: e.s, Store: store}, hidden: project2}
		if err := repairing.Run(e.ctx, w); err != nil {
			t.Fatalf("Run %d: %v", i, err)
		}
		wantOps(t, w.Ops, want...)
	}
	if p := e.item(b).Place.Parent; p == nil || *p != a {
		t.Errorf("hidden ring changed: parent %v", p)
	}
}

// 書いても輪が残り続けるときは、止まらずに止めてエラーを返す。
func TestCycleStopsWithoutProgress(t *testing.T) {
	e := newEnv(t)
	e.reparent(apptest.SampleEpic, apptest.SampleTask, change.NewSet("kind", item.Epic, item.Story))
	w := noopWriter{&apptest.RepairWriter{Stores: e.s, Store: store}}
	if err := repairing.Run(e.ctx, w); err == nil || !strings.Contains(err.Error(), "進みません") {
		t.Errorf("Run error = %v", err)
	}
}
