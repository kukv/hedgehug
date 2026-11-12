package syncing_test

import (
	"context"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

// 06「同期の結果」: 表示中で remote がある Store をすべて同期し、1 つのトーストにまとめる。
func TestSyncAllMixed(t *testing.T) {
	f := newFixture(t)
	all, err := f.svc.SyncAll(context.Background())
	if err != nil || all.Nothing {
		t.Fatalf("SyncAll = %+v, %v", all, err)
	}
	want := []string{"成功 2 件・失敗 1 件", "docs: アカウントが設定されていません"}
	if !slices.Equal(all.Lines, want) {
		t.Errorf("Lines = %q", all.Lines)
	}
	n := f.syncs()
	if n["S1"] != 1 || n["S4"] != 1 || n["S5"] != 0 || len(all.Outcomes) != 3 {
		t.Errorf("syncs = %v, outcomes = %+v", n, all.Outcomes)
	}
}

func TestSyncAllSucceeds(t *testing.T) {
	f := newFixture(t)
	f.targets.Set([]port.SyncTarget{{ID: "S1", Name: "team", Remote: "https://github.com/o/team", Account: "A1"}})
	all, err := f.svc.SyncAll(context.Background())
	if err != nil || !slices.Equal(all.Lines, []string{"1 件の Store を同期しました"}) {
		t.Errorf("SyncAll = %+v, %v", all, err)
	}
}

// 06「手動で同期する」: 同期する Store がなければ、何も同期せず、そのトーストを出す。
func TestSyncAllWithNothingToSync(t *testing.T) {
	f := newFixture(t)
	f.targets.Set([]port.SyncTarget{{ID: "S2", Name: "solo"}, {ID: "S5", Name: "hidden", Remote: "https://github.com/o/h", Account: "A1", Hidden: true}})
	all, err := f.svc.SyncAll(context.Background())
	if err != nil || !all.Nothing || !slices.Equal(all.Lines, []string{syncstatus.NothingToSync}) || len(f.rep.Calls()) != 0 {
		t.Errorf("SyncAll = %+v, %v", all, err)
	}
}
