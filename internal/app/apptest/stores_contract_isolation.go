package apptest

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/transfer"
)

// fixtureParts は、fixtureSeen が数える writeFixture の部分の数。
const fixtureParts = 8

// fixtureSeen は、writeFixture で書いたもののうち、読めた部分の数を返す。
// 見つからないときの port.ErrNotFound 以外のエラーは、まとめて返す。
func fixtureSeen(r port.Reader) (int, error) {
	n := 0
	var errs []error
	count := func(err error) {
		switch {
		case err == nil:
			n++
		case !errors.Is(err, port.ErrNotFound):
			errs = append(errs, err)
		}
	}
	_, err := r.Workspaces().Get(cWorkspace.ID)
	count(err)
	_, err = r.Projects().Get(cProject.ID)
	count(err)
	_, err = r.Items().Get(cItem.ID)
	count(err)
	for _, ref := range []change.Ref{
		{Kind: change.KindMembers},
		{Kind: change.KindWorkspace, ID: cWorkspace.ID},
		{Kind: change.KindProject, ID: cProject.ID},
		{Kind: change.KindItem, ID: cItem.ID},
	} {
		_, err = r.Changes().Of(ref)
		count(err)
	}
	ms, err := r.Members().List()
	if err != nil {
		errs = append(errs, err)
	} else if len(ms) > 0 {
		n++
	}
	return n, errors.Join(errs...)
}

// contractReadIsolated は、書いている途中の Write を、ほかの Read が途中まで見ないことを確かめる。
// Read は、Write が終わるまで待ってもよい。待たずに返すなら、途中の行を 1 つも見てはいけない。
func contractReadIsolated(t *testing.T, s port.Stores, id port.StoreID) {
	const timeout = 5 * time.Second
	started, release := make(chan struct{}), make(chan struct{})
	var released atomic.Bool
	var once sync.Once
	doRelease := func() { once.Do(func() { released.Store(true); close(release) }) }
	defer doRelease()

	wrote := make(chan error, 1)
	go func() {
		wrote <- s.Write(t.Context(), id, taro, func(tx port.Tx) error {
			if err := writeFixture(tx); err != nil {
				return err
			}
			close(started)
			<-release
			return nil
		})
	}()
	wait(t, started, timeout, "Write did not start")

	read := make(chan struct{})
	go func() {
		defer close(read)
		readDuringWrite(t, s, id, &released)
	}()
	select {
	case <-read: // 待たずに返した。中で何も見ていないことを確かめた。
	case <-time.After(100 * time.Millisecond): // Write が終わるまで待っている。
	}

	doRelease()
	select {
	case err := <-wrote:
		if err != nil {
			t.Fatalf("Write: %v", err)
		}
	case <-time.After(timeout):
		t.Fatal("Write did not finish after release")
	}
	wait(t, read, timeout, "Read did not finish after the write")
	mustRead(t, s, id, func(r port.Reader) error {
		if n, err := fixtureSeen(r); n != fixtureParts || err != nil {
			t.Errorf("after write: %d of %d parts seen, %v", n, fixtureParts, err)
		}
		return nil
	})
}

// readDuringWrite は、書いている途中に読む。見えるのは、全部か何もないかのどちらか。
// Write を放す前に読めたなら、何も見えてはいけない。
func readDuringWrite(t *testing.T, s port.Stores, id port.StoreID, released *atomic.Bool) {
	err := s.Read(t.Context(), id, func(r port.Reader) error {
		before := !released.Load()
		n, err := fixtureSeen(r)
		if err != nil || (n != 0 && n != fixtureParts) || (before && n != 0) {
			t.Errorf("read during write: %d of %d parts seen (before release: %v), %v", n, fixtureParts, before, err)
		}
		return nil
	})
	if err != nil {
		t.Errorf("Read during write: %v", err)
	}
}

func wait(t *testing.T, ch <-chan struct{}, timeout time.Duration, msg string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(timeout):
		t.Fatal(msg)
	}
}

// 転送記録は、同じ操作の行と同じ op・at を持ち、同じ Tx の中から読める。失敗した Write は残さない。
func contractTransfers(t *testing.T, s port.Stores, id port.StoreID) {
	mustWrite(t, s, id, taro, writeFixture)
	rec := transfer.Record{
		Entity: transfer.EntityItem, Target: cItem.ID, Old: "HH-1", New: "MB-3",
		ToStore: "01OTHERSTORE", ToStoreName: "team", ToRemote: "https://github.com/org/team",
	}
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		if err := tx.WriteTransfers().Append(rec); err != nil {
			return err
		}
		if got, err := tx.Transfers().List(); err != nil || len(got) != 1 {
			t.Errorf("in the tx: %+v, %v", got, err)
		}
		return tx.WriteItems().Append(cItem.ID, change.NewSet("title", "認証", "移動した"))
	})
	boom := errors.New("boom")
	if err := s.Write(t.Context(), id, taro, func(tx port.Tx) error {
		return errors.Join(tx.WriteTransfers().Append(rec), boom)
	}); !errors.Is(err, boom) {
		t.Fatalf("failing write = %v", err)
	}
	mustRead(t, s, id, func(r port.Reader) error { checkTransferStamped(t, r, rec); return nil })
	checkTransferRefused(t, s, id, rec)
}

