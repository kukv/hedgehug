package apptest

import (
	"context"
	"slices"
	"sync"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
)

// StoreGitHub は port.StoreGitHub の偽物。
type StoreGitHub struct {
	mu     sync.Mutex
	Stores []port.StoreGitHubInfo
	Login  string // マシンの設定「ログイン名」
}

func (s *StoreGitHub) List(context.Context) ([]port.StoreGitHubInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.Stores), nil
}

func (s *StoreGitHub) Unassign(_ context.Context, id account.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.Stores {
		if s.Stores[i].Account == id {
			s.Stores[i].Account = ""
		}
	}
	return nil
}

func (s *StoreGitHub) MachineLogin(context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Login, nil
}
