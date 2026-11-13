package apptest

import (
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

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