// checkTransferStamped は、保存した 1 行が、最後の操作の op・at と実行者で埋まり、ほかは書いたとおりであることを確かめる。
func checkTransferStamped(t *testing.T, r port.Reader, rec transfer.Record) {
	got, err := r.Transfers().List()
	log, _ := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: cItem.ID})
	if err != nil || len(got) != 1 || len(log) < 2 {
		t.Fatalf("transfers = %+v (%v), log = %d rows", got, err, len(log))
	}
	row := log[len(log)-1]
	g := got[0]
	if g.ID == "" || g.Op != row.Op || !g.At.Equal(row.At) || g.By != "taro" {
		t.Errorf("stamped = %+v, want op %s at %v by taro", g, row.Op, row.At)
	}
	g.ID, g.Op, g.At, g.By = "", "", rec.At, ""
	if g != rec {
		t.Errorf("record = %+v, want %+v", g, rec)
	}
}

// checkTransferRefused は、転送記録だけを書く Write も Summary が要り、使えない行は書けないことを確かめる。
func checkTransferRefused(t *testing.T, s port.Stores, id port.StoreID, rec transfer.Record) {
	noSummary := port.Operation{Actor: change.User("taro")}
	if err := s.Write(t.Context(), id, noSummary, func(tx port.Tx) error { return tx.WriteTransfers().Append(rec) }); !errors.Is(err, port.ErrInvalidOperation) {
		t.Errorf("no summary = %v, want ErrInvalidOperation", err)
	}
	for _, bad := range []transfer.Record{{Entity: "sprint", Target: cItem.ID, ToStore: "S"}, {Entity: "item", Target: "bad id", ToStore: "S"}, {Entity: "item", Target: cItem.ID}} {
		if err := s.Write(t.Context(), id, taro, func(tx port.Tx) error { return tx.WriteTransfers().Append(bad) }); !errors.Is(err, port.ErrInvalidOperation) {
			t.Errorf("Append(%+v) = %v, want ErrInvalidOperation", bad, err)
		}
	}
}

// 時刻を渡した転送記録は、その時刻のまま残る(移動先の moved_in と同じ時刻にそろえるため)。
func contractTransferAt(t *testing.T, s port.Stores, id port.StoreID) {
	mustWrite(t, s, id, taro, writeFixture)
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	rec := transfer.Record{Entity: transfer.EntityItem, Target: cItem.ID, Old: "HH-1", New: "MB-3", ToStore: "01OTHERSTORE", At: at}
	mustWrite(t, s, id, taro, func(tx port.Tx) error { return tx.WriteTransfers().Append(rec) })
	mustRead(t, s, id, func(r port.Reader) error {
		if got, err := r.Transfers().List(); err != nil || len(got) != 1 || !got[0].At.Equal(at) {
			t.Errorf("transfers = %+v, %v, want at %v", got, err, at)
		}
		return nil
	})
}

func contractPurgedEmpty(t *testing.T, s port.Stores, id port.StoreID) {
	mustWrite(t, s, id, taro, writeFixture)
	mustRead(t, s, id, func(r port.Reader) error {
		if got, err := r.Purged().List(); err != nil || len(got) != 0 {
			t.Errorf("Purged = %+v, %v", got, err)
		}
		return nil
	})
}

// contractRestored は、削除の行を取り消す restore の行(f が同じで、to が削除の op)を足すと、
// アイテムの Deleted の印が外れることを確かめる(C3a-7)。
func contractRestored(t *testing.T, s port.Stores, id port.StoreID) {
	mustWrite(t, s, id, taro, writeFixture)
	if err := DeleteItems(t.Context(), s, id, cItem.ID); err != nil {
		t.Fatal(err)
	}
	var op change.ID
	mustRead(t, s, id, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: cItem.ID})
		if err != nil || log[len(log)-1].T != change.Delete {
			t.Fatalf("log = %+v, %v", log, err)
		}
		op = log[len(log)-1].Op
		if got, gerr := r.Items().Get(cItem.ID); gerr != nil || got.Deleted == nil {
			t.Fatalf("deleted item = %+v, %v", got.Deleted, gerr)
		}
		return nil
	})
	mustWrite(t, s, id, port.Operation{Actor: change.Actor{Kind: change.ActorSync}, Summary: "復元"}, func(tx port.Tx) error {
		return tx.WriteItems().Append(cItem.ID, change.NewRestore(change.DeletedField, op))
	})
	mustRead(t, s, id, func(r port.Reader) error {
		if got, err := r.Items().Get(cItem.ID); err != nil || got.Deleted != nil {
			t.Errorf("restored item Deleted = %+v, %v", got.Deleted, err)
		}
		return nil
	})
}

// wantDeletedWorkspace は、削除済みのワークスペースも Get と List が Deleted 付きで返し、Settings も読めることを確かめる(B5b)。
func wantDeletedWorkspace(t *testing.T, r port.Reader) {
	t.Helper()
	if w, err := r.Workspaces().Get(cWorkspace.ID); err != nil || w.Deleted == nil || !w.Deleted.Root || w.Name != cWorkspace.Name {
		t.Errorf("Workspaces().Get(deleted) = %+v, %v", w, err)
	}
	if ws, err := r.Workspaces().List(); err != nil || len(ws) != 1 || ws[0].Deleted == nil {
		t.Errorf("Workspaces().List must return the deleted workspace: %+v, %v", ws, err)
	}
	if _, err := r.Workspaces().Settings(cWorkspace.ID); err != nil {
		t.Errorf("Workspaces().Settings(deleted) = %v", err)
	}
}
