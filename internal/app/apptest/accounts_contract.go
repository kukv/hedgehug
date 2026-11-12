package apptest

import (
	"context"
	"slices"
	"testing"

	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
)

// RunAccountsContract は、port.Accounts が満たすべき振る舞いを確かめる。偽物と、本物の実装
// (infra/machineconfig)の両方のテストから呼ぶ。newAccounts は、空の一覧を返す。
func RunAccountsContract(t *testing.T, newAccounts func(t *testing.T) port.Accounts) {
	t.Run("empty at first", func(t *testing.T) {
		got, err := newAccounts(t).List(context.Background())
		if err != nil || len(got) != 0 {
			t.Errorf("List = %+v, %v", got, err)
		}
	})
	t.Run("lists in the order added", func(t *testing.T) {
		accs := newAccounts(t)
		want := []account.Account{
			{ID: "01ACC00000000000000000002", Name: "work", Login: "taro", Method: account.GhCLI},
			{ID: "01ACC00000000000000000001", Name: "家", Login: "taro-home", Method: account.DeviceFlow},
		}
		for _, a := range want {
			if err := accs.Add(context.Background(), a); err != nil {
				t.Fatal(err)
			}
		}
		got, err := accs.List(context.Background())
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("List = %+v, %v; want %+v", got, err, want)
		}
	})
}
