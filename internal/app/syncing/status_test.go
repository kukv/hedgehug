package syncing_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

// 1 つの Store の未送信の数を読めなくても、状態の表示は失敗させず、その Store だけ 0 件にする。
func TestStatusToleratesAnUnsentError(t *testing.T) {
	f := newFixture(t)
	f.rep.UnsentErrs = map[port.StoreID]error{"S1": errors.New("cannot open the store")}
	st, err := f.svc.Status(context.Background())
	if err != nil || len(st) != 4 {
		t.Fatalf("Status = %+v, %v", st, err)
	}
	if st[0].Store != "S1" || st[0].Unsent != 0 {
		t.Errorf("S1 = %+v", st[0])
	}
}

// 06「同期の状態の表示」: remote がある Store の状態(非表示も含め、画面が分ける)。
func TestStatus(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.svc.SyncStore(ctx, "S1"); err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(time.Minute)
	f.rep.Errs["S1"] = port.ErrAuthFailed
	if _, err := f.svc.SyncStore(ctx, "S1"); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Status(ctx)
	if err != nil || len(st) != 4 {
		t.Fatalf("Status = %+v, %v", st, err)
	}
	if s := st[0]; s.Store != "S1" || s.Name != "team" || s.Reason != syncstatus.AuthFailed || s.Unsent != 4 || !s.LastSuccess.Equal(at) {
		t.Errorf("S1 = %+v", s)
	}
	if s := st[1]; s.Store != "S3" || s.Reason != syncstatus.NoAccount || !s.LastSuccess.IsZero() {
		t.Errorf("S3 = %+v", s)
	}
	if s := st[2]; s.Store != "S4" || !s.ReadOnly || s.Reason != "" {
		t.Errorf("S4 = %+v", s)
	}
	if s := st[3]; s.Store != "S5" || !s.Hidden {
		t.Errorf("S5 = %+v", s)
	}
}
