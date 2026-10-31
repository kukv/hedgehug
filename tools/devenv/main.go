// Command devenv sets up a throwaway development environment with two users,
// alice and bob, each with their own HEDGEHUG_HOME, git identity and store.
// It is for developers of Hedgehug and is not shipped.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

const (
	markerFile = ".hedgehug-devenv"
	storeName  = "me"

	sharedName   = "org-a"
	sharedPrefix = "ORG"
	host         = "github.com"
	accountName  = "personal"
)

// Bare remotes under the environment's directory: alice's own store, and the
// store alice and bob share.
var (
	personalRemote = filepath.Join("remotes", "kukv", "hh-tasks.git")
	sharedRemote   = filepath.Join("remotes", "org-a", "hh-shared.git")
)

var users = []string{"alice", "bob"}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "devenv:", err)
		os.Exit(1)
	}
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, cwd))
}

// usageError marks a mistake in the arguments (exit code 2).
type usageError struct{ error }

func run(args []string, stdout, stderr io.Writer, cwd string) int {
	err := dispatch(args, stdout, stderr, cwd)
	var ue usageError
	switch {
	case err == nil, errors.Is(err, flag.ErrHelp):
		return 0
	case errors.As(err, &ue):
		_, _ = fmt.Fprintln(stderr, "devenv:", err) // nothing useful to do if stderr is unwritable
		return 2
	default:
		_, _ = fmt.Fprintln(stderr, "devenv:", err) // nothing useful to do if stderr is unwritable
		return 1
	}
}

func dispatch(args []string, stdout, stderr io.Writer, cwd string) error {
	if len(args) == 0 {
		return usageError{errors.New("usage: devenv init|reset|env <user> [--dir <path>]")}
	}
	cmd, rest := args[0], args[1:]
	fs := flag.NewFlagSet("devenv "+cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "", "directory of the environment (default: .devenv at the repository root, found by walking up to go.mod)")

	var want int // number of positional arguments
	switch cmd {
	case "init", "reset":
	case "env":
		want = 1
	default:
		return usageError{fmt.Errorf("unknown command %q (init, reset or env)", cmd)}
	}
	// Allow the flag before or after the positional argument.
	var pos []string
	for len(rest) > 0 {
		if err := fs.Parse(rest); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return err
			}
			return usageError{err}
		}
		rest = fs.Args()
		if len(rest) > 0 {
			pos = append(pos, rest[0])
			rest = rest[1:]
		}
	}
	if len(pos) != want {
		return usageError{fmt.Errorf("%s takes %d argument(s), got %d", cmd, want, len(pos))}
	}

	d, err := resolveDir(*dir, cwd)
	if err != nil {
		return err
	}
	switch cmd {
	case "init":
		return initEnv(d, stdout)
	case "reset":
		return reset(d, stdout)
	default:
		return printEnv(d, pos[0], stdout)
	}
}

// resolveDir is the absolute directory of the environment. A relative --dir
// is relative to cwd; without --dir it is .devenv in the repository root.
func resolveDir(dir, cwd string) (string, error) {
	if dir == "" {
		root, err := findRoot(cwd)
		if err != nil {
			return "", err
		}
		return filepath.Join(root, ".devenv"), nil
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(cwd, dir)
	}
	return filepath.Clean(dir), nil
}

// findRoot walks up from dir to the directory that holds go.mod.
func findRoot(dir string) (string, error) {
	for d := dir; ; {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d, nil
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", fmt.Errorf("no go.mod found above %s; run in the repository or pass --dir", dir)
		}
		d = parent
	}
}

func initEnv(dir string, stdout io.Writer) error {
	if _, err := os.Lstat(dir); err == nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
		return fmt.Errorf("%s already exists; run `devenv reset` to recreate it", dir)
	}
	if err := os.MkdirAll(dir, 0o750); err != nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, markerFile), nil, 0o600); err != nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
		return err
	}
	for _, r := range []string{personalRemote, sharedRemote} {
		if err := gitrepo.InitBare(filepath.Join(dir, r)); err != nil {
			return err
		}
	}
	for _, u := range users {
		// Only alice's own store has a remote; bob's stays local.
		var remotes []string
		if u == users[0] {
			remotes = []string{filepath.Join(dir, personalRemote)}
		}
		if err := initUser(filepath.Join(dir, "users", u), u, remotes); err != nil {
			return fmt.Errorf("%s: %w", u, err)
		}
	}
	if err := initShared(dir); err != nil {
		return fmt.Errorf("%s: %w", sharedName, err)
	}
	_, err := fmt.Fprintf(stdout, "Created %s with users %s.\nUse one with: eval \"$(go run ./tools/devenv env %s --dir %s)\"\n",
		dir, strings.Join(users, " and "), users[0], shellQuote(dir))
	return err
}

// userEnv is what a user's hedgehug sees: the paths of their home and
// config, and the account that writes their commits.
type userEnv struct {
	paths  config.Paths
	author gitrepo.Signature
	acct   config.Account
}

