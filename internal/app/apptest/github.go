package apptest

import (
	"context"
	"sync"

	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// GitHub は port.GitHub の偽物。Users はトークンからログイン名への対応。Err があれば、どの問い合わせもそれを返す。
type GitHub struct {
	mu    sync.Mutex
	Users map[string]string
	Err   error
}

func (g *GitHub) Viewer(_ context.Context, token string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.Err != nil {
		return "", g.Err
	}
	login, ok := g.Users[token]
	if !ok {
		return "", &ghcall.Failure{Kind: ghcall.TokenRejected}
	}
	return login, nil
}

// AddUser は、トークン token をログイン名 login の人のものにする(通しのテストで、デバイスフローの許可に使う)。
func (g *GitHub) AddUser(token, login string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.Users == nil {
		g.Users = map[string]string{}
	}
	g.Users[token] = login
}

// Secrets は port.Secrets の偽物。Tokens はログイン名からトークンへの対応。ない、または Err があれば失敗する。
// Device はデバイスフローのトークンの組(ログイン名から)。NoKeychain なら、デバイスフローのトークンを読めず、置けない。
// DeleteErr があれば、DeleteDeviceToken は消さずにそれを返す(NoKeychain では消す処理を失敗させない)。
type Secrets struct {
	mu                sync.Mutex
	Tokens            map[string]string
	Device            map[string]account.DeviceToken
	NoKeychain        bool
	NoKeychainForSave bool // 読めるが置けない(取り直したトークンを置けないとき)
	DeleteErr         error
	Err               error
}

func (s *Secrets) Token(_ context.Context, a account.Account) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return "", s.Err
	}
	tok, ok := s.Tokens[a.Login]
	if !ok {
		return "", &ghcall.Failure{Kind: ghcall.KeychainMissing, Login: a.Login}
	}
	return tok, nil
}

func (s *Secrets) DeviceToken(_ context.Context, login string) (account.DeviceToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.NoKeychain {
		return account.DeviceToken{}, &ghcall.Failure{Kind: ghcall.KeychainUnavailable}
	}
	t, ok := s.Device[login]
	if !ok {
		return account.DeviceToken{}, &ghcall.Failure{Kind: ghcall.TokenRejected}
	}
	return t, nil
}

func (s *Secrets) SaveDeviceToken(_ context.Context, login string, t account.DeviceToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.NoKeychain || s.NoKeychainForSave {
		return &ghcall.Failure{Kind: ghcall.KeychainUnavailable}
	}
	if s.Device == nil {
		s.Device = map[string]account.DeviceToken{}
	}
	s.Device[login] = t
	return nil
}

func (s *Secrets) DeleteDeviceToken(_ context.Context, login string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.DeleteErr != nil {
		return s.DeleteErr
	}
	delete(s.Device, login)
	return nil
}
