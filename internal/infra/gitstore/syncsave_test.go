package gitstore

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/item"
)

const movedItem item.ID = "01MOVEDIT0000000000000001"

var saveOp = port.Operation{Actor: change.User("taro"), Summary: "移動"}

// createMoved は、移動先に足すアイテムのフォルダを作る(移動先の保存の代わり)。
func createMoved(tx port.Tx) error {
	return tx.WriteItems().Append(movedItem, item.Create(item.Item{
		ID: movedItem, Place: item.Place{Workspace: "01SAMPLEWS0000000000000001", Number: 9}, Kind: item.Epic, Rank: "a", Title: "移ってきた", Watchers: []string{},
	}))
}

// hasMoved は、移したアイテムのフォルダが作業ツリーにあるか。
func hasMoved(t *testing.T, m machine) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(m.dir, "items", string(movedItem)))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return err == nil
}

// 受け取って、保存して、送る。受け取ったものは保存の前に合わさっている。
func TestSyncSaveReceivesSavesAndSends(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	b.write(t, "b", set("rank", "a", "z"))
	b.sync(t, remote)

	res, err := a.rep.SyncSave(t.Context(), a.id, remote, saveOp, createMoved)
	if err != nil || res.Sent != 1 || res.Received != 1 || res.Seen == "" {
		t.Fatalf("SyncSave = %+v, %v, want Sent 1, Received 1", res, err)
	}
	b.sync(t, remote)
	sameFiles(t, a, b)
	if _, ok := files(t, b.dir)["items/"+string(movedItem)+"/log.jsonl"]; !ok {
		t.Error("the other machine did not get the saved item")
	}
}

// (1) の受け取りが断られたら、保存しない。
func TestSyncSaveDoesNotSaveWhenReceiveFails(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	before := snap(t, a)
	called := 0
	_, err := a.rep.SyncSave(t.Context(), a.id, port.Remote{URL: "https://remote.invalid/missing.git"}, saveOp, func(tx port.Tx) error {
		called++
		return createMoved(tx)
	})
	var se *port.SaveError
	if err == nil || errors.As(err, &se) || called != 0 {
		t.Errorf("err = %v, fn called %d times, want a receive failure and no call", err, called)
	}
	unchanged(t, a, before)
}

// Review Focus 1: 送信が断られたら、移動先に、移したアイテムのフォルダもコミットも残らない。受け取ったものは残る。
func TestSyncSaveRefusedSendLeavesOnlyWhatWasReceived(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	b.write(t, "b", set("rank", "a", "z"))
	b.sync(t, remote)
	a.rep.beforeSend = func() error { return port.ErrWriteDenied }

	_, err := a.rep.SyncSave(t.Context(), a.id, remote, saveOp, createMoved)
	var se *port.SaveError
	if !errors.Is(err, port.ErrWriteDenied) || errors.As(err, &se) {
		t.Fatalf("SyncSave = %v, want ErrWriteDenied (not a SaveError)", err)
	}
	if _, ok := files(t, a.dir)["items/"+string(movedItem)+"/log.jsonl"]; ok || hasMoved(t, a) {
		t.Error("the saved item's folder is still there")
	}
	// 先端は、受け取って合わせた結果(b の保存)で、保存のコミットはない。ファイルも作業ツリーもそれと同じ。
	sameFiles(t, a, b)
	if _, err := os.Stat(markerPath(a.dir)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the marker is left: %v", err)
	}
}

// 送信が重なったら、取り込み直してから fn を呼び直す。fn は新しい状態を読む。
func TestSyncSaveRunsFnAgainAfterAnOverlap(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	once := false
	a.rep.beforeSend = func() error {
		if !once {
			once = true
			b.write(t, "b", set("rank", "a", "z"))
			b.sync(t, remote)
		}
		return nil
	}
	var seen []string
	_, err := a.rep.SyncSave(t.Context(), a.id, remote, saveOp, func(tx port.Tx) error {
		it, err := tx.Items().Get("01SAMPLEIT0000000000000002")
		seen = append(seen, it.Rank)
		return errors.Join(err, createMoved(tx))
	})
	if err != nil || !reflect.DeepEqual(seen, []string{"V", "z"}) { // サンプルの順位は rank.First
		t.Fatalf("err = %v, fn saw ranks %v, want [V z] (2 回目は受け取った変更を読む)", err, seen)
	}
	b.sync(t, remote)
	sameFiles(t, a, b)
}

