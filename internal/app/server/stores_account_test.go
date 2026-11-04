package server

import (
	"testing"
)

// StoreItem.account is the ID (host/login) of the account that writes the
// store, the form the watchers and the assignees of a task have, and not the
// name of the account in config.yaml. The GUI compares it with the watchers.
func TestStoresAccountIsHostSlashLogin(t *testing.T) {
	t.Parallel()
	ts := newTestServer(t)
	ts.uc.DropDefaultAccount()
	ts.uc.AppendConfig("accounts:\n  - name: work\n    host: github.com\n    login: alice-login\n")

	var list []struct {
		Name    string `json:"name"`
		Account string `json:"account"`
	}
	ts.call("GET", "/api/stores", nil, 200, &list)
	if len(list) != 1 || list[0].Account != "github.com/alice-login" {
		t.Errorf("stores = %+v; want the account github.com/alice-login, not its name work", list)
	}
}
