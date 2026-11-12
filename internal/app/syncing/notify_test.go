package syncing_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/syncing"
	"github.com/kukv/hedgehug/internal/domain/change"
)

// 決定 C2-19: Write が成功したときだけ知らせる。
func TestNotifyWrites(t *testing.T) {
	inner := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)), apptest.NewIDs())
	inner.AddStore("S1")
	var got []port.StoreID
	s := syncing.NotifyWrites(inner, func(id port.StoreID) { got = append(got, id) })
	op := port.Operation{Actor: change.User("taro"), Summary: "x"}
	if err := s.Write(context.Background(), "S1", op, func(port.Tx) error { return nil }); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	if err := s.Write(context.Background(), "S1", op, func(port.Tx) error { return boom }); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if !slices.Equal(got, []port.StoreID{"S1"}) {
		t.Errorf("notified = %v", got)
	}
	if err := s.Read(context.Background(), "S1", func(port.Reader) error { return nil }); err != nil {
		t.Errorf("Read = %v", err)
	}
}
