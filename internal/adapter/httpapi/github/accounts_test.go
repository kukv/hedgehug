package github_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2/humatest"

	ghapi "github.com/kukv/hedgehug/internal/adapter/httpapi/github"
	"github.com/kukv/hedgehug/internal/app/apptest"
	ghapp "github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

func newAPI(t *testing.T, gh *apptest.GitHub) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	ghapi.Register(api, newServices(gh))
	return api
}

func newServices(gh *apptest.GitHub) ghapi.Services {
	accounts := apptest.NewAccounts()
	secrets := &apptest.Secrets{Tokens: map[string]string{"taro": "tok-taro"}}
	clock := apptest.NewClock(time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC))
	stores := &apptest.StoreGitHub{Login: "taro", Stores: []port.StoreGitHubInfo{{ID: "S1", Name: "チーム", Remote: true}}}
	session := ghapp.NewSession()
	device := apptest.NewDeviceFlow()
	hosts := apptest.Hosts{ID: "Iv23abc"}
	ids := apptest.NewIDs()
	return ghapi.Services{
		List:   ghapp.ListAccounts{Accounts: accounts, Session: session},
		Add:    ghapp.AddAccount{Accounts: accounts, Secrets: secrets, GitHub: gh, IDs: ids, Clock: clock},
		Edit:   ghapp.EditAccount{Session: session, Accounts: accounts, Secrets: secrets, GitHub: gh, Clock: clock},
		Delete: ghapp.DeleteAccount{Session: session, Accounts: accounts, Secrets: secrets, Stores: stores},
		Reauth: ghapp.Reauth{Session: session, Accounts: accounts, Secrets: secrets, GitHub: gh, Clock: clock},
		Impact: ghapp.Impact{Accounts: accounts, Stores: stores},
		SignIn: ghapp.SignIn{Session: session, Accounts: accounts, Secrets: secrets, GitHub: gh, Device: device, Hosts: hosts, IDs: ids, Clock: clock},
	}
}

func TestAddAndList(t *testing.T) {
	api := newAPI(t, &apptest.GitHub{Users: map[string]string{"tok-taro": "taro"}})
	res := api.Post("/api/accounts", map[string]string{"name": "work", "login": "taro", "method": "gh"})
	if res.Code != http.StatusCreated || !strings.Contains(res.Body.String(), `"login":"taro"`) {
		t.Fatalf("POST: %d %s", res.Code, res.Body)
	}
	res = api.Get("/api/accounts")
	var body struct {
		Accounts []struct{ ID, Name, Login, Method string }
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Accounts) != 1 || body.Accounts[0].Method != "gh" || body.Accounts[0].ID == "" {
		t.Errorf("GET: %s", res.Body)
	}
}

func TestRefusalsAre422WithTheSpecText(t *testing.T) {
	cases := []struct {
		name string
		gh   *apptest.GitHub
		body map[string]string
		want string
	}{
		{"input", &apptest.GitHub{}, map[string]string{"name": " ", "login": "taro", "method": "gh"}, "表示名とログイン名を入力してください"},
		{"mismatch", &apptest.GitHub{Users: map[string]string{"tok-taro": "jiro"}}, map[string]string{"name": "w", "login": "taro", "method": "gh"}, "このトークンは @jiro のものです。ログイン名を直してください"},
		{"unreachable", &apptest.GitHub{Err: &ghcall.Failure{Kind: ghcall.Unreachable}}, map[string]string{"name": "w", "login": "taro", "method": "gh"}, "GitHub に接続できません。接続を確かめて、もう一度お試しください"},
	}
	for _, c := range cases {
		res := newAPI(t, c.gh).Post("/api/accounts", c.body)
		var body struct{ Detail string }
		_ = json.Unmarshal(res.Body.Bytes(), &body)
		if res.Code != http.StatusUnprocessableEntity || body.Detail != c.want {
			t.Errorf("%s: %d %s, want detail %q", c.name, res.Code, res.Body, c.want)
		}
	}
}

func TestDeviceFlowIsNotAcceptedHere(t *testing.T) {
	res := newAPI(t, &apptest.GitHub{}).Post("/api/accounts", map[string]string{"name": "w", "login": "taro", "method": "device"})
	if res.Code != http.StatusUnprocessableEntity {
		t.Errorf("status %d", res.Code)
	}
}

func TestEditDeleteReauthImpact(t *testing.T) {
	gh := &apptest.GitHub{Users: map[string]string{"tok-taro": "taro"}}
	api := newAPI(t, gh)
	res := api.Post("/api/accounts", map[string]string{"name": "work", "login": "taro", "method": "gh"})
	var acc struct{ ID string }
	_ = json.Unmarshal(res.Body.Bytes(), &acc)

	res = api.Put("/api/accounts/"+acc.ID, map[string]string{"name": "仕事", "login": "taro", "method": "keychain"})
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"name":"仕事"`) {
		t.Fatalf("PUT: %d %s", res.Code, res.Body)
	}
	res = api.Post("/api/accounts/"+acc.ID+"/reauth", map[string]string{})
	if res.Code != http.StatusOK {
		t.Fatalf("reauth: %d %s", res.Code, res.Body)
	}
	res = api.Get("/api/accounts/" + acc.ID + "/edit-impact?login=jiro")
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"lines":[]`) {
		t.Errorf("edit-impact (no store uses it): %d %s", res.Code, res.Body)
	}
	res = api.Get("/api/accounts/" + acc.ID + "/delete-impact")
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "「仕事」を削除します。") {
		t.Errorf("delete-impact: %d %s", res.Code, res.Body)
	}
	if res = api.Delete("/api/accounts/" + acc.ID); res.Code != http.StatusNoContent {
		t.Fatalf("DELETE: %d %s", res.Code, res.Body)
	}
	if res = api.Delete("/api/accounts/" + acc.ID); res.Code != http.StatusNotFound {
		t.Errorf("DELETE again: %d", res.Code)
	}
}

func TestListShowsAuthError(t *testing.T) {
	gh := &apptest.GitHub{Users: map[string]string{"tok-taro": "taro"}}
	api := newAPI(t, gh)
	res := api.Post("/api/accounts", map[string]string{"name": "work", "login": "taro", "method": "gh"})
	var acc struct{ ID string }
	_ = json.Unmarshal(res.Body.Bytes(), &acc)
	gh.Err = &ghcall.Failure{Kind: ghcall.TokenRejected}
	res = api.Post("/api/accounts/"+acc.ID+"/reauth", map[string]string{})
	if res.Code != http.StatusUnprocessableEntity || !strings.Contains(res.Body.String(), "トークンが GitHub に受け付けられませんでした") {
		t.Errorf("reauth: %d %s", res.Code, res.Body)
	}
	res = api.Get("/api/accounts")
	if !strings.Contains(res.Body.String(), `"status":"ok"`) {
		t.Errorf("a failed reauth changed the state: %s", res.Body)
	}
}
