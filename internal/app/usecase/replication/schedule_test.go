package replication_test

import (
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/usecase/replication"
	"github.com/kukv/hedgehug/internal/app/usecase/tasks"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func TestLoadScheduleSkipsStoresWithoutRemote(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	sch, err := replication.LoadSchedule(ts.Session)
	if err != nil {
		t.Fatal(err)
	}
	if len(sch.Stores) != 0 {
		t.Errorf("Stores = %v; want none for a store without a remote", sch.Stores)
	}
	usecasetest.AddBareRemote(t, ts)
	sch, err = replication.LoadSchedule(ts.Session)
	if err != nil {
		t.Fatal(err)
	}
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	if len(sch.Stores) != 1 || sch.Stores[0] != "me" {
		t.Errorf("Stores = %v; want [me]", sch.Stores)
	}
	if sch.Interval != c.Sync.Interval || sch.PushInterval != c.Sync.PushInterval {
		t.Errorf("intervals = %+v; want %v and %v", sch.SyncIntervals, c.Sync.Interval, c.Sync.PushInterval)
	}
}

func TestLoadScheduleWithoutStores(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	ts.SetConfig(usecasetest.DefaultAccountYAML)
	if _, err := replication.LoadSchedule(ts.Session); !errors.Is(err, config.ErrNoStores) {
		t.Errorf("error = %v; want config.ErrNoStores", err)
	}
}

func TestPendingSinceIsSetByAWriteAndClearedByPush(t *testing.T) {
	t.Parallel()
	ts := usecasetest.New(t)
	usecasetest.AddBareRemote(t, ts)
	if got := replication.PendingSince(ts.Session); !got.IsZero() {
		t.Fatalf("PendingSince = %v before a write; want zero", got)
	}
	if _, err := tasks.Add(ts.Session, tasks.AddInput{Title: "a"}); err != nil {
		t.Fatal(err)
	}
	if got := replication.PendingSince(ts.Session); got.IsZero() {
		t.Fatal("PendingSince is zero after a write; want its time")
	}
	// The pending write is cleared by a sync that started after it.
	later := ts.Now().Add(time.Minute)
	ts.Now = func() time.Time { return later }
	rep, err := replication.SyncOne(ts.Session)
	if err != nil || rep.Err != nil {
		t.Fatalf("SyncOne = %v, %v", rep.Err, err)
	}
	if got := replication.PendingSince(ts.Session); !got.IsZero() {
		t.Errorf("PendingSince = %v after the push; want zero", got)
	}
}