func TestSyncSaveGivesUpAfterThreeOverlaps(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	n := 0
	a.rep.beforeSend = func() error {
		n++
		b.write(t, "b", set("rank", "", string(rune('a'+n))))
		b.sync(t, remote)
		return nil
	}
	calls := 0
	_, err := a.rep.SyncSave(t.Context(), a.id, remote, saveOp, func(tx port.Tx) error { calls++; return createMoved(tx) })
	if !errors.Is(err, port.ErrSendConflict) || n != 3 || calls != 3 {
		t.Errorf("err = %v after %d sends, fn called %d times, want ErrSendConflict, 3, 3", err, n, calls)
	}
	a.rep.beforeSend = nil
	a.sync(t, remote)
	b.sync(t, remote)
	sameFiles(t, a, b)
	if _, ok := files(t, a.dir)["items/"+string(movedItem)+"/log.jsonl"]; ok || hasMoved(t, a) {
		t.Error("an abandoned save came back")
	}
}

func TestSyncSaveWrapsFnErrorsAndSendsNothing(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	boom := errors.New("boom")
	_, err := a.rep.SyncSave(t.Context(), a.id, remote, saveOp, func(tx port.Tx) error { return boom })
	var se *port.SaveError
	if !errors.As(err, &se) || !errors.Is(err, boom) {
		t.Errorf("err = %v, want a SaveError around boom", err)
	}
	if res := b.sync(t, remote); res != (port.SyncResult{}) {
		t.Errorf("b received %+v, want nothing", res)
	}
}

// Review Focus 2: ロックは Store ごと。SyncSave の fn のあいだ(Store A のロックを持っている)、別の Store B には書け、
// A への書き込みは fn が終わるまで待たされる。fn が別の Store を読んで待つ実装が、向きの逆の 2 つの移動で止まらないことの土台。
func TestSyncSaveHoldsOnlyTheLockOfItsOwnStore(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	dirB := filepath.Join(t.TempDir(), "b")
	if err := Create(dirB, "01STORE0000000000000000002", t0); err != nil {
		t.Fatal(err)
	}
	idB, err := a.stores.Attach(dirB)
	if err != nil {
		t.Fatal(err)
	}
	writeA := func() error {
		return a.stores.Write(t.Context(), a.id, saveOp, func(tx port.Tx) error {
			return tx.WriteItems().Append("01SAMPLEIT0000000000000002", change.NewSet("title", nil, "別の書き込み"))
		})
	}
	blocked := make(chan error, 1)
	var otherStore error
	_, err = a.rep.SyncSave(t.Context(), a.id, remote, saveOp, func(tx port.Tx) error {
		go func() { blocked <- writeA() }()
		// 別の Store(B)には、いま書ける。
		otherStore = a.stores.Write(t.Context(), idB, saveOp, createMoved)
		select {
		case err := <-blocked:
			t.Errorf("fn の中で A に書けた(%v)。SyncSave が A のロックを持っていない", err)
		case <-time.After(300 * time.Millisecond):
		}
		return createMoved(tx)
	})
	if err != nil || otherStore != nil {
		t.Fatalf("SyncSave = %v, 別の Store への書き込み = %v", err, otherStore)
	}
	if err := <-blocked; err != nil { // fn が終わったあとに通る
		t.Errorf("fn のあとの A への書き込み = %v", err)
	}
}

func TestSyncSaveRefusesReceiveOnly(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	remote.ReceiveOnly = true
	before := snap(t, a)
	if _, err := a.rep.SyncSave(t.Context(), a.id, remote, saveOp, createMoved); !errors.Is(err, port.ErrInvalidOperation) {
		t.Errorf("err = %v, want ErrInvalidOperation", err)
	}
	unchanged(t, a, before)
}

