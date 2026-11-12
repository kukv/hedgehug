package machineconfig

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
	"github.com/kukv/hedgehug/internal/domain/account"
)

func TestContract(t *testing.T) {
	apptest.RunAccountsContract(t, func(t *testing.T) port.Accounts {
		return Open(filepath.Join(t.TempDir(), "hedgehug", "config.yaml")).Accounts()
	})
}

func TestPersistsWithoutTokens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hedgehug", "config.yaml")
	a := account.Account{ID: "01ACC00000000000000000001", Name: "work", Login: "taro", Method: account.Keychain}
	if err := Open(path).Accounts().Add(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	got, err := Open(path).Accounts().List(context.Background())
	if err != nil || len(got) != 1 || got[0] != a {
		t.Errorf("reopened List = %+v, %v", got, err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "accounts:\n  - id: 01ACC00000000000000000001\n    name: work\n    login: taro\n    method: keychain\n"
	if string(b) != want {
		t.Errorf("config.yaml =\n%s\nwant\n%s", b, want)
	}
	if strings.Contains(string(b), "token") {
		t.Error("config.yaml has a token field")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, %v", info.Mode().Perm(), err)
	}
}
