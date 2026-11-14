package gitstore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/planning"
)

// repairOnce は、m の同期の修復を、最初に呼ばれたときだけ、実行者 sync で Epic に 1 行書くものにする
// (本物の修復の保存と同じく、作者は bot で、行の実行者は sync)。
func repairOnce(m machine) {
	done := false
	m.rep.Repair = func(_ context.Context, w port.RepairWriter) error {
		if done {
			return nil
		}
		done = true
		op := port.Operation{Actor: change.Actor{Kind: change.ActorSync}, Summary: "修復"}
		return w.Write(op, func(tx port.Tx) error {
			return tx.WriteItems().Append(apptest.SampleEpic, change.NewSet("rank", "a", "b"))
		})
	}
}

// passed は、最後の呼び出しで渡された保存のうち、印が h のもの。
func (j *judgeRecorder) passed(t *testing.T, h plumbing.Hash) port.ReceivedSave {
	t.Helper()
	if len(j.calls) == 0 {
		t.Fatal("judge was not called")
	}
	for _, s := range j.calls[len(j.calls)-1] {
		if s.Key.Version == h.String() {
			return s
		}
	}
	t.Fatalf("%s was not passed: %v", h, j.last())
	return port.ReceivedSave{}
}

// wantRepair は、s が、実行者 sync の 1 行を Epic に足した修復の保存であること。
func wantRepair(t *testing.T, s port.ReceivedSave) {
	t.Helper()
	epic := change.Ref{Kind: change.KindItem, ID: apptest.SampleEpic}
	if !s.Repair || s.Merge || len(s.Entities) != 1 || s.Entities[0].Ref != epic || len(s.Entities[0].Rows) != 1 ||
		s.Entities[0].Rows[0].Actor.Kind != change.ActorSync {
		t.Errorf("repair save = %+v", s)
	}
}

// wantMergeRemoved は、s が、サンプルのタスクのフォルダを消した合わせで、New が行 1 つ(実行者が人)であること。
func wantMergeRemoved(t *testing.T, s port.ReceivedSave) {
	t.Helper()
	task := change.Ref{Kind: change.KindItem, ID: apptest.SampleTask}
	if !s.Merge || s.Repair || len(s.Entities) != 0 || len(s.Removed) != 1 {
		t.Fatalf("merge = %+v", s)
	}
	e := s.Removed[0]
	if e.Ref != task || len(e.Log) < 2 || len(e.New) != 1 || e.New[0].Actor.Kind != change.ActorUser || e.New[0].F != "title" {
		t.Errorf("removed = %+v", e)
	}
}

// A の同期で書いた修復の保存は、A の判定に渡る。もう一度同期しても渡らない。
func TestJudgeGetsOwnRepair(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	j := withJudge(a)
	repairOnce(a)
	a.sync(t, remote)
	b.write(t, "b", set("rank", "a", "b"))
	b.sync(t, remote)
	bs := headOf(t, b)
	a.write(t, "a", set("title", "ログイン画面を作る", "ログイン"))
	a.sync(t, remote)
	wantRepair(t, j.passed(t, headOf(t, a)))
	if s := j.passed(t, bs); s.Repair || s.Merge || len(s.Removed) != 0 {
		t.Errorf("received save = %+v", s)
	}
	if n := len(j.calls[len(j.calls)-1]); n != 2 {
		t.Errorf("passed %v, want the received save and the repair", j.last())
	}
	n := len(j.calls)
	a.sync(t, remote)
	if len(j.calls) != n {
		t.Errorf("judged again: %v", j.last())
	}
}

// A が書いた行のあるアイテムを、B が完全に削除してから合わせて送ると、A の同期で、その合わせが、消えた
// アイテムと A の行を持って渡る。
func TestRemovedInOthersMerge(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	j := withJudge(a)
	a.write(t, "a", set("title", "ログイン画面を作る", "ログイン"))
	a.sync(t, remote)
	b.purgeForTest(t, apptest.SampleTask)
	b.sync(t, remote)
	merge := headOf(t, b)
	a.sync(t, remote)
	wantMergeRemoved(t, j.passed(t, merge))
	if n := len(j.calls[len(j.calls)-1]); n != 2 {
		t.Errorf("passed %v, want B's save and the merge", j.last())
	}
}

