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
)

const (
	markerFile = ".hedgehug-devenv"
	storeName  = "me"
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
	for _, u := range users {
		if err := initUser(filepath.Join(dir, "users", u), u); err != nil {
			return fmt.Errorf("%s: %w", u, err)
		}
	}
	_, err := fmt.Fprintf(stdout, "Created %s with users %s.\nUse one with: eval \"$(go run ./tools/devenv env %s --dir %s)\"\n",
		dir, strings.Join(users, " and "), users[0], shellQuote(dir))
	return err
}

func initUser(home, u string) error {
	if err := os.MkdirAll(home, 0o750); err != nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
		return err
	}
	gitconfig := "[user]\n\tname = " + u + "\n\temail = " + u + "@example.com\n"
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(gitconfig), 0o600); err != nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
		return err
	}
	getenv := func(k string) string {
		switch k {
		case "HEDGEHUG_HOME", "HOME", "USERPROFILE":
			return home
		}
		return ""
	}
	p, err := config.Resolve(getenv)
	if err != nil {
		return err
	}
	storeDir := filepath.Join(p.StoresDir, storeName)
	author := gitrepo.Signature{Name: u, Email: u + "@example.com"}
	if _, err := store.Create(storeDir, &model.StoreMeta{ULID: model.NewULID(), Name: storeName}, nil, &model.Workspace{ULID: model.NewULID(), Name: storeName, IDPrefix: "DEV"}, author); err != nil {
		return err
	}
	c, err := config.Load(p)
	if err != nil {
		return err
	}
	if err := c.AddStore(storeName, storeDir); err != nil {
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
