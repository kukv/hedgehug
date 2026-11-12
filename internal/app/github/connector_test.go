package github_test

import (
	"context"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

var start = time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)

type connFixture struct {
	accounts *apptest.Accounts
	secrets  *apptest.Secrets
	gh       *countingGitHub
	device   *apptest.DeviceFlow
	stores   *apptest.StoreGitHub
	clock    *apptest.Clock
	session  *github.Session
	conn     github.Connector
}

// countingGitHub は、Viewer を呼んだ回数を数える。
type countingGitHub struct {
	apptest.GitHub
	calls int
}

func (g *countingGitHub) Viewer(ctx context.Context, token string) (string, error) {
	g.calls++
	return g.GitHub.Viewer(ctx, token)
}

var (
	work = account.Account{ID: "A1", Name: "work", Login: "taro", Method: account.GhCLI}
	home = account.Account{ID: "A2", Name: "家", Login: "hanako", Method: account.DeviceFlow}
)

func newConn(t *testing.T) connFixture {
	t.Helper()
	f := connFixture{
		accounts: apptest.NewAccounts(),
		secrets:  &apptest.Secrets{Tokens: map[string]string{"taro": "tok-taro"}},
		gh:       &countingGitHub{GitHub: apptest.GitHub{Users: map[string]string{"tok-taro": "taro", "tok-hanako": "hanako"}}},
		device:   apptest.NewDeviceFlow(),
		stores:   &apptest.StoreGitHub{Stores: []port.StoreGitHubInfo{{ID: "S1", Name: "チーム", Account: "A1", Remote: true}, {ID: "S2", Name: "個人"}}},
		clock:    apptest.NewClock(start),
		session:  github.NewSession(),
	}
	for _, a := range []account.Account{work, home} {
		if err := f.accounts.Add(context.Background(), a); err != nil {
			t.Fatal(err)
		}
	}
	f.conn = github.Connector{
		Session: f.session, Accounts: f.accounts, Secrets: f.secrets, GitHub: f.gh, Device: f.device,
		Hosts: apptest.Hosts{ID: "Iv23abc"}, Stores: f.stores, Clock: f.clock,
	}
	return f
}

func TestTokenChecksOnceUntilTheTokenChanges(t *testing.T) {
	f := newConn(t)
	for range 2 {
		if tok, err := f.conn.Token(context.Background(), work); err != nil || tok != "tok-taro" {
			t.Fatalf("Token = %q, %v", tok, err)
		}
	}
	if f.gh.calls != 1 {
		t.Errorf("asked GitHub %d times; want 1 (the first connection only)", f.gh.calls)
	}
	f.secrets.Tokens["taro"] = "tok-taro2"
	f.gh.Users["tok-taro2"] = "taro"
	if _, err := f.conn.Token(context.Background(), work); err != nil || f.gh.calls != 2 {
		t.Errorf("changed token: %v, calls %d", err, f.gh.calls)
	}
}

func TestTokenOfAnotherPersonIsAnAuthError(t *testing.T) {
	f := newConn(t)
	f.secrets.Tokens["taro"] = "tok-hanako"
	_, err := f.conn.Token(context.Background(), work)
	if !ghcall.Is(err, ghcall.AuthError) {
		t.Fatalf("Token = %v", err)
	}
	list, _ := github.ListAccounts{Accounts: f.accounts, Session: f.session}.Do(context.Background())
	if !list[0].AuthError || list[0].Reason != "このトークンは @hanako のものです。ログイン名を直してください" {
		t.Errorf("state = %+v", list[0])
	}
	// 認証エラーのあいだは、別のトークンになっても問い合わせない(戻るのは再認証か、D4 の自動の確かめ直し)。
	f.secrets.Tokens["taro"] = "tok-taro"
	calls := f.gh.calls
	if _, err := f.conn.Token(context.Background(), work); !ghcall.Is(err, ghcall.AuthError) || f.gh.calls != calls {
		t.Errorf("while broken: %v, calls %d → %d", err, calls, f.gh.calls)
	}
}

func TestRejectedTokenIsAnAuthError(t *testing.T) {
	f := newConn(t)
	f.secrets.Tokens["taro"] = "tok-revoked"
	_, err := f.conn.Token(context.Background(), work)
	var fail *ghcall.Failure
	if !errorsAs(err, &fail) || fail.Kind != ghcall.AuthError || fail.Name != "work" || fail.Login != "taro" {
		t.Fatalf("Token = %v", err)
	}
	list, _ := github.ListAccounts{Accounts: f.accounts, Session: f.session}.Do(context.Background())
	if !list[0].AuthError || list[0].Reason != "" {
		t.Errorf("state = %+v", list[0])
	}
}

func TestUnreachableIsNotAnAuthError(t *testing.T) {
	f := newConn(t)
	f.gh.Err = &ghcall.Failure{Kind: ghcall.Unreachable}
	if _, err := f.conn.Token(context.Background(), work); !ghcall.Is(err, ghcall.Unreachable) {
		t.Fatalf("Token = %v", err)
	}
	list, _ := github.ListAccounts{Accounts: f.accounts, Session: f.session}.Do(context.Background())
	if list[0].AuthError {
		t.Error("an unreachable GitHub made the account an auth error")
	}
}

