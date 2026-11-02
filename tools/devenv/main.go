// Command devenv sets up a throwaway development environment with two users,
// alice and bob, each with their own HEDGEHUG_HOME, git identity, accounts and
// stores, and the seed of the fake GitHub (fakegh) they talk to. It is for
// developers of Hedgehug and is not shipped.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/fakegh"
	"github.com/kukv/hedgehug/internal/gitrepo"
	"github.com/kukv/hedgehug/internal/model"
	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

const (
	markerFile    = ".hedgehug-devenv"
	defaultFakegh = "http://127.0.0.1:7777"
)

// bareRemotes are the bare repositories under the environment's directory:
// the "GitHub" that url_rewrites points https://github.com/ at.
var bareRemotes = []string{
	filepath.Join("remotes", "kukv", "hh-tasks.git"),
	filepath.Join("remotes", "org-a", "hh-shared.git"),
	filepath.Join("remotes", "org-b", "hh-ops.git"),
	webRemote,
}

// webRemote is the bare repository of the code repository org-a/web: not a
// store, it holds a README.
var webRemote = filepath.Join("remotes", "org-a", "web.git")

const (
	webURL    = "https://github.com/org-a/web.git"
	webRepoID = "github.com/org-a/web"
)

var users = []string{"alice", "bob"}

// account is an account of a user's config.yaml; its token is in the
// environment variable tokenVar.
type account struct{ name, login, tokenVar string }

// accounts are the accounts of each user: alice has her own and the one of
// her organizations.
var accounts = map[string][]account{
	"alice": {{"personal", "alice", "HH_DEV_TOKEN"}, {"work", "alice-corp", "HH_DEV_TOKEN_WORK"}},
	"bob":   {{"personal", "bob", "HH_DEV_TOKEN"}},
}

// tokens are what each user's HH_DEV_TOKEN* hold; devSeed has the same.
var tokens = map[string]map[string]string{
	"alice": {"HH_DEV_TOKEN": "dev-alice", "HH_DEV_TOKEN_WORK": "dev-alice-corp"}, //nolint:gosec // G101: fixed tokens of the fake GitHub, not credentials
	"bob":   {"HH_DEV_TOKEN": "dev-bob"},
}

// devStore is a store of a user: its remote as config.yaml has it (on
// "GitHub"; empty for a store with no remote), the id prefix of its first
// workspace, the account that writes it, and whether the user clones it (the
// one before made it) or makes it.
type devStore struct {
	name, remote, prefix, account string
	clone                         bool
}

