package stores_test

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/kukv/hedgehug/internal/adapter/httpapi/stores"
	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	appstores "github.com/kukv/hedgehug/internal/app/stores"
)

const loginRefusal = "英数字・ハイフン・アンダースコアのログイン名(39 文字以内)を入力してください"

// newLoginAPI は、アカウントなしの team-a と、アカウントを割り当てた team-b を持ち、マシンのログイン名が taro の API。
func newLoginAPI(t *testing.T) (humatest.TestAPI, *apptest.StoreGitHub, *apptest.Events) {
	t.Helper()
	gh := &apptest.StoreGitHub{Login: "taro", Stores: []port.StoreGitHubInfo{
		{ID: "sa", Name: "team-a"},
		{ID: "sb", Name: "team-b", Account: "a1"},
	}}
	ev := &apptest.Events{}
	_, api := humatest.New(t)
	stores.RegisterLogin(api, appstores.Login{GitHub: gh, Events: ev})
	return api, gh, ev
}

func detail(t *testing.T, body []byte) string {
	t.Helper()
	var v struct{ Detail string }
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("%v: %s", err, body)
	}
	return v.Detail
}

func TestGetLogin(t *testing.T) {
	api, _, _ := newLoginAPI(t)
	if res := api.Get("/api/settings/login"); res.Code != http.StatusOK || res.Body.String() != `{"login":"taro"}`+"\n" {
		t.Errorf("get: %d %s", res.Code, res.Body)
	}
}

func TestPreviewLogin(t *testing.T) {
	api, _, _ := newLoginAPI(t)
	cases := []struct{ login, want string }{
		{"tarou", `{"lines":["team-a での あなた が @taro から tarou に変わります。"]}`},
		{"Taro", `{"lines":[]}`},
	}
	for _, c := range cases {
		res := api.Post("/api/settings/login/preview", map[string]any{"login": c.login})
		if res.Code != http.StatusOK || res.Body.String() != c.want+"\n" {
			t.Errorf("preview %q: %d %s", c.login, res.Code, res.Body)
		}
	}
}

func TestPreviewLoginRefusalIs422(t *testing.T) {
	api, _, _ := newLoginAPI(t)
	res := api.Post("/api/settings/login/preview", map[string]any{"login": "ta ro"})
	if res.Code != http.StatusUnprocessableEntity || detail(t, res.Body.Bytes()) != loginRefusal {
		t.Errorf("preview: %d %s", res.Code, res.Body)
	}
}

func TestPutLogin(t *testing.T) {
	ctx := context.Background()
	api, gh, ev := newLoginAPI(t)
	if res := api.Put("/api/settings/login", map[string]any{"login": "tarou"}); res.Code != http.StatusNoContent {
		t.Fatalf("put: %d %s", res.Code, res.Body)
	}
	if got, _ := gh.MachineLogin(ctx); got != "tarou" {
		t.Errorf("login = %q", got)
	}
	if got := ev.List(); !slices.Equal(got, []port.Event{{Kind: port.EventStore, Store: "sa"}}) {
		t.Errorf("events = %+v", got)
	}
	res := api.Put("/api/settings/login", map[string]any{"login": "ta ro"})
	if res.Code != http.StatusUnprocessableEntity || detail(t, res.Body.Bytes()) != loginRefusal {
		t.Errorf("put refused: %d %s", res.Code, res.Body)
	}
	if got, _ := gh.MachineLogin(ctx); got != "tarou" {
		t.Errorf("login after refusal = %q", got)
	}
}
