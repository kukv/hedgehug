package gitstore

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// 見張りは、触っているあいだは止めず、stallAfter のあいだ触らなければ errStalled で取り消す。
func TestWatchCancelsOnlyWhenNothingMoves(t *testing.T) {
	saved := stallAfter
	stallAfter = 200 * time.Millisecond
	t.Cleanup(func() { stallAfter = saved })
	ctx, done := watch(context.Background())
	defer done()
	w := ctx.Value(watchKey{}).(*watchdog)
	for range 6 {
		time.Sleep(50 * time.Millisecond)
		w.touch()
	}
	if ctx.Err() != nil {
		t.Fatalf("cancelled while moving: %v", context.Cause(ctx))
	}
	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("not cancelled after stalling")
	}
	if !errors.Is(context.Cause(ctx), errStalled) {
		t.Errorf("cause = %v", context.Cause(ctx))
	}
}

// 見張りのない通信でも、応答の Date と now() の差を渡す。Date を読めない応答では渡さない。
func TestWatchedTransportMeasuresSkewWithoutWatchdog(t *testing.T) {
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	date := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if date == "" {
			w.Header()["Date"] = nil // Go は Date を自動で付けるので、nil で外す
			return
		}
		w.Header().Set("Date", date)
	}))
	defer srv.Close()
	roundTrip := func() (got []time.Duration) {
		ctx := withSkew(context.Background(), func(d time.Duration) { got = append(got, d) }, func() time.Time { return now })
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
		if err != nil {
			t.Fatal(err)
		}
		res, err := watchedTransport{base: http.DefaultTransport}.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		return got
	}

	date = now.Add(-10 * time.Minute).Format(http.TimeFormat)
	got := roundTrip()
	if len(got) != 1 || got[0] < -10*time.Minute-time.Second || got[0] > -10*time.Minute+time.Second {
		t.Fatalf("ずれ = %v, want 約 -10m", got)
	}

	date = "not a date"
	if got := roundTrip(); len(got) != 0 {
		t.Fatalf("読めない Date でずれが渡された: %v", got)
	}
	date = ""
	if got := roundTrip(); len(got) != 0 {
		t.Fatalf("Date なしでずれが渡された: %v", got)
	}
}
