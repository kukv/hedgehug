package members_test

import (
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/members"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/member"
)

func TestListAndMe(t *testing.T) {
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	svc := members.Service{Stores: s, Identity: apptest.Identity{Login: "hanako"}}
	got, err := svc.List(t.Context(), "S1")
	// member.FromLog はログイン名の順に返す。表示名の初期値はログイン名。
	want := []member.Member{{Login: "hanako", Name: "hanako"}, {Login: "taro", Name: "taro"}}
	if err != nil || len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("List = %+v, %v; want %+v", got, err, want)
	}
	if me, err := svc.Me(t.Context(), "S1"); err != nil || me != "hanako" {
		t.Errorf("Me = %q, %v", me, err)
	}
	if _, err := svc.List(t.Context(), "nope"); !errors.Is(err, port.ErrStoreNotFound) {
		t.Errorf("unknown store: %v", err)
	}
}
