package syncing_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/syncing"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/syncstatus"
)

func TestSyncStoreSucceeds(t *testing.T) {
	f := newFixture(t)
	out, err := f.svc.SyncStore(context.Background(), "S1")
	if err != nil || !out.OK() || out.Sent != 2 || out.Received != 1 {
		t.Fatalf("SyncStore = %+v, %v", out, err)
	}
	if got := out.Message(); got != "team を同期しました(送信 2 件・受信 1 件)" {
		t.Errorf("Message = %q", got)
	}
	calls := f.rep.Calls()
	want := port.Remote{URL: "https://github.com/o/team.git", Token: "tok-taro"}
	if len(calls) != 2 || calls[0].Remote != want || !calls[1].Mirror || calls[1].Remote.URL != "https://github.com/o/team-mirror" || calls[1].Remote.Token != "tok-taro" {
		t.Errorf("calls = %+v", calls)
	}
	rs, _ := f.state.Records(context.Background())
	if len(rs) != 1 || !rs[0].LastSuccess.Equal(at) || rs[0].Reason != "" {
		t.Errorf("records = %+v", rs)
	}
	if seen, _ := f.state.Seen(context.Background(), "S1", "github.com/o/team"); seen != "v2" {
		t.Errorf("Seen = %q", seen)
	}
	if got := f.afterCalls(); !slices.Equal(got, []port.StoreID{"S1"}) {
		t.Errorf("AfterSuccess = %v", got)
	}
	if evs := f.events.List(); len(evs) != 1 || evs[0] != (port.Event{Kind: port.EventSync, Store: "S1"}) {
		t.Errorf("events = %+v", evs)
	}
}

// 決定 C2-11: 前に受け取った位置を、同じ remote(書き方の違いは問わない)の次の同期に渡す。
func TestSyncStorePassesSeenForTheSameRemote(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.svc.SyncStore(ctx, "S1"); err != nil {
		t.Fatal(err)
	}
	ts, _ := f.targets.List(ctx)
	ts[0].Remote = "git@github.com:O/team"
	f.targets.Set(ts)
	if _, err := f.svc.SyncStore(ctx, "S1"); err != nil {
		t.Fatal(err)
	}
	ts[0].Remote = "https://github.com/o/other.git"
	f.targets.Set(ts)
	if _, err := f.svc.SyncStore(ctx, "S1"); err != nil {
		t.Fatal(err)
	}
	var seen []string
	for _, c := range f.rep.Calls() {
		if !c.Mirror {
			seen = append(seen, c.Remote.Seen)
		}
	}
	if !slices.Equal(seen, []string{"", "v2", ""}) {
		t.Errorf("Seen passed = %q", seen)
	}
}

// 06「同期のエラー」の表: remote の失敗ごとの理由と、記録と、手動のトースト。
func TestSyncStoreRecordsTheReason(t *testing.T) {
	cases := []struct {
		err  error
		want syncstatus.Reason
	}{
		{port.ErrAuthFailed, syncstatus.AuthFailed},
		{port.ErrUnreachable, syncstatus.Unreachable},
		{port.ErrWriteDenied, syncstatus.WriteDenied},
		{port.ErrRemoteUnreadable, syncstatus.Unreadable},
		{port.ErrRemoteNotStore, syncstatus.NotStore},
		{port.ErrRemoteNewerFormat, syncstatus.NewerFormat},
		{port.ErrNoCommonHistory, syncstatus.Unrelated},
		{port.ErrRemoteRewritten, syncstatus.Rewritten},
		{port.ErrSendConflict, syncstatus.SendConflict},
		{errors.New("disk full"), syncstatus.Unreachable},
	}
	for _, c := range cases {
		t.Run(string(c.want)+c.err.Error(), func(t *testing.T) { checkReason(t, c.err, c.want) })
	}
}

func checkReason(t *testing.T, cerr error, cwant syncstatus.Reason) {
	t.Helper()
	f := newFixture(t)
	f.rep.Errs["S1"] = cerr
	out, err := f.svc.SyncStore(context.Background(), "S1")
	if err != nil || out.OK() || out.Reason != cwant {
		t.Fatalf("SyncStore = %+v, %v", out, err)
	}
	if got, want := out.Message(), "team の同期に失敗しました: "+cwant.Short(); got != want {
		t.Errorf("Message = %q, want %q", got, want)
	}
	rs, _ := f.state.Records(context.Background())
	if len(rs) != 1 || rs[0].Reason != cwant || !rs[0].LastAttempt.Equal(at) || !rs[0].LastSuccess.IsZero() {
		t.Errorf("records = %+v", rs)
	}
	if len(f.afterCalls()) != 0 || f.syncs()["S1"] != 1 || len(f.rep.Calls()) != 1 {
		t.Errorf("after = %v, calls = %+v", f.afterCalls(), f.rep.Calls())
	}
	rejected := f.tokens.Rejected()
	if (cwant == syncstatus.AuthFailed) != (len(rejected) == 1) {
		t.Errorf("rejected = %v", rejected)
	}
}

