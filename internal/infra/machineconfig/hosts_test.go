package machineconfig

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kukv/hedgehug/internal/domain/account"
)

func TestClientID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if id, err := Open(path).Hosts().ClientID(context.Background()); err != nil || id != "" {
		t.Errorf("no file: %q, %v", id, err)
	}
	body := "hosts:\n  - {host: example.com, client_id: other}\n  - {host: github.com, client_id: \" Iv23abc \"}\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if id, err := Open(path).Hosts().ClientID(context.Background()); err != nil || id != "Iv23abc" {
		t.Errorf("ClientID = %q, %v", id, err)
	}
}

func TestHostsSurviveAccountWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("hosts:\n  - {host: github.com, client_id: Iv23abc}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f := Open(path)
	a := account.Account{ID: "01ACC00000000000000000001", Name: "work", Login: "taro", Method: account.GhCLI}
	if err := f.Accounts().Add(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if id, err := f.Hosts().ClientID(context.Background()); err != nil || id != "Iv23abc" {
		t.Errorf("after Add: %q, %v", id, err)
	}
}
