package github_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// memMissing は port.AppMissing の偽物(記録の順を保つ)。
type memMissing struct {
	rows map[account.ID][]port.MissingOrg
}

func (m *memMissing) List(context.Context) (map[account.ID][]port.MissingOrg, error) {
	out := map[account.ID][]port.MissingOrg{}
	for k, v := range m.rows {
		out[k] = slices.Clone(v)
	}
	return out, nil
}

func (m *memMissing) Put(_ context.Context, a account.ID, o port.MissingOrg) (bool, error) {
	m.rows[a] = append(m.rows[a], o)
	return true, nil
}

func (m *memMissing) Clear(_ context.Context, a account.ID, org, repo string) (bool, error) {
	for i, x := range m.rows[a] {
		if strings.EqualFold(x.Org, org) && strings.EqualFold(x.Repo, repo) {
			m.rows[a] = slices.Delete(slices.Clone(m.rows[a]), i, i+1)
			return true, nil
		}
	}
	return false, nil
}

func (m *memMissing) Forget(_ context.Context, a account.ID) error {
	delete(m.rows, a)
	return nil
}

type recheckFixture struct {
	connFixture
	missing *memMissing
	do      github.RecheckOrgs
}

func newRecheck(t *testing.T) recheckFixture {
	t.Helper()
	f := recheckFixture{connFixture: newConn(t), missing: &memMissing{rows: map[account.ID][]port.MissingOrg{
		"A1": {{Org: "org-a", Repo: "org-a/web"}, {Org: "Org-B", Repo: "Org-B/api"}},
	}}}
	f.gh.MissApp("org-a", true)
	f.gh.MissApp("org-b", true)
	f.do = github.RecheckOrgs{Accounts: f.accounts, Conn: f.conn, GitHub: f.gh, Missing: f.missing}
	return f
}

func (f recheckFixture) kept(t *testing.T) []port.MissingOrg {
	t.Helper()
	got, err := f.missing.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return got["A1"]
}

func TestListAccountsAppMissing(t *testing.T) {
	f := newRecheck(t)
	list := github.ListAccounts{Accounts: f.accounts, Session: f.session, Missing: f.missing}
	rows, err := list.Do(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(rows[0].Orgs, []string{"org-a", "Org-B"}) || len(rows[1].Orgs) != 0 {
		t.Errorf("orgs = %v / %v", rows[0].Orgs, rows[1].Orgs)
	}
	f.conn.Rejected("A1")
	rows, _ = list.Do(t.Context())
	if !rows[0].AuthError || len(rows[0].Orgs) != 0 {
		t.Errorf("auth error row = %+v", rows[0])
	}
	if len(f.kept(t)) != 2 {
		t.Errorf("records must stay: %v", f.kept(t))
	}
}

func TestRecheckOrgsClears(t *testing.T) {
	f := newRecheck(t)
	f.gh.MissApp("org-a", false)
	got, err := f.do.Do(t.Context(), "A1")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Cleared, []string{"org-a"}) || !slices.Equal(got.Still, []string{"Org-B"}) {
		t.Errorf("result = %+v", got)
	}
	if !slices.Equal(f.kept(t), []port.MissingOrg{{Org: "Org-B", Repo: "Org-B/api"}}) {
		t.Errorf("records = %v", f.kept(t))
	}
}

func TestRecheckOrgsRejected(t *testing.T) {
	f := newRecheck(t)
	f.gh.MissApp("org-a", false)
	f.gh.Fail("org-a/web", 0, &ghcall.Failure{Kind: ghcall.TokenRejected})
	_, err := f.do.Do(t.Context(), "A1")
	want := "work(@taro)の認証エラーのため、GitHub に接続できません。設定の「アカウントとメンバー」で再認証してください"
	if message(err) != want {
		t.Fatalf("err = %v", err)
	}
	list := github.ListAccounts{Accounts: f.accounts, Session: f.session, Missing: f.missing}
	if rows, _ := list.Do(t.Context()); !rows[0].AuthError {
		t.Error("the account must become an auth error")
	}
	if len(f.kept(t)) != 2 {
		t.Errorf("records = %v", f.kept(t))
	}
}

// 通信の失敗は、その組織を残して残りの組織を続け、最後に 1 つ目の失敗を返す(D4b 最終レビュー I-2)。
func TestRecheckOrgsUnreachable(t *testing.T) {
	f := newRecheck(t)
	f.gh.MissApp("org-a", false)
	f.gh.MissApp("org-b", false)
	f.gh.Fail("org-a/web", 0, &ghcall.Failure{Kind: ghcall.Unreachable})
	got, err := f.do.Do(t.Context(), "A1")
	if message(err) != "GitHub に接続できません。接続を確かめて、もう一度お試しください" {
		t.Fatalf("err = %v", err)
	}
	if !slices.Equal(got.Cleared, []string{"Org-B"}) || !slices.Equal(got.Still, []string{"org-a"}) {
		t.Errorf("result = %+v", got)
	}
	if !slices.Equal(f.kept(t), []port.MissingOrg{{Org: "org-a", Repo: "org-a/web"}}) {
		t.Errorf("records = %v", f.kept(t))
	}
}

// 記録したリポジトリが App 未導入でなく、ただ読めない(打ち間違い・削除)なら、App はもう妨げていないので
// 記録を消す(D4b 最終レビュー I-2)。
func TestRecheckOrgsUnreadableClears(t *testing.T) {
	f := newRecheck(t)
	f.gh.MissApp("org-a", false)
	f.gh.Fail("org-a/web", 0, &ghcall.Failure{Kind: ghcall.RepoUnreadable, Repo: "org-a/web"})
	got, err := f.do.Do(t.Context(), "A1")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Cleared, []string{"org-a"}) || !slices.Equal(got.Still, []string{"Org-B"}) {
		t.Errorf("result = %+v", got)
	}
	if !slices.Equal(f.kept(t), []port.MissingOrg{{Org: "Org-B", Repo: "Org-B/api"}}) {
		t.Errorf("records = %v", f.kept(t))
	}
}

// アカウントを削除したら、そのアカウントの App 未導入の記録も消す(D4b 最終レビュー I-2)。
func TestDeleteAccountForgetsAppMissing(t *testing.T) {
	f := newRecheck(t)
	f.missing.rows["A2"] = []port.MissingOrg{{Org: "org-c", Repo: "org-c/x"}}
	del := github.DeleteAccount{Session: f.session, Accounts: f.accounts, Secrets: f.secrets, Stores: f.stores, Missing: f.missing}
	if err := del.Do(t.Context(), "A1"); err != nil {
		t.Fatal(err)
	}
	got, err := f.missing.List(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(got["A1"]) != 0 || len(got["A2"]) != 1 {
		t.Errorf("records = %v", got)
	}
}
