package github_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	ghapi "github.com/kukv/hedgehug/internal/adapter/httpapi/github"
	"github.com/kukv/hedgehug/internal/app/apptest"
	ghapp "github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/itemlink"
)

func TestLinksAPI(t *testing.T) {
	_, api := humatest.New(t)
	ghapi.Register(api, linkServices(t))
	base := "/api/stores/S1/items/" + string(apptest.SampleTask) + "/links"

	res := api.Post(base, map[string]any{"kind": "pr", "repo": "org/web", "number": "41", "addToProject": false})
	if res.Code != http.StatusNoContent {
		t.Fatalf("add: %d %s", res.Code, res.Body)
	}
	res = api.Post(base, map[string]any{"kind": "pr", "repo": "org/web", "number": "x", "addToProject": false})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "番号を入力してください") {
		t.Errorf("refusal: %d %s", res.Code, res.Body)
	}
	res = api.Get(base)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"repo":"org/web","number":41,"title":"Add login form","state":"open","review":"pending","checks":"pending","error":""`) {
		t.Errorf("list: %d %s", res.Code, res.Body)
	}
	if res = api.Post(base+"/refresh", map[string]any{}); res.Code != http.StatusOK {
		t.Errorf("refresh: %d %s", res.Code, res.Body)
	}
	res = api.Get("/api/stores/S1/projects/" + string(apptest.SampleProject) + "/links")
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"display":"HH-2"`) {
		t.Errorf("project links: %d %s", res.Code, res.Body)
	}
	if res = api.Post(base+"/remove", map[string]any{"kind": "pr", "repo": "org/web", "number": 41}); res.Code != http.StatusNoContent {
		t.Errorf("remove: %d %s", res.Code, res.Body)
	}
	if res = api.Get("/api/stores/S1/items/01TESTIT0000000000000000NO/links"); res.Code != http.StatusNotFound {
		t.Errorf("missing item: %d", res.Code)
	}
}

func TestLinksRefreshAnswer(t *testing.T) {
	_, api := humatest.New(t)
	svc := linkServices(t)
	ghapi.Register(api, svc)
	gh, _ := svc.Links.Connector.GitHub.(*apptest.GitHub)
	base := "/api/stores/S1/items/" + string(apptest.SampleTask) + "/links"
	if res := api.Post(base, map[string]any{"kind": "pr", "repo": "org/web", "number": "41", "addToProject": false}); res.Code != http.StatusNoContent {
		t.Fatalf("add: %d %s", res.Code, res.Body)
	}
	gh.Put("org/web", port.Found{
		Kind: itemlink.PR, Repo: "org/web", Number: 41,
		Values: itemlink.Values{Title: "Add login form", State: itemlink.Merged, Review: itemlink.Approved}, Checks: itemlink.Success,
	})
	res := api.Post(base+"/refresh", map[string]any{})
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"state":"merged","review":"approved","checks":"success","error":""`) {
		t.Errorf("refresh: %d %s", res.Code, res.Body)
	}
	gh.Fail("org/web", 41, &ghcall.Failure{Kind: ghcall.Unreachable})
	res = api.Post(base+"/refresh", map[string]any{})
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"checks":"success","error":"状態を更新できませんでした: GitHub に接続できません。接続を確かめて、もう一度お試しください"`) {
		t.Errorf("refresh failure: %d %s", res.Code, res.Body)
	}
}

// linkServices は、accounts_test.go の newServices に、Store S1(サンプル)と、S1 に割り当てたアカウント A1(@taro)と、
// org/web#41 を持つ偽の GitHub で作った Links を足したもの。
func linkServices(t *testing.T) ghapi.Services {
	t.Helper()
	gh := &apptest.GitHub{Users: map[string]string{"tok-taro": "taro"}}
	gh.Put("org/web", port.Found{
		Kind: itemlink.PR, Repo: "org/web", Number: 41,
		Values: itemlink.Values{Title: "Add login form", State: itemlink.Open, Review: itemlink.Pending}, Checks: itemlink.Running,
	})
	svc := newServices(gh)
	clock := apptest.NewClock(time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))
	s := apptest.NewStores(clock, apptest.NewIDs())
	s.AddStore("S1")
	if err := apptest.Sample(t.Context(), s, "S1"); err != nil {
		t.Fatal(err)
	}
	accounts := apptest.NewAccounts()
	if err := accounts.Add(t.Context(), account.Account{ID: "A1", Name: "work", Login: "taro", Method: account.EnvVar}); err != nil {
		t.Fatal(err)
	}
	svc.Links = ghapp.Links{
		Stores: s, Identity: apptest.Identity{Login: "taro"},
		Connector: ghapp.Connector{
			Session: ghapp.NewSession(), Accounts: accounts, Secrets: &apptest.Secrets{Tokens: map[string]string{"taro": "tok-taro"}},
			GitHub: gh, Stores: &apptest.StoreGitHub{Stores: []port.StoreGitHubInfo{{ID: "S1", Name: "チーム", Account: "A1"}}}, Clock: clock,
		},
		Checks: apptest.NewChecks(), Events: &apptest.Events{},
	}
	return svc
}
