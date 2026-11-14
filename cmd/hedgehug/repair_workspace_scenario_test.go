//go:build !e2e

package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/numbering"
	"github.com/kukv/hedgehug/internal/domain/project"
	"github.com/kukv/hedgehug/internal/domain/transfer"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

const (
	mobileWS      workspace.ID = "01SAMPLEWS0000000000000009"
	mobileProject project.ID   = "01SAMPLEPR0000000000000009"
	mobileEpic    item.ID      = "01SAMPLEIT0000000000000009"
)

// addMobile は、ワークスペース「Mobile」(MB)、プロジェクト MB-P1、Epic MB-1 と、番号の行(項目 1・プロジェクト 1)を
// 1 回の保存で足す(internal/app/repairing のテストの addMobile と同じ値)。
func addMobile(t *testing.T, m machine) {
	t.Helper()
	n := 0
	gen := func() change.ID {
		n++
		return change.ID(fmt.Sprintf("01SAMPLEM9V%015d", n))
	}
	d, err := workspace.CreateWithSettings(workspace.Workspace{ID: mobileWS, Name: "Mobile", Prefix: "MB"}, workspace.DefaultSettings(gen))
	if err != nil {
		t.Fatal(err)
	}
	pr := project.Project{ID: mobileProject, Place: project.Place{Workspace: mobileWS, Number: 1}, Rank: "V", Title: "アプリ", Owner: "taro", Status: project.Todo}
	epic := item.Item{ID: mobileEpic, Place: item.Place{Workspace: mobileWS, Project: mobileProject, Number: 1}, Kind: item.Epic, Rank: "a", Title: "iOS", Priority: apptest.SamplePriorityMedium}
	m.write(t, "ワークスペースを作成", func(tx port.Tx) error {
		return errors.Join(
			tx.WriteWorkspaces().Append(mobileWS, d, numbering.Draft(numbering.Projects, 1), numbering.Draft(numbering.Items, 1)),
			tx.WriteProjects().Append(pr.ID, project.Create(pr)),
			tx.WriteItems().Append(epic.ID, item.Create(epic)),
		)
	})
}

// moveToMobile は、HH-1 を MB-2、HH-2 を MB-3 へ移す行(set place・Mobile の number・転送記録)を 1 回の保存で書く。
// 親は変えない(A5a の writeWithin と同じ形の生の行)。
func moveToMobile(t *testing.T, m machine) {
	t.Helper()
	moves := []struct {
		id       item.ID
		n        int
		from, to string
	}{
		{apptest.SampleEpic, 2, "HH-1", "MB-2"},
		{apptest.SampleTask, 3, "HH-2", "MB-3"},
	}
	olds := []item.Place{m.item(t, moves[0].id).Place, m.item(t, moves[1].id).Place} // 保存の中では読めない(ロック)
	m.write(t, "Mobile へ移動", func(tx port.Tx) error {
		var errs []error
		for i, mv := range moves {
			old := olds[i]
			to := old
			to.Workspace, to.Project, to.Number = mobileWS, mobileProject, mv.n
			errs = append(errs,
				tx.WriteItems().Append(mv.id, change.NewSet("place", old, to)),
				tx.WriteWorkspaces().Append(mobileWS, numbering.Draft(numbering.Items, mv.n)),
				tx.WriteTransfers().Append(transfer.Record{Entity: transfer.EntityItem, Target: mv.id, Old: mv.from, New: mv.to, ToStore: string(m.id)}),
			)
		}
		return errors.Join(errs...)
	})
}

// ワークスペースの違い: A が Mobile を足してから HH-1・HH-2 を Mobile へ移し、B が HH-1 の下に Task HH-3 を作ると、
// HH-3 は Mobile の MB-P1 の次の番号 MB-4 になり、転送記録に残る。
func TestRepairScenarioWorkspaceMismatch(t *testing.T) {
	writeA := func(t *testing.T, m machine) {
		t.Helper()
		addMobile(t, m)
		moveToMobile(t, m)
	}
	writeB := func(t *testing.T, m machine) {
		t.Helper()
		newTask(t, m, taskB)
	}
	bothOrders(t, writeA, writeB, func(t *testing.T, s scenario, _ bool) {
		for _, m := range s.machines() {
			want := transfer.Record{Entity: transfer.EntityItem, Target: taskB, Old: "HH-3", New: "MB-4", ToStore: string(m.id)}
			wantMovedNumbers(t, m)
			p := m.item(t, taskB).Place
			if p.Workspace != mobileWS || p.Project != mobileProject || p.Number != 4 {
				t.Errorf("new task on %s: place %+v, want Mobile / MB-P1 / 4", m.dir, p)
			}
			if !hasTransfer(t, m, want) {
				t.Errorf("transfers on %s have no %+v", m.dir, want)
			}
		}
	})
}

// wantMovedNumbers は、m で HH-1・HH-2 が Mobile の番号 2・3 のままであることを確かめる。
func wantMovedNumbers(t *testing.T, m machine) {
	t.Helper()
	for id, n := range map[item.ID]int{apptest.SampleEpic: 2, apptest.SampleTask: 3} {
		if p := m.item(t, id).Place; p.Workspace != mobileWS || p.Number != n {
			t.Errorf("%s on %s: place %+v, want Mobile / %d", id, m.dir, p, n)
		}
	}
}

// hasTransfer は、転送記録に、want の Entity・Target・Old・New・ToStore と合う行があるか。
func hasTransfer(t *testing.T, m machine, want transfer.Record) bool {
	t.Helper()
	var recs []transfer.Record
	err := m.stores.Read(t.Context(), m.id, func(r port.Reader) error {
		var err error
		recs, err = r.Transfers().List()
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range recs {
		if r.Entity == want.Entity && r.Target == want.Target && r.Old == want.Old && r.New == want.New && r.ToStore == want.ToStore {
			return true
		}
	}
	return false
}
