//go:build !e2e

package main

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/planning"
)

// 計画の同期の修復((6) 実行中の重なり・(7) 完了したスプリントのバックログ戻し)と、その通知を、本物の gitstore と
// repairing.Run と state.db の受信箱で、2 台のマシンの場面として確かめる(決定 C5a-12)。A のログイン名は taro、B は hanako。

const (
	sprintOne change.ID = "01SPRINT00000000000000001"
	sprintTwo change.ID = "01SPRINT00000000000000002"
)

// sprintAs は、login の操作として、ワークスペースのスプリントを old から s へ書く(old.ID が空なら作成)。
func sprintAs(t *testing.T, m machine, login string, old, s planning.Sprint) {
	t.Helper()
	m.writeAs(t, login, "スプリントを保存", func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, planning.SprintDrafts(old, s)...)
	})
}

// closeSprintAs は、login の操作として、実行中のスプリント s を完了する。
func closeSprintAs(t *testing.T, m machine, login string, s planning.Sprint) {
	t.Helper()
	closed := s
	closed.State = planning.Closed
	m.writeAs(t, login, "スプリントを完了", func(tx port.Tx) error {
		ds := append([]change.Draft{planning.CloseDraft(s.ID, planning.Confirmed{})}, planning.SprintDrafts(s, closed)...)
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, ds...)
	})
}

// taskAs は、login の操作として、サンプルのタスク HH-2 に行 ds を足す。
func taskAs(t *testing.T, m machine, login string, ds ...change.Draft) {
	t.Helper()
	m.writeAs(t, login, "アイテムを変更", setTask(ds...))
}

func planned(id change.ID, name, start, end string) planning.Sprint {
	return planning.Sprint{ID: id, Name: name, State: planning.Planned, Start: start, End: end}
}

func started(s planning.Sprint) planning.Sprint {
	s.State = planning.Active
	return s
}

func toSprint(id change.ID) change.Draft {
	return change.NewSet(item.SprintField, nil, string(id))
}

func statusChange(from, to change.ID) change.Draft {
	return change.NewSet("status", string(from), string(to))
}

// wantSprintState は、m のワークスペースにスプリント id があり、その状態が want なことを確かめる。
func wantSprintState(t *testing.T, m machine, id change.ID, want planning.SprintState) {
	t.Helper()
	err := m.stores.Read(t.Context(), m.id, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: apptest.SampleWorkspace})
		if err != nil {
			return err
		}
		for _, s := range planning.FromLog(log).Sprints {
			if s.ID == id {
				if s.State != want {
					t.Errorf("sprint %s on %s: state = %s, want %s", id, m.dir, s.State, want)
				}
				return nil
			}
		}
		t.Errorf("sprint %s on %s: not found", id, m.dir)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// A が Sprint 1・Sprint 2 を作って同期し、B が受け取る。A が Sprint 1(開始日 2026-11-16)、B が Sprint 2
// (2026-11-17)を開始する。A が同期し、B が同期すると、B のマシンが開始日の遅い Sprint 2 を計画中に戻し、B(開始した人)の
// 受信箱に 1 行。B がもう一度同期しても 1 行。A が同期して受け取っても、A には作らない。
func TestRepairNoticeTwoActive(t *testing.T) {
	remote, a, b := noticePair(t)
	one := planned(sprintOne, "Sprint 1", "2026-11-16", "2026-11-29")
	two := planned(sprintTwo, "Sprint 2", "2026-11-17", "2026-11-30")
	sprintAs(t, a, "taro", planning.Sprint{}, one)
	sprintAs(t, a, "taro", planning.Sprint{}, two)
	a.sync(t, remote)
	b.sync(t, remote)
	sprintAs(t, a, "taro", one, started(one))
	sprintAs(t, b, "hanako", two, started(two))
	a.sync(t, remote)
	b.sync(t, remote)
	const want = "実行中のスプリントが 2 つになったため、「Sprint 2」を計画中に戻しました"
	wantRepair(t, b, want)
	wantSprintState(t, b, sprintOne, planning.Active)
	wantSprintState(t, b, sprintTwo, planning.Planned)
	b.sync(t, remote)
	wantRepair(t, b, want)
	a.sync(t, remote)
	wantSprintState(t, a, sprintOne, planning.Active)
	wantSprintState(t, a, sprintTwo, planning.Planned)
	wantNoRepair(t, a)
}

// A が Sprint 1 を完了し、B が同時に HH-2 を Sprint 1 に入れる。A が同期し、B が同期すると、B のマシンが HH-2 を
// バックログに戻し(スプリントが空)、HH-2 を入れた B の受信箱に 1 行。A が同期して受け取っても、A には作らない。
func TestRepairNoticeClosedSprint(t *testing.T) {
	remote, a, b := noticePair(t)
	one := started(planned(sprintOne, "Sprint 1", "2026-11-16", "2026-11-29"))
	sprintAs(t, a, "taro", planning.Sprint{}, one)
	a.sync(t, remote)
	b.sync(t, remote)
	closeSprintAs(t, a, "taro", one)
	taskAs(t, b, "hanako", toSprint(sprintOne))
	a.sync(t, remote)
	b.sync(t, remote)
	const want = "スプリント「Sprint 1」は完了しているため、HH-2 をバックログに戻しました"
	wantRepair(t, b, want)
	wantSprintState(t, b, sprintOne, planning.Closed)
	if s := b.item(t, apptest.SampleTask).Sprint; s != "" {
		t.Errorf("HH-2 sprint on B = %q, want empty", s)
	}
	b.sync(t, remote)
	wantRepair(t, b, want)
	a.sync(t, remote)
	if s := a.item(t, apptest.SampleTask).Sprint; s != "" {
		t.Errorf("HH-2 sprint on A = %q, want empty", s)
	}
	wantNoRepair(t, a)
}

// HH-2 が Done で入っている Sprint 1 を A が完了して同期し、B が同期して完了を受け取ってから、B が HH-2 を
// 未着手に戻して保存し、A が同時に HH-1 のタイトルを変える。A が同期し、B が同期(合わせ)しても、HH-2 は Sprint 1
// のままで、どちらにも同期の修復の通知はない。
func TestRepairNoticeReopenStays(t *testing.T) {
	remote, a, b := noticePair(t)
	one := started(planned(sprintOne, "Sprint 1", "2026-11-16", "2026-11-29"))
	sprintAs(t, a, "taro", planning.Sprint{}, one)
	taskAs(t, a, "taro", toSprint(sprintOne), statusChange(apptest.SampleStatusTodo, apptest.SampleStatusDone))
	a.sync(t, remote)
	b.sync(t, remote)
	closeSprintAs(t, a, "taro", one)
	a.sync(t, remote)
	b.sync(t, remote)
	taskAs(t, b, "hanako", statusChange(apptest.SampleStatusDone, apptest.SampleStatusTodo))
	titleAs(t, a, "taro", apptest.SampleEpic, "A の Epic")
	a.sync(t, remote)
	b.sync(t, remote)
	if got := b.item(t, apptest.SampleEpic).Title; got != "A の Epic" {
		t.Errorf("HH-1 title on B = %q, want the title A saved", got)
	}
	a.sync(t, remote)
	for _, m := range []machine{a, b} {
		it := m.item(t, apptest.SampleTask)
		if it.Sprint != sprintOne || it.Status != apptest.SampleStatusTodo {
			t.Errorf("HH-2 on %s: sprint %q status %q, want sprint %q Todo", m.dir, it.Sprint, it.Status, sprintOne)
		}
		wantNoRepair(t, m)
	}
}
