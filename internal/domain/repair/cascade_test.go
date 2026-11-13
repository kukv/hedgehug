package repair_test

import (
	"reflect"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/repair"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func deletion(op string) *change.Deletion { return &change.Deletion{Op: change.ID(op)} }

func under(id string, p project.ID, parent *item.ID, d *change.Deletion) item.Item {
	return item.Item{ID: item.ID(id), Place: item.Place{Project: p, Parent: parent}, Deleted: d}
}

func TestCascades(t *testing.T) {
	epic, e3 := item.ID("e1"), item.ID("e3")
	deadEpic := under("e1", "p1", nil, deletion("opE"))
	task := under("t1", "p1", &epic, nil)
	liveEpicInDeadProject := under("e2", "p2", nil, nil)
	both := under("t2", "p2", &e3, nil)
	deadParentInDeadProject := under("e3", "p2", nil, deletion("opE3"))
	gone := under("t3", "p1", &epic, deletion("opT"))
	deadProject := project.Project{ID: "p2", Deleted: deletion("opP")}
	liveProject := project.Project{ID: "p1"}

	got := repair.Cascades(
		[]item.Item{both, gone, task, liveEpicInDeadProject, deadEpic, deadParentInDeadProject},
		[]project.Project{liveProject, deadProject})
	want := []repair.Cascade{
		{Child: "e2", Op: "opP"},
		{Child: "t1", Op: "opE"},
		{Child: "t2", Op: "opE3"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Cascades = %v, want %v", got, want)
	}
}

// 親が復元済み(削除されていない)なら子は入らない。親がリストになく、プロジェクトも生きているなら入らない。
func TestCascadesNotNeeded(t *testing.T) {
	epic, missing := item.ID("e1"), item.ID("zz")
	restored := under("e1", "p1", nil, nil)
	child := under("t1", "p1", &epic, nil)
	orphan := under("t2", "p1", &missing, nil)
	got := repair.Cascades([]item.Item{restored, child, orphan}, []project.Project{{ID: "p1"}})
	if len(got) != 0 {
		t.Errorf("Cascades = %v, want none", got)
	}
}

// 削除済みのワークスペースの下の、削除されていないプロジェクトだけを、ワークスペースの削除の op で返す(B5b-6)。
func TestProjectCascadesUnderDeletedWorkspace(t *testing.T) {
	wss := []workspace.Workspace{{ID: "w1", Deleted: deletion("o9")}, {ID: "w2"}}
	projects := []project.Project{
		{ID: "p3", Place: project.Place{Workspace: "w2"}},
		{ID: "p2", Place: project.Place{Workspace: "w1"}, Deleted: deletion("o5")},
		{ID: "p1", Place: project.Place{Workspace: "w1"}},
	}
	got := repair.ProjectCascades(wss, projects)
	want := []repair.ProjectCascade{{Project: "p1", Op: "o9"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ProjectCascades = %v, want %v", got, want)
	}
}
