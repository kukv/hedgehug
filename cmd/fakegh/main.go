// Command fakegh serves a fake of the GitHub API that Hedgehug uses, and
// changes its state while it runs. It is for the development environment
// and tests, and is not shipped.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/lib/fakegh"
)

const (
	usage = `usage:
  fakegh serve [--addr 127.0.0.1:7777] [--seed <json>]
  fakegh perm set <owner/repo> <login> <read|write|admin|none>
  fakegh repo add <owner/repo> [--topic t]... [--public]
  fakegh device approve <user_code> --user <login>
  fakegh token add <token> --user <login>
  fakegh token expire <login>
  fakegh pr open <owner/repo> --branch <b> [--title t] [--body b] [--draft]
  fakegh pr review <owner/repo#n> (--request | --state approved|changes_requested)
  fakegh pr checks <owner/repo#n> <success|failure|pending>
  fakegh pr comment <owner/repo#n> --body <b>
  fakegh pr merge <owner/repo#n>
  fakegh issue open <owner/repo> --title <t> [--body b]
  fakegh issue edit <owner/repo#n> [--title t] [--body b] [--assignee a]... [--label l]... [--milestone m]
  fakegh repo label <owner/repo> <name>
  fakegh repo milestone <owner/repo> <title>
  fakegh issue close <owner/repo#n> [--reason completed|not_planned]
The commands but serve take --server (default http://127.0.0.1:7777).`
	defaultServer = "http://127.0.0.1:7777"
)

type usageError struct{ error }

// topics is a flag that may be given more than once.
type topics []string

func (t *topics) String() string     { return strings.Join(*t, ",") }
func (t *topics) Set(v string) error { *t = append(*t, v); return nil }

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	err := dispatch(args, stdout)
	var ue usageError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &ue):
		_, _ = fmt.Fprintf(stderr, "fakegh: %v\n%s\n", err, usage)
		return 2
	default:
		_, _ = fmt.Fprintln(stderr, "fakegh:", err)
		return 1
	}
}

// flagSet makes the flag set of one command. Its own errors are not shown:
// run prints the one error it gets, with the usage.
func flagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

// parse parses the flags of fs wherever they are among args and returns the
// positional arguments; it wants exactly n of them.
func parse(fs *flag.FlagSet, args []string, n int) ([]string, error) {
	var pos []string
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return nil, usageError{err}
		}
		args = fs.Args()
		if len(args) > 0 {
			pos, args = append(pos, args[0]), args[1:]
		}
	}
	if len(pos) != n {
		return nil, usageError{fmt.Errorf("want %d arguments, got %d", n, len(pos))}
	}
	return pos, nil
}

// admin is the fake at url, which the commands change.
func admin(url string) fakegh.Admin {
	return fakegh.Admin{URL: strings.TrimSuffix(url, "/")}
}