// 判定がエラーを返したら、自分の修復の保存と、消えたものがある合わせは判定済みにせず、次の同期でもう一度渡る。
func TestJudgeErrorKeepsOwnRepair(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	j := withJudge(a)
	repairOnce(a)
	a.sync(t, remote)
	a.write(t, "a", set("title", "ログイン画面を作る", "ログイン"))
	b.purgeForTest(t, apptest.SampleTask)
	b.sync(t, remote)
	j.err = errors.New("通知を書けない")
	a.sync(t, remote)
	fix := headOf(t, a)
	merge := headCommit(t, a.dir).ParentHashes[0]
	wantRepair(t, j.passed(t, fix))
	wantMergeRemoved(t, j.passed(t, merge))
	for _, h := range []plumbing.Hash{fix, merge} {
		if j.known(t, a, h) != port.NotJudged {
			t.Errorf("%s was marked although judging failed", h)
		}
	}
	j.err = nil
	a.sync(t, remote)
	if len(j.calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(j.calls))
	}
	wantRepair(t, j.passed(t, fix))
	wantMergeRemoved(t, j.passed(t, merge))
	if j.known(t, a, fix) != port.SameVersion {
		t.Error("the repair was not marked after judging")
	}
}

// 閲覧のみの同期は ReadOnly を渡し、ふつうの同期は渡さない。
func TestJudgeReadOnlyFlag(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	j := withJudge(b)
	view := port.Remote{URL: remote.URL, ReceiveOnly: true}
	if _, err := b.rep.Sync(t.Context(), b.id, view); err != nil {
		t.Fatal(err)
	}
	a.write(t, "a", set("rank", "a", "b"))
	a.sync(t, remote)
	if _, err := b.rep.Sync(t.Context(), b.id, view); err != nil {
		t.Fatal(err)
	}
	a.write(t, "a2", set("rank", "b", "c"))
	a.sync(t, remote)
	b.sync(t, remote)
	if len(j.readOnly) != 2 || !j.readOnly[0] || j.readOnly[1] {
		t.Errorf("ReadOnly = %v, want [true false]", j.readOnly)
	}
}

// 本物の (1) の修復(完全に削除したものの下に作った子を消す)の保存は、Repair で、消えた子とそのログを持って渡る。
func TestJudgeGetsPurgeRepair(t *testing.T) {
	remote, _, b := purgedThenStep(t, apptest.SampleTask)
	j := withJudge(b)
	b.sync(t, remote)
	s := j.passed(t, headOf(t, b))
	step := change.Ref{Kind: change.KindItem, ID: sampleStep}
	if !s.Repair || s.Merge || len(s.Entities) != 0 || len(s.Removed) != 1 {
		t.Fatalf("repair save = %+v", s)
	}
	if e := s.Removed[0]; e.Ref != step || len(e.Log) == 0 || e.Log[0].T != change.Create || len(e.New) != 0 {
		t.Errorf("removed = %+v", e)
	}
}

// B が完全に削除したスプリントを A が同時に改名していたとき、B の合わせは、A の同期の判定に、取り除かれたスプリントの
// 行(A の改名が New)つきで 1 回だけ渡る。
func TestDroppedPlanReachesJudge(t *testing.T) {
	const s1 change.ID = "01SPRINT0000000000000000S1"
	remote := newRemote(t)
	a := firstMachine(t, remote)
	j := withJudge(a)
	sprint := planning.Sprint{ID: s1, Name: "Sprint 1", State: planning.Planned}
	a.write(t, "スプリント", func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, planning.SprintDrafts(planning.Sprint{}, sprint)...)
	})
	a.sync(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	b.rep.Repair = noRepair
	b.purgePlanForTest(t, apptest.SampleWorkspace, s1, "sprint", "Sprint 1")
	renamed := sprint
	renamed.Name = "Sprint 1 改"
	a.write(t, "名前", func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, planning.SprintDrafts(sprint, renamed)...)
	})
	a.sync(t, remote)
	b.sync(t, remote)
	merge := headOf(t, b)
	a.sync(t, remote)
	got := 0
	for _, calls := range j.calls {
		for _, s := range calls {
			if s.Key.Version != merge.String() {
				continue
			}
			got++
			if !s.Merge || len(s.DroppedPlan) != 1 {
				t.Fatalf("merge = %+v", s)
			}
			d := s.DroppedPlan[0]
			if d.Workspace != apptest.SampleWorkspace || d.Target != s1 || len(d.New) == 0 || len(d.Log) <= len(d.New) {
				t.Errorf("DroppedPlan = %+v", d)
			}
		}
	}
	if got != 1 {
		t.Errorf("the merge was passed %d times, want 1", got)
	}
}
