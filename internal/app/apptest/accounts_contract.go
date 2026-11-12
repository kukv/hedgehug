package apptest

import (
	"context"
	"errors"
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
	t.Run("updates in place and removes", func(t *testing.T) { runUpdateRemove(t, newAccounts(t)) })
}

func runUpdateRemove(t *testing.T, accs port.Accounts) {
	a := account.Account{ID: "01ACC00000000000000000001", Name: "work", Login: "taro", Method: account.GhCLI}
	b := account.Account{ID: "01ACC00000000000000000002", Name: "家", Login: "taro-home", Method: account.DeviceFlow}
	for _, x := range []account.Account{a, b} {
		if err := accs.Add(context.Background(), x); err != nil {
			t.Fatal(err)
		}
	}
	a2 := account.Account{ID: a.ID, Name: "仕事", Login: "jiro", Method: account.Keychain}
	if err := accs.Update(context.Background(), a2); err != nil {
		t.Fatal(err)
	}
	if err := accs.Remove(context.Background(), b.ID); err != nil {
		t.Fatal(err)
	}
	got, err := accs.List(context.Background())
	if err != nil || !slices.Equal(got, []account.Account{a2}) {
		t.Errorf("List = %+v, %v", got, err)
	}
	missing := account.Account{ID: "01ACC00000000000000000009", Name: "x", Login: "x", Method: account.GhCLI}
	if err := accs.Update(context.Background(), missing); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Update missing = %v", err)
	}
	if err := accs.Remove(context.Background(), missing.ID); !errors.Is(err, port.ErrNotFound) {
		t.Errorf("Remove missing = %v", err)
	}
}
