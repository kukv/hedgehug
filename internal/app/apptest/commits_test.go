package apptest_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/workspace"
)

func newCommitLog() (*apptest.Stores, *apptest.CommitLog) {
	clock := apptest.NewClock(time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC))
	s := apptest.NewStores(clock, apptest.NewIDs())
	s.AddStore("S1")
	return s, apptest.NewCommitLog(s, clock)
}

func TestFakeCommitLog(t *testing.T) {
	apptest.RunStoreCommitsContract(t, func(t *testing.T) (port.Stores, port.StoreCommits, port.StoreID) {
		_, c := newCommitLog()
		return c, c, "S1"
	})
}

// retentionStores は、内側の Write のあとで保持日数を変える(Write の中からは、偽物が同じロックを取るので変えられない)。
type retentionStores struct {
	port.Stores
	base *apptest.Stores
	days int
}

func (r retentionStores) Write(ctx context.Context, id port.StoreID, op port.Operation, fn func(tx port.Tx) error) error {
	if err := r.Stores.Write(ctx, id, op, fn); err != nil {
		return err
	}
	r.base.SetRetentionDays(id, r.days)
	return nil
}

// 保持日数を変えた Write だけが StoreLog のコミットになり、前の Write で変えた分は関係のない保存に乗らない。
func TestCommitLogStoreLog(t *testing.T) {
	s, _ := newCommitLog()
	clock := apptest.NewClock(time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC))
	c := apptest.NewCommitLog(retentionStores{Stores: s, base: s, days: 7}, clock)
	op := port.Operation{Actor: change.User("taro"), Summary: "メンバーを追加"}
	join := func(tx port.Tx) error { return tx.WriteMembers().Append(member.Init(), member.Join("taro")) }
	if err := c.Write(t.Context(), "S1", op, join); err != nil {
		t.Fatal(err)
	}
	s.SetRetentionDays("S1", 14) // Write の外での変更は、次の Write に乗らない
	other := apptest.NewCommitLog(s, clock)
	if err := other.Write(t.Context(), "S1", op, func(tx port.Tx) error { return tx.WriteMembers().Append(member.Join("hanako")) }); err != nil {
		t.Fatal(err)
	}
	got, err := other.Commits(t.Context(), "S1")
	if err != nil || len(got) != 1 || got[0].StoreLog {
		t.Fatalf("outside change: %+v, %v", got, err)
	}
	got, err = c.Commits(t.Context(), "S1")
	if err != nil || len(got) != 1 || !got[0].StoreLog {
		t.Fatalf("Commits = %+v, %v; want one StoreLog commit", got, err)
	}
}

// 複数のエンティティに行を足した保存の Op は、いつも整列済みの先頭の行の op になる。members の行にだけ別の op を
// 持たせ(同期の操作だけが書ける)、整列前の順に依存すれば落ちるようにする。
func TestCommitLogOpIsStable(t *testing.T) {
	for range 20 {
		_, c := newCommitLog()
		op := port.Operation{Actor: change.Actor{Kind: change.ActorSync}, Summary: "同期"}
		const other change.ID = "01OPSTABLEOP00000000000001"
		if err := c.Write(t.Context(), "S1", op, func(tx port.Tx) error {
			d, err := workspace.Create(workspace.Workspace{ID: "01OPSTABLEWS0000000000001", Name: "Web", Prefix: "HH"})
			if err != nil {
				return err
			}
			return errors.Join(tx.WriteMembers().Append(member.Init().InOp(other)), tx.WriteWorkspaces().Append("01OPSTABLEWS0000000000001", d))
		}); err != nil {
			t.Fatal(err)
		}
		got, err := c.Commits(t.Context(), "S1")
		if err != nil || len(got) != 1 || len(got[0].Changes) != 2 || got[0].Changes[0].Ref.Kind != change.KindMembers || got[0].Op != other {
			t.Fatalf("Commits = %+v, %v", got, err)
		}
	}
}
