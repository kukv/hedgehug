package accounts_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/accounts"
	"github.com/kukv/hedgehug/internal/app/usecase/usecasetest"
)

func withAccounts(t *testing.T) *usecasetest.Session {
	t.Helper()
	ts := usecasetest.New(t)
	ts.AppendConfig("accounts:\n  - {name: work, host: github.com, login: w}\n  - {name: me, host: github.com, login: m}\n")
	return ts
}

func TestRepoAccountsAreReplacedInOrder(t *testing.T) {
	t.Parallel()
	ts := withAccounts(t)
	rules := []accounts.RepoAccountView{{Match: "github.com/org-a/*", Account: "work"}, {Match: "*", Account: "me"}}
	got, err := accounts.SetRepoAccounts(ts.Session, rules)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, rules) {
		t.Fatalf("rules = %+v", got)
	}
	slices.Reverse(rules)
	if got, err = accounts.SetRepoAccounts(ts.Session, rules); err != nil || !slices.Equal(got, rules) {
		t.Fatalf("reversed = %+v, %v", got, err)
	}
	got, err = accounts.RepoAccounts(ts.Session)
	if err != nil || !slices.Equal(got, rules) {
		t.Fatalf("read back = %+v, %v", got, err)
	}
	if got, err = accounts.SetRepoAccounts(ts.Session, nil); err != nil || len(got) != 0 {
		t.Fatalf("emptied = %+v, %v", got, err)
	}
	_, paths, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(paths.ConfigFile) //nolint:gosec // the config of the test
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "repo_accounts") {
		t.Errorf("the key stays:\n%s", b)
	}
}

func TestRepoAccountsRefuseWhatConfigWouldNotRead(t *testing.T) {
	t.Parallel()
	ts := withAccounts(t)
	for name, r := range map[string]accounts.RepoAccountView{
		"unknown account": {Match: "*", Account: "nobody"},
		"bad pattern":     {Match: "[", Account: "me"},
		"no match":        {Match: "", Account: "me"},
	} {
		_, err := accounts.SetRepoAccounts(ts.Session, []accounts.RepoAccountView{r})
		usecasetest.MustCode(t, err, usecase.KindInvalid, "repo_accounts.invalid")
		if got, _ := accounts.RepoAccounts(ts.Session); len(got) != 0 {
			t.Errorf("%s: written: %+v", name, got)
		}
	}
}

func TestAuthSource(t *testing.T) {
	t.Parallel()
	ts := withAccounts(t)
	r, err := accounts.AuthSource(ts.Session, "work", "gh")
	if err != nil || r.Token != "gh" {
		t.Fatalf("source = %+v, %v", r, err)
	}
	c, _, err := ts.Config()
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(c.Accounts, func(a config.Account) bool { return a.Name == "work" })
	if i < 0 || c.Accounts[i].Token.Kind != "gh" {
		t.Errorf("accounts = %+v", c.Accounts)
	}
	_, err = accounts.AuthSource(ts.Session, "work", "keychain")
	usecasetest.MustCode(t, err, usecase.KindUsage, "auth.token_source")
	_, err = accounts.AuthSource(ts.Session, "nobody", "gh")
	usecasetest.MustCode(t, err, usecase.KindNotFound, "account.not_found")
}
