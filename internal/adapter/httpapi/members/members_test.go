package members_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/members"
	"github.com/kukv/hedgehug/internal/app/apptest"
	appmembers "github.com/kukv/hedgehug/internal/app/members"
	"github.com/kukv/hedgehug/internal/app/port"
)

func newStores(t *testing.T) *apptest.Stores {
	t.Helper()
	s := apptest.NewStores(apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)), apptest.NewIDs())
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestMembersAndMe(t *testing.T) {
	_, api := humatest.New(t)
	members.Register(api, appmembers.Service{Stores: newStores(t), Identity: apptest.Identity{Login: "taro"}})
	res := api.Get("/api/stores/S1/members")
	if res.Code != http.StatusOK || res.Body.String() != `[{"login":"hanako","name":"hanako"},{"login":"taro","name":"taro"}]`+"\n" {
		t.Errorf("members: %d %s", res.Code, res.Body)
	}
	if res := api.Get("/api/stores/S1/me"); res.Code != http.StatusOK || res.Body.String() != `{"login":"taro"}`+"\n" {
		t.Errorf("me: %d %s", res.Code, res.Body)
	}
	if res := api.Get("/api/stores/nope/members"); res.Code != http.StatusNotFound {
		t.Errorf("unknown store: %d", res.Code)
	}
}

type noIdentity struct{}

func (noIdentity) Me(context.Context, port.StoreID) (string, error) {
	return "", errors.New("このマシンのログイン名が未設定です")
}

// 「あなた」が決まらないときは 500。画面はオーナーの初めの値を「なし」にする(A2 の決定 3)。
func TestMeWithoutIdentityIs500(t *testing.T) {
	_, api := humatest.New(t)
	members.Register(api, appmembers.Service{Stores: newStores(t), Identity: noIdentity{}})
	if res := api.Get("/api/stores/S1/me"); res.Code != http.StatusInternalServerError {
		t.Errorf("me: %d %s", res.Code, res.Body)
	}
}
