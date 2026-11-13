package syncing_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/change"
	"github.com/kukv/hedgehug/internal/domain/member"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

var deliverOp = port.Operation{Actor: change.User("taro"), Summary: "移動"}

func addTaro(tx port.Tx) error { return tx.WriteMembers().Append(member.Init(), member.Join("taro")) }

func newDeliverFixture(t *testing.T) (*fixture, *apptest.Stores) {
	t.Helper()
	f := newFixture(t)
	stores := apptest.NewStores(apptest.NewClock(at), apptest.NewIDs())
	for _, id := range []port.StoreID{"S1", "S2", "S3", "S4"} {
		stores.AddStore(id)
	}
	f.svc.Stores = stores
	f.rep.Stores = stores
	return f, stores
}

func memberCount(t *testing.T, s port.Stores, id port.StoreID) int {
	t.Helper()
	n := 0
	if err := s.Read(t.Context(), id, func(r port.Reader) error { ms, err := r.Members().List(); n = len(ms); return err }); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestDeliverSavesAndSendsWithTheStoresToken(t *testing.T) {
	f, stores := newDeliverFixture(t)
	reason, err := f.svc.Deliver(t.Context(), "S1", deliverOp, addTaro)
	if reason != "" || err != nil || memberCount(t, stores, "S1") != 1 {
		t.Fatalf("Deliver = %q, %v, members %d", reason, err, memberCount(t, stores, "S1"))
	}
	calls := f.rep.Calls()
	if len(calls) != 2 || !calls[0].Save || calls[0].Remote != (port.Remote{URL: "https://github.com/o/team.git", Token: "tok-taro"}) || !calls[1].Mirror {
		t.Errorf("calls = %+v, want a SyncSave then the mirror", calls)
	}
	rs, _ := f.state.Records(t.Context())
	if len(rs) != 1 || !rs[0].LastSuccess.Equal(at) || rs[0].Reason != "" {
		t.Errorf("records = %+v", rs)
	}
	if evs := f.events.List(); len(evs) != 1 || evs[0] != (port.Event{Kind: port.EventSync, Store: "S1"}) {
		t.Errorf("events = %+v", evs)
	}
	if got := f.afterCalls(); len(got) != 0 {
		t.Errorf("AfterSuccess = %v, want none", got)
	}
	// 次の Deliver には、受け取った位置を渡す。
	if _, err := f.svc.Deliver(t.Context(), "S1", deliverOp, func(port.Tx) error { return nil }); err != nil {
		t.Fatal(err)
	}
	var seen []string
	for _, c := range f.rep.Calls() {
		if c.Save {
			seen = append(seen, c.Remote.Seen)
		}
	}
	if !slices.Equal(seen, []string{"", "v2"}) {
		t.Errorf("Seen passed = %q", seen)
	}
}

// 送信が断られたら、理由を返し、記録し、何も保存しない。
func TestDeliverRefusedSendSavesNothingAndRecordsTheReason(t *testing.T) {
	f, stores := newDeliverFixture(t)
	f.rep.SendErrs = map[port.StoreID]error{"S1": port.ErrWriteDenied}
	reason, err := f.svc.Deliver(t.Context(), "S1", deliverOp, addTaro)
	if reason != syncstatus.WriteDenied || err != nil || memberCount(t, stores, "S1") != 0 {
		t.Fatalf("Deliver = %q, %v, members %d", reason, err, memberCount(t, stores, "S1"))
	}
	rs, _ := f.state.Records(t.Context())
	if len(rs) != 1 || rs[0].Reason != syncstatus.WriteDenied {
		t.Errorf("records = %+v", rs)
	}
	if got := f.tokens.Rejected(); len(got) != 0 {
		t.Errorf("Rejected = %v", got)
	}
	f.rep.SendErrs = map[port.StoreID]error{"S1": port.ErrAuthFailed}
	if reason, _ := f.svc.Deliver(t.Context(), "S1", deliverOp, addTaro); reason != syncstatus.AuthFailed {
		t.Errorf("reason = %q", reason)
	}
	if got := f.tokens.Rejected(); !slices.Equal(got, []port.StoreID{"S1"}) {
		t.Errorf("Rejected = %v, want [S1]", got)
	}
}

func TestDeliverWithoutRemoteJustWrites(t *testing.T) {
	f, stores := newDeliverFixture(t)
	reason, err := f.svc.Deliver(t.Context(), "S2", deliverOp, addTaro)
	if reason != "" || err != nil || memberCount(t, stores, "S2") != 1 || len(f.rep.Calls()) != 0 {
		t.Errorf("Deliver = %q, %v, members %d, calls %+v", reason, err, memberCount(t, stores, "S2"), f.rep.Calls())
	}
}

func TestDeliverWithoutAccountSavesNothing(t *testing.T) {
	f, stores := newDeliverFixture(t)
	reason, err := f.svc.Deliver(t.Context(), "S3", deliverOp, addTaro)
	rs, _ := f.state.Records(t.Context())
	if reason != syncstatus.NoAccount || err != nil || memberCount(t, stores, "S3") != 0 || len(f.rep.Calls()) != 0 || len(rs) != 0 {
		t.Errorf("Deliver = %q, %v, members %d, calls %+v, records %+v", reason, err, memberCount(t, stores, "S3"), f.rep.Calls(), rs)
	}
}

func TestDeliverRefusesAReadOnlyStore(t *testing.T) {
	f, stores := newDeliverFixture(t)
	_, err := f.svc.Deliver(t.Context(), "S4", deliverOp, addTaro)
	if !errors.Is(err, port.ErrInvalidOperation) || memberCount(t, stores, "S4") != 0 || len(f.rep.Calls()) != 0 {
		t.Errorf("Deliver = %v, members %d, calls %+v", err, memberCount(t, stores, "S4"), f.rep.Calls())
	}
}

// failingSucceeded は、Succeeded だけが失敗する同期の状態。
type failingSucceeded struct{ *apptest.SyncState }

func (failingSucceeded) Succeeded(context.Context, port.StoreID, time.Time, string, string) error {
	return errors.New("disk full")
}

// 送れたあとに状態を記録できなくても、保存は送られているので成功を返す。
func TestDeliverSucceedsWhenRecordingFailsAfterSending(t *testing.T) {
	f, stores := newDeliverFixture(t)
	f.svc.State = failingSucceeded{f.state}
	reason, err := f.svc.Deliver(t.Context(), "S1", deliverOp, addTaro)
	if reason != "" || err != nil || memberCount(t, stores, "S1") != 1 {
		t.Fatalf("Deliver = %q, %v, members %d, want saved", reason, err, memberCount(t, stores, "S1"))
	}
	if calls := f.rep.Calls(); len(calls) != 2 || !calls[1].Mirror {
		t.Errorf("calls = %+v, want the mirror too", calls)
	}
	if evs := f.events.List(); len(evs) != 1 {
		t.Errorf("events = %+v", evs)
	}
}

// 送れなかった保存を捨てられなかったときは、理由にせず error で返す(保存されたかもしれない)。
func TestDeliverReturnsASaveThatWasNotDiscardedAsAnError(t *testing.T) {
	f, _ := newDeliverFixture(t)
	f.rep.SendErrs = map[port.StoreID]error{"S1": errors.Join(port.ErrSendConflict, port.ErrSaveNotDiscarded)}
	reason, err := f.svc.Deliver(t.Context(), "S1", deliverOp, addTaro)
	rs, _ := f.state.Records(t.Context())
	if reason != "" || !errors.Is(err, port.ErrSaveNotDiscarded) || len(rs) != 0 {
		t.Errorf("Deliver = %q, %v, records %+v", reason, err, rs)
	}
}

// 保存の失敗は、理由にせず error で返し、同期の失敗として記録しない。
func TestDeliverReturnsSaveErrors(t *testing.T) {
	f, _ := newDeliverFixture(t)
	boom := errors.New("boom")
	reason, err := f.svc.Deliver(t.Context(), "S1", deliverOp, func(port.Tx) error { return boom })
	rs, _ := f.state.Records(t.Context())
	var se *port.SaveError
	if reason != "" || !errors.Is(err, boom) || errors.As(err, &se) || len(rs) != 0 || len(f.events.List()) != 0 {
		t.Errorf("Deliver = %q, %v, records %+v, events %+v", reason, err, rs, f.events.List())
	}
	if _, err := f.svc.Deliver(t.Context(), "S9", deliverOp, addTaro); !errors.Is(err, port.ErrStoreNotFound) {
		t.Errorf("unknown store = %v", err)
	}
}

// SyncSave の中の同期の修復が失敗したとき(SaveError で返る)は、通信の失敗の理由(remote に接続できません)にせず、
// error で返して、同期の失敗として記録しない。
func TestDeliverReturnsARepairFailureAsAnError(t *testing.T) {
	f, stores := newDeliverFixture(t)
	boom := errors.New("repair failed")
	f.rep.Errs = map[port.StoreID]error{"S1": &port.SaveError{Err: boom}}
	reason, err := f.svc.Deliver(t.Context(), "S1", deliverOp, addTaro)
	rs, _ := f.state.Records(t.Context())
	if reason != "" || !errors.Is(err, boom) || len(rs) != 0 || memberCount(t, stores, "S1") != 0 {
		t.Errorf("Deliver = %q, %v, records %+v", reason, err, rs)
	}
}
