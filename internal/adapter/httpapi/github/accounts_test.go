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
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

func newAPI(t *testing.T, gh *apptest.GitHub) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t)
	accounts := apptest.NewAccounts()
	secrets := &apptest.Secrets{Tokens: map[string]string{"taro": "tok-taro"}}
	ghapi.Register(api,
		ghapp.ListAccounts{Accounts: accounts},
		ghapp.AddAccount{
			Accounts: accounts, Secrets: secrets, GitHub: gh, IDs: apptest.NewIDs(),
			Clock: apptest.NewClock(time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)),
		})
	return api
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
