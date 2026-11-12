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

// Secrets は port.Secrets の偽物。Tokens はログイン名からトークンへの対応。ない、または Err があれば失敗する。
type Secrets struct {
	mu     sync.Mutex
	Tokens map[string]string
	Err    error
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