func dispatch(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return usageError{errors.New("a command is needed")}
	}
	if args[0] == "serve" {
		return serveCmd(args[1:], stdout)
	}
	if len(args) < 2 {
		return usageError{fmt.Errorf("unknown command %q", args[0])}
	}
	cmd, rest := args[0]+" "+args[1], args[2:]
	switch cmd {
	case "perm set":
		fs := flagSet("fakegh perm set")
		server := fs.String("server", defaultServer, "the URL of the running fake")
		pos, err := parse(fs, rest, 3)
		if err != nil {
			return err
		}
		return admin(*server).SetPerm(pos[0], pos[1], pos[2])
	case "repo add":
		fs := flagSet("fakegh repo add")
		server := fs.String("server", defaultServer, "the URL of the running fake")
		var ts topics
		fs.Var(&ts, "topic", "a topic; give it again for more")
		public := fs.Bool("public", false, "make it public")
		pos, err := parse(fs, rest, 1)
		if err != nil {
			return err
		}
		return admin(*server).AddRepo(pos[0], ts, *public)
	case "repo label", "repo milestone":
		fs := flagSet("fakegh " + cmd)
		server := fs.String("server", defaultServer, "the URL of the running fake")
		pos, err := parse(fs, rest, 2)
		if err != nil {
			return err
		}
		if cmd == "repo label" {
			return admin(*server).AddLabel(pos[0], pos[1])
		}
		return admin(*server).AddMilestone(pos[0], pos[1])
	case "device approve":
		fs := flagSet("fakegh device approve")
		server := fs.String("server", defaultServer, "the URL of the running fake")
		user := fs.String("user", "", "the login")
		pos, err := parse(fs, rest, 1)
		if err != nil {
			return err
		}
		if *user == "" {
			return usageError{errors.New("--user is required")}
		}
		return admin(*server).ApproveDevice(pos[0], *user)
	case "token add":
		fs := flagSet("fakegh token add")
		server := fs.String("server", defaultServer, "the URL of the running fake")
		user := fs.String("user", "", "the login")
		pos, err := parse(fs, rest, 1)
		if err != nil {
			return err
		}
		if *user == "" {
			return usageError{errors.New("--user is required")}
		}
		return admin(*server).AddToken(pos[0], *user)
	case "token expire":
		fs := flagSet("fakegh token expire")
		server := fs.String("server", defaultServer, "the URL of the running fake")
		pos, err := parse(fs, rest, 1)
		if err != nil {
			return err
		}
		return admin(*server).ExpireTokens(pos[0])
	case "pr open":
		fs := flagSet("fakegh pr open")
		server := fs.String("server", defaultServer, "the URL of the running fake")
		branch := fs.String("branch", "", "the branch")
		title := fs.String("title", "", "the title")
		body := fs.String("body", "", "the body")
		draft := fs.Bool("draft", false, "open it as a draft")
		pos, err := parse(fs, rest, 1)
		if err != nil {
			return err
		}
		if *branch == "" {
			return usageError{errors.New("--branch is required")}
		}
		return admin(*server).Pull("open", fakegh.AdminBody{Repo: pos[0], Head: *branch, Title: *title, Body: *body, Draft: *draft})
	case "pr review":
		fs := flagSet("fakegh pr review")
		server := fs.String("server", defaultServer, "the URL of the running fake")
		request := fs.Bool("request", false, "ask for a review")
		state := fs.String("state", "", "approved or changes_requested")
		pos, err := parse(fs, rest, 1)
		if err != nil {
			return err
		}
		repo, n, err := repoNumber(pos[0])
		if err != nil {
			return err
		}
		return admin(*server).Pull("review", fakegh.AdminBody{Repo: repo, Number: n, Request: *request, State: *state})
	case "pr checks":
		fs := flagSet("fakegh pr checks")
		server := fs.String("server", defaultServer, "the URL of the running fake")
		pos, err := parse(fs, rest, 2)
		if err != nil {
			return err
		}
		repo, n, err := repoNumber(pos[0])
		if err != nil {
			return err
		}
		return admin(*server).Pull("checks", fakegh.AdminBody{Repo: repo, Number: n, State: pos[1]})
	case "pr comment":
		fs := flagSet("fakegh pr comment")
		server := fs.String("server", defaultServer, "the URL of the running fake")
		body := fs.String("body", "", "the comment")
		pos, err := parse(fs, rest, 1)
		if err != nil {
			return err
		}
		repo, n, err := repoNumber(pos[0])
		if err != nil {
			return err
		}
		return admin(*server).Pull("comment", fakegh.AdminBody{Repo: repo, Number: n, Body: *body})
	case "pr merge":
		fs := flagSet("fakegh pr merge")
		server := fs.String("server", defaultServer, "the URL of the running fake")
		pos, err := parse(fs, rest, 1)
		if err != nil {
			return err
		}
		repo, n, err := repoNumber(pos[0])
		if err != nil {
			return err
		}
		return admin(*server).Pull("merge", fakegh.AdminBody{Repo: repo, Number: n})
	case "issue open":
		fs := flagSet("fakegh issue open")
		server := fs.String("server", defaultServer, "the URL of the running fake")
		title := fs.String("title", "", "the title")
		body := fs.String("body", "", "the body")
		pos, err := parse(fs, rest, 1)
		if err != nil {
			return err
		}
		return admin(*server).Issue("open", fakegh.AdminBody{Repo: pos[0], Title: *title, Body: *body})
	case "issue edit":
		fs := flagSet("fakegh issue edit")
		server := fs.String("server", defaultServer, "the URL of the running fake")
		title := fs.String("title", "", "the title")
		body := fs.String("body", "", "the body")
		var assignees, labels topics
		fs.Var(&assignees, "assignee", "an assignee; give it again for more")
		fs.Var(&labels, "label", "a label; give it again for more")
		milestone := fs.String("milestone", "", "the milestone; - takes it off")
		pos, err := parse(fs, rest, 1)
		if err != nil {
			return err
		}
		repo, n, err := repoNumber(pos[0])
		if err != nil {
			return err
		}
		return admin(*server).Issue("edit", fakegh.AdminBody{Repo: repo, Number: n, Title: *title, Body: *body, Assignees: assignees, Labels: labels, Milestone: *milestone})
	case "issue close":
		fs := flagSet("fakegh issue close")
		server := fs.String("server", defaultServer, "the URL of the running fake")
		reason := fs.String("reason", "completed", "completed or not_planned")
		pos, err := parse(fs, rest, 1)
		if err != nil {
			return err
		}
		repo, n, err := repoNumber(pos[0])
		if err != nil {
			return err
		}
		return admin(*server).Issue("close", fakegh.AdminBody{Repo: repo, Number: n, Reason: *reason})
	}
	return usageError{fmt.Errorf("unknown command %q", cmd)}
}

func serveCmd(args []string, stdout io.Writer) error {
	fs := flagSet("fakegh serve")
	addr := fs.String("addr", "127.0.0.1:7777", "where to listen")
	seed := fs.String("seed", "", "a JSON file of the state to start with")
	if _, err := parse(fs, args, 0); err != nil {
		return err
	}
	var st fakegh.State
	if *seed != "" {
		var err error
		if st, err = fakegh.Load(*seed); err != nil {
			return err
		}
	}
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "fakegh listening on http://%s\n", ln.Addr())
	return serveOn(ln, fakegh.New(st))
}

// serveOn serves h on ln until ln is closed or fails.
func serveOn(ln net.Listener, h http.Handler) error {
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second}
	return srv.Serve(ln)
}

// repoNumber splits owner/repo#n.
func repoNumber(s string) (string, int, error) {
	repo, num, ok := strings.Cut(s, "#")
	n, err := strconv.Atoi(num)
	if !ok || err != nil {
		return "", 0, usageError{fmt.Errorf("want owner/repo#n, got %q", s)}
	}
	return repo, n, nil
}
