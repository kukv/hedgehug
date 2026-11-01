package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/auth"
	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/github"
)

func (a *app) authCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage the accounts and their tokens",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SetOut(cmd.ErrOrStderr())
			_ = cmd.Usage()
			return usageErrorf("a subcommand is needed")
		},
	}
	var host string
	var withToken bool
	login := &cobra.Command{
		Use:   "login [<name>]",
		Short: "Log in with the device flow, or keep a token given on stdin",
		Args:  asUsage(cobra.MaximumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			return a.authLogin(name, host, cmd.Flags().Changed("host"), withToken)
		},
	}
	login.Flags().StringVar(&host, "host", "github.com", "the host of a new account")
	login.Flags().BoolVar(&withToken, "with-token", false, "read a token from stdin and keep it in the keychain")
	status := &cobra.Command{
		Use:   "status",
		Short: "Check the token of each account",
		Args:  cobra.NoArgs,
		RunE:  func(_ *cobra.Command, _ []string) error { return a.authStatus() },
	}
	logout := &cobra.Command{
		Use:   "logout <name>",
		Short: "Remove the token of an account from the keychain",
		Args:  exactArgs(1),
		RunE:  func(_ *cobra.Command, args []string) error { return a.authLogout(args[0]) },
	}
	cmd.AddCommand(login, status, logout)
	return cmd
}

// sameLogin compares two logins as GitHub does, ignoring case.
func sameLogin(x, y string) bool { return strings.EqualFold(x, y) }

// readToken reads the token from the first line of stdin.
func (a *app) readToken() (string, error) {
	line, err := bufio.NewReader(a.stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read the token from stdin: %w", err)
	}
	tok := strings.TrimSpace(line)
	if tok == "" {
		return "", errors.New("no token on stdin")
	}
	return tok, nil
}

func (a *app) authLogin(name, host string, hostGiven, withToken bool) error {
	c, _, err := a.ss.Config()
	if err != nil {
		return err
	}
	acct, known := c.Account(name)
	if known {
		if hostGiven && host != acct.Host {
			return usageErrorf("account %s is of %s, not of %s", name, acct.Host, host)
		}
		// An account with no token: falls through to the device flow, or
		// to --with-token, which gives it one.
		switch acct.Token.Kind {
		case "gh", "env":
			_, err := fmt.Fprintf(a.stdout, "Nothing to log in for %s: it uses %s\n", name, acct.Token.String())
			return err
		case "keychain":
			if !withToken {
				return usageErrorf("account %s keeps a token in the keychain; give it on stdin with --with-token", name)
			}
		case "device":
			if withToken {
				return usageErrorf("account %s logs in with the device flow; leave out --with-token", name)
			}
		}
		host = acct.Host
	}
	h := c.Host(host)
	var tok string
	var dt auth.DeviceToken
	kind := "device"
	if withToken {
		kind = "keychain"
		if tok, err = a.readToken(); err != nil {
			return err
		}
	} else {
		dt, err = auth.DeviceLogin(h, a.ss.AuthOptions(c, config.Account{Host: host}), a.showDeviceCode)
		if err != nil {
			return err
		}
		tok = dt.AccessToken
	}
	login, err := github.New(h, tok, a.ss.HTTPClient).User()
	if err != nil {
		return fmt.Errorf("check the token: %w", err)
	}
	if !known {
		// An account of the same host and login takes the token instead.
		for _, x := range c.Accounts {
			if x.Host == host && sameLogin(x.Login, login) {
				acct, known = x, true
				break
			}
		}
	}
	added := false
	if known {
		if !sameLogin(acct.Login, login) {
			return fmt.Errorf("the token is of %s/%s, not of account %s (%s)", host, login, acct.Name, acct.ID())
		}
		acct.Token = config.TokenSource{Kind: kind}
		if err := c.SetAccountToken(acct.Name, acct.Token); err != nil {
			return err
		}
	} else {
		if name == "" {
			name = login
		}
		acct = config.Account{Name: name, Host: host, Login: login, Token: config.TokenSource{Kind: kind}}
		if err := c.AddAccount(acct); err != nil {
			return err
		}
		added = true
	}
	// The keychain first: config.yaml names a token only once it is kept.
	k := a.ss.AuthOptions(c, acct).Keychain()
	if kind == "device" {
		err = auth.SaveDeviceToken(acct, dt, k)
	} else {
		err = auth.SaveToken(acct, tok, k)
	}
	if err != nil {
		return err
	}
	if err := c.Save(); err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(struct {
			Account string `json:"account"`
			ID      string `json:"id"`
			Added   bool   `json:"added"`
		}{acct.Name, acct.ID(), added})
	}
	_, err = fmt.Fprintf(a.stdout, "Logged in as %s (account %s)\n", acct.ID(), acct.Name)
	return err
}

