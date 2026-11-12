package github

import (
	"context"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
	"github.com/kukv/hedgehug/internal/domain/refusal"
)

// noClientIDText は、client_id がないときの「アカウントを追加」の赤字(07「デバイスフロー」)。
const noClientIDText = "デバイスフローを使うには、config.yaml の hosts に github.com の client_id を設定してください。gh CLI のトークン・環境変数・OS のキーチェーンは、設定なしで使えます"

// slowDownStep は、GitHub が slow_down を返したときに延ばす間隔(RFC 8628)。
const slowDownStep = 5 * time.Second

// SignIn は、デバイスフローのサインイン(07「デバイスフロー」)。登録・編集・再認証で使う。
// 流れはサーバーのメモリに持ち、画面は ID で問い合わせる(この小計画の決定 D2-8・D2-9)。
type SignIn struct {
	Session  *Session
	Accounts port.Accounts
	Secrets  port.Secrets
	GitHub   port.GitHub
	Device   port.DeviceFlow
	Hosts    port.Hosts
	IDs      port.IDs
	Clock    port.Clock
}

// SignInInput の Account が空なら登録、あれば、そのアカウントの編集か再認証(再認証はいまの値を渡す)。
type SignInInput struct {
	Account account.ID
	Name    string
	Login   string
}

// SignInCode は、画面に出すコード。device_code は持たない。
type SignInCode struct {
	ID              string
	UserCode        string
	VerificationURI string
	ExpiresIn       time.Duration
	Interval        time.Duration
}

type SignInState string

const (
	SignInWaiting  SignInState = "waiting"
	SignInExpired  SignInState = "expired"
	SignInDenied   SignInState = "denied"
	SignInMismatch SignInState = "mismatch" // ログイン名の欄の下に Message を出す
	SignInFailed   SignInState = "failed"   // 「アカウントを追加」に Message を出す
	SignInDone     SignInState = "done"
)

type SignInResult struct {
	State    SignInState
	Message  string
	Interval time.Duration // 次に問い合わせるまでの間隔
	Account  account.Account
}

// flow はサインインの途中の流れ。
type flow struct {
	target     account.Account // 登録・保存する値(登録では ID が空)
	previous   account.Account // 編集・再認証の前の値
	clientID   string
	deviceCode string
	expiresAt  time.Time
	interval   time.Duration
}

func (u SignIn) Start(ctx context.Context, in SignInInput) (SignInCode, error) {
	now := u.Clock.Now()
	u.Session.dropExpired(now)
	f, err := u.prepare(ctx, in)
	if err != nil {
		return SignInCode{}, ghcall.Refuse(err, now)
	}
	dc, err := u.Device.StartDevice(ctx, f.clientID)
	if err != nil {
		return SignInCode{}, ghcall.Refuse(err, now)
	}
	f.deviceCode, f.expiresAt, f.interval = dc.DeviceCode, now.Add(dc.ExpiresIn), dc.Interval
	id := string(u.IDs.New())
	u.Session.addFlow(id, f)
	return SignInCode{ID: id, UserCode: dc.UserCode, VerificationURI: dc.VerificationURI, ExpiresIn: dc.ExpiresIn, Interval: dc.Interval}, nil
}

// prepare は、入力を 07「登録」の表の順に確かめ、client_id を読む。
func (u SignIn) prepare(ctx context.Context, in SignInInput) (*flow, error) {
	existing, err := u.Accounts.List(ctx)
	if err != nil {
		return nil, err
	}
	f := &flow{}
	if in.Account != "" {
		if f.previous, err = find(ctx, u.Accounts, in.Account); err != nil {
			return nil, err
		}
	}
	if f.target, err = account.Check(account.Account{Name: in.Name, Login: in.Login, Method: account.DeviceFlow}, existing, in.Account); err != nil {
		return nil, err
	}
	f.target.ID = in.Account
	if f.clientID, err = u.Hosts.ClientID(ctx); err != nil {
		return nil, err
	}
	if f.clientID == "" {
		return nil, refusal.New(noClientIDText)
	}
	return f, nil
}

// Poll は、許可されたかを GitHub に 1 回だけ聞く。GitHub に届かないときは待ち続ける。
func (u SignIn) Poll(ctx context.Context, id string) (SignInResult, error) {
	f, ok := u.Session.flow(id)
	if !ok {
		return SignInResult{}, port.ErrNotFound
	}
	if !u.Clock.Now().Before(f.expiresAt) {
		return u.end(id, SignInResult{State: SignInExpired}), nil
	}
	g, err := u.Device.PollDevice(ctx, f.clientID, f.deviceCode)
	if err != nil {
		return SignInResult{State: SignInWaiting, Interval: u.Session.intervalOf(f)}, nil
	}
	switch g.Status {
	case port.DeviceSlowDown:
		return SignInResult{State: SignInWaiting, Interval: u.Session.slowDown(f, slowDownStep)}, nil
	case port.DeviceExpired:
		return u.end(id, SignInResult{State: SignInExpired}), nil
	case port.DeviceDenied:
		return u.end(id, SignInResult{State: SignInDenied}), nil
	case port.DeviceGranted:
		return u.finish(ctx, id, f, g.Token)
	}
	return SignInResult{State: SignInWaiting, Interval: u.Session.intervalOf(f)}, nil
}

// finish は、許可されたトークンの持ち主を確かめ、キーチェーンに置いて、アカウントを登録・保存する。
func (u SignIn) finish(ctx context.Context, id string, f *flow, tok account.DeviceToken) (SignInResult, error) {
	login, err := u.GitHub.Viewer(ctx, tok.Access)
	if err != nil {
		// 許可は済んでいるが確かめられない。流れを捨て、理由を「アカウントを追加」に出す。
		return u.end(id, SignInResult{State: SignInFailed, Message: ghcall.Refuse(err, u.Clock.Now()).Error()}), nil
	}
	if !strings.EqualFold(login, f.target.Login) {
		return u.end(id, SignInResult{State: SignInMismatch, Message: mismatchText(login)}), nil
	}
	if err := u.Secrets.SaveDeviceToken(ctx, f.target.Login, tok); err != nil {
		return u.end(id, SignInResult{State: SignInFailed, Message: ghcall.Refuse(err, u.Clock.Now()).Error()}), nil
	}
	a, err := u.save(ctx, f)
	if err != nil {
		u.Session.endFlow(id)
		return SignInResult{}, err
	}
	u.Session.markOK(a.ID, [32]byte{})
	return u.end(id, SignInResult{State: SignInDone, Account: a}), nil
}

func (u SignIn) save(ctx context.Context, f *flow) (account.Account, error) {
	a := f.target
	if a.ID == "" {
		a.ID = account.ID(u.IDs.New())
		return a, u.Accounts.Add(ctx, a)
	}
	if err := u.Accounts.Update(ctx, a); err != nil {
		return account.Account{}, err
	}
	// デバイスフローのままログイン名を変えたら、古いログイン名の項目は誰も読まないので消す(決定 D2-13)。
	if f.previous.Method == account.DeviceFlow && !strings.EqualFold(f.previous.Login, a.Login) {
		return a, u.Secrets.DeleteDeviceToken(ctx, f.previous.Login)
	}
	return a, nil
}

func (u SignIn) end(id string, r SignInResult) SignInResult {
	u.Session.endFlow(id)
	return r
}

// Cancel は、流れを捨てる(キャンセル・Esc・外側のクリック)。何も登録しない。
func (u SignIn) Cancel(id string) { u.Session.endFlow(id) }