// 決定 C2-9: トークンを読めなければ「認証に失敗しました」。GitHub に届かなければ「接続できません」。
func TestSyncStoreTokenFailures(t *testing.T) {
	cases := []struct {
		kind ghcall.Kind
		want syncstatus.Reason
	}{
		{ghcall.KeychainMissing, syncstatus.AuthFailed},
		{ghcall.EnvMissing, syncstatus.AuthFailed},
		{ghcall.GhTokenMissing, syncstatus.AuthFailed},
		{ghcall.GhTimeout, syncstatus.AuthFailed},
		{ghcall.AuthError, syncstatus.AuthFailed},
		{ghcall.Unreachable, syncstatus.Unreachable},
		{ghcall.RateLimited, syncstatus.Unreachable},
	}
	for _, c := range cases {
		f := newFixture(t)
		f.tokens.Errs = map[port.StoreID]error{"S1": &ghcall.Failure{Kind: c.kind}}
		out, err := f.svc.SyncStore(context.Background(), "S1")
		if err != nil || out.Reason != c.want || len(f.rep.Calls()) != 0 {
			t.Errorf("%s: %+v, %v", c.kind, out, err)
		}
		if rs, _ := f.state.Records(context.Background()); len(rs) != 1 || rs[0].Reason != c.want {
			t.Errorf("%s: records = %+v", c.kind, rs)
		}
	}
}

// 決定 C2-16: 割り当てがない(トークンの入口が NoAccount を返した場合も)なら、同期せず、記録しない。
func TestSyncStoreWithoutAccount(t *testing.T) {
	f := newFixture(t)
	out, err := f.svc.SyncStore(context.Background(), "S3")
	if err != nil || out.Reason != syncstatus.NoAccount || out.Message() != "docs にアカウントが設定されていないため同期できません" {
		t.Errorf("S3 = %+v, %v", out, err)
	}
	f.tokens.Errs = map[port.StoreID]error{"S1": &ghcall.Failure{Kind: ghcall.NoAccount}}
	if out, _ := f.svc.SyncStore(context.Background(), "S1"); out.Reason != syncstatus.NoAccount {
		t.Errorf("S1 = %+v", out)
	}
	if rs, _ := f.state.Records(context.Background()); len(rs) != 0 || len(f.rep.Calls()) != 0 {
		t.Errorf("records = %+v, calls = %+v", rs, f.rep.Calls())
	}
}

func TestSyncStoreWithoutRemote(t *testing.T) {
	f := newFixture(t)
	out, err := f.svc.SyncStore(context.Background(), "S2")
	if err != nil || !out.Skipped || out.OK() || out.Message() != "solo には remote がないため、同期するものはありません" {
		t.Errorf("S2 = %+v, %v", out, err)
	}
	if rs, _ := f.state.Records(context.Background()); len(rs) != 0 || len(f.events.List()) != 0 {
		t.Errorf("records = %+v, events = %+v", rs, f.events.List())
	}
}

// 決定 C2-13: 閲覧のみは受け取るだけ。ミラーに送らず、完全な削除の呼び出し口も呼ばない。
func TestSyncStoreReadOnly(t *testing.T) {
	f := newFixture(t)
	out, err := f.svc.SyncStore(context.Background(), "S4")
	if err != nil || !out.OK() || out.Received != 3 {
		t.Fatalf("S4 = %+v, %v", out, err)
	}
	calls := f.rep.Calls()
	if len(calls) != 1 || !calls[0].Remote.ReceiveOnly {
		t.Errorf("calls = %+v", calls)
	}
	if len(f.afterCalls()) != 0 {
		t.Errorf("AfterSuccess = %v", f.afterCalls())
	}
}

// 06 のミラーの段落: ミラーに送れなくても、同期は成功のまま。ミラーごとに記録する。
func TestSyncStoreMirrorFailure(t *testing.T) {
	f := newFixture(t)
	f.rep.MirrorErrs = map[string]error{"https://github.com/o/team-mirror": port.ErrMirrorAhead}
	out, err := f.svc.SyncStore(context.Background(), "S1")
	if err != nil || !out.OK() || out.MirrorFailures != 1 {
		t.Fatalf("S1 = %+v, %v", out, err)
	}
	ms, _ := f.state.Mirrors(context.Background(), "S1")
	if len(ms) != 1 || ms[0].Reason != syncstatus.MirrorAhead {
		t.Errorf("mirrors = %+v", ms)
	}
}

