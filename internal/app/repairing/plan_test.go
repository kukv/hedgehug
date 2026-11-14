package repairing_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/planning"
	"github.com/kukv/hedgehug/internal/domain/workspace"
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

const (
	sprint0   change.ID = "01SPRINT000000000000000000"
	sprint2   change.ID = "01SPRINT000000000000000002"
	sprint9   change.ID = "01SPRINT000000000000000009"
	release9  change.ID = "01RELEASE00000000000000009"
	thirdTask item.ID   = "01SAMPLEIT0000000000000007"
)

// plan は、ワークスペース ws のスプリントとリリース。
func (e *env) plan(ws workspace.ID) planning.Plan {
	e.t.Helper()
	var p planning.Plan
	err := e.s.Read(e.ctx, store, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: ws})
		p = planning.FromLog(log)
		return err
	})
	if err != nil {
		e.t.Fatal(err)
	}
	return p
}

func (e *env) sprintState(id change.ID) planning.SprintState {
	e.t.Helper()
	for _, s := range e.plan(apptest.SampleWorkspace).Sprints {
		if s.ID == id {
			return s.State
		}
	}
	e.t.Fatalf("sprint %s not found", id)
	return ""
}

// addSprint は、サンプルのワークスペースに、状態 state のスプリント id を書く。
func (e *env) addSprint(id change.ID, name string, state planning.SprintState) planning.Sprint {
	s := planning.Sprint{ID: id, Name: name, State: state}
	e.write(taro, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, planning.SprintDrafts(planning.Sprint{}, s)...)
	})
	return s
}

// start は、計画中のスプリント s を、開始日 date で開始する行を書く。
func (e *env) start(s planning.Sprint, date string) map[change.ID]bool {
	s2 := s
	s2.State, s2.Start = planning.Active, date
	return e.write(taro, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, planning.SprintDrafts(s, s2)...)
	})
}

// closeSprint は、実行中のスプリント s を完了する行を書く。
func (e *env) closeSprint(s planning.Sprint) map[change.ID]bool {
	s2 := s
	s2.State = planning.Closed
	return e.write(taro, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace,
			append([]change.Draft{planning.CloseDraft(s.ID, planning.Confirmed{})}, planning.SprintDrafts(s, s2)...)...)
	})
}

// 実行中のスプリントが 2 つになったら、開始日の遅い方を計画中に戻す。
func TestStopExtraActive(t *testing.T) {
	e := newEnv(t)
	s1 := e.addSprint(sprint1, "Sprint 1", planning.Planned)
	s2 := e.addSprint(sprint2, "Sprint 2", planning.Planned)
	ours := e.start(s1, "2026-11-16")
	theirs := e.start(s2, "2026-11-17")
	wantOps(t, e.run(ours, theirs), "スプリント「Sprint 2」を計画中に戻す")
	if a, b := e.sprintState(sprint1), e.sprintState(sprint2); a != planning.Active || b != planning.Planned {
		t.Errorf("states = %s, %s", a, b)
	}
}

// 完了したスプリントに、同時に入れられた未完了のアイテムは、バックログに戻す。
func TestBacklogClosed(t *testing.T) {
	e := newEnv(t)
	s1 := e.addSprint(sprint1, "Sprint 1", planning.Active)
	ours := e.closeSprint(s1)
	theirs := e.appendItem(taro, apptest.SampleTask, change.NewSet(item.SprintField, nil, sprint1))
	wantOps(t, e.run(ours, theirs), "HH-2 をバックログに戻す")
	if it := e.item(apptest.SampleTask); it.Sprint != "" {
		t.Errorf("HH-2 sprint = %q", it.Sprint)
	}
	if s := e.plan(apptest.SampleWorkspace).Sprints[0]; s.State != planning.Closed || s.Confirmed == nil {
		t.Errorf("sprint = %+v", s)
	}
}

