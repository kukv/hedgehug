package gitstore

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/member"
)

var _ port.Replicator = (*Replicator)(nil)

// title は、サンプルのタスクのタイトルを読む。
func title(t *testing.T, m machine) string {
	t.Helper()
	var got string
	err := m.stores.Read(t.Context(), m.id, func(r port.Reader) error {
		it, err := r.Items().Get(apptest.SampleTask)
		got = it.Title
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func set(field string, from, to any) func(tx port.Tx) error {
	return func(tx port.Tx) error {
		return tx.WriteItems().Append(apptest.SampleTask, change.NewSet(field, from, to))
	}
}

func TestSyncFirstSendAndClone(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0)
	if got := title(t, b); got != "ログイン画面を作る" {
		t.Errorf("clone reads %q", got)
	}
	sameFiles(t, a, b)
	if res := b.sync(t, remote); res != (port.SyncResult{}) {
		t.Errorf("nothing to do, got %+v", res)
	}
}

func TestSyncCountsSaves(t *testing.T) {
	remote := newRemote(t)
	dir := t.TempDir() + "/a"
	if err := Create(dir, "01STORE0000000000000000001", t0); err != nil {
		t.Fatal(err)
	}
	a := newMachine(t, dir, "A", t0)
	if err := apptest.Sample(t.Context(), a.stores, a.id); err != nil {
		t.Fatal(err)
	}
	// Store を作った最初のコミットは保存ではないので数えない(この計画の決定 3)。
	if res := a.sync(t, remote); res != (port.SyncResult{Sent: 1}) {
		t.Errorf("first send = %+v, want Sent 1", res)
	}
	b := cloneMachine(t, remote, "B", t0)
	a.write(t, "1", set("title", "", "x"))
	a.write(t, "2", set("title", "", "y"))
	a.sync(t, remote)
	if res := b.sync(t, remote); res != (port.SyncResult{Received: 2}) {
		t.Errorf("fast-forward = %+v, want Received 2", res)
	}
}

func TestSyncKeepsBothFieldsAndConverges(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	a.write(t, "タイトル", set("title", "ログイン画面を作る", "ログイン"))
	b.write(t, "順位", set("rank", "a", "b"))
	a.sync(t, remote)
	// b は a の 1 件を受け取り、合わせて、自分の 1 件を送る。マージのコミットは数えない。
	if res := b.sync(t, remote); res != (port.SyncResult{Sent: 1, Received: 1}) {
		t.Errorf("b sync = %+v", res)
	}
	a.sync(t, remote)
	sameFiles(t, a, b)
	if got := title(t, a); got != "ログイン" {
		t.Errorf("title = %q", got)
	}
	y := files(t, a.dir)["items/"+string(apptest.SampleTask)+"/item.yaml"]
	if !strings.Contains(y, "rank: b\n") || !strings.Contains(y, "title: ログイン\n") {
		t.Errorf("item.yaml =\n%s", y)
	}
}

func TestSyncLaterSaveWinsWhicheverSyncsFirst(t *testing.T) {
	for _, bFirst := range []bool{false, true} {
		remote := newRemote(t)
		a := firstMachine(t, remote)
		b := cloneMachine(t, remote, "B", t0.Add(time.Hour)) // b の時計のほうが進んでいる
		a.write(t, "a", set("title", "", "A"))
		b.write(t, "b", set("title", "", "B"))
		first, second := a, b
		if bFirst {
			first, second = b, a
		}
		first.sync(t, remote)
		second.sync(t, remote)
		first.sync(t, remote)
		sameFiles(t, a, b)
		if got := title(t, a); got != "B" {
			t.Errorf("bFirst=%v: title = %q, want B", bFirst, got)
		}
	}
}

func TestSyncSeenChangeWinsOverClock(t *testing.T) {
	// a と b が同時に書き、c が両方を見てから書いた。c の時計が最も遅れていても c が勝つ(保存形式「テスト」)。
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(2*time.Hour))
	c := cloneMachine(t, remote, "C", t0.Add(-time.Hour))
	a.write(t, "a", set("title", "", "A"))
	b.write(t, "b", set("title", "", "B"))
	a.sync(t, remote)
	b.sync(t, remote)
	c.sync(t, remote)
	c.write(t, "c", set("title", "B", "C"))
	c.sync(t, remote)
	a.sync(t, remote)
	b.sync(t, remote)
	for _, m := range []machine{a, b, c} {
		if got := title(t, m); got != "C" {
			t.Errorf("%s: title = %q, want C", m.dir, got)
		}
	}
	sameFiles(t, a, b)
	sameFiles(t, a, c)
}

func TestSyncKeepsBothAddsOfMultiValue(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	a.write(t, "a", func(tx port.Tx) error { return tx.WriteMembers().Append(member.Join("jiro")) })
	b.write(t, "b", func(tx port.Tx) error { return tx.WriteMembers().Append(member.Join("saburo")) })
	a.sync(t, remote)
	b.sync(t, remote)
	a.sync(t, remote)
	sameFiles(t, a, b)
	var logins []string
	err := a.stores.Read(t.Context(), a.id, func(r port.Reader) error {
		ms, err := r.Members().List()
		for _, m := range ms {
			logins = append(logins, m.Login)
		}
		return err
	})
	if err != nil || strings.Join(logins, ",") != "hanako,jiro,saburo,taro" {
		t.Errorf("members = %v, %v", logins, err)
	}
}

func TestSyncRetriesWhenSendOverlaps(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	a.write(t, "a", set("title", "", "A"))
	once := false
	a.rep.beforeSend = func() error {
		if !once {
			once = true
			b.write(t, "b", set("rank", "a", "z"))
			b.sync(t, remote)
		}
		return nil
	}
	if res := a.sync(t, remote); res != (port.SyncResult{Sent: 1, Received: 1}) {
		t.Errorf("a sync = %+v, want Sent 1 and the overlapping save received", res)
	}
	b.sync(t, remote)
	sameFiles(t, a, b)
}

func TestSyncGivesUpAfterThreeOverlaps(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	a.write(t, "a", set("title", "", "A"))
	n := 0
	a.rep.beforeSend = func() error {
		n++
		b.write(t, "b", set("rank", "", string(rune('a'+n))))
		b.sync(t, remote)
		return nil
	}
	_, err := a.rep.Sync(t.Context(), a.id, remote)
	if !errors.Is(err, port.ErrSendConflict) || n != 3 {
		t.Errorf("err = %v after %d tries, want ErrSendConflict after 3", err, n)
	}
	// 受け取って合わせたものは残り、次の同期で続ける。
	a.rep.beforeSend = nil
	a.sync(t, remote)
	b.sync(t, remote)
	sameFiles(t, a, b)
}
