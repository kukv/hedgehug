//go:build !e2e

package main

import (
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

// 同期の修復の通知を、本物の gitstore と repairing.Run と state.db の受信箱で、2 台のマシンの場面として確かめる
// (決定 C4b2-14)。A のログイン名は taro、B は hanako。サンプルのメンバーには表示名がないので、名前はログイン名。

const restoreText = "HH-1 は hanako が同時に変更したため、削除を取り消しました"

// titleAs は、login の操作として、アイテム id のタイトルを title にする。
func titleAs(t *testing.T, m machine, login string, id item.ID, title string) {
	t.Helper()
	old := m.item(t, id).Title
	m.writeAs(t, login, "タイトルを変更", func(tx port.Tx) error {
		return tx.WriteItems().Append(id, change.NewSet("title", old, title))
	})
}

// deleteEpicAs は、login の操作として、HH-1 を(below と一緒に)削除済みにする。
func deleteEpicAs(t *testing.T, m machine, login string, below ...item.ID) {
	t.Helper()
	m.writeAs(t, login, "アイテムを削除", func(tx port.Tx) error {
		errs := []error{tx.WriteItems().Append(apptest.SampleEpic, change.NewDeleteEntity(true))}
		for _, b := range below {
			errs = append(errs, tx.WriteItems().Append(b, change.NewDeleteEntity(false)))
		}
		return errors.Join(errs...)
	})
}

// wantRepair は、m の受信箱の「同期の修復」が、差出人が空(hedgehug)で本文が text の 1 行だけなことを確かめる。
func wantRepair(t *testing.T, m machine, text string) {
	t.Helper()
	rows := m.notices(t, port.NoticeRepair)
	if len(rows) != 1 || rows[0].Text != text || rows[0].Sender != "" {
		t.Errorf("repair rows on %s = %+v, want one %q", m.dir, rows, text)
	}
}

// wantNoRepair は、m の受信箱に「同期の修復」の行がないことを確かめる。
func wantNoRepair(t *testing.T, m machine) {
	t.Helper()
	if rows := m.notices(t, port.NoticeRepair); len(rows) != 0 {
		t.Errorf("repair rows on %s = %+v, want none", m.dir, rows)
	}
}

// repairedBy は、m の同期が取り消しの修復の保存を書いて送ったことを確かめる。相手は削除と変更のどちらかを送り
// 終えていて、まだこの同期の結果を受け取っていない。同期の直後に、m が送り、m の先端が remote の先端で、HH-1 が
// m で削除済みでないことで見る(remote を進めたのは m の送信だけなので、合わせと修復を書いたのは m)。
func repairedBy(t *testing.T, m machine, remote port.Remote) {
	t.Helper()
	before := head(t, remote.URL)
	res := m.sync(t, remote)
	if res.Sent == 0 || head(t, m.dir) != head(t, remote.URL) || head(t, remote.URL) == before {
		t.Fatalf("sync of %s = %+v, want it to send its repair to the remote", m.dir, res)
	}
	wantLive(t, m, true, apptest.SampleEpic)
}

// B が HH-1 を変えて同期し、A が(B の変更を受け取る前に)HH-1 を削除して同期すると、A のマシンが取り消しの修復を
// 書き、A の受信箱に 1 行。A がもう一度同期しても 1 行。B には作らない。
func TestRepairNoticeRestoreOnMyMachine(t *testing.T) {
	remote, a, b := noticePair(t)
	titleAs(t, b, "hanako", apptest.SampleEpic, "B の Epic")
	b.sync(t, remote)
	deleteEpicAs(t, a, "taro", apptest.SampleTask)
	repairedBy(t, a, remote)
	wantRepair(t, a, restoreText)
	a.sync(t, remote)
	wantRepair(t, a, restoreText)
	if res := b.sync(t, remote); res.Sent != 0 {
		t.Errorf("sync of B = %+v, want B to only receive", res)
	}
	wantNoRepair(t, b)
}

// A が HH-1 を削除して同期し、B が(A の削除を受け取る前に)HH-1 を変えて同期すると、B のマシンが修復を書く。
// A が同期すると、A に同じ本文が 1 行。B には作らない。
func TestRepairNoticeRestoreFromOther(t *testing.T) {
	remote, a, b := noticePair(t)
	deleteEpicAs(t, a, "taro", apptest.SampleTask)
	a.sync(t, remote)
	titleAs(t, b, "hanako", apptest.SampleEpic, "B の Epic")
	repairedBy(t, b, remote)
	wantNoRepair(t, a)
	if res := a.sync(t, remote); res.Sent != 0 {
		t.Errorf("sync of A = %+v, want A to only receive", res)
	}
	wantRepair(t, a, restoreText)
	a.sync(t, remote)
	wantRepair(t, a, restoreText)
	wantNoRepair(t, b)
}

// A と A2 はどちらも taro のマシン。A のマシンが修復を書くと A に 1 行、A2 が同期すると A2 にも 1 行。A は 1 行のまま。
func TestRepairNoticeRestoreTwoMachinesSameLogin(t *testing.T) {
	remote, a, b := noticePair(t)
	a2 := noticed(t, cloneMachine(t, remote, "D", scenarioStart.Add(2*time.Hour)), "taro")
	a2.sync(t, remote)
	titleAs(t, b, "hanako", apptest.SampleEpic, "B の Epic")
	b.sync(t, remote)
	deleteEpicAs(t, a, "taro", apptest.SampleTask)
	repairedBy(t, a, remote)
	wantRepair(t, a, restoreText)
	a2.sync(t, remote)
	wantRepair(t, a2, restoreText)
	a.sync(t, remote)
	wantRepair(t, a, restoreText)
	wantNoRepair(t, b)
}

// A が Epic HH-1 をアプリの削除のとおり(子 HH-2 に root: false の削除も書いて)削除して同期し、B が(受け取る前に)
// HH-2 を変えて同期すると、HH-2 は削除済みのままで修復の保存は書かれないが、合わせから B に親の削除の 1 行
// (決定 C4b2-17)。A には作らない。
func TestRepairNoticeCascade(t *testing.T) {
	remote, a, b := noticePair(t)
	deleteEpicAs(t, a, "taro", apptest.SampleTask)
	a.sync(t, remote)
	titleAs(t, b, "hanako", apptest.SampleTask, "B のタイトル")
	b.sync(t, remote)
	wantLive(t, b, false, apptest.SampleEpic, apptest.SampleTask)
	const want = "HH-1 が削除されたため、HH-2 も削除済みに入りました"
	wantRepair(t, b, want)
	b.sync(t, remote)
	wantRepair(t, b, want)
	a.sync(t, remote)
	wantNoRepair(t, a)
}

// A が HH-1 を(HH-2 と一緒に)削除して同期し、B が(受け取る前に)HH-1 の下に Task HH-3 を足して同期すると、B の
// マシンが HH-3 を削除済みに入れる修復の保存を書き、B に親の削除の 1 行。A には作らない。
func TestRepairNoticeCascadeAdded(t *testing.T) {
	remote, a, b := noticePair(t)
	deleteEpicAs(t, a, "taro", apptest.SampleTask)
	a.sync(t, remote)
	newTaskAs(t, b, "hanako", taskB)
	b.sync(t, remote)
	wantLive(t, b, false, apptest.SampleEpic, taskB)
	const want = "HH-1 が削除されたため、HH-3 も削除済みに入りました"
	wantRepair(t, b, want)
	b.sync(t, remote)
	wantRepair(t, b, want)
	a.sync(t, remote)
	wantNoRepair(t, a)
}

// A と B が同時に HH-3 の Task を作り、A が先に同期すると、B の同期で B の HH-3 が HH-4 に振り直り、B に 1 行。
func TestRepairNoticeNumber(t *testing.T) {
	remote, a, b := noticePair(t)
	newTaskAs(t, a, "taro", taskA)
	newTaskAs(t, b, "hanako", taskB)
	a.sync(t, remote)
	b.sync(t, remote)
	if n := b.item(t, taskB).Place.Number; n != 4 {
		t.Fatalf("B's task number = %d, want 4", n)
	}
	const want = "2 台で同時に HH-3 が作られたため、あとから届いた方を HH-4 に振り直しました"
	wantRepair(t, b, want)
	b.sync(t, remote)
	wantRepair(t, b, want)
	a.sync(t, remote)
	wantNoRepair(t, a)
}

// B が HH-1 を削除して同期し、A が(受け取る前に)HH-1 を変えて同期すると、A のマシンが取り消しの修復を書く。
// 宛先は削除した B(hanako)。B が閲覧のみで同期して修復の保存を受け取ると、B には作らない。閲覧のみでなければ 1 行。
func TestRepairNoticeReadOnly(t *testing.T) {
	for _, readOnly := range []bool{true, false} {
		name := "read only"
		if !readOnly {
			name = "writable"
		}
		t.Run(name, func(t *testing.T) {
			remote, a, b := noticePair(t)
			deleteEpicAs(t, b, "hanako", apptest.SampleTask)
			b.sync(t, remote)
			titleAs(t, a, "taro", apptest.SampleEpic, "A の Epic")
			repairedBy(t, a, remote)
			wantNoRepair(t, a)
			view := remote
			view.ReceiveOnly = readOnly
			b.sync(t, view)
			wantLive(t, b, true, apptest.SampleEpic)
			if readOnly {
				wantNoRepair(t, b)
				return
			}
			wantRepair(t, b, "HH-1 は taro が同時に変更したため、削除を取り消しました")
		})
	}
}
