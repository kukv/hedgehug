package github_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2/humatest"

	ghapi "github.com/kukv/hedgehug/internal/adapter/httpapi/github"
	"github.com/kukv/hedgehug/internal/app/apptest"
	ghapp "github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
)

// oneMissing は、アカウント 1 つに記録を 1 つ持つ port.AppMissing の偽物。
type oneMissing struct {
	rows map[account.ID][]port.MissingOrg
}

func (m *oneMissing) List(context.Context) (map[account.ID][]port.MissingOrg, error) {
	return m.rows, nil
}

func (m *oneMissing) Put(_ context.Context, a account.ID, o port.MissingOrg) (bool, error) {
	m.rows[a] = append(m.rows[a], o)
	return true, nil
}

func (m *oneMissing) Clear(_ context.Context, a account.ID, org, repo string) (bool, error) {
	kept := m.rows[a][:0:0]
	for _, x := range m.rows[a] {
		if !strings.EqualFold(x.Org, org) || !strings.EqualFold(x.Repo, repo) {
			kept = append(kept, x)
		}
	}
	m.rows[a] = kept
	return true, nil
}

func (m *oneMissing) Forget(_ context.Context, a account.ID) error {
	delete(m.rows, a)
	return nil
}

// newMissingAPI は、example-org が App 未導入のアカウントを 1 つ登録した API と、その ID。
func newMissingAPI(t *testing.T, gh *apptest.GitHub) (humatest.TestAPI, string, *oneMissing) {
	t.Helper()
	svc := newServices(gh)
	m := &oneMissing{rows: map[account.ID][]port.MissingOrg{}}
	conn := ghapp.Connector{
		Session: svc.List.Session, Accounts: svc.List.Accounts, Secrets: svc.Reauth.Secrets, GitHub: gh, Clock: svc.Reauth.Clock,
	}
	svc.List.Missing = m
	svc.Recheck = ghapp.RecheckOrgs{Accounts: svc.List.Accounts, Conn: conn, GitHub: gh, Missing: m}
	_, api := humatest.New(t)
	ghapi.Register(api, svc)
	id := addTaro(t, api)
	m.rows[account.ID(id)] = []port.MissingOrg{{Org: "example-org", Repo: "example-org/web"}}
	gh.MissApp("example-org", true)
	return api, id, m
}

func TestAccountsStatusAppMissing(t *testing.T) {
	api, _, _ := newMissingAPI(t, &apptest.GitHub{Users: map[string]string{"tok-taro": "taro"}})
	res := api.Get("/api/accounts")
	var body struct {
		Accounts []struct {
			Status string
			Orgs   []string
		}
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Accounts) != 1 || body.Accounts[0].Status != "app_missing" || len(body.Accounts[0].Orgs) != 1 || body.Accounts[0].Orgs[0] != "example-org" {
		t.Errorf("GET: %s", res.Body)
	}
}

func TestRecheckAccount(t *testing.T) {
	gh := &apptest.GitHub{Users: map[string]string{"tok-taro": "taro"}}
	api, id, m := newMissingAPI(t, gh)
	res := api.Post("/api/accounts/"+id+"/recheck", map[string]string{})
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"still":["example-org"]`) || !strings.Contains(res.Body.String(), `"cleared":[]`) {
		t.Fatalf("still missing: %d %s", res.Code, res.Body)
	}
	gh.MissApp("example-org", false)
	res = api.Post("/api/accounts/"+id+"/recheck", map[string]string{})
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"cleared":["example-org"]`) || !strings.Contains(res.Body.String(), `"still":[]`) {
		t.Fatalf("cleared: %d %s", res.Code, res.Body)
	}
	if len(m.rows[account.ID(id)]) != 0 {
		t.Errorf("records = %v", m.rows)
	}
	if res = api.Post("/api/accounts/nope/recheck", map[string]string{}); res.Code != http.StatusNotFound {
		t.Errorf("unknown account: %d %s", res.Code, res.Body)
	}
}
