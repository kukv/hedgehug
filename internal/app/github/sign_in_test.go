package github_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/github"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

func (f connFixture) signIn() github.SignIn {
	return github.SignIn{
		Session: f.session, Accounts: f.accounts, Secrets: f.secrets, GitHub: f.gh, Device: f.device,
		Hosts: apptest.Hosts{ID: "Iv23abc"}, IDs: apptest.NewIDs(), Clock: f.clock,
	}
}

var granted = port.DeviceGrant{Status: port.DeviceGranted, Token: account.DeviceToken{Access: "tok-saburo", Refresh: "r"}}

func TestSignInAddsAnAccount(t *testing.T) {
	f := newConn(t)
	f.gh.Users["tok-saburo"] = "saburo"
	f.device.Polls = []port.DeviceGrant{{Status: port.DevicePending}, granted}
	s := f.signIn()
	code, err := s.Start(context.Background(), github.SignInInput{Name: " 副業 ", Login: "saburo"})
	if err != nil || code.ID == "" || code.UserCode != "WDJB-MJHT" || code.Interval != 5*time.Second {
		t.Fatalf("Start = %+v, %v", code, err)
	}
	if r, err := s.Poll(context.Background(), code.ID); err != nil || r.State != github.SignInWaiting {
		t.Fatalf("1st Poll = %+v, %v", r, err)
	}
	r, err := s.Poll(context.Background(), code.ID)
	if err != nil || r.State != github.SignInDone || r.Account.Name != "副業" || r.Account.Method != account.DeviceFlow {
		t.Fatalf("2nd Poll = %+v, %v", r, err)
	}
	if list, _ := f.accounts.List(context.Background()); len(list) != 3 || list[2] != r.Account {
		t.Errorf("accounts %+v", list)
	}
	if f.secrets.Device["saburo"].Access != "tok-saburo" {
		t.Errorf("keychain %+v", f.secrets.Device)
	}
	// 終わった流れは、もう問い合わせられない。
	if _, err := s.Poll(context.Background(), code.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("after done: %v", err)
	}
}

func TestSignInStartRefusals(t *testing.T) {
	f := newConn(t)
	s := f.signIn()
	if _, err := s.Start(context.Background(), github.SignInInput{Name: "work", Login: "x"}); message(err) != "同じ表示名のアカウントがあります" {
		t.Errorf("same name: %v", err)
	}
	s.Hosts = apptest.Hosts{}
	want := "デバイスフローを使うには、config.yaml の hosts に github.com の client_id を設定してください。gh CLI のトークン・環境変数・OS のキーチェーンは、設定なしで使えます"
	if _, err := s.Start(context.Background(), github.SignInInput{Name: "x", Login: "x"}); message(err) != want {
		t.Errorf("no client id: %v", err)
	}
	s.Hosts = apptest.Hosts{ID: "Iv23abc"}
	f.device.StartErr = &ghcall.Failure{Kind: ghcall.Unreachable}
	if _, err := s.Start(context.Background(), github.SignInInput{Name: "x", Login: "x"}); message(err) != "GitHub に接続できません。接続を確かめて、もう一度お試しください" {
		t.Errorf("unreachable: %v", err)
	}
	if f.count(t) != 2 {
		t.Error("an account was added")
	}
}

func (f connFixture) count(t *testing.T) int {
	t.Helper()
	list, _ := f.accounts.List(context.Background())
	return len(list)
}

func TestSignInWaitsWhileOffline(t *testing.T) {
	f := newConn(t)
	f.device.PollErr = &ghcall.Failure{Kind: ghcall.Unreachable}
	s := f.signIn()
	code, _ := s.Start(context.Background(), github.SignInInput{Name: "x", Login: "x"})
	if r, err := s.Poll(context.Background(), code.ID); err != nil || r.State != github.SignInWaiting {
		t.Errorf("Poll = %+v, %v", r, err)
	}
}

func TestSignInSlowDown(t *testing.T) {
	f := newConn(t)
	f.device.Polls = []port.DeviceGrant{{Status: port.DeviceSlowDown}}
	s := f.signIn()
	code, _ := s.Start(context.Background(), github.SignInInput{Name: "x", Login: "x"})
	if r, _ := s.Poll(context.Background(), code.ID); r.State != github.SignInWaiting || r.Interval != 10*time.Second {
		t.Errorf("Poll = %+v", r)
	}
}

func TestSignInEnds(t *testing.T) {
	cases := map[port.DeviceStatus]github.SignInState{port.DeviceExpired: github.SignInExpired, port.DeviceDenied: github.SignInDenied}
	for status, want := range cases {
		f := newConn(t)
		f.device.Polls = []port.DeviceGrant{{Status: status}}
		s := f.signIn()
		code, _ := s.Start(context.Background(), github.SignInInput{Name: "x", Login: "x"})
		if r, _ := s.Poll(context.Background(), code.ID); r.State != want {
			t.Errorf("%s: %+v", status, r)
		}
		if _, err := s.Poll(context.Background(), code.ID); !errors.Is(err, port.ErrNotFound) {
			t.Errorf("%s: the flow is left: %v", status, err)
		}
	}
}

func TestSignInExpiresByTheClock(t *testing.T) {
	f := newConn(t)
	s := f.signIn()
	code, _ := s.Start(context.Background(), github.SignInInput{Name: "x", Login: "x"})
	f.clock.Step = 16 * time.Minute
	f.clock.Now()
	if r, _ := s.Poll(context.Background(), code.ID); r.State != github.SignInExpired {
		t.Errorf("Poll = %+v", r)
	}
}