// stores are the stores of each user, in the order they are made: alice's
// first, so that bob can clone what she pushed.
var stores = map[string][]devStore{
	"alice": {
		{name: "me", remote: "https://github.com/kukv/hh-tasks.git", prefix: "DEV", account: "personal"},
		{name: "org-a", remote: "https://github.com/org-a/hh-shared.git", prefix: "ORG", account: "work"},
		{name: "org-b", remote: "https://github.com/org-b/hh-ops.git", prefix: "OPS", account: "work"},
	},
	"bob": {
		{name: "me", prefix: "DEV", account: "personal"},
		{name: "org-a", remote: "https://github.com/org-a/hh-shared.git", prefix: "ORG", account: "personal", clone: true},
		{name: "org-b", remote: "https://github.com/org-b/hh-ops.git", prefix: "OPS", account: "personal", clone: true},
	},
}

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
		return usageError{errors.New("usage: devenv init|reset|env <user> [--dir <path>] [--fakegh <url>]")}
	}
	cmd, rest := args[0], args[1:]
	fs := flag.NewFlagSet("devenv "+cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("dir", "", "directory of the environment (default: .devenv at the repository root, found by walking up to go.mod)")
	fakeghURL := fs.String("fakegh", defaultFakegh, "the URL of the fake GitHub that the users' config.yaml point github.com at (init and reset)")

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
	if cmd != "env" {
		if u, err := url.Parse(*fakeghURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return usageError{fmt.Errorf("--fakegh %q: want an http or https URL", *fakeghURL)}
		}
	}

	d, err := resolveDir(*dir, cwd)
	if err != nil {
		return err
	}
	switch cmd {
	case "init":
		return initEnv(d, *fakeghURL, stdout)
	case "reset":
		return reset(d, *fakeghURL, stdout)
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

func initEnv(dir, fakeghURL string, stdout io.Writer) error {
	if _, err := os.Lstat(dir); err == nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
		return fmt.Errorf("%s already exists; run `devenv reset` to recreate it", dir)
	}
	if err := os.MkdirAll(dir, 0o750); err != nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, markerFile), nil, 0o600); err != nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
		return err
	}
	for _, r := range bareRemotes {
		if err := gitrepo.InitBare(filepath.Join(dir, r)); err != nil {
			return err
		}
	}
	if err := seedWeb(dir); err != nil {
		return fmt.Errorf("web: %w", err)
	}
	seed := filepath.Join(dir, "fakegh.json")
	if err := writeSeed(seed); err != nil {
		return err
	}
	for _, u := range users {
		if err := initUser(dir, u, fakeghURL); err != nil {
			return fmt.Errorf("%s: %w", u, err)
		}
	}
	fu, err := url.Parse(fakeghURL)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "Created %s with users %s.\nStart the fake GitHub with: go run ./cmd/fakegh serve --addr %s --seed %s\nUse one with: eval \"$(go run ./tools/devenv env %s --dir %s)\"\nThen add a task from the code repository with: cd %s && hedgehug add \"...\"\n",
		dir, strings.Join(users, " and "), fu.Host, shellQuote(seed), users[0], shellQuote(dir),
		shellQuote(filepath.Join(dir, "users", "alice", "src", "web")))
	return err
}

// devSeed is the state the fake GitHub starts with: the users and the two
// organizations, the repositories (the three stores have the hedgehug-store
// topic; org-a/web is code and has none), who may do what with them, and the
// tokens the HH_DEV_TOKEN variables hold. bob reads org-b/hh-ops only.
func devSeed() fakegh.State {
	interval := 5
	return fakegh.State{
		Users: []string{"alice", "alice-corp", "bob"},
		Orgs: map[string][]string{
			"org-a": {"alice-corp", "bob"},
			"org-b": {"alice-corp", "bob"},
		},
		Tokens: map[string]string{"dev-alice": "alice", "dev-alice-corp": "alice-corp", "dev-bob": "bob"},
		Repos: []*fakegh.Repo{
			{Owner: "kukv", Name: "hh-tasks", Private: true, Topics: []string{"hedgehug-store"}, Perms: map[string]string{"alice": "admin"}},
			{Owner: "org-a", Name: "hh-shared", Private: true, Topics: []string{"hedgehug-store"}, Perms: map[string]string{"alice-corp": "admin", "bob": "write"}},
			{Owner: "org-b", Name: "hh-ops", Private: true, Topics: []string{"hedgehug-store"}, Perms: map[string]string{"alice-corp": "admin", "bob": "read"}},
			{Owner: "org-a", Name: "web", Private: true, Perms: map[string]string{"alice-corp": "write", "bob": "write"}},
		},
		DeviceInterval: &interval,
	}
}

// writeSeed writes devSeed to path, for `fakegh serve --seed`.
func writeSeed(path string) error {
	b, err := json.MarshalIndent(devSeed(), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o600) //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
}

