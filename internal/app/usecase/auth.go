package usecase

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/app/remote/auth"
	"github.com/kukv/hedgehug/internal/app/storage/config"
	"github.com/kukv/hedgehug/internal/lib/github"
)

// AuthLoginInput is what hh auth login is given. The CLI reads Token from
// stdin for --with-token and shows the code of the device flow in ShowCode;
// the operation asks nothing.
type AuthLoginInput struct {
	Name      string
	Host      string // "" is github.com
	HostGiven bool   // --host was given: it must be the host of an account of that name
	WithToken bool   // keep Token in the keychain, instead of the device flow
	Token     string
	ShowCode  func(*github.DeviceCode) // the device flow's code; nil shows nothing
}

// AuthLoginResult is the account a login set the token of. Uses is set
// instead when the account takes its token from gh or an environment
// variable ("gh", "env:VAR"): Hedgehug has nothing to log in, and nothing was
// written.
type AuthLoginResult struct {
	Account string `json:"account"`
	ID      string `json:"id"`
	Added   bool   `json:"added"`
	Uses    string `json:"-"`
	Pinned  Pinned `json:"-"` // when an account was added: the stores that were given the account they used
}

// AuthLogin gets a token by the device flow, or takes the one given, checks
// that GitHub says it is of the account's login (compared without case), and
// keeps it in the keychain. config.yaml names the token source only after
// the keychain has the token; when config.yaml cannot be written, the
// keychain is put back as it was.
func (ss *Session) AuthLogin(in AuthLoginInput) (AuthLoginResult, error) {
	c, _, err := ss.Config()
	if err != nil {
		return AuthLoginResult{}, err
	}
	name, host := in.Name, in.Host
	if host == "" {
		host = "github.com"
	}
	acct, known := c.Account(name)
	if name != "" && known {
		if in.HostGiven && !config.SameHost(host, acct.Host) {
			return AuthLoginResult{}, newErr(KindUsage, "auth.host", fmt.Errorf("account %s is of %s, not of %s", name, acct.Host, host),
				"account", name, "host", acct.Host)
		}
		// An account with no token: falls through to the device flow, or
		// to --with-token, which gives it one.
		switch acct.Token.Kind {
		case "gh", "env":
			return AuthLoginResult{Account: acct.Name, ID: acct.ID(), Uses: acct.Token.String()}, nil
		case "keychain":
			if !in.WithToken {
				return AuthLoginResult{}, newErr(KindUsage, "auth.need_token",
					fmt.Errorf("account %s keeps a token in the keychain; give it on stdin with --with-token", name), "account", name)
			}
		case "device":
			if in.WithToken {
				return AuthLoginResult{}, newErr(KindUsage, "auth.device_only",
					fmt.Errorf("account %s logs in with the device flow; leave out --with-token", name), "account", name)
			}
		}
		host = acct.Host
	} else {
		known = false
	}
	h := c.Host(host)
	var tok string
	var dt auth.DeviceToken
	kind := "device"
	if in.WithToken {
		kind = "keychain"
		if tok = strings.TrimSpace(in.Token); tok == "" {
			return AuthLoginResult{}, newErr(KindInvalid, "auth.token", errors.New("no token on stdin"))
		}
	} else {
		show := in.ShowCode
		if show == nil {
			show = func(*github.DeviceCode) {}
		}
		// The polling holds no lock: auth.lock is taken to save, below.
		if dt, err = auth.DeviceLogin(h, ss.AuthOptions(c, config.Account{Host: host}), show); err != nil {
			return AuthLoginResult{}, err
		}
		tok = strings.TrimSpace(dt.AccessToken)
	}
	login, err := github.New(h.GitHub(), tok, ss.HTTPClient).User()
	if err != nil {
		if errors.Is(err, github.ErrUnauthorized) {
			// No "run hedgehug auth login" hint: that is what is being run.
			return AuthLoginResult{}, newErr(KindInvalid, "auth.unauthorized",
				fmt.Errorf("%s refused the token: %w; check that it is valid and has not expired", host, err), "account", name)
		}
		who := acct
		if !known {
			who = config.Account{Name: "<name>", Host: host, Login: "(new account)"}
			if name != "" {
				who.Name = name
			}
		}
		return AuthLoginResult{}, githubErr(err, who, "check the token")
	}
	if login = strings.TrimSpace(login); login == "" {
		return AuthLoginResult{}, errors.New("check the token: GitHub gave no login")
	}
	if !known {
		// An account of the same host and login takes the token instead.
		for _, x := range c.Accounts {
			if config.SameAccount(x.ID(), host+"/"+login) {
				acct, known = x, true
				break
			}
		}
	}
	added := false
	if known {
		if !strings.EqualFold(acct.Login, login) {
			return AuthLoginResult{}, fmt.Errorf("the token is of %s/%s, not of account %s (%s); it stays valid on GitHub, so revoke it in GitHub's settings (Settings → Applications → Authorized GitHub Apps) if it is not wanted", host, login, acct.Name, acct.ID())
		}
		acct.Token = config.TokenSource{Kind: kind}
	} else {
		if name == "" {
			name = login
		}
		acct = config.Account{Name: name, Host: host, Login: login, Token: config.TokenSource{Kind: kind}}
		added = true
	}

	// The keychain first, under auth.lock: config.yaml names a token only
	// once it is kept.
	opt := ss.AuthOptions(c, acct)
	k := opt.Keychain()
	key := auth.Key(acct)
	var prev string
	had := false
	err = auth.WithLock(opt, func() error {
		v, err := k.Get(auth.Service, key)
		switch {
		case err == nil:
			prev, had = v, true
		case !errors.Is(err, auth.ErrNotFound):
			return err
		}
		if kind == "device" {
			return auth.SaveDeviceToken(acct, dt, k)
		}
		return auth.SaveToken(acct, tok, k)
	})
	if err != nil {
		return AuthLoginResult{}, err
	}
	var pinned Pinned
	err = ss.updateConfig(c, func(c *config.Config) error {
		if added {
			for _, x := range c.Accounts { // another writer may have added it since the read
				if config.SameAccount(x.ID(), acct.ID()) {
					return newErr(KindConflict, "account.exists", fmt.Errorf("account %s (%s) is set up already; run hedgehug auth login %s", x.Name, x.ID(), x.Name),
						"account", x.Name)
				}
			}
			p, err := addAccount(c, acct)
			pinned = p
			return err
		}
		if _, ok := c.Account(acct.Name); !ok {
			return newErr(KindNotFound, "account.not_found", fmt.Errorf("no account is named %q", acct.Name), "name", acct.Name)
		}
		return c.SetAccountToken(acct.Name, acct.Token)
	})
	if err != nil {
		// Put the keychain back as it was.
		rerr := auth.WithLock(opt, func() error {
			if had {
				return k.Set(auth.Service, key, prev)
			}
			return auth.Delete(acct, k)
		})
		if rerr != nil {
			return AuthLoginResult{}, fmt.Errorf("%w (and the keychain could not be put back as it was: %v; config.yaml was not changed)", err, rerr)
		}
		return AuthLoginResult{}, err
	}
	return AuthLoginResult{Account: acct.Name, ID: acct.ID(), Added: added, Pinned: pinned}, nil
}

