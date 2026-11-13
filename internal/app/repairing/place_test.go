package repairing_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/projectrule"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

// movedEnv は、Epic HH-1 を HH-P2 へ移し、HH-P2 が hidden のステータスを使わないようにした状態にする。HH-2 は Review のまま HH-P1 に残る。
func movedEnv(t *testing.T, hidden change.ID) *env {
	t.Helper()
	e := newEnv(t)
	old := e.item(apptest.SampleEpic).Place
	to := old
	to.Project = project2
	e.appendItem(taro, apptest.SampleEpic, change.NewSet("place", old, to))
	e.write(taro, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewAdd(projectrule.HiddenField(project2), hidden))
	})
	return e
}

// inProgress は、「分類ごとの既定」になっている、進行中の分類のステータス。
func (e *env) inProgress() change.ID {
	e.t.Helper()
	var id change.ID
	err := e.s.Read(e.ctx, store, func(r port.Reader) error {
		st, err := r.Workspaces().Settings(apptest.SampleWorkspace)
		id = st.CategoryDefaults[workspace.CategoryDoing]
		return err
	})
	if err != nil || id == "" {
		e.t.Fatalf("category default = %q, %v", id, err)
	}
	return id
}

// 親の移動先 HH-P2 が使わないステータス(Review)の子は、移すときに同じ分類の既定へ置き換わる。移動と同じ保存で、実行者は同期。
func TestMatchProjectsReplacesUnusedStatus(t *testing.T) {
	e := movedEnv(t, review)
	before := len(e.rows())
	if ops := e.run(nil, nil); len(ops) != 1 {
		t.Fatalf("ops = %v, want one", ops)
	}
	got := e.item(apptest.SampleTask)
	if got.Place.Project != project2 || got.Status != e.inProgress() {
		t.Errorf("project = %s status = %q, want %s %q", got.Place.Project, got.Status, project2, e.inProgress())
	}
	var added []change.Record
	for _, r := range e.rows()[before:] {
		if r.Actor.Kind == change.ActorSync {
			added = append(added, r)
		}
	}
	if len(added) != 2 || added[0].Op != added[1].Op {
		t.Errorf("sync rows = %+v, want place and status in one op", added)
	}
}

// 移動先が使うステータスなら変えない。
func TestMatchProjectsKeepsUsedStatus(t *testing.T) {
	e := movedEnv(t, apptest.SampleStatusTodo)
	e.run(nil, nil)
	got := e.item(apptest.SampleTask)
	if got.Place.Project != project2 || got.Status != review {
		t.Errorf("project = %s status = %q, want %s %q", got.Place.Project, got.Status, project2, review)
	}
}

// Epic HH-1 を HH-P2 へ移すと、その下の HH-2 と Step HH-3 が HH-P1 に残る。親の浅い順に HH-P2 へ移す。番号は変えない。
func TestProjectMismatchMovesChildren(t *testing.T) {
	e := newEnv(t)
	addStep(e)
	old := e.item(apptest.SampleEpic).Place
	to := old
	to.Project = project2
	e.appendItem(taro, apptest.SampleEpic, change.NewSet("place", old, to))
	title := e.project(project2).Title
	wantOps(t, e.run(nil, nil),
		"HH-2 をプロジェクト「"+title+"」に移動(親に合わせる)",
		"HH-3 をプロジェクト「"+title+"」に移動(親に合わせる)")
	for id, n := range map[item.ID]int{apptest.SampleTask: 2, step: 3} {
		p := e.item(id).Place
		if p.Project != project2 || p.Number != n || p.Parent == nil {
			t.Errorf("%s place = %+v, want project %s number %d with parent", id, p, project2, n)
		}
	}
}
