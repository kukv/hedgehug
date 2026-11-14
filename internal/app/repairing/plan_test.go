package repairing_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
)

const (
	sprint1  change.ID = "01SPRINT000000000000000001"
	release1 change.ID = "01RELEASE00000000000000001"
)

// withPlan は、サンプルのワークスペースに実行中のスプリント「Sprint 1」とリリース「v1」を書く。
func withPlan(e *env) {
	sp := planning.Sprint{ID: sprint1, Name: "Sprint 1", State: planning.Active}
	r := planning.Release{ID: release1, Name: "v1", State: planning.Unreleased}
	e.write(taro, func(tx port.Tx) error {
		ds := append(planning.SprintDrafts(planning.Sprint{}, sp), planning.ReleaseDrafts(planning.Release{}, r)...)
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, ds...)
	})
}

// ワークスペースの削除と、そのワークスペースのスプリント・リリースの変更が重なっても、スプリント・リリースは
// ワークスペースの中身なので、ワークスペースの削除は取り消さない(B5b-7 と同じ。F1)。
func TestWorkspaceDeleteKeptWhenPlanChanges(t *testing.T) {
	sp := planning.Sprint{ID: sprint1, Name: "Sprint 1", State: planning.Active}
	closed := sp
	closed.State = planning.Closed
	renamed := sp
	renamed.Name = "Sprint 9"
	r := planning.Release{ID: release1, Name: "v1", State: planning.Unreleased}
	released := r
	released.State = planning.Released
	for name, ds := range map[string][]change.Draft{
		"スプリントの名前": planning.SprintDrafts(sp, renamed),
		"スプリントの完了": append([]change.Draft{planning.CloseDraft(sprint1, planning.Confirmed{})}, planning.SprintDrafts(sp, closed)...),
		"完了の確定値の行": {planning.CloseDraft(sprint1, planning.Confirmed{})},
		"リリースの状態":  planning.ReleaseDrafts(r, released),
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			withPlan(e)
			del := deleteWorkspace(e)
			wsOp := e.workspace().Deleted.Op
			theirs := e.write(taro, func(tx port.Tx) error {
				return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, ds...)
			})
			wantOps(t, e.run(del, theirs))
			if w := e.workspace(); w.Deleted == nil {
				t.Error("workspace was restored")
			}
			wantSampleDeleted(e, wsOp)
		})
	}
}

// スプリント自身の削除と、同じスプリントの名前の変更が重なったら、スプリントの削除を取り消す(C3a-21・F1-2)。
func TestSprintDeleteUndoneByRename(t *testing.T) {
	e := newEnv(t)
	withPlan(e)
	e.appendItem(taro, apptest.SampleTask, change.NewSet(item.SprintField, nil, sprint1))
	// 削除は、入っているアイテムからスプリントを外す行を同じ保存で書く(F1-13)。
	del := e.write(taro, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Append(apptest.SampleTask, change.NewSet(item.SprintField, sprint1, nil)),
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace, planning.DeleteSprint(sprint1)),
		)
	})
	sp := planning.Sprint{ID: sprint1, Name: "Sprint 1", State: planning.Active}
	renamed := sp
	renamed.Name = "Sprint 9"
	theirs := e.write(taro, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, planning.SprintDrafts(sp, renamed)...)
	})
	wantOps(t, e.run(del, theirs), "「Sprint 9」の削除を取り消し")
	var p planning.Plan
	err := e.s.Read(e.ctx, store, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace})
		p = planning.FromLog(log)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if live := p.LiveSprints(); len(live) != 1 || live[0].Name != "Sprint 9" {
		t.Errorf("live sprints = %+v", live)
	}
	if it := e.item(apptest.SampleTask); it.Sprint != sprint1 {
		t.Errorf("HH-2 sprint = %q, want %q", it.Sprint, sprint1)
	}
}

// アイテムの削除と、そのアイテムが入っているスプリントの完了が重なっても、完了は削除済みのアイテムに
// 触れないので、削除は取り消さない(05「完了」。完了で一緒に書いたスプリント・順位の行は変更に数えない)。
func TestItemDeleteKeptWhenSprintCloses(t *testing.T) {
	e := newEnv(t)
	withPlan(e)
	e.appendItem(taro, apptest.SampleTask, change.NewSet(item.SprintField, nil, sprint1))
	del := e.appendItem(taro, apptest.SampleTask, change.NewDeleteEntity(true))
	sp := planning.Sprint{ID: sprint1, Name: "Sprint 1", State: planning.Active}
	closed := sp
	closed.State = planning.Closed
	task := e.item(apptest.SampleTask)
	theirs := e.write(taro, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteItems().Append(apptest.SampleTask, change.NewSet(item.SprintField, sprint1, nil), change.NewSet("rank", task.Rank, task.Rank+"0")),
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace, append([]change.Draft{planning.CloseDraft(sprint1, planning.Confirmed{})}, planning.SprintDrafts(sp, closed)...)...),
		)
	})
	wantOps(t, e.run(del, theirs))
	if it := e.item(apptest.SampleTask); it.Deleted == nil {
		t.Error("HH-2 was restored")
	}
}