// 06「手動で同期する」: 同じ Store の同期が動いていれば、新しく始めずに、その結果を使う。
func TestSyncStoreJoinsTheRunningSync(t *testing.T) {
	f := newFixture(t)
	f.rep.Gate = make(chan struct{})
	outs := make(chan port.StoreID, 2)
	for range 2 {
		go func() {
			out, err := f.svc.SyncStore(context.Background(), "S1")
			if err == nil && out.OK() {
				outs <- out.Store
			}
		}()
	}
	waitFor(t, func() bool { return f.syncs()["S1"] == 1 })
	time.Sleep(50 * time.Millisecond) // 2 つ目が始まっていないことを確かめる時間
	f.rep.Gate <- struct{}{}
	for range 2 {
		select {
		case <-outs:
		case <-time.After(2 * time.Second):
			t.Fatal("a caller did not get the result")
		}
	}
	if n := f.syncs()["S1"]; n != 1 {
		t.Errorf("Sync ran %d times", n)
	}
}

// 同期が panic で終わっても、その結果を待っていた呼び出しは、成功に見える空の結果でなくエラーを受け取る。
func TestSyncStoreReportsPanicToWaiters(t *testing.T) {
	f := newFixture(t)
	f.rep.Gate = make(chan struct{})
	f.svc.AfterSuccess = func(context.Context, port.StoreID) { panic("boom") }
	go func() {
		defer func() { _ = recover() }()
		_, _ = f.svc.SyncStore(context.Background(), "S1")
	}()
	waitFor(t, func() bool { return f.syncs()["S1"] == 1 })
	type result struct {
		out syncing.Outcome
		err error
	}
	got := make(chan result, 1)
	go func() {
		out, err := f.svc.SyncStore(context.Background(), "S1")
		got <- result{out, err}
	}()
	time.Sleep(50 * time.Millisecond) // 2 つ目が待ちに入る時間
	f.rep.Gate <- struct{}{}
	select {
	case r := <-got:
		if r.err == nil {
			t.Errorf("the waiter got %+v with no error", r.out)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the waiter did not return")
	}
}

// 決定 C2-17: 始めた要求が切れても、同期は最後まで続ける。
func TestSyncStoreOutlivesTheRequest(t *testing.T) {
	f := newFixture(t)
	f.rep.Gate = make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan bool, 1)
	go func() {
		out, _ := f.svc.SyncStore(ctx, "S1")
		done <- out.OK()
	}()
	waitFor(t, func() bool { return f.syncs()["S1"] == 1 })
	cancel()
	f.rep.Gate <- struct{}{}
	if ok := <-done; !ok {
		t.Error("the sync was cancelled with the request")
	}
}

func TestSyncStoreUnknownStore(t *testing.T) {
	f := newFixture(t)
	if _, err := f.svc.SyncStore(context.Background(), "S9"); !errors.Is(err, port.ErrStoreNotFound) {
		t.Errorf("err = %v", err)
	}
}

// 06「同期のあとの完全な削除」: 非表示の Store では、自動では行わない。
func TestSyncStoreHiddenSkipsAfterSuccess(t *testing.T) {
	f := newFixture(t)
	out, err := f.svc.SyncStore(context.Background(), "S5")
	if err != nil || !out.OK() {
		t.Fatalf("S5 = %+v, %v", out, err)
	}
	if got := f.afterCalls(); len(got) != 0 {
		t.Errorf("AfterSuccess = %v", got)
	}
}

// AfterSuccess が panic しても、その Store の次の同期が止まらない。
func TestSyncStoreRecoversFromPanic(t *testing.T) {
	f := newFixture(t)
	f.svc.AfterSuccess = func(context.Context, port.StoreID) { panic("boom") }
	func() {
		defer func() { _ = recover() }()
		_, _ = f.svc.SyncStore(context.Background(), "S1")
	}()
	f.svc.AfterSuccess = nil
	done := make(chan bool, 1)
	go func() {
		out, err := f.svc.SyncStore(context.Background(), "S1")
		done <- err == nil && out.OK()
	}()
	select {
	case ok := <-done:
		if !ok {
			t.Error("second sync failed")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("second sync is wedged")
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("timed out")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