// AuthStatus is what auth status says of one account. State is "ok", "error"
// (Error says why) or "none" (the account has no token, so nothing was
// checked and Token, Expires, Login and Error are left out).
type AuthStatus struct {
	Name    string     `json:"name"`
	ID      string     `json:"id"`
	Token   string     `json:"token,omitempty"`
	Expires *time.Time `json:"expires,omitempty"` // of a device token that ends
	State   string     `json:"state"`
	Login   string     `json:"login,omitempty"` // what GitHub says the token is of
	Error   string     `json:"error,omitempty"`
}

// Pinned is what addAccount wrote before it added an account: Stores, the
// stores that had no account: of their own, were given Account, the only
// account there was.
type Pinned struct {
	Account string
	Stores  []string
}

// addAccount adds acct to c. A store with no account: uses the only account
// there is, and has none to use once a second one is added (config.yaml then
// fails to load), so the stores are given that account first.
func addAccount(c *config.Config, acct config.Account) (Pinned, error) {
	var pinned Pinned
	if len(c.Accounts) == 1 {
		pinned.Account = c.Accounts[0].Name
		for _, e := range c.Stores {
			if e.Account != "" {
				continue
			}
			if err := c.SetStoreAccount(e.Name, pinned.Account); err != nil {
				return Pinned{}, err
			}
			pinned.Stores = append(pinned.Stores, e.Name)
		}
	}
	return pinned, c.AddAccount(acct)
}

// AuthAddInput is what hh auth add is given. Host "" is github.com. Token is
// "gh", "env:VAR" or "keychain"; "" is an account that uses no token.
type AuthAddInput struct {
	Name  string
	Login string
	Host  string
	Token string
}

// AuthAddResult is the account AuthAdd wrote, as auth add --json prints it.
// Token is where its token comes from, left out for none.
type AuthAddResult struct {
	Name   string `json:"name"`
	ID     string `json:"id"`
	Token  string `json:"token,omitempty"`
	Pinned Pinned `json:"-"`
}

// AuthAdd adds an account to config.yaml. It gets no token (neither gh nor
// the keychain is asked) and asks GitHub nothing; auth status checks it. The
// stores that would be left with no account get it in the same write
// (addAccount).
func (ss *Session) AuthAdd(in AuthAddInput) (AuthAddResult, error) {
	name, login, host := strings.TrimSpace(in.Name), strings.TrimSpace(in.Login), strings.TrimSpace(in.Host)
	if name == "" {
		return AuthAddResult{}, newErr(KindUsage, "auth.name", errors.New("an account name is needed"))
	}
	if login == "" {
		return AuthAddResult{}, newErr(KindUsage, "auth.login", errors.New("the login of the account is needed"))
	}
	if host == "" {
		host = "github.com"
	}
	tok, err := parseAddToken(strings.TrimSpace(in.Token))
	if err != nil {
		return AuthAddResult{}, err
	}
	acct := config.Account{Name: name, Host: host, Login: login, Token: tok}
	c, _, err := ss.Config()
	if err != nil {
		return AuthAddResult{}, err
	}
	var pinned Pinned
	if err := ss.updateConfig(c, func(c *config.Config) error {
		if err := checkNewAccount(c, acct); err != nil {
			return err
		}
		p, err := addAccount(c, acct)
		pinned = p
		return err
	}); err != nil {
		return AuthAddResult{}, err
	}
	return AuthAddResult{Name: acct.Name, ID: acct.ID(), Token: tok.String(), Pinned: pinned}, nil
}

