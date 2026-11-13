package apptest

import (
	"context"
	"slices"
	"sync"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

// Accounts は port.Accounts の偽物。
type Accounts struct {
	mu   sync.Mutex
	list []account.Account
}

func NewAccounts() *Accounts { return &Accounts{} }

func (a *Accounts) List(context.Context) ([]account.Account, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Clone(a.list), nil
}

func (a *Accounts) Add(_ context.Context, acc account.Account) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.list = append(a.list, acc)
	return nil
}

func (a *Accounts) Update(_ context.Context, acc account.Account) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := range a.list {
		if a.list[i].ID == acc.ID {
			a.list[i] = acc
			return nil
		}
	}
	return port.ErrNotFound
}

func (a *Accounts) Remove(_ context.Context, id account.ID) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := range a.list {
		if a.list[i].ID == id {
			a.list = slices.Delete(a.list, i, i+1)
			return nil
		}
	}
	return port.ErrNotFound
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
