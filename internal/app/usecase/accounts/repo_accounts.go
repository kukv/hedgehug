package accounts

import (
	"fmt"
	"strings"

	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// RepoAccountView is a rule of repo_accounts.
type RepoAccountView struct {
	Match   string `json:"match" minLength:"1" doc:"A pattern of repo IDs, as path.Match reads it; * is every repository"`
	Account string `json:"account" minLength:"1" doc:"The name of the account"`
}

// RepoAccounts lists repo_accounts, in the order it applies.
func RepoAccounts(ss *usecase.Session) ([]RepoAccountView, error) {
	c, _, err := ss.Config()
	if err != nil {
		return nil, err
	}
	out := make([]RepoAccountView, len(c.RepoAccounts))
	for i, r := range c.RepoAccounts {
		out[i] = RepoAccountView(r)
	}
	return out, nil
}

// SetRepoAccounts replaces repo_accounts: the order is what the rules mean.
func SetRepoAccounts(ss *usecase.Session, rules []RepoAccountView) ([]RepoAccountView, error) {
	c, _, err := ss.Config()
	if err != nil {
		return nil, err
	}
	next := make([]config.RepoAccount, len(rules))
	for i, r := range rules {
		next[i] = config.RepoAccount(r)
	}
	if err := ss.UpdateConfig(c, func(c *config.Config) error {
		if err := c.SetRepoAccounts(next); err != nil {
			return usecase.NewErr(usecase.KindInvalid, "repo_accounts.invalid", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return RepoAccounts(ss)
}

// AuthSourceResult is what AuthSource wrote.
type AuthSourceResult struct {
	Name  string `json:"name"`
	Token string `json:"token" doc:"Where the token comes from now"`
}

// AuthSource changes where the token of the account name comes from, to gh or
// env:<VAR>: the account of a GitHub App that is not installed somewhere goes
// on with a token that can read there. A token kept in the keychain stays.
func AuthSource(ss *usecase.Session, name, token string) (AuthSourceResult, error) {
	s := strings.TrimSpace(token)
	if s == "" || s == "keychain" {
		return AuthSourceResult{}, usecase.NewErr(usecase.KindUsage, "auth.token_source",
			fmt.Errorf("token %q: want gh or env:<VAR>", s), "token", s)
	}
	tok, err := parseAddToken(s)
	if err != nil {
		return AuthSourceResult{}, err
	}
	c, _, err := ss.Config()
	if err != nil {
		return AuthSourceResult{}, err
	}
	if err := ss.UpdateConfig(c, func(c *config.Config) error {
		if !hasAccount(c, name) {
			return usecase.NewErr(usecase.KindNotFound, "account.not_found", fmt.Errorf("no account is named %q", name), "name", name)
		}
		return c.SetAccountToken(name, tok)
	}); err != nil {
		return AuthSourceResult{}, err
	}
	return AuthSourceResult{Name: name, Token: tok.String()}, nil
}

func hasAccount(c *config.Config, name string) bool {
	for _, a := range c.Accounts {
		if a.Name == name {
			return true
		}
	}
	return false
}
