package apptest

import (
	"context"
	"slices"
	"sync"

	"github.com/kukv/hedgehug/internal/domain/account"
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
