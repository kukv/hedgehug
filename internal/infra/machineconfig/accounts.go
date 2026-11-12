package machineconfig

import (
	"context"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
)

type accountYAML struct {
	ID     string `yaml:"id"`
	Name   string `yaml:"name"`
	Login  string `yaml:"login"`
	Method string `yaml:"method"`
}

// Accounts は、config.yaml の accounts を読み書きする port.Accounts。
func (f *File) Accounts() port.Accounts { return accounts{f} }

type accounts struct{ f *File }

func (a accounts) List(context.Context) ([]account.Account, error) {
	c, err := a.f.read()
	if err != nil {
		return nil, err
	}
	out := make([]account.Account, 0, len(c.Accounts))
	for _, y := range c.Accounts {
		out = append(out, account.Account{ID: account.ID(y.ID), Name: y.Name, Login: y.Login, Method: account.Method(y.Method)})
	}
	return out, nil
}

func (a accounts) Add(_ context.Context, acc account.Account) error {
	return a.f.update(func(c *config) error {
		c.Accounts = append(c.Accounts, accountYAML{ID: string(acc.ID), Name: acc.Name, Login: acc.Login, Method: string(acc.Method)})
		return nil
	})
}
