package apptest_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

var (
	_ port.DeviceFlow  = (*apptest.DeviceFlow)(nil)
	_ port.Hosts       = apptest.Hosts{}
	_ port.StoreGitHub = (*apptest.StoreGitHub)(nil)
	_ port.Secrets     = (*apptest.Secrets)(nil)
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
	if got := in.Notices(); len(got) != 1 || got[0] != n {
		t.Errorf("Notices = %+v", got)
	}
	in.Err = errors.New("full")
	if err := in.Add(t.Context(), n); err == nil || len(in.Notices()) != 1 {
		t.Errorf("Add with Err = %v, %d", err, len(in.Notices()))
	}
}
