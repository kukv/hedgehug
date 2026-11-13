package apptest

import (
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
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

// contractUserRestoreSeenInTx は、利用者の操作の中で書いた restore の行を、同じ Write の中の読み取りが見ることを
// 確かめる。ReviveItem を 2 回呼んでも、restore の行は 1 つだけ書く(C3a-screen)。
func contractUserRestoreSeenInTx(t *testing.T, s port.Stores, id port.StoreID) {
	mustWrite(t, s, id, taro, writeFixture)
	if err := DeleteItems(t.Context(), s, id, cItem.ID); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		ctx := port.WithRevived(t.Context())
		if err := port.ReviveItem(ctx, tx, cItem.ID); err != nil {
			return err
		}
		if got, err := tx.Items().Get(cItem.ID); err != nil || got.Deleted != nil {
			t.Errorf("in the same write: Deleted = %+v, %v", got.Deleted, err)
		}
		return port.ReviveItem(ctx, tx, cItem.ID)
	})
	mustRead(t, s, id, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindItem, ID: cItem.ID})
		n := 0
		for _, rec := range log {
			if rec.T == change.Restore {
				n++
			}
		}
		if err != nil || n != 1 {
			t.Errorf("restore rows = %d, %v", n, err)
		}
		return nil
	})
}

// contractUserSettingRestoreSeenInTx は、利用者の操作の中で書いた設定の restore の行を、同じ Write の中の
// 設定の読み取りが見ることを確かめる。ReviveSetting を 2 回呼んでも、restore の行は 1 つだけ書く(C3c)。
func contractUserSettingRestoreSeenInTx(t *testing.T, s port.Stores, id port.StoreID) {
	review := workspace.Value{ID: "01CONTRACTRV00000000000001", Name: "Review", Color: "blue", Category: workspace.CategoryDoing, Rank: "a"}
	f := workspace.ParseList("statuses").Ref(review.ID)
	mustWrite(t, s, id, taro, writeFixture)
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(cWorkspace.ID, workspace.ValueDrafts(workspace.ParseList("statuses"), review)...)
	})
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		return tx.WriteWorkspaces().Append(cWorkspace.ID, change.NewDelete(f))
	})
	mustWrite(t, s, id, taro, func(tx port.Tx) error {
		reviveSettingTwice(t, tx, f, review.ID)
		return nil
	})
	mustRead(t, s, id, func(r port.Reader) error {
		log, err := r.Changes().Of(change.Ref{Kind: change.KindWorkspace, ID: cWorkspace.ID})
		n := 0
		for _, rec := range log {
			if rec.T == change.Restore {
				n++
			}
		}
		if err != nil || n != 1 {
			t.Errorf("restore rows = %d, %v", n, err)
		}
		return nil
	})
}

// reviveSettingTwice は、同じ tx の中で f に ReviveSetting を 2 回呼ぶ。1 回目は true で、そのあとの設定の読み取りに
// 値 v があり、2 回目は false であることを確かめる。
func reviveSettingTwice(t *testing.T, tx port.Tx, f string, v change.ID) {
	t.Helper()
	ctx := port.WithRevived(t.Context())
	if ok, err := port.ReviveSetting(ctx, tx, cWorkspace.ID, f); err != nil || !ok {
		t.Errorf("first ReviveSetting = %v, %v", ok, err)
	}
	st, err := tx.Workspaces().Settings(cWorkspace.ID)
	if err != nil || !slices.ContainsFunc(st.Statuses, func(x workspace.Value) bool { return x.ID == v }) {
		t.Errorf("in the same write: statuses = %+v, %v", st.Statuses, err)
	}
	if ok, err := port.ReviveSetting(ctx, tx, cWorkspace.ID, f); err != nil || ok {
		t.Errorf("second ReviveSetting = %v, %v", ok, err)
	}
}