// parseAddToken reads --token of auth add: gh, env:VAR or keychain. device
// is left out: it comes only from a login.
func parseAddToken(s string) (config.TokenSource, error) {
	switch s {
	case "":
		return config.TokenSource{}, nil
	case "gh", "keychain":
		return config.TokenSource{Kind: s}, nil
	}
	if v, ok := strings.CutPrefix(s, "env:"); ok && v != "" {
		return config.TokenSource{Kind: "env", Env: v}, nil
	}
	return config.TokenSource{}, newErr(KindUsage, "auth.token_source", fmt.Errorf("--token %q: want gh, env:<VAR> or keychain", s), "token", s)
}

// checkNewAccount refuses an account whose name, or whose host and login
// (compared without case), an account of c has already: the rules config.yaml
// is loaded by.
func checkNewAccount(c *config.Config, acct config.Account) error {
	for _, x := range c.Accounts {
		switch {
		case x.Name == acct.Name:
			return newErr(KindConflict, "account.exists", fmt.Errorf("an account named %q is set up already (%s)", x.Name, x.ID()), "name", x.Name)
		case config.SameAccount(x.ID(), acct.ID()):
			return newErr(KindConflict, "account.exists", fmt.Errorf("account %s (%s) is set up already", x.Name, x.ID()), "name", x.Name)
		}
	}
	return nil
}

// AuthStatus checks the token of each account against GitHub. When any is in
// error it returns the whole result with ErrFindings.
func (ss *Session) AuthStatus() ([]AuthStatus, error) {
	c, _, err := ss.Config()
	if err != nil {
		return nil, err
	}
	out := make([]AuthStatus, 0, len(c.Accounts))
	bad := false
	for _, acct := range c.Accounts {
		x := AuthStatus{Name: acct.Name, ID: acct.ID(), State: "none"}
		if acct.Token.Kind == "" {
			out = append(out, x)
			continue
		}
		x.Token, x.State = acct.Token.String(), "ok"
		tok, err := ss.Token(c, acct)
		if err == nil {
			x.Login, err = github.New(c.Host(acct.Host).GitHub(), tok, ss.HTTPClient).User()
			err = github.Describe(err, acct.ID(), acct.Name, "check the token")
		}
		switch {
		case err != nil:
			x.State, x.Error = "error", err.Error()
		case !strings.EqualFold(strings.TrimSpace(x.Login), acct.Login):
			x.State, x.Error = "error", fmt.Sprintf("the token is of %s/%s", acct.Host, x.Login)
		}
		if acct.Token.Kind == "device" {
			if dt, err := auth.LoadDeviceToken(acct, ss.AuthOptions(c, acct).Keychain()); err == nil && !dt.ExpiresAt.IsZero() {
				t := dt.ExpiresAt
				x.Expires = &t
			}
		}
		bad = bad || x.State == "error"
		out = append(out, x)
	}
	if bad {
		return out, ErrFindings
	}
	return out, nil
}

// AuthLogoutResult is what a logout did. Token is where the account's token
// comes from ("" for none), so that the caller can say what Hedgehug does not
// keep when Removed is false.
type AuthLogoutResult struct {
	Account string `json:"account"`
	ID      string `json:"id"`
	Token   string `json:"token,omitempty"`
	Removed bool   `json:"removed"`
}

// AuthLogout removes the token of the account name from the keychain, under
// auth.lock. The account stays in config.yaml. Removed is false for an
// account whose token Hedgehug does not keep (gh, env, none).
func (ss *Session) AuthLogout(name string) (AuthLogoutResult, error) {
	if name == "" {
		return AuthLogoutResult{}, newErr(KindUsage, "auth.name", errors.New("an account name is needed"))
	}
	c, _, err := ss.Config()
	if err != nil {
		return AuthLogoutResult{}, err
	}
	acct, _, err := ss.PickAccount(c, name)
	if err != nil {
		return AuthLogoutResult{}, err
	}
	res := AuthLogoutResult{Account: acct.Name, ID: acct.ID(), Token: acct.Token.String()}
	if acct.Token.Kind != "keychain" && acct.Token.Kind != "device" {
		return res, nil
	}
	opt := ss.AuthOptions(c, acct)
	if err := auth.WithLock(opt, func() error { return auth.Delete(acct, opt.Keychain()) }); err != nil {
		return AuthLogoutResult{}, err
	}
	res.Removed = true
	return res, nil
}
