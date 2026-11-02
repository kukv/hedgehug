package server

import (
	"os"
	"testing"
)

// StoreItem.account is the ID (host/login) of the account that writes the
// store, the form the watchers and the assignees of a task have, and not the
// name of the account in config.yaml. The GUI compares it with the watchers.
func TestStoresAccountIsHostSlashLogin(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	_, p, err := ts.session().Config()
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(p.ConfigFile, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString("accounts:\n  - name: work\n    host: github.com\n    login: alice-login\n"); err != nil {
		t.Fatal(err)
	}

	var list []struct {
		Name    string `json:"name"`
		Account string `json:"account"`
	}
	ts.call("GET", "/api/stores", nil, 200, &list)
	if len(list) != 1 || list[0].Account != "github.com/alice-login" {
		t.Errorf("stores = %+v; want the account github.com/alice-login, not its name work", list)
	}
}
