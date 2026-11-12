package gitstore

import (
	"context"
	"errors"
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
