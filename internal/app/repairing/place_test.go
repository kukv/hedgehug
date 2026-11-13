package repairing_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

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
