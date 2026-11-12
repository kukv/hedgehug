package github_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

type fixture struct {
	accounts *apptest.Accounts
	secrets  *apptest.Secrets
	gh       *apptest.GitHub
	add      github.AddAccount
}

func newFixture() fixture {
	f := fixture{
		accounts: apptest.NewAccounts(),
		secrets:  &apptest.Secrets{Tokens: map[string]string{"taro": "tok-taro", "jiro": "tok-jiro"}},
		gh:       &apptest.GitHub{Users: map[string]string{"tok-taro": "Taro", "tok-jiro": "hanako"}},
	}
	f.add = github.AddAccount{
		Accounts: f.accounts, Secrets: f.secrets, GitHub: f.gh,
		IDs: apptest.NewIDs(), Clock: apptest.NewClock(time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)),
	}
	return f
}

func (f fixture) count(t *testing.T) int {
	t.Helper()
	list, err := f.accounts.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return len(list)
}

func message(err error) string {
	var r *refusal.Refusal
	if errors.As(err, &r) {
		return r.Message
	}
	return ""
}

func TestAddAccount(t *testing.T) {
	f := newFixture()
	got, err := f.add.Do(context.Background(), github.AddAccountInput{Name: " work ", Login: "taro", Method: account.Keychain})
	if err != nil {
		t.Fatal(err)
	}
	// GitHub のログイン名は大文字と小文字を区別しないので、Taro と taro は同じ人。
	if got.ID == "" || got.Name != "work" || got.Login != "taro" || f.count(t) != 1 {
		t.Errorf("added %+v, count %d", got, f.count(t))
	}
	list, _ := github.ListAccounts{Accounts: f.accounts, Session: github.NewSession()}.Do(context.Background())
	if len(list) != 1 || list[0].Account != got {
		t.Errorf("ListAccounts = %+v", list)
	}
}

func TestAddAccountRefusesAndAddsNothing(t *testing.T) {
	cases := []struct {
		name  string
		in    github.AddAccountInput
		setup func(f fixture)
		want  string
	}{
		{
			"input",
			github.AddAccountInput{Name: "", Login: "taro", Method: account.GhCLI},
			nil,
			"表示名とログイン名を入力してください",
		},
		{
			"no token",
			github.AddAccountInput{Name: "w", Login: "saburo", Method: account.Keychain},
			nil,
			"OS のキーチェーンに、サービス hedgehug・アカウント github.com/saburo のトークンが見つかりません",
		},
		{
			"token rejected",
			github.AddAccountInput{Name: "w", Login: "taro", Method: account.GhCLI},
			func(f fixture) { f.gh.Users = map[string]string{} },
			"トークンが GitHub に受け付けられませんでした。認証方法を確かめてください",
		},
		{
			"unreachable",
			github.AddAccountInput{Name: "w", Login: "taro", Method: account.GhCLI},
			func(f fixture) { f.gh.Err = &ghcall.Failure{Kind: ghcall.Unreachable} },
			"GitHub に接続できません。接続を確かめて、もう一度お試しください",
		},
		{
			"login mismatch",
			github.AddAccountInput{Name: "w", Login: "jiro", Method: account.EnvVar},
			nil,
			"このトークンは @hanako のものです。ログイン名を直してください",
		},
	}
	for _, c := range cases {
		f := newFixture()
		if c.setup != nil {
			c.setup(f)
		}
		_, err := f.add.Do(context.Background(), c.in)
		if got := message(err); got != c.want || f.count(t) != 0 {
			t.Errorf("%s: message %q (err %v), count %d; want %q", c.name, got, err, f.count(t), c.want)
		}
	}
}

func TestAddAccountDeviceFlowIsSeparate(t *testing.T) {
	f := newFixture()
	_, err := f.add.Do(context.Background(), github.AddAccountInput{Name: "w", Login: "taro", Method: account.DeviceFlow})
	if !errors.Is(err, github.ErrDeviceFlow) || f.count(t) != 0 {
		t.Errorf("err = %v", err)
	}
}

func TestAddAccountChecksDuplicatesBeforeAskingGitHub(t *testing.T) {
	f := newFixture()
	if _, err := f.add.Do(context.Background(), github.AddAccountInput{Name: "work", Login: "taro", Method: account.GhCLI}); err != nil {
		t.Fatal(err)
	}
	f.gh.Err = errors.New("must not be called")
	_, err := f.add.Do(context.Background(), github.AddAccountInput{Name: "WORK", Login: "jiro", Method: account.GhCLI})
	if got := message(err); got != "同じ表示名のアカウントがあります" {
		t.Errorf("err = %v", err)
	}
}

func errorsAs(err error, target **ghcall.Failure) bool { return errors.As(err, target) }
