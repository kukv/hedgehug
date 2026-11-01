package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestSameHostAndAccount(t *testing.T) {
	if !SameHost("GitHub.com", "github.com") || SameHost("github.com", "gitlab.com") {
		t.Error("SameHost is wrong")
	}
	if !SameAccount("github.com/Alice", "GitHub.com/alice") || SameAccount("github.com/alice", "github.com/bob") {
		t.Error("SameAccount is wrong")
	}
}

func TestHostsIgnoreCase(t *testing.T) {
	p := testPaths(t)
	writeConfig(t, p, "hosts:\n  - {host: GHE.example.com, api: \"https://ghe.example.com/x\"}\n")
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if !c.IsGitHubHost("ghe.example.com") || !c.IsGitHubHost("GITHUB.COM") {
		t.Error("IsGitHubHost is case-sensitive")
	}
	if got := c.Host("ghe.example.com").API; got != "https://ghe.example.com/x" {
		t.Errorf("Host(...).API = %q", got)
	}
	if got := c.Host("GitHub.com").API; got != "https://api.github.com" {
		t.Errorf("Host(GitHub.com).API = %q", got)
	}
}

func TestLoadCommonRuleErrors(t *testing.T) {
	for _, tc := range []struct{ name, yaml, want string }{
		{"hosts differ by case", "hosts:\n  - {host: GHE.example.com}\n  - {host: ghe.example.com}\n", ":3: hosts: \"ghe.example.com\" is listed twice"},
		{"accounts differ by case", "accounts:\n  - {name: a, host: github.com, login: Alice}\n  - {name: b, host: GitHub.com, login: alice}\n", ":3: accounts: \"GitHub.com/alice\" is listed twice"},
		{"blank host", "hosts:\n  - {host: \" \"}\n", ":2: hosts: host is missing"},
		{"empty rewrite from", "url_rewrites:\n  - {from: \"\", to: x}\n", ":2: url_rewrites: want a mapping with from and to"},
		{"blank rewrite to", "url_rewrites:\n  - {from: x, to: \"  \"}\n", ":2: url_rewrites: want a mapping with from and to"},
		{"list rewrite from", "url_rewrites:\n  - {from: [x], to: y}\n", ":2: url_rewrites: from and to must be strings"},
		{"blank account login", "accounts:\n  - {name: a, host: h, login: \" \"}\n", ":2: accounts: login is missing"},
		{"blank repo match", "accounts:\n  - {name: a, host: h, login: l}\nrepo_accounts:\n  - {match: \" \", account: a}\n", ":4: repo_accounts: want a mapping with match and account"},
		{"list repo account", "accounts:\n  - {name: a, host: h, login: l}\nrepo_accounts:\n  - {match: \"*\", account: [a]}\n", ":4: repo_accounts: match and account must be strings"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testPaths(t)
			writeConfig(t, p, tc.yaml)
			_, err := Load(p)
			if err == nil || !strings.Contains(err.Error(), p.ConfigFile+tc.want) {
				t.Errorf("err = %v, want one with %q", err, p.ConfigFile+tc.want)
			}
		})
	}
}

func TestLoadBlankOptionalValuesAreAbsent(t *testing.T) {
	p := testPaths(t)
	writeConfig(t, p, "accounts:\n  - {name: a, host: h, login: l, token: \"  \"}\nhosts:\n  - {host: h, api: \" \", web: \"\", client_id: \" \"}\ndefault_store: \"\"\n")
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.Accounts[0].Token != (TokenSource{}) || c.DefaultStore != "" {
		t.Errorf("token = %+v, default_store = %q", c.Accounts[0].Token, c.DefaultStore)
	}
	if h := c.Host("h"); h.API != "https://h/api/v3" || h.Web != "https://h" || h.ClientID != "" {
		t.Errorf("host = %+v", h)
	}
}

func TestLoadNonScalarErrorsHaveOneShape(t *testing.T) {
	for _, tc := range []struct{ name, yaml, want string }{
		{"default_store", "default_store: [a]\n", ":1: default_store must be a store name"},
		{"url_rewrites", "url_rewrites: x\n", ":1: url_rewrites must be a list"},
		{"hosts", "hosts: x\n", ":1: hosts must be a list"},
		{"repo_accounts", "repo_accounts: x\n", ":1: repo_accounts must be a list"},
		{"token", "accounts:\n  - {name: a, host: h, login: l, token: [gh]}\n", ":2: accounts: token must be"},
		{"host api", "hosts:\n  - {host: h, api: [x]}\n", ":2: hosts: api must be"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testPaths(t)
			writeConfig(t, p, tc.yaml)
			_, err := Load(p)
			if err == nil || !strings.Contains(err.Error(), p.ConfigFile+tc.want) {
				t.Errorf("err = %v, want one with %q", err, p.ConfigFile+tc.want)
			}
		})
	}
}

func TestUpdateKeepsBothWriters(t *testing.T) {
	p := testPaths(t)
	writeConfig(t, p, "# keep\nunknown: 1\naccounts:\n  - {name: a, host: github.com, login: alice}\n")
	c1, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	c2, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, c := range []*Config{c1, c2} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			name := []string{"b", "c"}[i]
			errs[i] = c.Update(func(c *Config) error {
				return c.AddAccount(Account{Name: name, Host: "github.com", Login: name})
			})
		}()
	}
	wg.Wait()
	if errs[0] != nil || errs[1] != nil {
		t.Fatal(errs)
	}
	got, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Accounts) != 3 {
		t.Errorf("accounts = %+v", got.Accounts)
	}
	b, _ := os.ReadFile(p.ConfigFile)
	if !strings.Contains(string(b), "# keep") || !strings.Contains(string(b), "unknown: 1") {
		t.Errorf("config.yaml = %s", b)
	}
	if len(c1.Accounts) < 2 || len(c2.Accounts) < 2 {
		t.Errorf("Update left the receiver stale: %d, %d accounts", len(c1.Accounts), len(c2.Accounts))
	}
}

func TestUpdateErrorWritesNothing(t *testing.T) {
	p := testPaths(t)
	const src = "accounts:\n  - {name: a, host: github.com, login: alice}\n"
	writeConfig(t, p, src)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	err = c.Update(func(c *Config) error {
		if err := c.AddAccount(Account{Name: "b", Host: "github.com", Login: "b"}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Errorf("err = %v", err)
	}
	if b, _ := os.ReadFile(p.ConfigFile); string(b) != src {
		t.Errorf("config.yaml changed: %s", b)
	}
	if len(c.Accounts) != 1 {
		t.Errorf("the receiver changed: %+v", c.Accounts)
	}
}

func TestUpdateOnMissingFile(t *testing.T) {
	p := testPaths(t)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Update(func(c *Config) error { return c.AddStore("me", filepath.Join(p.Home, "me"), nil) }); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil || len(got.Stores) != 1 || len(c.Stores) != 1 {
		t.Errorf("stores = %+v, %v", got.Stores, err)
	}
}
