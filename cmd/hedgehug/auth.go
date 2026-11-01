package main

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kukv/hedgehug/internal/github"
	"github.com/kukv/hedgehug/internal/ops"
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

// readToken reads the first line of stdin. An empty line is left for the
// operation to refuse.
func (a *app) readToken() (string, error) {
	line, err := a.readLine()
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read the token from stdin: %w", err)
	}
	return line, nil
}

func (a *app) authLogin(name, host string, hostGiven, withToken bool) error {
	in := ops.AuthLoginInput{Name: name, Host: host, HostGiven: hostGiven, WithToken: withToken, ShowCode: a.showDeviceCode}
	if withToken {
		tok, err := a.readToken()
		if err != nil {
			return err
		}
		in.Token = tok
	}
	res, err := a.ss.AuthLogin(in)
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
	_, err = fmt.Fprintf(a.stdout, "Logged in as %s (account %s)\n", res.ID, res.Account)
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
	line, _ := a.readLine()
	if ans := strings.ToLower(strings.TrimSpace(line)); ans == "" || ans == "y" || ans == "yes" {
		if err := a.openURL(dc.VerificationURI); err != nil {
			_, _ = fmt.Fprintln(a.stderr, "warning: the browser did not open:", err)
		}
	}
}

func (a *app) authStatus() error {
	out, err := a.ss.AuthStatus()
	if err != nil && !errors.Is(err, ops.ErrFindings) {
		return err
	}
	if a.jsonOut {
		if perr := a.printJSON(out); perr != nil {
			return perr
		}
		return err
	}
	if len(out) == 0 {
		_, perr := fmt.Fprintln(a.stdout, "No account is set up; run hedgehug auth login")
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
	res, err := a.ss.AuthLogout(name)
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
