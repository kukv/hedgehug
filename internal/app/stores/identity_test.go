package stores_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/app/stores"
	"github.com/kukv/hedgehug/internal/domain/account"
)

func TestIdentityMe(t *testing.T) {
	acc := account.Account{ID: "a1", Name: "work", Login: "work-login", Method: account.GhCLI}
	tests := []struct {
		name    string
		login   string
		account account.ID
		store   port.StoreID
		want    string
		wantErr error
	}{
		{"account", "machine", "a1", "s1", "work-login", nil},
		{"no account", "machine", "", "s1", "machine", nil},
		{"unregistered account", "machine", "gone", "s1", "machine", nil},
		{"neither", "", "", "s1", "", port.ErrNoLogin},
		{"unregistered account and no machine login", "", "gone", "s1", "", port.ErrNoLogin},
		{"unknown store", "machine", "a1", "s9", "", port.ErrStoreNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			accounts := apptest.NewAccounts()
			if err := accounts.Add(ctx, acc); err != nil {
				t.Fatal(err)
			}
			gh := &apptest.StoreGitHub{Login: tt.login, Stores: []port.StoreGitHubInfo{{ID: "s1", Account: tt.account}}}
			got, err := stores.Identity{GitHub: gh, Accounts: accounts}.Me(ctx, tt.store)
			if !errors.Is(err, tt.wantErr) || got != tt.want {
				t.Errorf("Me = %q, %v", got, err)
			}
		})
	}
}
