//go:build !e2e

package main

import (
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// 上書きの通知(決定 C4c1-2〜C4c1-12)を、本物の gitstore と state.db と noticing.Service で、2 台(と同じ人の別の
// マシン)の場面として確かめる。A と C のログイン名は hanako(C は hanako の別のマシン)、B は taro。
// 値の勝ち負けは保存の時刻(at)で決まるので、マシンの時計の始まりで、どちらの値があとかを決める。

const (
	statusOverwritten = "ステータスが上書きされました"
	descConflict      = "説明の同じ箇所が同時に編集されたため、両方を残しました"
)

// overwriteTrio は、同じ remote から始め、通知を判定する A(hanako)と B(taro)と C(hanako の別のマシン)。
// B の時計は bStart から始まる(A は scenarioStart から)。どのマシンも一度同期して、clone の前の保存を判定済みにする。
func overwriteTrio(t *testing.T, bStart time.Time) (port.Remote, machine, machine, machine) {
	t.Helper()
	remote := newBareRemote(t)
	a := noticed(t, firstMachine(t, remote), "hanako")
	b := noticed(t, cloneMachine(t, remote, "B", bStart), "taro")
	c := noticed(t, cloneMachine(t, remote, "C", scenarioStart.Add(2*time.Hour)), "hanako")
	b.sync(t, remote)
	c.sync(t, remote)
	return remote, a, b, c
}

// setStatusAs は、login の操作として、HH-2 のステータスを Todo から to にする。
func setStatusAs(t *testing.T, m machine, login string, to change.ID) {
	t.Helper()
	m.writeAs(t, login, "ステータスを変更", setTask(change.NewSet("status", apptest.SampleStatusTodo, to)))
}

// wantStatus は、m の HH-2 のステータスが want なことを確かめる(どちらの値が勝ったかの前提)。
func wantStatus(t *testing.T, m machine, want change.ID) {
	t.Helper()
	if got := m.item(t, apptest.SampleTask).Status; got != want {
		t.Fatalf("status of HH-2 on %s = %s, want %s", m.dir, got, want)
	}
}

// wantOverwrite は、m の受信箱の「上書き」が、HH-2 の項目 field の、本文 text・差出人 sender の 1 行だけなことを確かめる。
func wantOverwrite(t *testing.T, m machine, field, text, sender string) {
	t.Helper()
	rows := m.notices(t, port.NoticeOverwrite)
	if len(rows) != 1 || rows[0].Item != apptest.SampleTask || rows[0].Field != field || rows[0].Text != text || rows[0].Sender != sender {
		t.Errorf("overwrite rows on %s = %+v, want one %q of %s from %q", m.dir, rows, text, field, sender)
	}
}

// wantNoOverwrite は、m の受信箱に「上書き」の行がないことを確かめる。
func wantNoOverwrite(t *testing.T, m machine) {
	t.Helper()
	if rows := m.notices(t, port.NoticeOverwrite); len(rows) != 0 {
		t.Errorf("overwrite rows on %s = %+v, want none", m.dir, rows)
	}
}

// syncOrder は、A と B を first の順で同期し、最初に送ったマシンがもう一方の値と合わせを受け取るまで同期する。
func syncOrder(t *testing.T, remote port.Remote, aFirst bool, a, b machine) {
	t.Helper()
	first, second := a, b
	if !aFirst {
		first, second = b, a
	}
	first.sync(t, remote)
	second.sync(t, remote)
	first.sync(t, remote)
}

var overwriteOrders = []struct {
	name   string
	aFirst bool
}{
	{"A sends first", true},
	{"B sends first", false},
}

// A(hanako)がステータスを InProgress、B(taro)が同時に(あとの時刻で)Done にする。どちらが先に送っても、
// A の受信箱に「ステータスが上書きされました」が 1 行、B には 0 行。もう一度同期しても変わらない。
func TestOverwriteScenarioStatus(t *testing.T) {
	for _, tc := range overwriteOrders {
		t.Run(tc.name, func(t *testing.T) {
			remote, a, b, _ := overwriteTrio(t, scenarioStart.Add(time.Hour))
			setStatusAs(t, a, "hanako", apptest.SampleStatusInProgress)
			setStatusAs(t, b, "taro", apptest.SampleStatusDone)
			syncOrder(t, remote, tc.aFirst, a, b)
			wantStatus(t, a, apptest.SampleStatusDone)
			wantStatus(t, b, apptest.SampleStatusDone)
			wantOverwrite(t, a, "status", statusOverwritten, "taro")
			wantNoOverwrite(t, b)
			a.sync(t, remote)
			b.sync(t, remote)
			wantOverwrite(t, a, "status", statusOverwritten, "taro")
			wantNoOverwrite(t, b)
		})
	}
}

// A の値が送られたあと、C(hanako の別のマシン)が A の値を受け取り、そのあと同時の B の値(勝つ)を受け取る。
// C で作った値ではないので C には作らず(C がもう一度同期しても)、A にだけ 1 行(Review Focus 1)。
func TestOverwriteScenarioSameLoginOtherMachine(t *testing.T) {
	remote, a, b, c := overwriteTrio(t, scenarioStart.Add(time.Hour))
	setStatusAs(t, a, "hanako", apptest.SampleStatusInProgress)
	setStatusAs(t, b, "taro", apptest.SampleStatusDone)
	a.sync(t, remote)
	c.sync(t, remote)
	wantStatus(t, c, apptest.SampleStatusInProgress)
	b.sync(t, remote)
	c.sync(t, remote)
	wantStatus(t, c, apptest.SampleStatusDone)
	wantNoOverwrite(t, c)
	a.sync(t, remote)
	wantStatus(t, a, apptest.SampleStatusDone)
	wantOverwrite(t, a, "status", statusOverwritten, "taro")
	wantNoOverwrite(t, b)
	c.sync(t, remote)
	wantNoOverwrite(t, c)
}

// A の時計が B より進んでいて、A の値(InProgress)が勝つ。どちらが先に送っても、B に 1 行(差出人は hanako)、
// A には 0 行。もう一度同期しても変わらない(Review Focus 5)。
func TestOverwriteScenarioClockAhead(t *testing.T) {
	for _, tc := range overwriteOrders {
		t.Run(tc.name, func(t *testing.T) {
			remote, a, b, _ := overwriteTrio(t, scenarioStart.Add(-time.Hour))
			setStatusAs(t, a, "hanako", apptest.SampleStatusInProgress)
			setStatusAs(t, b, "taro", apptest.SampleStatusDone)
			syncOrder(t, remote, tc.aFirst, a, b)
			wantStatus(t, a, apptest.SampleStatusInProgress)
			wantStatus(t, b, apptest.SampleStatusInProgress)
			wantOverwrite(t, b, "status", statusOverwritten, "hanako")
			wantNoOverwrite(t, a)
			a.sync(t, remote)
			b.sync(t, remote)
			wantOverwrite(t, b, "status", statusOverwritten, "hanako")
			wantNoOverwrite(t, a)
		})
	}
}

// describeAs は、login の操作として、HH-2 の説明を text にする(本文と desc の行を同じ保存に書く)。
func describeAs(t *testing.T, m machine, login, text string) {
	t.Helper()
	m.writeAs(t, login, "説明を変更", func(tx port.Tx) error {
		if err := tx.WriteItems().Describe(apptest.SampleTask, text); err != nil {
			return err
		}
		return tx.WriteItems().Append(apptest.SampleTask, change.NewDesc())
	})
}

// A と B が、説明の 2 行目を同時に別の文に変える。どちらが先に送っても、両方の文が残り、A と B の受信箱に
// 「説明の同じ箇所が同時に編集されたため、両方を残しました」が 1 行ずつ(差出人はもう一方の人。Review Focus 4)。
func TestOverwriteScenarioDescription(t *testing.T) {
	for _, tc := range overwriteOrders {
		t.Run(tc.name, func(t *testing.T) {
			remote, a, b, _ := overwriteTrio(t, scenarioStart.Add(time.Hour))
			describeAs(t, a, "hanako", "1行目\n2行目\n3行目\n")
			a.sync(t, remote)
			b.sync(t, remote)
			describeAs(t, a, "hanako", "1行目\n2行目 (A)\n3行目\n")
			describeAs(t, b, "taro", "1行目\n2行目 (B)\n3行目\n")
			syncOrder(t, remote, tc.aFirst, a, b)
			for _, m := range []machine{a, b} {
				desc := files(t, m.dir)["items/"+string(apptest.SampleTask)+"/description.md"]
				if !strings.Contains(desc, "2行目 (A)") || !strings.Contains(desc, "2行目 (B)") {
					t.Fatalf("description on %s = %q, want both lines", m.dir, desc)
				}
			}
			wantOverwrite(t, a, "desc", descConflict, "taro")
			wantOverwrite(t, b, "desc", descConflict, "hanako")
			a.sync(t, remote)
			b.sync(t, remote)
			wantOverwrite(t, a, "desc", descConflict, "taro")
			wantOverwrite(t, b, "desc", descConflict, "hanako")
		})
	}
}
