package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/app/usecase"
	"github.com/kukv/hedgehug/internal/app/usecase/accounts"
	"github.com/kukv/hedgehug/internal/lib/plural"
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
	cmd.AddCommand(a.authAddCmd(), login, status, logout)
	return cmd
}

// readToken reads the first line of stdin. An empty line is left for the
// operation to refuse.
func (a *app) readToken() (string, error) {
	line, err := a.readLine()
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read the token from stdin: %w", err)
	}
	return line, nil
}

func (a *app) authAddCmd() *cobra.Command {
	var login, host, token string
	cmd := &cobra.Command{
		Use:   "add <name> --login <login>",
		Short: "Add an account to config.yaml",
		Long: `Add writes an account to config.yaml. It gets no token and asks GitHub nothing; hedgehug auth status checks the account.

--token says where the token comes from: gh, env:<VAR> or keychain. Without it the account uses no token. For keychain, give the token afterwards with hedgehug auth login <name> --with-token.

When this is the second account, the stores that had no account: are given the first, so that they keep the account they used.`,
		Args: exactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if login == "" {
				return usageErrorf("--login is required: the GitHub login of the account")
			}
			return a.authAdd(args[0], login, host, token)
		},
	}
	cmd.Flags().StringVar(&login, "login", "", "the GitHub login of the account")
	cmd.Flags().StringVar(&host, "host", "github.com", "the host of the account")
	cmd.Flags().StringVar(&token, "token", "", "where the token comes from: gh, env:<VAR> or keychain (default: none)")
	return cmd
}

func (a *app) authAdd(name, login, host, token string) error {
	res, err := accounts.AuthAdd(a.ss, accounts.AuthAddInput{Name: name, Login: login, Host: host, Token: token})
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(res)
	}
	if _, err := fmt.Fprintf(a.stdout, "Added account %s (%s)\n", res.Name, res.ID); err != nil {
		return err
	}
	if err := a.printPinned(res.Pinned); err != nil {
		return err
	}
	if res.Token == "keychain" {
		_, err = fmt.Fprintf(a.stdout, "Give it the token with: hedgehug auth login %s --with-token\n", res.Name)
	}
	return err
}

// printPinned tells which stores were given an account when a second account
// was added; nothing when none was.
func (a *app) printPinned(p accounts.Pinned) error {
	if len(p.Stores) == 0 {
		return nil
	}
	_, err := fmt.Fprintf(a.stdout, "Set account %s on %s %s\n", p.Account, plural.Pick(len(p.Stores), "store", "stores"), strings.Join(p.Stores, ", "))
	return err
}

func (a *app) authLogin(name, host string, hostGiven, withToken bool) error {
	in := accounts.AuthLoginInput{Name: name, Host: host, HostGiven: hostGiven, WithToken: withToken, OnCode: a.showDeviceCode}
	if withToken {
		tok, err := a.readToken()
		if err != nil {
			return err
		}
		in.Token = tok
	}
	res, err := accounts.AuthLogin(a.ss, in)
	if err != nil {
		return err
	}
	if res.Uses != "" {
		_, err := fmt.Fprintf(a.stdout, "Nothing to log in for %s: it uses %s\n", res.Account, res.Uses)
		return err
	}
	if a.jsonOut {
		return a.printJSON(res)
	}
	if _, err := fmt.Fprintf(a.stdout, "Logged in as %s (account %s)\n", res.ID, res.Account); err != nil {
		return err
	}
	return a.printPinned(res.Pinned)
}

// showDeviceCode tells where to enter the code, and on a terminal offers to
// open the browser there.
func (a *app) showDeviceCode(dc accounts.DeviceCode) {
	_, _ = fmt.Fprintf(a.stderr, "Open %s and enter the code %s\n", dc.VerificationURI, dc.UserCode)
	if a.openURL == nil || !a.isTerminal(a.stdin) {
		return
	}
	_, _ = fmt.Fprint(a.stderr, "Open the browser? [Y/n] ")
	line, _ := a.readLine()
	if ans := strings.ToLower(strings.TrimSpace(line)); ans == "" || ans == "y" || ans == "yes" {
		go func() { a.warnBrowser(a.openURL(dc.VerificationURI)) }() // do not wait for the opener
	}
}

func (a *app) authStatus() error {
	out, err := accounts.AuthStatuses(a.ss)
	if err != nil && !errors.Is(err, usecase.ErrFindings) {
		return err
	}
	if a.jsonOut {
		if perr := a.printJSON(out); perr != nil {
			return perr
		}
		return err
	}
	if len(out) == 0 {
		_, perr := fmt.Fprintln(a.stdout, "No account is set up; add one with hedgehug auth add <name> --login <login> (or hedgehug auth login)")
		return perr
	}
	rows := make([][]string, len(out))
	for i, x := range out {
		token, state := x.Token, "ok"
		switch x.State {
		case "none":
			token, state = "-", "-"
		case "error":
			state = x.Error
		}
		rows[i] = []string{x.Name, x.ID, token, expiryText(x.Expires), state}
	}
	if perr := writeTable(a.stdout, []string{"NAME", "ACCOUNT", "TOKEN", "EXPIRES", "STATE"}, rows, nil); perr != nil {
		return perr
	}
	return err
}

// expiryText is the expiry for the table: - for a token that has none.
func expiryText(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return timeText(t)
}

func (a *app) authLogout(name string) error {
	res, err := accounts.AuthLogout(a.ss, name)
	if err != nil {
		return err
	}
	if a.jsonOut {
		return a.printJSON(struct {
			Account string `json:"account"`
			Removed bool   `json:"removed"`
		}{res.Account, res.Removed})
	}
	if !res.Removed {
		uses := "it has no token"
		if res.Token != "" {
			uses = "it uses " + res.Token
		}
		_, err = fmt.Fprintf(a.stdout, "Hedgehug keeps nothing for %s: %s\n", res.Account, uses)
		return err
	}
	_, err = fmt.Fprintf(a.stdout, "Removed the token of %s from the keychain\n", res.ID)
	return err
}
