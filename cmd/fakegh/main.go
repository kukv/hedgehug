// Command fakegh serves a fake of the GitHub API that Hedgehug uses, and
// changes its state while it runs. It is for the development environment
// and tests, and is not shipped.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/fakegh"
)

const usage = `usage:
  fakegh serve [--addr 127.0.0.1:7777] [--seed <json>]
  fakegh perm set <owner/repo> <login> <read|write|admin|none>
  fakegh repo add <owner/repo> [--topic t]... [--public]
  fakegh device approve <user_code> --user <login>
  fakegh token add <token> --user <login>
  fakegh token expire <login>
The commands but serve take --server (default http://127.0.0.1:7777).`

type usageError struct{ error }

// topics is a flag that may be given more than once.
type topics []string

func (t *topics) String() string     { return strings.Join(*t, ",") }
func (t *topics) Set(v string) error { *t = append(*t, v); return nil }

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	err := dispatch(args, stdout, stderr)
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

func dispatch(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return usageError{errors.New("a command is needed")}
	}
	fs := flag.NewFlagSet("fakegh", flag.ContinueOnError)
	fs.SetOutput(stderr)
	server := fs.String("server", "http://127.0.0.1:7777", "the URL of the running fake")
	user := fs.String("user", "", "the login")
	var ts topics
	cmd := args[0]
	if len(args) > 1 && cmd != "serve" {
		cmd += " " + args[1]
	}
	admin := func() fakegh.Admin { return fakegh.Admin{URL: strings.TrimSuffix(*server, "/")} }
	switch cmd {
	case "serve":
		addr := fs.String("addr", "127.0.0.1:7777", "where to listen")
		seed := fs.String("seed", "", "a JSON file of the state to start with")
		if _, err := parse(fs, args[1:], 0); err != nil {
			return err
		}
		var st fakegh.State
		if *seed != "" {
			var err error
			if st, err = fakegh.Load(*seed); err != nil {
				return err
			}
		}
		_, _ = fmt.Fprintf(stdout, "fakegh listening on http://%s\n", *addr)
		srv := &http.Server{Addr: *addr, Handler: fakegh.New(st), ReadHeaderTimeout: 10 * time.Second}
		return srv.ListenAndServe()
	case "perm set":
		pos, err := parse(fs, args[2:], 3)
		if err != nil {
			return err
		}
		return admin().SetPerm(pos[0], pos[1], pos[2])
	case "repo add":
		fs.Var(&ts, "topic", "a topic; give it again for more")
		public := fs.Bool("public", false, "make it public")
		pos, err := parse(fs, args[2:], 1)
		if err != nil {
			return err
		}
		return admin().AddRepo(pos[0], ts, *public)
	case "device approve":
		pos, err := parse(fs, args[2:], 1)
		if err != nil {
			return err
		}
		if *user == "" {
			return usageError{errors.New("--user is required")}
		}
		return admin().ApproveDevice(pos[0], *user)
	case "token add":
		pos, err := parse(fs, args[2:], 1)
		if err != nil {
			return err
		}
		if *user == "" {
			return usageError{errors.New("--user is required")}
		}
		return admin().AddToken(pos[0], *user)
	case "token expire":
		pos, err := parse(fs, args[2:], 1)
		if err != nil {
			return err
		}
		return admin().ExpireTokens(pos[0])
	}
	return usageError{fmt.Errorf("unknown command %q", cmd)}
}
