package apptest_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

var (
	_ port.DeviceFlow  = (*apptest.DeviceFlow)(nil)
	_ port.Hosts       = apptest.Hosts{}
	_ port.StoreGitHub = (*apptest.StoreGitHub)(nil)
	_ port.Secrets     = (*apptest.Secrets)(nil)
	_ port.SyncLog     = (*apptest.SyncLog)(nil)
)

func TestFakeDeviceFlowPollsRepeatTheLast(t *testing.T) {
	d := apptest.NewDeviceFlow()
	d.Polls = []port.DeviceGrant{{Status: port.DevicePending}, {Status: port.DeviceDenied}}
	var got []port.DeviceStatus
	for range 3 {
		g, err := d.PollDevice(context.Background(), "cid", "device-1")
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, g.Status)
	}
	if got[0] != port.DevicePending || got[1] != port.DeviceDenied || got[2] != port.DeviceDenied {
		t.Errorf("statuses = %v", got)
	}
}

func TestFakeStoreGitHubUnassign(t *testing.T) {
	s := &apptest.StoreGitHub{Stores: []port.StoreGitHubInfo{
		{Name: "チーム", Account: "A1"}, {Name: "個人", Account: "A2"}, {Name: "OSS", Account: "A1"},
	}}
	if err := s.Unassign(context.Background(), "A1"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.List(context.Background())
	if got[0].Account != "" || got[1].Account != "A2" || got[2].Account != "" {
		t.Errorf("List = %+v", got)
	}
}

func TestInboxKeepsNotices(t *testing.T) {
	var in apptest.Inbox
	n := port.Notice{Kind: port.NoticeAutomation, Store: "S1", Text: "「完了で外す」で担当者を なし に変更しました"}
	if err := in.Add(t.Context(), n); err != nil {
		t.Fatal(err)
	}
	if got := in.Notices(); len(got) != 1 || !reflect.DeepEqual(got[0], n) {
		t.Errorf("Notices = %+v", got)
	}
	in.Err = errors.New("full")
	if err := in.Add(t.Context(), n); err == nil || len(in.Notices()) != 1 {
		t.Errorf("Add with Err = %v, %d", err, len(in.Notices()))
	}
}

// SyncLog の偽物は、本物(statedb)と同じく 0・0 を書かず、新しい順(同じ時刻なら後に書いたものが先)に返す。
func TestFakeSyncLog(t *testing.T) {
	ctx := context.Background()
	l := &apptest.SyncLog{}
	at := time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)
	adds := []struct {
		store    port.StoreID
		at       time.Time
		sent, rc int
	}{{"S1", at, 1, 0}, {"S1", at.Add(time.Second), 0, 0}, {"S1", at.Add(time.Minute), 0, 2}, {"S1", at, 3, 0}, {"S2", at, 1, 1}}
	for _, a := range adds {
		if err := l.Add(ctx, a.store, a.at, a.sent, a.rc); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := l.List(ctx, "S1")
	if len(got) != 3 || got[0].Received != 2 || got[1].Sent != 3 || got[2].Sent != 1 || got[1].ID <= got[2].ID || got[0].Store != "S1" {
		t.Errorf("List(S1) = %+v", got)
	}
	if err := l.ForgetStore(ctx, "S1"); err != nil {
		t.Fatal(err)
	}
	if got, _ := l.List(ctx, "S1"); len(got) != 0 {
		t.Errorf("List(S1) after ForgetStore = %+v", got)
	}
	if got, _ := l.List(ctx, "S2"); len(got) != 1 {
		t.Errorf("List(S2) = %+v", got)
	}
}