// configHead is the start of the config.yaml of user u, written before the
// stores are added: the accounts with their token variables, github.com on
// the fake, and url_rewrites to the bare repositories.
func configHead(dir, u, fakeghURL string) string {
	var b strings.Builder
	b.WriteString("accounts:\n")
	for _, a := range accounts[u] {
		fmt.Fprintf(&b, "  - name: %s\n    host: github.com\n    login: %s\n    token: env:%s\n", a.name, a.login, a.tokenVar)
	}
	fmt.Fprintf(&b, "hosts:\n  - host: github.com\n    api: %s\n    web: %s\n", fakeghURL, fakeghURL)
	fmt.Fprintf(&b, "url_rewrites:\n  - from: \"https://github.com/\"\n    to: %q\n", filepath.ToSlash(filepath.Join(dir, "remotes"))+"/")
	return b.String()
}

// initUser makes the home of the user u: .gitconfig (the author, and the
// rewrite of url_rewrites as insteadOf), config.yaml and the stores.
func initUser(dir, u, fakeghURL string) error {
	home := filepath.Join(dir, "users", u)
	if err := os.MkdirAll(home, 0o750); err != nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
		return err
	}
	gitconfig := "[user]\n\tname = " + u + "\n\temail = " + u + "@example.com\n" +
		"[url \"" + filepath.ToSlash(filepath.Join(dir, "remotes")) + "/\"]\n\tinsteadOf = https://github.com/\n"
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(gitconfig), 0o600); err != nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
		return err
	}
	getenv := func(k string) string {
		switch k {
		case "HEDGEHUG_HOME", "HOME":
			return home
		}
		return ""
	}
	p, err := config.Resolve(getenv)
	if err != nil {
		return err
	}
	// The accounts, hosts and url_rewrites are written before the config is
	// loaded, so that AddStore keeps them.
	if err := os.WriteFile(p.ConfigFile, []byte(configHead(dir, u, fakeghURL)), 0o600); err != nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
		return err
	}
	for _, ds := range stores[u] {
		if err := addDevStore(p, getenv, ds); err != nil {
			return fmt.Errorf("store %s: %w", ds.name, err)
		}
	}
	if u == "alice" {
		return addWebProject(home, p, getenv)
	}
	return nil
}

// seedWeb puts one commit, a README, in the bare repository of web. It works
// in a directory under dir that it removes, and pushes to the bare repository
// by its path, so that no rewrite is needed before config.yaml exists.
func seedWeb(dir string) error {
	work, err := os.MkdirTemp(dir, ".web-seed-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(work) }() //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
	r, err := gitrepo.Init(work)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(work, "README.md"), []byte("# web\n"), 0o600); err != nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
		return err
	}
	if _, err := r.Commit(gitrepo.Commit{
		Paths:   []string{"README.md"},
		Subject: "Add README",
		Author:  gitrepo.Signature{Name: "alice", Email: "alice@example.com"},
		When:    time.Now(),
	}); err != nil {
		return err
	}
	if err := r.SetRemotes([]string{filepath.Join(dir, webRemote)}); err != nil {
		return err
	}
	return r.Push("origin")
}

// addWebProject adds the project "Web release" to alice's org-a store, with
// web in its repos, pushes it so that bob's clone of org-a has it, and clones
// web into src/web. The clone keeps origin at the URL on GitHub.
func addWebProject(home string, p config.Paths, getenv func(string) string) error {
	var warnings []string
	ss := &ops.Session{Getenv: getenv, Now: time.Now, StoreName: "org-a", Warn: func(m string) { warnings = append(warnings, m) }}
	if _, err := ss.Add(ops.AddInput{Title: "Web release", Kind: "project", Repos: []string{webRepoID}}); err != nil {
		return fmt.Errorf("add project: %w", err)
	}
	if len(warnings) > 0 {
		return fmt.Errorf("add project: warning: %s", strings.Join(warnings, "; "))
	}
	c, err := config.Load(p)
	if err != nil {
		return err
	}
	e, err := c.Select("org-a")
	if err != nil {
		return err
	}
	acct, ok := c.Writer(e)
	if !ok {
		return errors.New("org-a has no account that writes it")
	}
	author, err := config.CommitAuthor(acct, true, getenv)
	if err != nil {
		return err
	}
	s, err := store.Open(e.Path)
	if err != nil {
		return err
	}
	conn := gitrepo.Conn{Rewrite: c.Rewrite}
	if _, err := sync.Run(s, sync.Options{Remotes: e.Remotes, Author: author, Account: acct.ID(), Me: c.Me(), Conn: conn}); err != nil {
		return err
	}
	_, err = gitrepo.CloneWith(webURL, filepath.Join(home, "src", "web"), conn)
	return err
}