func newUserEnv(home, u string) (*userEnv, error) {
	getenv := func(k string) string {
		switch k {
		case "HEDGEHUG_HOME", "HOME", "USERPROFILE":
			return home
		}
		return ""
	}
	p, err := config.Resolve(getenv)
	if err != nil {
		return nil, err
	}
	acct := config.Account{Name: accountName, Host: host, Login: u}
	author, err := config.CommitAuthor(acct, true, getenv)
	if err != nil {
		return nil, err
	}
	return &userEnv{paths: p, author: author, acct: acct}, nil
}

// sync syncs s once with remotes, as the user's account. It runs in this
// process, so no background sync starts.
func (e *userEnv) sync(s *store.Store, remotes []string) error {
	_, err := sync.Run(s, sync.Options{Remotes: remotes, Author: e.author, Account: e.acct.ID(), Me: []string{e.acct.ID()}})
	return err
}

func initUser(home, u string, remotes []string) error {
	if err := os.MkdirAll(home, 0o750); err != nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
		return err
	}
	gitconfig := "[user]\n\tname = " + u + "\n\temail = " + u + "@example.com\n"
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(gitconfig), 0o600); err != nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
		return err
	}
	e, err := newUserEnv(home, u)
	if err != nil {
		return err
	}
	// The account is written before the config is loaded, so that AddStore keeps it.
	accounts := fmt.Sprintf("accounts:\n  - name: %s\n    host: %s\n    login: %s\n", e.acct.Name, e.acct.Host, e.acct.Login)
	if err := os.WriteFile(e.paths.ConfigFile, []byte(accounts), 0o600); err != nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
		return err
	}
	storeDir := filepath.Join(e.paths.StoresDir, storeName)
	s, err := store.Create(storeDir, &model.StoreMeta{ULID: model.NewULID(), Name: storeName}, nil, &model.Workspace{ULID: model.NewULID(), Name: storeName, IDPrefix: "DEV"}, e.author, e.acct.ID())
	if err != nil {
		return err
	}
	if err := s.Repo().RegisterMergeDriver(); err != nil {
		return err
	}
	if len(remotes) > 0 {
		if err := e.sync(s, remotes); err != nil {
			return err
		}
	}
	return e.addStore(storeName, storeDir, remotes)
}

// initShared makes the store org-a: alice creates it and pushes it, bob
// clones it.
func initShared(dir string) error {
	remote := []string{filepath.Join(dir, sharedRemote)}
	alice, err := newUserEnv(filepath.Join(dir, "users", users[0]), users[0])
	if err != nil {
		return err
	}
	aliceDir := filepath.Join(alice.paths.StoresDir, sharedName)
	s, err := store.Create(aliceDir, &model.StoreMeta{ULID: model.NewULID(), Name: sharedName}, nil, &model.Workspace{ULID: model.NewULID(), Name: sharedName, IDPrefix: sharedPrefix}, alice.author, alice.acct.ID())
	if err != nil {
		return err
	}
	if err := s.Repo().RegisterMergeDriver(); err != nil {
		return err
	}
	if err := alice.sync(s, remote); err != nil {
		return err
	}
	if err := alice.addStore(sharedName, aliceDir, remote); err != nil {
		return err
	}

	bob, err := newUserEnv(filepath.Join(dir, "users", users[1]), users[1])
	if err != nil {
		return err
	}
	bobDir := filepath.Join(bob.paths.StoresDir, sharedName)
	if _, err := gitrepo.Clone(remote[0], bobDir); err != nil {
		return err
	}
	bs, err := store.Open(bobDir)
	if err != nil {
		return err
	}
	if err := bs.Repo().RegisterMergeDriver(); err != nil {
		return err
	}
	if err := bs.Repo().SetRemotes(remote); err != nil {
		return err
	}
	return bob.addStore(sharedName, bobDir, remote)
}

func (e *userEnv) addStore(name, path string, remotes []string) error {
	c, err := config.Load(e.paths)
	if err != nil {
		return err
	}
	if err := c.AddStore(name, path, remotes); err != nil {
		return err
	}
	if err := c.SetStoreAccount(name, e.acct.Name); err != nil {
		return err
	}
	return c.Save()
}

// reset removes dir when it is a devenv (it holds the marker file) and
// creates it again. A missing dir is only created.
func reset(dir string, stdout io.Writer) error {
	if _, err := os.Lstat(dir); err == nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
		if _, err := os.Stat(filepath.Join(dir, markerFile)); err != nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
			return fmt.Errorf("%s was not made by devenv (no %s in it); not removing it", dir, markerFile)
		}
		if err := os.RemoveAll(dir); err != nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
			return err
		}
	}
	return initEnv(dir, stdout)
}

func printEnv(dir, user string, stdout io.Writer) error {
	home := filepath.Join(dir, "users", user)
	known := false
	for _, u := range users {
		known = known || u == user
	}
	if !known {
		return fmt.Errorf("no user named %q (users: %s)", user, strings.Join(users, ", "))
	}
	if _, err := os.Stat(home); err != nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
		return fmt.Errorf("%s: %w; run `devenv init` first", home, err)
	}
	_, err := fmt.Fprintf(stdout, "export HEDGEHUG_HOME=%s\nexport GIT_CONFIG_GLOBAL=%s\n",
		shellQuote(home), shellQuote(filepath.Join(home, ".gitconfig")))
	return err
}

// shellQuote quotes s for sh with single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
