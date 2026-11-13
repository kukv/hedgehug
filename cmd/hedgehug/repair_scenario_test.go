//go:build !e2e

package main

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
	"github.com/kukv/hedgehug/internal/domain/repair"
)

// 同期の修復を、本物の gitstore と repairing.Run で、2 台のマシンの場面として確かめる。

const (
	taskA item.ID = "01NEWTASKA0000000000000001"
	taskB item.ID = "01NEWTASKB0000000000000001"
)

// scenario は、同じ remote から始めた 2 台のマシン。
type scenario struct {
	remote port.Remote
	a, b   machine
}

// play は、A と B が同じ remote から始め、それぞれ書いてから、aFirst なら A → B → A、でなければ B → A → B の順に
// 同期する。両方のファイルがバイト単位で同じで、作業ツリーがきれいなことと、そのあと何も書かないことを確かめる。
func play(t *testing.T, aFirst bool, writeA, writeB func(t *testing.T, m machine)) scenario {
	t.Helper()
	remote := newBareRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", scenarioStart.Add(time.Hour))
	writeA(t, a)
	writeB(t, b)
	first, second := a, b
	if !aFirst {
		first, second = b, a
	}
	first.sync(t, remote)
	second.sync(t, remote)
	first.sync(t, remote)
	sameFiles(t, a, b)
	s := scenario{remote: remote, a: a, b: b}
	settled(t, s)
	return s
}

// settled は、修復済みの remote と合ったマシンがもう一度同期しても、3 台目のマシン C を clone して同期しても、
// 何も書かず、コミットが増えないことを確かめる(Review Focus 2)。
func settled(t *testing.T, s scenario) {
	t.Helper()
	tip := head(t, s.remote.URL)
	for _, m := range []machine{s.a, s.b} {
		if res := m.sync(t, s.remote); res.Sent != 0 || res.Received != 0 || head(t, m.dir) != tip {
			t.Errorf("sync again of %s = %+v, head %s, want nothing at %s", m.dir, res, head(t, m.dir), tip)
		}
	}
	c := cloneMachine(t, s.remote, "C", scenarioStart.Add(2*time.Hour))
	if res := c.sync(t, s.remote); res.Sent != 0 || head(t, c.dir) != tip {
		t.Errorf("sync of the third machine = %+v, head %s, want nothing at %s", res, head(t, c.dir), tip)
	}
	if got := head(t, s.remote.URL); got != tip {
		t.Errorf("remote moved: %s -> %s", tip, got)
	}
	cleanWorktree(t, c.dir)
	sameFiles(t, s.a, c)
}

// bothOrders は、同じ場面を A が先・B が先の 2 通りで作り、それぞれで check を行う。2 通りとも同じ値を確かめる
// ことで、同期の順によらないことを確かめる。
func bothOrders(t *testing.T, writeA, writeB func(t *testing.T, m machine), check func(t *testing.T, s scenario, aFirst bool)) {
	t.Helper()
	for _, aFirst := range []bool{true, false} {
		name := "A first"
		if !aFirst {
			name = "B first"
		}
		t.Run(name, func(t *testing.T) {
			s := play(t, aFirst, writeA, writeB)
			check(t, s, aFirst)
		})
	}
}

func (s scenario) machines() []machine { return []machine{s.a, s.b} }

func setTitle(id item.ID, title string) func(t *testing.T, m machine) {
	return func(t *testing.T, m machine) {
		t.Helper()
		old := m.item(t, id).Title
		m.write(t, "タイトルを変更", func(tx port.Tx) error {
			return tx.WriteItems().Append(id, change.NewSet("title", old, title))
		})
	}
}

func deleteEpic(t *testing.T, m machine) {
	t.Helper()
	if err := apptest.DeleteItems(t.Context(), m.stores, m.id, apptest.SampleEpic, apptest.SampleTask); err != nil {
		t.Fatal(err)
	}
}

func wantLive(t *testing.T, m machine, live bool, ids ...item.ID) {
	t.Helper()
	for _, id := range ids {
		if d := m.item(t, id).Deleted; (d == nil) != live {
			t.Errorf("%s on %s: deleted = %+v, want live %v", id, m.dir, d, live)
		}
	}
}

// 削除と変更: A が HH-2 を削除し、B が HH-2 のタイトルを変えると、削除が取り消され、タイトルは B の値。
func TestRepairScenarioDeleteAndChange(t *testing.T) {
	deleteTask := func(t *testing.T, m machine) {
		t.Helper()
		if err := apptest.DeleteItems(t.Context(), m.stores, m.id, apptest.SampleTask); err != nil {
			t.Fatal(err)
		}
	}
	bothOrders(t, deleteTask, setTitle(apptest.SampleTask, "B のタイトル"), func(t *testing.T, s scenario, _ bool) {
		for _, m := range s.machines() {
			wantLive(t, m, true, apptest.SampleTask)
			if got := m.item(t, apptest.SampleTask).Title; got != "B のタイトル" {
				t.Errorf("HH-2 title on %s = %q", m.dir, got)
			}
		}
	})
}

// 親の削除と親の変更: A が HH-1 を(HH-2 と一緒に)削除し、B が HH-1 のタイトルを変えると、どちらも削除済みでない。
func TestRepairScenarioParentDeleteAndParentChange(t *testing.T) {
	bothOrders(t, deleteEpic, setTitle(apptest.SampleEpic, "B の Epic"), func(t *testing.T, s scenario, _ bool) {
		for _, m := range s.machines() {
			wantLive(t, m, true, apptest.SampleEpic, apptest.SampleTask)
		}
	})
}

