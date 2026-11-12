package github

import (
	"context"
	"errors"
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
	f := &flow{}
	var err error
	if in.Account != "" {
		if f.previous, err = find(ctx, u.Accounts, in.Account); err != nil {
			return nil, err
		}
	}
	if f.target, err = u.check(ctx, account.Account{ID: in.Account, Name: in.Name, Login: in.Login, Method: account.DeviceFlow}); err != nil {
		return nil, err
	}
	if f.clientID, err = u.Hosts.ClientID(ctx); err != nil {
		return nil, err
	}
	if f.clientID == "" {
		return nil, refusal.New(noClientIDText)
	}
	return f, nil
}

// check は、いまのアカウントの一覧で a を確かめる(07「登録」の表)。a.ID は編集するアカウント(登録では空)。
// 始めるときと、許可を待つあいだに別の登録が済んだときのために保存の直前とで使う。
func (u SignIn) check(ctx context.Context, a account.Account) (account.Account, error) {
	existing, err := u.Accounts.List(ctx)
	if err != nil {
		return account.Account{}, err
	}
	return account.Check(a, existing, a.ID)
}

// Poll は、許可されたかを GitHub に 1 回だけ聞く。
func (u SignIn) Poll(ctx context.Context, id string) (SignInResult, error) {
	f, ok := u.Session.lookupFlow(id)
	if !ok {
		return SignInResult{}, port.ErrNotFound
	}
	// 期限はいちばん先に見る。許可されたトークンを持っていても、期限が過ぎたら流れごと捨てる。
	if !u.Clock.Now().Before(f.expiresAt) {
		return u.end(id, SignInResult{State: SignInExpired}), nil
	}
	if tok, ok := u.Session.grantOf(f); ok {
		// 許可は済んでいて device_code はもう使えない。持ち主の確かめだけをやり直す。
		return u.verifyGrant(ctx, id, f, tok)
	}
	g, err := u.Device.PollDevice(ctx, f.clientID, f.deviceCode)
	if err != nil {
		// GitHub に届かないときは、つながるまで待ち続ける(07「待っているあいだに通信が切れたときは」)。
		return u.waiting(f), nil
	}
	switch g.Status {
	case port.DeviceSlowDown:
		return SignInResult{State: SignInWaiting, Interval: u.Session.slowDown(f, slowDownStep)}, nil
	case port.DeviceExpired:
		return u.end(id, SignInResult{State: SignInExpired}), nil
	case port.DeviceDenied:
		return u.end(id, SignInResult{State: SignInDenied}), nil
	case port.DeviceGranted:
		return u.verifyGrant(ctx, id, f, g.Token)
	}
	return u.waiting(f), nil
}

// end は、流れを捨てて r を返す。
func (u SignIn) end(id string, r SignInResult) SignInResult {
	u.Session.endFlow(id)
	return r
}

// failure は、理由を「アカウントを追加」に出す結果。
func (u SignIn) failure(err error) SignInResult {
	return SignInResult{State: SignInFailed, Message: ghcall.Refuse(err, u.Clock.Now()).Error()}
}

func (u SignIn) waiting(f *flow) SignInResult {
	return SignInResult{State: SignInWaiting, Interval: u.Session.intervalOf(f)}
}

// verifyGrant は、許可されたトークンの持ち主を GitHub で確かめる。GitHub に届かない(通信できない・利用上限)
// ときは、トークンを流れに持ったまま待つ(07「待っているあいだに通信が切れたときは、つながるまで待ち続ける」)。
func (u SignIn) verifyGrant(ctx context.Context, id string, f *flow, tok account.DeviceToken) (SignInResult, error) {
	login, err := u.GitHub.Viewer(ctx, tok.Access)
	if ghcall.Is(err, ghcall.Unreachable) || ghcall.Is(err, ghcall.RateLimited) {
		u.Session.holdGrant(f, tok)
		return u.waiting(f), nil
	}
	if err != nil {
		return u.end(id, u.failure(err)), nil
	}
	if !strings.EqualFold(login, f.target.Login) {
		return u.end(id, SignInResult{State: SignInMismatch, Message: mismatchText(login)}), nil
	}
	return u.commit(ctx, id, tok)
}

// commit は、流れを取り出してから、確かめたトークンをキーチェーンに置いて、アカウントを登録・保存する。
// 取り出せなければ(確かめているあいだにキャンセルされた)、何も置かず、何も登録しない(07「ダイアログを閉じる」)。
func (u SignIn) commit(ctx context.Context, id string, tok account.DeviceToken) (SignInResult, error) {
	f, ok := u.Session.takeFlow(id)
	if !ok {
		return SignInResult{}, port.ErrNotFound
	}
	// 置く前に確かめ直す。置いたあとでは、同じログイン名の別のデバイスフローのアカウントの項目を上書きしてしまう。
	if _, err := u.check(ctx, f.target); err != nil {
		var r *refusal.Refusal
		if errors.As(err, &r) {
			return u.failure(err), nil
		}
		return SignInResult{}, err
	}
	if err := u.Secrets.SaveDeviceToken(ctx, f.target.Login, tok); err != nil {
		return u.failure(err), nil
	}
	a, err := u.save(ctx, f)
	if err != nil {
		return SignInResult{}, err
	}
	u.Session.markOK(a.ID, [32]byte{})
	return SignInResult{State: SignInDone, Account: a}, nil
}

// save は、登録なら新しい ID で足し、編集・再認証なら値を入れ替える。
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
	// キーチェーンの名前は大文字と小文字を区別するので、そのまま比べる。消せなくても保存は失敗にしない。
	if f.previous.Method == account.DeviceFlow && f.previous.Login != a.Login {
		_ = u.Secrets.DeleteDeviceToken(ctx, f.previous.Login)
	}
	return a, nil
}

// Cancel は、流れを捨てる(キャンセル・Esc・外側のクリック)。何も登録しない。
func (u SignIn) Cancel(id string) { u.Session.endFlow(id) }
