package main

import (
	"path/filepath"

	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/kukv/hedgehug/internal/auth"
	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/gitrepo"
)

// authOptions is what getting the token of acct needs.
func (a *app) authOptions(c *config.Config, acct config.Account) auth.Options {
	return auth.Options{
		Getenv: a.getenv, Run: a.run, Keyring: a.keyring, Now: a.now, HTTP: a.httpClient,
		Host:     c.Host(acct.Host),
		LockFile: filepath.Join(filepath.Dir(c.File()), "auth.lock"),
	}
}

// tokenFor returns the token of acct, getting it once per run.
func (a *app) tokenFor(c *config.Config, acct config.Account) (string, error) {
	if tok, ok := a.tokens[acct.Name]; ok {
		return tok, nil
	}
	tok, err := auth.Token(acct, a.authOptions(c, acct))
	if err != nil {
		return "", err
	}
	if a.tokens == nil {
		a.tokens = map[string]string{}
	}
	a.tokens[acct.Name] = tok
	return tok, nil
}

// connFor is how the remotes of the store e are reached: through
// url_rewrites, and with the token of its account over http and https. A
// store with no account, an account with no token, and an ssh or local
// remote get no auth from here (ssh falls to go-git's ssh-agent).
func (a *app) connFor(c *config.Config, e config.StoreEntry) gitrepo.Conn {
	acct, ok := c.Writer(e)
	return gitrepo.Conn{
		Rewrite: c.Rewrite,
		Auth: func(u string) (transport.AuthMethod, error) {
			if !ok || acct.Token.Kind == "" || !gitrepo.IsHTTP(u) {
				return nil, nil
			}
			tok, err := a.tokenFor(c, acct)
			if err != nil {
				return nil, err
			}
			return &githttp.BasicAuth{Username: "x-access-token", Password: tok}, nil
		},
	}
}