// 親の削除と子の変更: A が HH-1 を削除し、B が HH-2 のタイトルを変えると、HH-1・HH-2 とも削除済みのまま。
func TestRepairScenarioParentDeleteAndChildChange(t *testing.T) {
	bothOrders(t, deleteEpic, setTitle(apptest.SampleTask, "B のタイトル"), func(t *testing.T, s scenario, _ bool) {
		for _, m := range s.machines() {
			wantLive(t, m, false, apptest.SampleEpic, apptest.SampleTask)
		}
	})
}

// 親の削除と子の追加: A が HH-1 を削除し、B が HH-1 の下に Task を作ると、作った Task も HH-1 の削除の op で削除済み。
func TestRepairScenarioParentDeleteAndChildAdd(t *testing.T) {
	var op change.ID
	writeA := func(t *testing.T, m machine) {
		t.Helper()
		deleteEpic(t, m)
		op = m.item(t, apptest.SampleEpic).Deleted.Op
	}
	writeB := func(t *testing.T, m machine) {
		t.Helper()
		newTask(t, m, taskB)
	}
	bothOrders(t, writeA, writeB, func(t *testing.T, s scenario, _ bool) {
		for _, m := range s.machines() {
			if d := m.item(t, taskB).Deleted; d == nil || d.Op != op || d.Root {
				t.Errorf("new task on %s: deleted = %+v, want op %s root false", m.dir, d, op)
			}
		}
	})
}

// 番号の重なり: A と B がそれぞれ HH-3 の Task を作ると、先に送った方が HH-3 のまま、もう一方が HH-4 になる。
func TestRepairScenarioNumberCollision(t *testing.T) {
	writeA := func(t *testing.T, m machine) {
		t.Helper()
		newTask(t, m, taskA)
	}
	writeB := func(t *testing.T, m machine) {
		t.Helper()
		newTask(t, m, taskB)
	}
	bothOrders(t, writeA, writeB, func(t *testing.T, s scenario, aFirst bool) {
		want := map[item.ID]int{taskA: 3, taskB: 4}
		if !aFirst {
			want = map[item.ID]int{taskA: 4, taskB: 3}
		}
		for _, m := range s.machines() {
			for id, n := range want {
				if it := m.item(t, id); it.Place.Number != n || it.Deleted != nil {
					t.Errorf("%s on %s: number %d deleted %+v, want number %d live", id, m.dir, it.Place.Number, it.Deleted, n)
				}
			}
		}
	})
}

// 親の循環: A が生の行で HH-1 を Story にして親を HH-2 にし、B が同じ時期に HH-2 のタイトルを変えると、同期のあと
// 輪が切れ、「修復で作った Epic」(内部 ID・番号とも決まった値)の下に HH-2・HH-1 が入る。
func TestRepairScenarioCycle(t *testing.T) {
	writeA := func(t *testing.T, m machine) {
		t.Helper()
		old := m.item(t, apptest.SampleEpic).Place
		to := old
		parent := apptest.SampleTask
		to.Parent = &parent
		m.write(t, "親を変更", func(tx port.Tx) error {
			return tx.WriteItems().Append(apptest.SampleEpic, change.NewSet("kind", item.Epic, item.Story), change.NewSet("place", old, to))
		})
	}
	epic := repair.EpicID(apptest.SampleProject, func(change.ID) bool { return false })
	bothOrders(t, writeA, setTitle(apptest.SampleTask, "B のタイトル"), func(t *testing.T, s scenario, _ bool) {
		for _, m := range s.machines() {
			if it := m.item(t, epic); it.Title != repair.RepairEpicTitle || it.Kind != item.Epic || it.Place.Number != 3 || it.Place.Parent != nil {
				t.Errorf("repair epic on %s = %+v", m.dir, it)
			}
			for _, id := range []item.ID{apptest.SampleTask, apptest.SampleEpic} {
				if p := m.item(t, id).Place.Parent; p == nil || *p != epic {
					t.Errorf("%s on %s: parent %v, want %s", id, m.dir, p, epic)
				}
			}
			noCycle(t, m)
		}
	})
}

// noCycle は、プロジェクトのどのアイテムから親を辿っても、親のないアイテムに着くことを確かめる。
func noCycle(t *testing.T, m machine) {
	t.Helper()
	its := m.items(t, apptest.SampleProject)
	parent := map[item.ID]*item.ID{}
	for _, it := range its {
		parent[it.ID] = it.Place.Parent
	}
	for _, it := range its {
		id, steps := it.ID, 0
		for p := parent[id]; p != nil; p = parent[id] {
			if id, steps = *p, steps+1; steps > len(its) {
				t.Errorf("%s on %s: parents form a cycle", it.ID, m.dir)
				break
			}
		}
	}
}

// プロジェクトの違い: A が HH-1 を HH-P2 へ移し、B が HH-1 の下に Task を作ると、HH-1 の下はすべて HH-P2 になる。
func TestRepairScenarioProjectMismatch(t *testing.T) {
	writeA := func(t *testing.T, m machine) {
		t.Helper()
		old := m.item(t, apptest.SampleEpic).Place
		to := old
		to.Project = sampleProject2
		m.write(t, "プロジェクトを変更", func(tx port.Tx) error {
			return tx.WriteItems().Append(apptest.SampleEpic, change.NewSet("place", old, to))
		})
	}
	writeB := func(t *testing.T, m machine) {
		t.Helper()
		newTask(t, m, taskB)
	}
	bothOrders(t, writeA, writeB, func(t *testing.T, s scenario, _ bool) {
		for _, m := range s.machines() {
			for _, id := range []item.ID{apptest.SampleEpic, apptest.SampleTask, taskB} {
				if p := m.item(t, id).Place.Project; p != sampleProject2 {
					t.Errorf("%s on %s: project %s, want %s", id, m.dir, p, sampleProject2)
				}
			}
		}
	})
}
