package gitstore

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/repair"
)

var syncOp = port.Operation{Actor: change.Actor{Kind: change.ActorSync}, Summary: "修復"}

// diverged は、A と B が同じタイトルを別々に変えた状態を作る(A は送ってある)。B が同期すると合わせる。
func diverged(t *testing.T) (remote port.Remote, b machine) {
	t.Helper()
	remote = newRemote(t)
	a := firstMachine(t, remote)
	b = cloneMachine(t, remote, "B", t0)
	a.write(t, "A の変更", set("title", "x", "A"))
	b.write(t, "B の変更", set("title", "x", "B"))
	a.sync(t, remote)
	return remote, b
}

// titleToR は、タイトルが R でなければ R にする修復(R なら何も書かない)。calls に呼ばれた回数を数える。
func titleToR(calls *int) port.RepairFunc {
	return func(_ context.Context, w port.RepairWriter) error {
		*calls++
		var got string
		if err := w.Read(func(r port.Reader) error {
			it, err := r.Items().Get(apptest.SampleTask)
			got = it.Title
			return err
		}); err != nil {
			return err
		}
		if got == "R" {
			return nil
		}
		return w.Write(syncOp, set("title", got, "R"))
	}
}

func TestRepairRunsBeforeSendAndIsPushed(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0)
	a.write(t, "A の変更", set("title", "x", "A"))
	b.write(t, "B の変更", set("title", "x", "B"))
	var facts repair.Facts
	b.rep.Repair = func(_ context.Context, w port.RepairWriter) error {
		facts = w.Facts()
		return w.Write(syncOp, func(tx port.Tx) error {
			return tx.WriteItems().Append(apptest.SampleTask, change.NewSet("title", "?", "R"))
		})
	}
	a.sync(t, remote)
	res := b.sync(t, remote) // b は合わせて、修復を書いて送る
	if len(facts.Ours) == 0 || len(facts.Theirs) == 0 {
		t.Fatalf("facts on merge = %+v", facts)
	}
	// 自分の側は B が書いた行だけ、remote の側は A が書いた行だけ(letterIDs の文字で分かる)。
	for id := range facts.Ours {
		if !strings.HasPrefix(string(id), "01B") || facts.Side(id) != 1 {
			t.Errorf("ours has %s", id)
		}
	}
	for id := range facts.Theirs {
		if !strings.HasPrefix(string(id), "01A") || facts.Side(id) != 2 {
			t.Errorf("theirs has %s", id)
		}
	}
	// 送った数には修復の保存も入る(決定 C3a-19): B の変更と修復。
	if res.Sent != 2 {
		t.Errorf("sent = %d, want 2", res.Sent)
	}
	a.sync(t, port.Remote{URL: remote.URL}) // a は早送りで受け取る
	sameFiles(t, a, b)
	if got := title(t, a); got != "R" {
		t.Fatalf("title = %q", got)
	}
}

func TestRepairSkippedWithoutMerge(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0)
	calls := 0
	b.rep.Repair = titleToR(&calls)
	a.write(t, "A の変更", set("title", "x", "A"))
	a.sync(t, remote)
	b.sync(t, remote) // 早送りで受け取る
	b.sync(t, remote) // 受け取るものがない
	b.write(t, "B の変更", set("title", "A", "B"))
	b.sync(t, remote) // 送るだけ
	if calls != 0 {
		t.Fatalf("repair called %d times without a merge", calls)
	}
}

func TestRepairNotCalledWhenReceiveOnly(t *testing.T) {
	remote, b := diverged(t)
	calls := 0
	b.rep.Repair = titleToR(&calls)
	if _, err := b.rep.Sync(t.Context(), b.id, port.Remote{URL: remote.URL, ReceiveOnly: true}); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("repair called %d times on a receive-only store", calls)
	}
}

func TestRepairWriteDoesNotDeadlock(t *testing.T) {
	remote, b := diverged(t)
	calls := 0
	b.rep.Repair = titleToR(&calls)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := b.rep.Sync(t.Context(), b.id, remote)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("sync with a repair did not finish in 5 seconds")
	}
	if calls != 1 || title(t, b) != "R" {
		t.Fatalf("calls = %d, title = %q", calls, title(t, b))
	}
}