// addDevStore makes (and pushes) or clones the store ds as the user of p, and
// adds it to their config.yaml with its account. It works in this process, so
// no background sync starts, and reaches the remote through url_rewrites.
func addDevStore(p config.Paths, getenv func(string) string, ds devStore) error {
	c, err := config.Load(p)
	if err != nil {
		return err
	}
	acct, ok := c.Account(ds.account)
	if !ok {
		return fmt.Errorf("no account named %s", ds.account)
	}
	author, err := config.CommitAuthor(acct, true, getenv)
	if err != nil {
		return err
	}
	var remotes []string
	if ds.remote != "" {
		remotes = []string{ds.remote}
	}
	conn := gitrepo.Conn{Rewrite: c.Rewrite}
	dir := filepath.Join(p.StoresDir, ds.name)
	var s *store.Store
	if ds.clone {
		if _, err = gitrepo.CloneWith(ds.remote, dir, conn); err != nil {
			return err
		}
		s, err = store.Open(dir)
	} else {
		ws := &model.Workspace{ULID: model.NewULID(), Name: ds.name, IDPrefix: ds.prefix}
		s, err = store.Create(dir, &model.StoreMeta{ULID: model.NewULID(), Name: ds.name}, nil, ws, author, acct.ID(), time.Now)
	}
	if err != nil {
		return err
	}
	if err := s.Repo().RegisterMergeDriver(); err != nil {
		return err
	}
	if !ds.clone && len(remotes) > 0 {
		if _, err := sync.Run(s, sync.Options{Remotes: remotes, Author: author, Account: acct.ID(), Me: c.Me(), Conn: conn}); err != nil {
			return err
		}
	}
	if err := c.AddStore(ds.name, dir, remotes); err != nil {
		return err
	}
	if err := c.SetStoreAccount(ds.name, ds.account); err != nil {
		return err
	}
	return c.Save()
}

// reset removes dir when it is a devenv (it holds the marker file) and
// creates it again. A missing dir is only created.
func reset(dir, fakeghURL string, stdout io.Writer) error {
	if _, err := os.Lstat(dir); err == nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
		if _, err := os.Stat(filepath.Join(dir, markerFile)); err != nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
			return fmt.Errorf("%s was not made by devenv (no %s in it); not removing it", dir, markerFile)
		}
		if err := os.RemoveAll(dir); err != nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
			return err
		}
	}
	return initEnv(dir, fakeghURL, stdout)
}

func printEnv(dir, user string, stdout io.Writer) error {
	home := filepath.Join(dir, "users", user)
	if !slices.Contains(users, user) {
		return fmt.Errorf("no user named %q (users: %s)", user, strings.Join(users, ", "))
	}
	if _, err := os.Stat(home); err != nil { //nolint:gosec // G703: the path comes from the developer own --dir or cwd in a local dev tool
		return fmt.Errorf("%s: %w; run `devenv init` first", home, err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "export HEDGEHUG_HOME=%s\nexport GIT_CONFIG_GLOBAL=%s\n", shellQuote(home), shellQuote(filepath.Join(home, ".gitconfig")))
	for _, k := range slices.Sorted(maps.Keys(tokens[user])) {
		fmt.Fprintf(&b, "export %s=%s\n", k, shellQuote(tokens[user][k]))
	}
	_, err := io.WriteString(stdout, b.String())
	return err
}

// shellQuote quotes s for sh with single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