// 完了を受け取ったあとに再オープンしたアイテムは、完了したスプリントに残す。
func TestBacklogClosedKeepsReopened(t *testing.T) {
	e := newEnv(t)
	s1 := e.addSprint(sprint1, "Sprint 1", planning.Active)
	e.appendItem(taro, apptest.SampleTask, change.NewSet(item.SprintField, nil, sprint1))
	e.closeSprint(s1)
	ours := e.appendItem(taro, apptest.SampleTask, change.NewSet("title", "ログイン画面を作る", "ログイン画面"))
	theirs := e.appendItem(taro, apptest.SampleTask, change.NewSet("status", review, apptest.SampleStatusInProgress))
	wantOps(t, e.run(ours, theirs))
	if it := e.item(apptest.SampleTask); it.Sprint != sprint1 {
		t.Errorf("HH-2 sprint = %q", it.Sprint)
	}
}

// 存在しないスプリントとリリースを指す欄は、完全に削除したときとワークスペースにないときで件名を分ける。
func TestClearDanglingSprint(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(e *env)
		field   string
		target  change.ID
		want    string
	}{
		{"deleted sprint", func(*env) {}, item.SprintField, sprint9, "HH-2 をバックログに戻す(スプリントが完全に削除)"},
		{"sprint of another workspace", func(e *env) {
			addMobile(e)
			e.write(taro, func(tx port.Tx) error {
				return tx.WriteWorkspaces().Append(mobileWS, planning.SprintDrafts(planning.Sprint{}, planning.Sprint{ID: sprint9, Name: "Other", State: planning.Planned})...)
			})
		}, item.SprintField, sprint9, "HH-2 をバックログに戻す(スプリントがワークスペースにない)"},
		{"deleted release", func(*env) {}, item.ReleaseField, release9, "HH-2 のリリースを外す(リリースが完全に削除)"},
		{"release of another workspace", func(e *env) {
			addMobile(e)
			e.write(taro, func(tx port.Tx) error {
				return tx.WriteWorkspaces().Append(mobileWS, planning.ReleaseDrafts(planning.Release{}, planning.Release{ID: release9, Name: "Other", State: planning.Unreleased})...)
			})
		}, item.ReleaseField, release9, "HH-2 のリリースを外す(リリースがワークスペースにない)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			tt.prepare(e)
			e.appendItem(taro, apptest.SampleTask, change.NewSet(tt.field, nil, tt.target))
			wantOps(t, e.run(map[change.ID]bool{}, map[change.ID]bool{}), tt.want)
			if it := e.item(apptest.SampleTask); it.Sprint != "" || it.Release != "" {
				t.Errorf("HH-2 = sprint %q, release %q", it.Sprint, it.Release)
			}
		})
	}
}

// スプリントもリリースもないアイテムは、欄ごとに 1 回ずつ保存する。
func TestDanglingBothFields(t *testing.T) {
	e := newEnv(t)
	e.appendItem(taro, apptest.SampleTask, change.NewSet(item.SprintField, nil, sprint9), change.NewSet(item.ReleaseField, nil, release9))
	wantOps(t, e.run(map[change.ID]bool{}, map[change.ID]bool{}),
		"HH-2 をバックログに戻す(スプリントが完全に削除)", "HH-2 のリリースを外す(リリースが完全に削除)")
}

// (6)・(7)・(8) がすべて要るときは、その順に保存する。
func TestPlanRepairOrder(t *testing.T) {
	e := newEnv(t)
	s0 := e.addSprint(sprint0, "Sprint 0", planning.Active)
	s1 := e.addSprint(sprint1, "Sprint 1", planning.Planned)
	s2 := e.addSprint(sprint2, "Sprint 2", planning.Planned)
	third := item.Item{ID: thirdTask, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Number: 3}, Kind: item.Task, Rank: "V", Title: "別のタスク", Status: apptest.SampleStatusTodo, Priority: apptest.SamplePriorityMedium}
	e.write(taro, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace, numbering.Draft(numbering.Items, 3)),
			tx.WriteItems().Append(third.ID, item.Create(third), change.NewSet(item.SprintField, nil, sprint9)),
		)
	})
	ours := union(e.closeSprint(s0), e.start(s1, "2026-11-16"))
	theirs := union(
		e.appendItem(taro, apptest.SampleTask, change.NewSet(item.SprintField, nil, sprint0)),
		e.start(s2, "2026-11-17"),
	)
	wantOps(t, e.run(ours, theirs),
		"スプリント「Sprint 2」を計画中に戻す", "HH-2 をバックログに戻す", "HH-3 をバックログに戻す(スプリントが完全に削除)")
}