func TestRepairOnRetryWritesOnce(t *testing.T) {
	remote, b := diverged(t)
	calls := 0
	b.rep.Repair = titleToR(&calls)
	sends := 0
	b.rep.beforeSend = func() error {
		sends++
		if sends == 1 {
			return errRejected
		}
		return nil
	}
	b.sync(t, remote)
	if calls != 2 {
		t.Errorf("repair called %d times, want 2 (once per try)", calls)
	}
	var log []change.Record
	err := b.stores.Read(t.Context(), b.id, func(r port.Reader) error {
		var err error
		log, err = r.Changes().Of(change.Ref{Kind: change.KindItem, ID: apptest.SampleTask})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, r := range log {
		if r.T == change.Set && r.F == "title" && string(r.To) == `"R"` {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d repair rows, want 1", n)
	}
}

// 送信が重なったとき(remote が動いた)、2 回目の試みは修復のコミットの上にもう一度合わせ、2 つの合わせの
// 行をまとめた Facts で修復を呼ぶ。
func TestRepairAfterRemoteMovedCoversBothMerges(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0)
	a.write(t, "A の変更", set("title", "x", "A"))
	b.write(t, "B の変更", set("title", "x", "B"))
	a.sync(t, remote)
	calls := 0
	repairTitle := titleToR(&calls)
	var seen []repair.Facts
	b.rep.Repair = func(ctx context.Context, w port.RepairWriter) error {
		seen = append(seen, w.Facts())
		return repairTitle(ctx, w)
	}
	sends := 0
	b.rep.beforeSend = func() error {
		sends++
		if sends == 1 { // B が送る前に、A がもう一度変えて送る。B の送信は拒否される
			a.write(t, "A の変更 2", set("title", "A", "A2"))
			a.sync(t, remote)
		}
		return nil
	}
	b.sync(t, remote)
	if len(seen) != 2 {
		t.Fatalf("repair called %d times, want 2", len(seen))
	}
	for id := range seen[0].Ours {
		if seen[1].Side(id) != 1 {
			t.Errorf("second facts lost our row %s", id)
		}
	}
	for id := range seen[0].Theirs {
		if seen[1].Side(id) != 2 {
			t.Errorf("second facts lost their row %s", id)
		}
	}
	if len(seen[1].Theirs) <= len(seen[0].Theirs) || len(seen[1].Ours) <= len(seen[0].Ours) {
		t.Errorf("second facts did not grow:\n%+v\n%+v", seen[0], seen[1])
	}
	a.sync(t, remote)
	sameFiles(t, a, b)
	if got := title(t, a); got != "R" {
		t.Fatalf("title = %q", got)
	}
}

// remote が空(theirs がゼロ)なら、手元の履歴のすべてが送っていないものなので、前に別の remote へ送った
// 合わせでも修復を呼ぶ(修復は 2 回目に何も書かない)。送ったあとの同期では呼ばない。
func TestRepairOnEmptyRemoteRunsOnceThenStops(t *testing.T) {
	remote, b := diverged(t)
	calls := 0
	b.rep.Repair = titleToR(&calls)
	b.sync(t, remote) // 合わせて修復し、送る
	head := headCommit(t, b.dir).Hash
	empty := newRemote(t)
	if res := b.sync(t, empty); res.Sent == 0 {
		t.Fatalf("nothing sent to the empty remote: %+v", res)
	}
	if calls != 2 {
		t.Fatalf("repair called %d times, want 2 (the merge is unsent to the empty remote)", calls)
	}
	if got := headCommit(t, b.dir).Hash; got != head {
		t.Errorf("repair wrote again on the empty remote: %s -> %s", head, got)
	}
	b.sync(t, empty)
	if calls != 2 {
		t.Fatalf("repair called %d times after the merge was sent, want 2", calls)
	}
}

func TestRepairErrorStopsSync(t *testing.T) {
	remote, b := diverged(t)
	before := remoteHead(t, remote)
	boom := errors.New("boom")
	b.rep.Repair = func(context.Context, port.RepairWriter) error { return boom }
	if _, err := b.rep.Sync(t.Context(), b.id, remote); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	if after := remoteHead(t, remote); after != before {
		t.Fatalf("remote moved: %s -> %s", before, after)
	}
}

func TestRepairRetriedAfterFailure(t *testing.T) {
	remote, b := diverged(t)
	var seen []repair.Facts
	b.rep.Repair = func(_ context.Context, w port.RepairWriter) error {
		seen = append(seen, w.Facts())
		if len(seen) == 1 {
			return errors.New("boom")
		}
		return nil
	}
	if _, err := b.rep.Sync(t.Context(), b.id, remote); err == nil {
		t.Fatal("first sync did not fail")
	}
	b.sync(t, remote) // remote は変わっていない。合わせのコミットはもう先端にある
	if len(seen) != 2 {
		t.Fatalf("repair called %d times, want 2", len(seen))
	}
	first, second := seen[0], seen[1]
	if len(first.Ours) == 0 || len(first.Theirs) == 0 {
		t.Fatalf("first facts = %+v", first)
	}
	if !sameSet(first.Ours, second.Ours) || !sameSet(first.Theirs, second.Theirs) {
		t.Fatalf("facts changed:\n%+v\n%+v", first, second)
	}
}

func sameSet(a, b map[change.ID]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for id := range a {
		if !b[id] {
			return false
		}
	}
	return true
}