// showDeviceCode tells where to enter the code, and on a terminal offers to
// open the browser there.
func (a *app) showDeviceCode(dc *github.DeviceCode) {
	_, _ = fmt.Fprintf(a.stderr, "Open %s and enter the code %s\n", dc.VerificationURI, dc.UserCode)
	if a.openURL == nil || !a.isTerminal(a.stdin) {
		return
	}
	_, _ = fmt.Fprint(a.stderr, "Open the browser? [Y/n] ")
	line, _ := bufio.NewReader(a.stdin).ReadString('\n')
	if ans := strings.ToLower(strings.TrimSpace(line)); ans == "" || ans == "y" || ans == "yes" {
		if err := a.openURL(dc.VerificationURI); err != nil {
			_, _ = fmt.Fprintln(a.stderr, "warning: the browser did not open:", err)
		}
	}
}

type authStatusJSON struct {
	Name    string     `json:"name"`
	ID      string     `json:"id"`
	Token   string     `json:"token"`
	OK      bool       `json:"ok"`
	Login   string     `json:"login,omitempty"` // what GitHub says the token is of
	Expires *time.Time `json:"expires,omitempty"`
	Error   string     `json:"error,omitempty"`
}

func (a *app) authStatus() error {
	c, _, err := a.ss.Config()
	if err != nil {
		return err
	}
	out := make([]authStatusJSON, len(c.Accounts))
	bad := false
	for i, acct := range c.Accounts {
		x := authStatusJSON{Name: acct.Name, ID: acct.ID(), Token: acct.Token.String(), OK: true}
		out[i] = x
		if acct.Token.Kind == "" {
			continue
		}
		tok, err := a.ss.Token(c, acct)
		if err == nil {
			x.Login, err = github.New(c.Host(acct.Host), tok, a.ss.HTTPClient).User()
			err = github.Describe(err, acct.ID(), acct.Name, "check the token")
		}
		switch {
		case err != nil:
			x.OK, x.Error = false, err.Error()
		case !sameLogin(x.Login, acct.Login):
			x.OK, x.Error = false, fmt.Sprintf("the token is of %s/%s", acct.Host, x.Login)
		}
		if acct.Token.Kind == "device" {
			if dt, err := auth.LoadDeviceToken(acct, a.ss.AuthOptions(c, acct).Keychain()); err == nil && !dt.ExpiresAt.IsZero() {
				x.Expires = &dt.ExpiresAt
			}
		}
		bad = bad || !x.OK
		out[i] = x
	}
	if a.jsonOut {
		if err := a.printJSON(out); err != nil {
			return err
		}
	} else {
		if len(out) == 0 {
			_, err := fmt.Fprintln(a.stdout, "No account is set up; run hedgehug auth login")
			return err
		}
		rows := make([][]string, len(out))
		for i, x := range out {
			token, state := x.Token, "ok"
			if token == "" {
				token, state = "-", "-"
			} else if !x.OK {
				state = x.Error
			}
			rows[i] = []string{x.Name, x.ID, token, expiryText(x.Expires), state}
		}
		if err := writeTable(a.stdout, []string{"NAME", "ACCOUNT", "TOKEN", "EXPIRES", "STATE"}, rows, nil); err != nil {
			return err
		}
	}
	if bad {
		return errFindings
	}
	return nil
}

// expiryText is the expiry for the table: - for a token that has none.
func expiryText(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return timeText(t)
}

func (a *app) authLogout(name string) error {
	c, _, err := a.ss.Config()
	if err != nil {
		return err
	}
	acct, ok := c.Account(name)
	if !ok {
		return fmt.Errorf("no account is named %q", name)
	}
	removed := false
	if acct.Token.Kind == "keychain" || acct.Token.Kind == "device" {
		if err := auth.Delete(acct, a.ss.AuthOptions(c, acct).Keychain()); err != nil {
			return err
		}
		removed = true
	}
	if a.jsonOut {
		return a.printJSON(struct {
			Account string `json:"account"`
			Removed bool   `json:"removed"`
		}{name, removed})
	}
	if !removed {
		uses := "it has no token"
		if acct.Token.Kind != "" {
			uses = "it uses " + acct.Token.String()
		}
		_, err = fmt.Fprintf(a.stdout, "Hedgehug keeps nothing for %s: %s\n", name, uses)
		return err
	}
	_, err = fmt.Fprintf(a.stdout, "Removed the token of %s from the keychain\n", acct.ID())
	return err
}
