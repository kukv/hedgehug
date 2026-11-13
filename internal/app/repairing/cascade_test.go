package repairing_test

import (
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/repairing"
	"github.com/kukv/hedgehug/internal/domain/item"
)

const step item.ID = "01SAMPLEIT0000000000000003"

// addStep は、HH-2 の下に Step HH-3 を足す。
func addStep(e *env) {
	parent := apptest.SampleTask
	it := item.Item{ID: step, Place: item.Place{Workspace: apptest.SampleWorkspace, Project: apptest.SampleProject, Parent: &parent, Number: 3}, Kind: item.Step, Rank: "a", Title: "下書き", Status: apptest.SampleStatusTodo, Priority: apptest.SamplePriorityMedium}
	e.write(taro, func(tx port.Tx) error { return tx.WriteItems().Append(it.ID, item.Create(it)) })
}

// 削除済みの親の下に残った子は、親の削除の op で削除済みになり、その下も同じ Run の中で続く。
func TestCascadeUnderDeletedItem(t *testing.T) {
	e := newEnv(t)
	addStep(e)
	if err := apptest.DeleteItems(e.ctx, e.s, store, apptest.SampleEpic); err != nil {
		t.Fatal(err)
	}
	epicOp := e.item(apptest.SampleEpic).Deleted.Op
	ops := e.run(nil, nil)
	wantOps(t, ops, "HH-2 を削除", "HH-3 を削除")
	for _, id := range []item.ID{apptest.SampleTask, step} {
		d := e.item(id).Deleted
		if d == nil || d.Op != epicOp || d.Root {
			t.Errorf("%s deleted = %+v, want op %s root false", id, d, epicOp)
		}
	}
}

// プロジェクトが削除済みなら、生きている Epic はプロジェクトの削除の op で削除済みになる。
func TestCascadeUnderDeletedProject(t *testing.T) {
	e := newEnv(t)
	if err := apptest.DeleteProject(e.ctx, e.s, store, apptest.SampleProject); err != nil {
		t.Fatal(err)
	}
	projectOp := e.project(apptest.SampleProject).Deleted.Op
	wantOps(t, e.run(nil, nil), "HH-1 を削除", "HH-2 を削除")
	if d := e.item(apptest.SampleEpic).Deleted; d == nil || d.Op != projectOp || d.Root != false {
		t.Errorf("HH-1 deleted = %+v, want op %s", d, projectOp)
	}
}

// 書いても削除済みにならず、同じ子が残り続けるときは、止まらずに止めてエラーを返す。
type noopWriter struct{ *apptest.RepairWriter }

func (noopWriter) Write(port.Operation, func(port.Tx) error) error { return nil }

func TestCascadeStopsWithoutProgress(t *testing.T) {
	e := newEnv(t)
	if err := apptest.DeleteProject(e.ctx, e.s, store, apptest.SampleProject); err != nil {
		t.Fatal(err)
	}
	w := noopWriter{&apptest.RepairWriter{Stores: e.s, Store: store}}
	if err := repairing.Run(e.ctx, w); err == nil || !strings.Contains(err.Error(), "進みません") {
		t.Errorf("Run error = %v", err)
	}
}
