package github_test

import (
	"context"
	"errors"
	"strings"
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

// 期限を過ぎた流れは、次のサインインを始めるときに捨てる(閉じたダイアログの流れを残し続けない)。
func TestSignInDropsExpiredFlows(t *testing.T) {
	f := newConn(t)
	s := f.signIn()
	first, _ := s.Start(context.Background(), github.SignInInput{Name: "x", Login: "x"})
	f.clock.Step = 16 * time.Minute
	f.clock.Now()
	if _, err := s.Start(context.Background(), github.SignInInput{Name: "y", Login: "y"}); err != nil {
		t.Fatal(err)
	}
	if r, err := s.Poll(context.Background(), first.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Poll = %+v, %v", r, err)
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

// cancelingGitHub は、持ち主を確かめているあいだに、ダイアログが閉じられたことにする。
type cancelingGitHub struct {
	port.GitHub
	cancel func()
}

func (g cancelingGitHub) Viewer(ctx context.Context, token string) (string, error) {
	g.cancel()
	return g.GitHub.Viewer(ctx, token)
}

// 許可のあとの保存とキャンセルが重なったら、何も置かず、何も登録しない(裁定 Q4)。
func TestSignInCancelledWhileSaving(t *testing.T) {
	f := newConn(t)
	f.gh.Users["tok-saburo"] = "saburo"
	f.device.Polls = []port.DeviceGrant{granted}
	s := f.signIn()
	code, _ := s.Start(context.Background(), github.SignInInput{Name: "x", Login: "saburo"})
	s.GitHub = cancelingGitHub{GitHub: f.gh, cancel: func() { s.Cancel(code.ID) }}
	if r, err := s.Poll(context.Background(), code.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Poll = %+v, %v", r, err)
	}
	if f.count(t) != 2 || len(f.secrets.Device) != 0 {
		t.Error("an account or a token was kept")
	}
}

// blockingGitHub は、持ち主を確かめているあいだ、release が閉じられるまで待つ(entered で待ちに入ったことを知らせる)。
type blockingGitHub struct {
	port.GitHub
	entered chan struct{}
	release chan struct{}
}

func (g blockingGitHub) Viewer(ctx context.Context, token string) (string, error) {
	close(g.entered)
	<-g.release
	return g.GitHub.Viewer(ctx, token)
}

// 同じ流れの Poll が重なったら、あとの Poll は GitHub に聞かずに待つと答える。許可を受けた Poll の登録を失わせない。
func TestSignInOverlappingPollWaits(t *testing.T) {
	f := newConn(t)
	f.gh.Users["tok-saburo"] = "saburo"
	f.device.Polls = []port.DeviceGrant{granted, {Status: port.DeviceExpired}}
	s := f.signIn()
	code, _ := s.Start(context.Background(), github.SignInInput{Name: "x", Login: "saburo"})
	b := blockingGitHub{GitHub: f.gh, entered: make(chan struct{}), release: make(chan struct{})}
	s.GitHub = b
	type out struct {
		r   github.SignInResult
		err error
	}
	first := make(chan out, 1)
	go func() {
		r, err := s.Poll(context.Background(), code.ID)
		first <- out{r, err}
	}()
	<-b.entered
	if r, err := s.Poll(context.Background(), code.ID); err != nil || r.State != github.SignInWaiting || r.Interval != code.Interval {
		t.Errorf("overlapping Poll = %+v, %v", r, err)
	}
	close(b.release)
	if o := <-first; o.err != nil || o.r.State != github.SignInDone {
		t.Errorf("1st Poll = %+v, %v", o.r, o.err)
	}
	if f.count(t) != 3 {
		t.Errorf("accounts = %d, want 3", f.count(t))
	}
}

// 許可を待つあいだに同じ表示名・ログイン名のアカウントが登録されたら、保存の直前に断る(裁定 Q1)。
func TestSignInRechecksBeforeSaving(t *testing.T) {
	cases := map[string]account.Account{
		"同じログイン名のアカウントがあります": {ID: "A3", Name: "別", Login: "Saburo", Method: account.DeviceFlow},
		"同じ表示名のアカウントがあります":   {ID: "A3", Name: "X", Login: "jiro", Method: account.GhCLI},
	}
	for want, other := range cases {
		f := newConn(t)
		f.gh.Users["tok-saburo"] = "saburo"
		f.device.Polls = []port.DeviceGrant{granted}
		s := f.signIn()
		code, _ := s.Start(context.Background(), github.SignInInput{Name: "x", Login: "saburo"})
		if err := f.accounts.Add(context.Background(), other); err != nil {
			t.Fatal(err)
		}
		r, err := s.Poll(context.Background(), code.ID)
		if err != nil || r.State != github.SignInFailed || r.Message != want {
			t.Errorf("%s: Poll = %+v, %v", want, r, err)
		}
		if _, ok := f.secrets.Device["saburo"]; ok || f.count(t) != 3 {
			t.Errorf("%s: a token or an account was kept: %+v", want, f.secrets.Device)
		}
	}
}

// 削除したアカウントの編集・再認証のサインインの流れは捨てる。
func TestSignInOfADeletedAccount(t *testing.T) {
	f := newConn(t)
	f.gh.Users["tok-saburo"] = "hanako"
	f.device.Polls = []port.DeviceGrant{granted}
	s := f.signIn()
	code, _ := s.Start(context.Background(), github.SignInInput{Account: home.ID, Name: home.Name, Login: home.Login})
	if err := f.del().Do(context.Background(), home.ID); err != nil {
		t.Fatal(err)
	}
	if r, err := s.Poll(context.Background(), code.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Poll = %+v, %v", r, err)
	}
	if len(f.secrets.Device) != 0 {
		t.Errorf("a token was kept: %+v", f.secrets.Device)
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
	if _, err := s.Poll(context.Background(), code.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("the flow is left: %v", err)
	}
}

// 許可のあとで持ち主を確かめられないときは、トークンを持ったまま待ち、次は確かめだけをやり直す。
// device_code はもう使えないので、PollDevice をもう一度呼んだら expired になる。
func TestSignInRetriesTheOwnerCheck(t *testing.T) {
	for _, kind := range []ghcall.Kind{ghcall.Unreachable, ghcall.RateLimited} {
		f := newConn(t)
		f.gh.Users["tok-saburo"] = "saburo"
		f.gh.Err = &ghcall.Failure{Kind: kind}
		f.device.Polls = []port.DeviceGrant{granted, {Status: port.DeviceExpired}}
		s := f.signIn()
		code, _ := s.Start(context.Background(), github.SignInInput{Name: "x", Login: "saburo"})
		if r, err := s.Poll(context.Background(), code.ID); err != nil || r.State != github.SignInWaiting || r.Interval != 5*time.Second {
			t.Fatalf("%s: 1st Poll = %+v, %v", kind, r, err)
		}
		f.gh.Err = nil
		if r, err := s.Poll(context.Background(), code.ID); err != nil || r.State != github.SignInDone {
			t.Fatalf("%s: 2nd Poll = %+v, %v", kind, r, err)
		}
		if f.count(t) != 3 || f.secrets.Device["saburo"].Access != "tok-saburo" {
			t.Errorf("%s: not added: %+v", kind, f.secrets.Device)
		}
	}
}

// 期限はいちばん先に見る。許可されたトークンを持っていても、期限が過ぎたら捨てる(裁定 Q3)。
func TestSignInExpiresWhileHoldingTheGrant(t *testing.T) {
	f := newConn(t)
	f.gh.Users["tok-saburo"] = "saburo"
	f.gh.Err = &ghcall.Failure{Kind: ghcall.Unreachable}
	f.device.Polls = []port.DeviceGrant{granted}
	s := f.signIn()
	code, _ := s.Start(context.Background(), github.SignInInput{Name: "x", Login: "saburo"})
	if r, _ := s.Poll(context.Background(), code.ID); r.State != github.SignInWaiting {
		t.Fatalf("1st Poll = %+v", r)
	}
	f.gh.Err = nil
	f.clock.Step = 16 * time.Minute
	f.clock.Now()
	if r, err := s.Poll(context.Background(), code.ID); err != nil || r.State != github.SignInExpired {
		t.Errorf("2nd Poll = %+v, %v", r, err)
	}
	if _, err := s.Poll(context.Background(), code.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("the flow is left: %v", err)
	}
	if f.count(t) != 2 || len(f.secrets.Device) != 0 {
		t.Error("an account or a token was kept")
	}
}

func TestSignInOwnerCheckRejected(t *testing.T) {
	f := newConn(t)
	f.device.Polls = []port.DeviceGrant{granted}
	s := f.signIn()
	code, _ := s.Start(context.Background(), github.SignInInput{Name: "x", Login: "saburo"})
	r, err := s.Poll(context.Background(), code.ID)
	if err != nil || r.State != github.SignInFailed || r.Message == "" || strings.Contains(r.Message, "tok-saburo") {
		t.Errorf("Poll = %+v, %v", r, err)
	}
	if f.count(t) != 2 || len(f.secrets.Device) != 0 {
		t.Error("an account or a token was kept")
	}
}

func TestSignInReauthsAndEdits(t *testing.T) {
	f := newConn(t)
	f.secrets.Device = map[string]account.DeviceToken{"taro": {Access: "tok-other"}}
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
	if f.secrets.Device["taro"].Access != "tok-other" {
		t.Errorf("the item of taro was touched: %+v", f.secrets.Device)
	}
}

// デバイスフローのままログイン名を変えたら、古いログイン名の項目を消す(決定 D2-13)。
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

// キーチェーンの名前は大文字と小文字を区別するので、大文字と小文字だけの変更でも古い項目を消す。
func TestSignInEditChangesOnlyTheCase(t *testing.T) {
	f := newConn(t)
	f.secrets.Device = map[string]account.DeviceToken{"hanako": {Access: "tok-old"}}
	f.gh.Users["tok-saburo"] = "hanako"
	f.device.Polls = []port.DeviceGrant{granted}
	s := f.signIn()
	code, _ := s.Start(context.Background(), github.SignInInput{Account: home.ID, Name: home.Name, Login: "Hanako"})
	if r, _ := s.Poll(context.Background(), code.ID); r.State != github.SignInDone {
		t.Fatalf("Poll = %+v", r)
	}
	if _, ok := f.secrets.Device["Hanako"]; !ok {
		t.Errorf("the new item is missing: %+v", f.secrets.Device)
	}
	if _, ok := f.secrets.Device["hanako"]; ok {
		t.Errorf("the old item was left: %+v", f.secrets.Device)
	}
}

// 古い項目を消せなくても、編集は成功にする。
func TestSignInEditWhenTheOldTokenCannotBeDeleted(t *testing.T) {
	f := newConn(t)
	f.secrets.Device = map[string]account.DeviceToken{"hanako": {Access: "tok-old"}}
	f.secrets.DeleteErr = &ghcall.Failure{Kind: ghcall.KeychainUnavailable}
	f.gh.Users["tok-saburo"] = "saburo"
	f.device.Polls = []port.DeviceGrant{granted}
	s := f.signIn()
	code, _ := s.Start(context.Background(), github.SignInInput{Account: home.ID, Name: home.Name, Login: "saburo"})
	if r, err := s.Poll(context.Background(), code.ID); err != nil || r.State != github.SignInDone {
		t.Fatalf("Poll = %+v, %v", r, err)
	}
	if list, _ := f.accounts.List(context.Background()); list[1].Login != "saburo" {
		t.Errorf("accounts %+v", list)
	}
}

// 再認証(ログイン名が同じ)では、項目を新しいトークンで置き換え、消さない。
func TestSignInReauthReplacesTheDeviceToken(t *testing.T) {
	f := newConn(t)
	f.secrets.Device = map[string]account.DeviceToken{"hanako": {Access: "tok-old"}}
	f.gh.Users["tok-saburo"] = "hanako"
	f.device.Polls = []port.DeviceGrant{granted}
	s := f.signIn()
	code, _ := s.Start(context.Background(), github.SignInInput{Account: home.ID, Name: home.Name, Login: home.Login})
	if r, _ := s.Poll(context.Background(), code.ID); r.State != github.SignInDone {
		t.Fatalf("Poll = %+v", r)
	}
	if f.secrets.Device["hanako"].Access != "tok-saburo" {
		t.Errorf("keychain %+v", f.secrets.Device)
	}
}

func TestSignInWithoutKeychain(t *testing.T) {
	f := newConn(t)
	f.gh.Users["tok-saburo"] = "saburo"
	f.secrets.NoKeychain = true
	f.device.Polls = []port.DeviceGrant{granted}
	s := f.signIn()
	code, _ := s.Start(context.Background(), github.SignInInput{Name: "x", Login: "saburo"})
	r, err := s.Poll(context.Background(), code.ID)
	if err != nil || r.State != github.SignInFailed || r.Message != "このマシンでは OS のキーチェーンを使えません。gh CLI のトークンか環境変数を使ってください" || f.count(t) != 2 {
		t.Errorf("Poll = %+v, %v", r, err)
	}
	if _, err := s.Poll(context.Background(), code.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("the flow is left: %v", err)
	}
}
