package repairing_test

import (
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	priorityHigh change.ID = "01SAMPLEVAL000000000000007"
	closed       change.ID = "01CLOSED000000000000000001"
	shipped      change.ID = "01SHIPPED00000000000000001"
)

var priorities = workspace.ParseList("priorities")

func (e *env) settings() workspace.Settings {
	e.t.Helper()
	var st workspace.Settings
	err := e.s.Read(e.ctx, store, func(r port.Reader) error {
		var err error
		st, err = r.Workspaces().Settings(apptest.SampleWorkspace)
		return err
	})
	if err != nil {
		e.t.Fatal(err)
	}
	return st
}

func (e *env) hasStatus(id change.ID) bool {
	for _, s := range e.settings().Statuses {
		if s.ID == id {
			return true
		}
	}
	return false
}

// workspaceWrite は、ワークスペースのログに ds を書く。
func (e *env) workspaceWrite(ds ...change.Draft) map[change.ID]bool {
	return e.write(taro, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(apptest.SampleWorkspace, ds...)
	})
}

// remote の側で削除した値を、自分の側で進行中の既定にしたら、削除を取り消す。
func TestRestoreReferencedDefault(t *testing.T) {
	e := newEnv(t)
	del := e.workspaceWrite(change.NewDelete(reviewRef))
	pick := e.workspaceWrite(change.NewSet("category_default/doing", apptest.SampleStatusInProgress, review))
	wantOps(t, e.run(pick, del), "「Review」の削除を取り消し")
	if !e.hasStatus(review) {
		t.Errorf("statuses = %+v, want Review", e.settings().Statuses)
	}
}

// 削除の操作で置き換えたアイテムのステータスも戻る。
func TestRestoreReferencedUndoesReplacements(t *testing.T) {
	e := newEnv(t)
	del := e.write(taro, func(tx port.Tx) error {
		return errors.Join(
			tx.WriteWorkspaces().Append(apptest.SampleWorkspace, change.NewDelete(reviewRef)),
			tx.WriteItems().Append(apptest.SampleTask, change.NewSet("status", review, apptest.SampleStatusTodo)),
		)
	})
	pick := e.workspaceWrite(change.NewSet("category_default/doing", apptest.SampleStatusInProgress, review))
	wantOps(t, e.run(pick, del), "「Review」の削除を取り消し")
	if it := e.item(apptest.SampleTask); it.Status != review {
		t.Errorf("HH-2 status = %s, want Review", it.Status)
	}
}

// 初期優先度にした値の同時の削除も取り消す。
func TestRestoreReferencedInitialPriority(t *testing.T) {
	e := newEnv(t)
	del := e.workspaceWrite(change.NewDelete(priorities.Ref(priorityHigh)))
	pick := e.workspaceWrite(change.NewSet("initial_priority", apptest.SamplePriorityMedium, priorityHigh))
	wantOps(t, e.run(pick, del), "「High」の削除を取り消し")
	if got := e.settings().InitialPriority; got != priorityHigh {
		t.Errorf("initial priority = %s, want High", got)
	}
}

// 完了の分類が、両側の削除で 0 個になったら、自分の側の削除だけを取り消す。
// 既定の付け替え(remote の側)と同時で、既定は todo の分類にした Closed を指す。
func TestRestoreReferencedEmptyCategory(t *testing.T) {
	e := newEnv(t)
	e.workspaceWrite(append(
		workspace.ValueDrafts(statuses, workspace.Value{ID: closed, Name: "Closed", Color: "gray", Category: workspace.CategoryDone, Rank: "b"}),
		workspace.ValueDrafts(statuses, workspace.Value{ID: shipped, Name: "Shipped", Color: "gray", Category: workspace.CategoryDone, Rank: "c"})...)...)
	theirs := e.workspaceWrite(
		change.NewSet("category_default/done", apptest.SampleStatusDone, closed),
		change.NewDelete(statuses.Ref(apptest.SampleStatusDone)),
	)
	ours := e.workspaceWrite(
		change.NewSet(statuses.Field(closed, "category"), workspace.CategoryDone, workspace.CategoryTodo),
		change.NewDelete(statuses.Ref(shipped)),
	)
	wantOps(t, e.run(ours, theirs), "「Shipped」の削除を取り消し")
	if !e.hasStatus(shipped) || e.hasStatus(apptest.SampleStatusDone) {
		t.Errorf("statuses = %+v, want only Shipped restored", e.settings().Statuses)
	}
}

// 名前の変更と重なった削除を restoreOverlaps が取り消したら、この段は何も書かない。
func TestRestoreReferencedAfterOverlap(t *testing.T) {
	e := newEnv(t)
	del := e.workspaceWrite(change.NewDelete(reviewRef))
	ours := e.workspaceWrite(
		change.NewSet(statuses.Field(review, "name"), "Review", "レビュー"),
		change.NewSet("category_default/doing", apptest.SampleStatusInProgress, review),
	)
	wantOps(t, e.run(ours, del), "「レビュー」の削除を取り消し")
}