// 受け取りで remote の書き換えを見つけたら、保存せず、受け取った位置の記録を戻す(未送信の数が動かない)。
func TestSyncSaveStopsWhenTheRemoteWasRewritten(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	old := remoteHead(t, remote)
	rename(t, a, "新しい名前")
	res, err := a.rep.Sync(t.Context(), a.id, remote)
	if err != nil {
		t.Fatal(err)
	}
	setRemoteBranch(t, remote, old)
	rename(t, a, "未送信の名前")
	before, unsent := snap(t, a), unsentOf(t, a)
	called := false
	_, err = a.rep.SyncSave(t.Context(), a.id, port.Remote{URL: remote.URL, Seen: res.Seen}, saveOp, func(tx port.Tx) error {
		called = true
		return createMoved(tx)
	})
	if !errors.Is(err, port.ErrRemoteRewritten) || called {
		t.Errorf("SyncSave = %v, fn called %v, want ErrRemoteRewritten and no call", err, called)
	}
	unchanged(t, a, before)
	if n := unsentOf(t, a); n != unsent {
		t.Errorf("Unsent = %d; want %d", n, unsent)
	}
}

// 送れたあとに remote の位置を記録できなくても、送れた保存は捨てずに成功とする。
func TestSyncSaveKeepsASentSaveWhenTheRemoteRefCannotBeRecorded(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	head := headRefName(t, a)
	ref := filepath.Join(a.dir, ".git", filepath.FromSlash(remotePrefix+head))
	a.rep.beforeSend = func() error {
		// remote の位置のファイルをフォルダに替えて、書けなくする。
		return errors.Join(os.RemoveAll(ref), os.MkdirAll(filepath.Join(ref, "x"), 0o700))
	}
	if _, err := a.rep.SyncSave(t.Context(), a.id, remote, saveOp, createMoved); err != nil {
		t.Fatalf("SyncSave = %v, want success", err)
	}
	if !hasMoved(t, a) {
		t.Error("the sent save was discarded")
	}
	b.sync(t, remote)
	if !hasMoved(t, b) {
		t.Error("the other machine did not get the sent save")
	}
}

// 送れなかった保存を捨てられなかったら、やり直さず、ErrSendConflict と目印を返す。保存は残り、次の同期で送る。
func TestSyncSaveStopsWhenTheSaveCannotBeDiscarded(t *testing.T) {
	remote := newRemote(t)
	a := firstMachine(t, remote)
	b := cloneMachine(t, remote, "B", t0.Add(time.Hour))
	sends := 0
	a.rep.beforeSend = func() error {
		sends++
		// 保存の途中の印をフォルダにして、捨てる処理(guarded)が印を書けないようにする。
		return errors.Join(errRejected, os.Mkdir(markerPath(a.dir), 0o700))
	}
	calls := 0
	_, err := a.rep.SyncSave(t.Context(), a.id, remote, saveOp, func(tx port.Tx) error { calls++; return createMoved(tx) })
	if !errors.Is(err, port.ErrSendConflict) || !errors.Is(err, port.ErrSaveNotDiscarded) || sends != 1 || calls != 1 {
		t.Fatalf("err = %v after %d sends and %d calls, want ErrSendConflict and ErrSaveNotDiscarded, 1, 1", err, sends, calls)
	}
	if !hasMoved(t, a) {
		t.Error("the save is gone though it could not be discarded")
	}
	a.rep.beforeSend = nil
	a.sync(t, remote) // 印(空のフォルダ)を戻してから、残った保存を送る
	b.sync(t, remote)
	sameFiles(t, a, b)
	if !hasMoved(t, b) {
		t.Error("the save left behind was not sent by the next sync")
	}
}

// headRefName は、m の先端のブランチの短い名前。
func headRefName(t *testing.T, m machine) string {
	t.Helper()
	repo, err := git.PlainOpen(m.dir)
	if err != nil {
		t.Fatal(err)
	}
	h, err := repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	return h.Name().Short()
}
