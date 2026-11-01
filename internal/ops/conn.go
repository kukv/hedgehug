package ops

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	gosync "sync"

	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/kukv/hedgehug/internal/auth"
	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/github"
	"github.com/kukv/hedgehug/internal/gitrepo"
)

// tokenCache keeps the token of each account, so that one run (the CLI) or
// one request (the GUI) gets it once. A token is of the account, not of the
// store, so the sessions of one caller share it. Each account has an entry of
// its own, so a slow fetch holds up only the callers of that account. A token
// that could not be got is not kept: the next call asks again.
type tokenCache struct {
	mu gosync.Mutex // guards m, not the fetches
	m  map[string]*tokenEntry
}

type tokenEntry struct {
	mu  gosync.Mutex // held while the token is fetched
	tok string
	ok  bool
}

// entry is the entry of acct, made on first use. The key is the name and the
// ID, so that a session that outlives a change of the account behind a name
// does not hand out the token of the old one.
func (tc *tokenCache) entry(acct config.Account) *tokenEntry {
	key := acct.Name + "\x00" + strings.ToLower(acct.ID())
	tc.mu.Lock()
	defer tc.mu.Unlock()
	e := tc.m[key]
	if e == nil {
		e = &tokenEntry{}
		tc.m[key] = e
	}
	return e
}

// tokenCache is the cache of ss, made on first use.
func (ss *Session) tokenCache() *tokenCache {
	ss.tokMu.Lock()
	defer ss.tokMu.Unlock()
	if ss.tokens == nil {
		ss.tokens = &tokenCache{m: map[string]*tokenEntry{}}
	}
	return ss.tokens
}

// AuthOptions is what getting the token of acct needs.
func (ss *Session) AuthOptions(c *config.Config, acct config.Account) auth.Options {
	return auth.Options{
		Getenv: ss.Getenv, Run: ss.Run, Keyring: ss.Keyring, Now: ss.Now, HTTP: ss.HTTPClient,
		Host:     c.Host(acct.Host),
		LockFile: filepath.Join(filepath.Dir(c.File()), "auth.lock"),
	}
}

// Token returns the token of acct, getting it once per session (and the
// sessions made by ForStore).
func (ss *Session) Token(c *config.Config, acct config.Account) (string, error) {
	e := ss.tokenCache().entry(acct)
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.ok {
		return e.tok, nil
	}
	tok, err := auth.Token(acct, ss.AuthOptions(c, acct))
	if err != nil {
		return "", err
	}
	e.tok, e.ok = tok, true
	return tok, nil
}

// hostOf is the host name of the URL u, without the port; ok is false when u
// is not a URL with one.
func hostOf(u string) (string, bool) {
	p, err := url.Parse(u)
	if err != nil || p.Hostname() == "" {
		return "", false
	}
	return p.Hostname(), true
}

// Conn is how the remotes of the store e are reached: through
// url_rewrites, and with the token of its account over http and https, when
// the host reached (after the rewrite) is the host of the account. A store
// with no account, an account with no token, another host, and an ssh or
// local remote get no auth from here (ssh falls to go-git's ssh-agent).
func (ss *Session) Conn(c *config.Config, e config.StoreEntry) gitrepo.Conn {
	acct, ok := c.Writer(e)
	return gitrepo.Conn{
		Rewrite: c.Rewrite,
		Auth: func(u string) (transport.AuthMethod, error) {
			if !ok || acct.Token.Kind == "" || !gitrepo.IsHTTP(u) {
				return nil, nil
			}
			if h, ok := hostOf(u); !ok || !config.SameHost(h, acct.Host) {
				return nil, nil
			}
			tok, err := ss.Token(c, acct)
			if err != nil {
				return nil, err
			}
			return &githttp.BasicAuth{Username: "x-access-token", Password: tok}, nil
		},
	}
}

// GitHub is a client of host with the token of acct. An account with no token
// is KindInvalid, "auth.token".
func (ss *Session) GitHub(c *config.Config, acct config.Account, host string) (*github.Client, error) {
	if acct.Token.Kind == "" {
		return nil, newErr(KindInvalid, "auth.token",
			fmt.Errorf("account %s has no token: to set; run hedgehug auth login %s, or set token: in config.yaml", acct.Name, acct.Name),
			"account", acct.Name)
	}
	tok, err := ss.Token(c, acct)
	if err != nil {
		return nil, err
	}
	return github.New(c.Host(host), tok, ss.HTTPClient), nil
}

// PickAccount is the account name names, or the only one when name is empty;
// ok is false when there is none to pick. A name that is no account is
// KindNotFound, "account.not_found".
func (ss *Session) PickAccount(c *config.Config, name string) (config.Account, bool, error) {
	if name != "" {
		acct, ok := c.Account(name)
		if !ok {
			names := make([]string, len(c.Accounts))
			for i, x := range c.Accounts {
				names[i] = x.Name
			}
			return config.Account{}, false, newErr(KindNotFound, "account.not_found",
				fmt.Errorf("no account is named %q (accounts: %s)", name, strings.Join(names, ", ")),
				"name", name, "candidates", names)
		}
		return acct, true, nil
	}
	if len(c.Accounts) == 1 {
		return c.Accounts[0], true, nil
	}
	return config.Account{}, false, nil
}