func TestSignInByAnotherPerson(t *testing.T) {
	f := newConn(t)
	f.gh.Users["tok-saburo"] = "shiro"
	f.device.Polls = []port.DeviceGrant{granted}
	s := f.signIn()
	code, _ := s.Start(context.Background(), github.SignInInput{Name: "x", Login: "saburo"})
	r, err := s.Poll(context.Background(), code.ID)
	if err != nil || r.State != github.SignInMismatch || r.Message != "このトークンは @shiro のものです。ログイン名を直してください" {
		t.Errorf("Poll = %+v, %v", r, err)
	}
	if f.count(t) != 2 || len(f.secrets.Device) != 0 {
		t.Error("an account or a token was kept")
	}
}

func TestSignInWithoutKeychain(t *testing.T) {
	f := newConn(t)
	f.gh.Users["tok-saburo"] = "saburo"
	f.secrets.NoKeychain = true
	f.device.Polls = []port.DeviceGrant{granted}
	s := f.signIn()
	code, _ := s.Start(context.Background(), github.SignInInput{Name: "x", Login: "saburo"})
	r, _ := s.Poll(context.Background(), code.ID)
	if r.State != github.SignInFailed || r.Message != "このマシンでは OS のキーチェーンを使えません。gh CLI のトークンか環境変数を使ってください" || f.count(t) != 2 {
		t.Errorf("Poll = %+v", r)
	}
}

func TestSignInReauthsAndEdits(t *testing.T) {
	f := newConn(t)
	f.gh.Users["tok-saburo"] = "hanako"
	f.device.Polls = []port.DeviceGrant{granted}
	f.conn.Rejected(home.ID)
	s := f.signIn()
	// 再認証: いまの値のまま、アカウントの ID を渡す。
	code, err := s.Start(context.Background(), github.SignInInput{Account: home.ID, Name: home.Name, Login: home.Login})
	if err != nil {
		t.Fatal(err)
	}
	if r, _ := s.Poll(context.Background(), code.ID); r.State != github.SignInDone || r.Account != home {
		t.Fatalf("reauth: %+v", r)
	}
	if f.state(t, 1).AuthError || f.count(t) != 2 {
		t.Errorf("after reauth: %+v", f.state(t, 1))
	}
	// 編集: work を、デバイスフローの saburo に替える。古いキーチェーンの項目は、デバイスフローでなかったので触らない。
	f.gh.Users["tok-saburo"] = "saburo"
	f.device.Polls = []port.DeviceGrant{granted}
	code, _ = s.Start(context.Background(), github.SignInInput{Account: work.ID, Name: "work", Login: "saburo"})
	r, _ := s.Poll(context.Background(), code.ID)
	want := account.Account{ID: work.ID, Name: "work", Login: "saburo", Method: account.DeviceFlow}
	if r.State != github.SignInDone || r.Account != want {
		t.Fatalf("edit: %+v", r)
	}
	if list, _ := f.accounts.List(context.Background()); list[0] != want {
		t.Errorf("accounts %+v", list)
	}
}

func TestSignInEditMovesTheDeviceToken(t *testing.T) {
	f := newConn(t)
	f.secrets.Device = map[string]account.DeviceToken{"hanako": {Access: "tok-old"}}
	f.gh.Users["tok-saburo"] = "saburo"
	f.device.Polls = []port.DeviceGrant{granted}
	s := f.signIn()
	code, _ := s.Start(context.Background(), github.SignInInput{Account: home.ID, Name: home.Name, Login: "saburo"})
	if r, _ := s.Poll(context.Background(), code.ID); r.State != github.SignInDone {
		t.Fatalf("Poll = %+v", r)
	}
	if _, ok := f.secrets.Device["hanako"]; ok {
		t.Error("the token of the old login was left")
	}
}

func TestSignInCancel(t *testing.T) {
	f := newConn(t)
	s := f.signIn()
	code, _ := s.Start(context.Background(), github.SignInInput{Name: "x", Login: "x"})
	s.Cancel(code.ID)
	if _, err := s.Poll(context.Background(), code.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Poll = %v", err)
	}
}

func TestSignInWaitsWhenTheOwnerCannotBeChecked(t *testing.T) {
	f := newConn(t)
	f.device.Polls = []port.DeviceGrant{granted, {Status: port.DeviceExpired}}
	f.gh.Err = &ghcall.Failure{Kind: ghcall.Unreachable}
	s := f.signIn()
	code, _ := s.Start(context.Background(), github.SignInInput{Name: "x", Login: "saburo"})
	if r, err := s.Poll(context.Background(), code.ID); err != nil || r.State != github.SignInWaiting {
		t.Fatalf("offline after the grant: %+v, %v", r, err)
	}
	// つながったら、GitHub に許可を聞き直さずに(device_code はもう使えない)、持ち主だけを確かめて登録する。
	f.gh.Err = nil
	f.gh.Users["tok-saburo"] = "saburo"
	r, err := s.Poll(context.Background(), code.ID)
	if err != nil || r.State != github.SignInDone || f.count(t) != 3 {
		t.Errorf("back online: %+v, %v", r, err)
	}
}

func TestSignInCanceledAfterTheGrant(t *testing.T) {
	f := newConn(t)
	f.device.Polls = []port.DeviceGrant{granted}
	f.gh.Err = &ghcall.Failure{Kind: ghcall.Unreachable}
	s := f.signIn()
	code, _ := s.Start(context.Background(), github.SignInInput{Name: "x", Login: "saburo"})
	_, _ = s.Poll(context.Background(), code.ID)
	s.Cancel(code.ID)
	f.gh.Err = nil
	f.gh.Users["tok-saburo"] = "saburo"
	if _, err := s.Poll(context.Background(), code.ID); !errors.Is(err, port.ErrNotFound) || f.count(t) != 2 || len(f.secrets.Device) != 0 {
		t.Errorf("Poll = %v, count %d, keychain %v", err, f.count(t), f.secrets.Device)
	}
}
