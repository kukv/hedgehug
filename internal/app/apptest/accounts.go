package apptest

import (
	"context"
	"slices"
	"sync"

	"github.com/kukv/hedgehug/internal/app/port"
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
