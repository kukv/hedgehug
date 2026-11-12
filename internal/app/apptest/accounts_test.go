package apptest_test

import (
	"testing"

	"github.com/kukv/hedgehug/internal/app/apptest"
	"github.com/kukv/hedgehug/internal/app/port"
)

func TestFakeAccounts(t *testing.T) {
	apptest.RunAccountsContract(t, func(*testing.T) port.Accounts { return apptest.NewAccounts() })
}
