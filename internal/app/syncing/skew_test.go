package syncing_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/syncing"
)

func TestSkewObserveRecordsOnlyWhenTheWarningChanges(t *testing.T) {
	records := apptest.NewClockSkews()
	events := &apptest.Events{}
	skew := &syncing.Skew{Records: records, Events: events}

	steps := []struct {
		d          time.Duration
		wantMin    int
		wantEvents int
	}{
		{-7*time.Minute - 30*time.Second, 7, 1},
		{-7*time.Minute - 10*time.Second, 7, 1},
		{6 * time.Minute, -6, 2},
		{time.Minute, 0, 3},
		{-2 * time.Minute, 0, 3},
	}
	for _, s := range steps {
		skew.Observe(s.d)
		if records.Minutes != s.wantMin {
			t.Fatalf("Observe(%v): 記録 = %d, want %d", s.d, records.Minutes, s.wantMin)
		}
		got := events.List()
		if len(got) != s.wantEvents {
			t.Fatalf("Observe(%v): 出来事 = %d 個, want %d", s.d, len(got), s.wantEvents)
		}
		for _, ev := range got {
			if ev != (port.Event{Kind: port.EventSync}) {
				t.Fatalf("出来事 = %+v", ev)
			}
		}
	}
}

func TestSkewLoadsTheRecordedValueFirst(t *testing.T) {
	records := apptest.NewClockSkews()
	records.Minutes = 7
	events := &apptest.Events{}
	skew := &syncing.Skew{Records: records, Events: events}

	skew.Observe(-7 * time.Minute)
	if n := len(events.List()); n != 0 {
		t.Fatalf("出来事 = %d 個, want 0", n)
	}
	got, err := skew.Minutes(context.Background())
	if err != nil || got != 7 {
		t.Fatalf("Minutes = %d, %v, want 7", got, err)
	}
}

func TestSkewObserveIsSafeWhenCalledTogether(t *testing.T) {
	records := apptest.NewClockSkews()
	events := &apptest.Events{}
	skew := &syncing.Skew{Records: records, Events: events}

	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() { skew.Observe(-8 * time.Minute) })
	}
	wg.Wait()
	if records.Minutes != 8 {
		t.Fatalf("記録 = %d, want 8", records.Minutes)
	}
	if n := len(events.List()); n != 1 {
		t.Fatalf("出来事 = %d 個, want 1", n)
	}
}

func TestSkewRetriesTheWriteAfterAFailure(t *testing.T) {
	records := apptest.NewClockSkews()
	events := &apptest.Events{}
	skew := &syncing.Skew{Records: records, Events: events}

	records.SetErr = errors.New("書けない")
	skew.Observe(-7 * time.Minute)
	if records.Minutes != 0 || len(events.List()) != 0 {
		t.Fatalf("失敗のあと: 記録 = %d, 出来事 = %d 個, want 0, 0", records.Minutes, len(events.List()))
	}

	records.SetErr = nil
	skew.Observe(-7 * time.Minute)
	if records.Minutes != 7 {
		t.Fatalf("記録 = %d, want 7", records.Minutes)
	}
	got := events.List()
	if len(got) != 1 || got[0] != (port.Event{Kind: port.EventSync}) {
		t.Fatalf("出来事 = %+v, want 1 個の sync", got)
	}
}

func TestSkewReloadsWhenTheFirstReadFails(t *testing.T) {
	records := apptest.NewClockSkews()
	records.Minutes = 7
	events := &apptest.Events{}
	skew := &syncing.Skew{Records: records, Events: events}

	// 読めない間は 0 から続ける。0 のままなら書かない。
	records.SkewErr = errors.New("読めない")
	skew.Observe(time.Minute)
	if records.Minutes != 7 || len(events.List()) != 0 {
		t.Fatalf("読めないとき: 記録 = %d, 出来事 = %d 個, want 7, 0", records.Minutes, len(events.List()))
	}

	// 読めるようになったら読み直し、記録済みの 7 と同じなので書かない。
	records.SkewErr = nil
	skew.Observe(-7 * time.Minute)
	if records.Minutes != 7 || len(events.List()) != 0 {
		t.Fatalf("読み直したあと: 記録 = %d, 出来事 = %d 個, want 7, 0", records.Minutes, len(events.List()))
	}

	skew.Observe(time.Minute)
	if records.Minutes != 0 || len(events.List()) != 1 {
		t.Fatalf("警告が消えたとき: 記録 = %d, 出来事 = %d 個, want 0, 1", records.Minutes, len(events.List()))
	}
}
