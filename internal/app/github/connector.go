package github

import (
	"context"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// Connector は、GitHub に接続するたびにトークンを得る入口(07「接続のたびの照合」「デバイスフロー」の期限付き
// トークン、「GitHub との通信に使うアカウント」)。認証エラーへの変更はここに集める。
type Connector struct {
	Session  *Session
	Accounts port.Accounts
	Secrets  port.Secrets
	GitHub   port.GitHub
	Device   port.DeviceFlow
	Hosts    port.Hosts
	Stores   port.StoreGitHub
	Clock    port.Clock
}

// TokenFor は、Store に割り当てたアカウントのトークンと、そのアカウントを返す。割り当てがなければ
// NoAccount(repo は文言に使う)。認証エラーなら AuthError。別のアカウントでは読まない。
func (c Connector) TokenFor(ctx context.Context, store port.StoreID, repo string) (string, account.Account, error) {
	a, err := c.storeAccount(ctx, store)
	if err != nil {
		return "", account.Account{}, err
	}
	if a.ID == "" {
		return "", account.Account{}, &ghcall.Failure{Kind: ghcall.NoAccount, Repo: repo}
	}
	tok, err := c.Token(ctx, a)
	return tok, a, err
}

func (c Connector) storeAccount(ctx context.Context, store port.StoreID) (account.Account, error) {
	stores, err := c.Stores.List(ctx)
	if err != nil {
		return account.Account{}, err
	}
	var id account.ID
	for _, s := range stores {
		if s.ID == store {
			id = s.Account
		}
	}
	if id == "" {
		return account.Account{}, nil
	}
	accs, err := c.Accounts.List(ctx)
	if err != nil {
		return account.Account{}, err
	}
	for _, a := range accs {
		if a.ID == id {
			return a, nil
		}
	}
	return account.Account{}, nil
}

// Rejected は、a のトークンが GitHub に拒否された(401)ことを知らせ、認証エラーにする(07「状態」)。
func (c Connector) Rejected(id account.ID) { c.Session.markBroken(id, "") }

// Token は、a のトークンを返す。認証エラーなら AuthError の Failure。
func (c Connector) Token(ctx context.Context, a account.Account) (string, error) {
	if broken, _ := c.Session.state(a.ID); broken {
		return "", authError(a)
	}
	if a.Method == account.DeviceFlow {
		return c.deviceToken(ctx, a)
	}
	tok, err := c.Secrets.Token(ctx, a)
	if err != nil {
		return "", err
	}
	fp := fingerprint(tok)
	if c.Session.checked(a.ID, fp) {
		return tok, nil
	}
	login, err := c.GitHub.Viewer(ctx, tok)
	switch {
	case ghcall.Is(err, ghcall.TokenRejected):
		c.Session.markBroken(a.ID, "")
		return "", authError(a)
	case err != nil:
		return "", err
	case !strings.EqualFold(login, a.Login):
		c.Session.markBroken(a.ID, mismatchText(login))
		return "", authError(a)
	}
	c.Session.markOK(a.ID, fp)
	return tok, nil
}

// deviceToken は、デバイスフローのトークン。期限が近ければ取り直す(この小計画の決定 D2-6・D2-7)。
func (c Connector) deviceToken(ctx context.Context, a account.Account) (string, error) {
	t, err := c.Secrets.DeviceToken(ctx, a.Login)
	if err != nil {
		return "", c.brokenOr(a, err)
	}
	if t.Fresh(c.Clock.Now()) {
		return t.Access, nil
	}
	c.Session.refresh.Lock()
	defer c.Session.refresh.Unlock()
	if t, err = c.Secrets.DeviceToken(ctx, a.Login); err != nil {
		return "", c.brokenOr(a, err)
	}
	if t.Fresh(c.Clock.Now()) {
		return t.Access, nil // ロックを待つあいだに、ほかの接続が取り直した
	}
	clientID, err := c.Hosts.ClientID(ctx)
	if err != nil {
		return "", err
	}
	if t.Refresh == "" || t.RefreshExpired(c.Clock.Now()) || clientID == "" {
		c.Session.markBroken(a.ID, "")
		return "", authError(a)
	}
	nt, err := c.Device.RefreshDevice(ctx, clientID, t.Refresh)
	if err != nil {
		return "", c.brokenOr(a, err)
	}
	if err := c.Secrets.SaveDeviceToken(ctx, a.Login, nt); err != nil {
		// 古い更新用のトークンはもう使えないので、新しい組を置けなければ使えない。
		c.Session.markBroken(a.ID, "")
		return "", authError(a)
	}
	return nt.Access, nil
}

// brokenOr は、拒否(TokenRejected)なら認証エラーにし、それ以外の失敗はそのまま返す。
func (c Connector) brokenOr(a account.Account, err error) error {
	if ghcall.Is(err, ghcall.TokenRejected) {
		c.Session.markBroken(a.ID, "")
		return authError(a)
	}
	return err
}

func authError(a account.Account) error {
	return &ghcall.Failure{Kind: ghcall.AuthError, Name: a.Name, Login: a.Login}
}

// mismatchText は、トークンのログイン名が登録と違うときの文言(07「接続のたびの照合」。登録と同じ文言)。
// 文言を 2 か所に持たないよう、ghcall.Refuse で作る。
func mismatchText(login string) string {
	return ghcall.Refuse(&ghcall.Failure{Kind: ghcall.LoginMismatch, Login: login}, time.Time{}).Error()
}
