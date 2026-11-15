package apptest_test

import (
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/member"
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

// 保持日数の変更は、行がなくても Store 全体の記録(StoreLog)を変えた保存として 1 件になる。
func TestCommitLogStoreLog(t *testing.T) {
	s, c := newCommitLog()
	op := port.Operation{Actor: change.User("taro"), Summary: "メンバーを追加"}
	if err := c.Write(t.Context(), "S1", op, func(tx port.Tx) error {
		return tx.WriteMembers().Append(member.Init(), member.Join("taro"))
	}); err != nil {
		t.Fatal(err)
	}
	s.SetRetentionDays("S1", 7)
	if err := c.Write(t.Context(), "S1", op, func(port.Tx) error { return nil }); err != nil {
		t.Fatal(err)
	}
	got, err := c.Commits(t.Context(), "S1")
	if err != nil || len(got) != 2 {
		t.Fatalf("Commits = %d, %v, want 2", len(got), err)
	}
	if !got[0].StoreLog || len(got[0].Changes) != 0 || got[1].StoreLog {
		t.Errorf("StoreLog = %v / %v, changes = %d", got[0].StoreLog, got[1].StoreLog, len(got[0].Changes))
	}
}