func TestRejectedByALaterCall(t *testing.T) {
	f := newConn(t)
	f.conn.Rejected(work.ID)
	if _, err := f.conn.Token(context.Background(), work); !ghcall.Is(err, ghcall.AuthError) {
		t.Errorf("Token = %v", err)
	}
}

func TestDeviceTokenFreshIsUsedAsItIs(t *testing.T) {
	f := newConn(t)
	f.secrets.Device = map[string]account.DeviceToken{"hanako": {Access: "tok-hanako", Refresh: "r", ExpiresAt: start.Add(time.Hour)}}
	if tok, err := f.conn.Token(context.Background(), home); err != nil || tok != "tok-hanako" {
		t.Errorf("Token = %q, %v", tok, err)
	}
	if f.gh.calls != 0 || f.device.Refreshes != 0 {
		t.Errorf("device flow token was checked (%d) or refreshed (%d)", f.gh.calls, f.device.Refreshes)
	}
}

func TestDeviceTokenIsRefreshedBeforeItEnds(t *testing.T) {
	f := newConn(t)
	f.secrets.Device = map[string]account.DeviceToken{"hanako": {Access: "tok-old", Refresh: "r", ExpiresAt: start.Add(4 * time.Minute)}}
	f.device.Refreshed = account.DeviceToken{Access: "tok-new", Refresh: "r2", ExpiresAt: start.Add(8 * time.Hour)}
	if tok, err := f.conn.Token(context.Background(), home); err != nil || tok != "tok-new" {
		t.Fatalf("Token = %q, %v", tok, err)
	}
	if got := f.secrets.Device["hanako"]; got.Access != "tok-new" || got.Refresh != "r2" {
		t.Errorf("kept %+v", got)
	}
}

func TestDeviceRefreshFailuresAreAuthErrors(t *testing.T) {
	cases := map[string]func(f *connFixture){
		"rejected": func(*connFixture) {},
		"refresh expired": func(f *connFixture) {
			f.device.Refreshed = account.DeviceToken{Access: "tok-new", Refresh: "r2"} // 取り直しに行けば成功する
			f.secrets.Device["hanako"] = account.DeviceToken{Access: "tok-old", Refresh: "r", ExpiresAt: start, RefreshExpiresAt: start}
		},
		"no client id": func(f *connFixture) {
			f.device.Refreshed = account.DeviceToken{Access: "tok-new", Refresh: "r2"}
			f.conn.Hosts = apptest.Hosts{}
		},
		"not in keychain": func(f *connFixture) { delete(f.secrets.Device, "hanako") },
		"cannot keep it": func(f *connFixture) {
			f.device.Refreshed = account.DeviceToken{Access: "tok-new"}
			f.secrets.NoKeychainForSave = true
		},
	}
	for name, setup := range cases {
		f := newConn(t)
		f.secrets.Device = map[string]account.DeviceToken{"hanako": {Access: "tok-old", Refresh: "r", ExpiresAt: start}}
		setup(&f)
		if _, err := f.conn.Token(context.Background(), home); !ghcall.Is(err, ghcall.AuthError) {
			t.Errorf("%s: Token = %v", name, err)
		}
		// 取り直せない場合は、GitHub に問い合わせない。
		if (name == "refresh expired" || name == "no client id") && f.device.Refreshes != 0 {
			t.Errorf("%s: asked GitHub to refresh %d times", name, f.device.Refreshes)
		}
	}
}

func TestDeviceRefreshUnreachableKeepsTheAccount(t *testing.T) {
	// 到達できない・利用上限は、認証エラーにしない(07「状態」の「利用上限では状態を変えない」)。
	for _, kind := range []ghcall.Kind{ghcall.Unreachable, ghcall.RateLimited} {
		f := newConn(t)
		f.secrets.Device = map[string]account.DeviceToken{"hanako": {Access: "tok-old", Refresh: "r", ExpiresAt: start}}
		f.device.RefreshErr = &ghcall.Failure{Kind: kind}
		if _, err := f.conn.Token(context.Background(), home); !ghcall.Is(err, kind) {
			t.Fatalf("%s: Token = %v", kind, err)
		}
		list, _ := github.ListAccounts{Accounts: f.accounts, Session: f.session}.Do(context.Background())
		if list[1].AuthError {
			t.Errorf("%s made the account an auth error", kind)
		}
	}
}

func TestTokenFor(t *testing.T) {
	f := newConn(t)
	tok, a, err := f.conn.TokenFor(context.Background(), "S1", "kukv/hedgehug")
	if err != nil || tok != "tok-taro" || a.ID != work.ID {
		t.Errorf("S1: %q %+v %v", tok, a, err)
	}
	_, _, err = f.conn.TokenFor(context.Background(), "S2", "kukv/hedgehug")
	var fail *ghcall.Failure
	if !errorsAs(err, &fail) || fail.Kind != ghcall.NoAccount || fail.Repo != "kukv/hedgehug" {
		t.Errorf("S2 (no account): %v", err)
	}
}
