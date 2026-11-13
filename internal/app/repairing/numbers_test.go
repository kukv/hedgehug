package repairing_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/repairing"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/project"
)

const (
	dupTask    item.ID    = "01DUPLICATE00000000000001"
	dupProject project.ID = "01DUPLICATE00000000000002"
)

// numberRows は、ワークスペースのログの number の行のうち、kind の連番で番号 n を使った行の数。
func (e *env) numberRows(kind numbering.Kind, n int) int {
	e.t.Helper()
	count := 0
	for _, r := range e.rows() {
		var got int
		if r.T == numbering.Type && r.F == string(kind) && json.Unmarshal(r.To, &got) == nil && got == n {
			count++
		}
	}
	return count
}

// 番号 2 の Task をもう 1 つ作ると、自分の側で作った方が、使った最大番号の次に振り直される。
func TestRenumberItem(t *testing.T) {
	e := newEnv(t)
	parent := apptest.SampleEpic
	it := item.Item{ID: dupTask, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: &parent, Number: 2}, Kind: item.Task, Rank: "b", Title: "重なった", Status: apptest.SampleStatusTodo, Priority: apptest.SamplePriorityMedium}
	ours := e.appendItem(taro, dupTask, item.Create(it))
	wantOps(t, e.run(ours, nil), "HH-2 を HH-3 に振り直し")
	if n := e.item(dupTask).Place.Number; n != 3 {
		t.Errorf("number = %d, want 3", n)
	}
	if n := e.item(apptest.SampleTask).Place.Number; n != 2 {
		t.Errorf("sample task number = %d, want 2", n)
	}
	if n := e.numberRows(numbering.Items, 3); n != 1 {
		t.Errorf("number 3 rows = %d, want 1", n)
	}
}

// プロジェクトの番号の重なりも、同じように振り直す。
func TestRenumberProject(t *testing.T) {
	e := newEnv(t)
	p := project.Project{ID: dupProject, Place: project.Place{Workspace: apptest.SampleWorkspace, Number: 2}, Rank: "X", Title: "重なった", Owner: "taro", Status: project.Todo}
	ours := e.write(taro, func(tx port.Tx) error {
		if err := tx.WriteWorkspaces().Append(apptest.SampleWorkspace, numbering.Draft(numbering.Projects, 2)); err != nil {
			return err
		}
		return tx.WriteProjects().Append(p.ID, project.Create(p))
	})
	wantOps(t, e.run(ours, nil), "HH-P2 を HH-P3 に振り直し")
	if n := e.project(dupProject).Place.Number; n != 3 {
		t.Errorf("number = %d, want 3", n)
	}
	if n := e.numberRows(numbering.Projects, 3); n != 1 {
		t.Errorf("number 3 rows = %d, want 1", n)
	}
}

// 書いても番号が変わらず、同じ振り直しが残り続けるときは、止まらずに止めてエラーを返す。
func TestRenumberStopsWithoutProgress(t *testing.T) {
	e := newEnv(t)
	parent := apptest.SampleEpic
	it := item.Item{ID: dupTask, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: &parent, Number: 2}, Kind: item.Task, Rank: "b", Title: "重なった", Status: apptest.SampleStatusTodo, Priority: apptest.SamplePriorityMedium}
	ours := e.appendItem(taro, dupTask, item.Create(it))
	w := noopWriter{&apptest.RepairWriter{Stores: e.s, Store: store}}
	w.F.Ours = ours
	if err := repairing.Run(e.ctx, w); err == nil || !strings.Contains(err.Error(), "進みません") {
		t.Errorf("Run error = %v", err)
	}
}
