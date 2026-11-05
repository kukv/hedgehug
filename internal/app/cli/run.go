// Package cli is the body of the hh command: its subcommands, flags and
// output. cmd/hedgehug only calls Main.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pkg/browser"
	"golang.org/x/term"

	"github.com/kukv/hedgehug/internal/app/cli/prompt"
	"github.com/kukv/hedgehug/internal/app/usecase"
)

// version is what --version prints. Main sets it from the version that
// cmd/hedgehug gets with -ldflags at release; the tests keep "dev".
var version = "dev"

// env is what one run of the command sees of the world outside.
type env struct {
	stdin          io.Reader
	stdout, stderr io.Writer
	getenv         func(string) string
	isTerminal     func(any) bool // reports whether a stream is a terminal
	now            func() time.Time
	getwd          func() (string, error) // the current directory; nil means none: hedgehug add skips the stages that read it, and start and pr are usage errors
	beforeUpdate   func()                 // for tests: runs before each write; nil in the binary
	keyring        usecase.Keyring        // nil is the OS keychain
	httpClient     *http.Client           // nil is a client with a 30-second timeout
	// run runs a command (gh) and gives its output; nil runs the command.
	run func(name string, args ...string) (stdout, stderr []byte, err error)
	// openURL opens a URL in the browser; nil means no browser can be opened.
	openURL func(url string) error
	// uiContext is for tests: what hh ui runs until; nil is Ctrl-C and SIGTERM.
	uiContext func() (context.Context, context.CancelFunc)
}

// Main runs the command with the arguments of the process and returns its
// exit code. v is the version --version prints.
func Main(v string) int {
	version = v
	kr, err := mainKeyring(os.Getenv)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "hedgehug:", err)
		return 1
	}
	return run(os.Args[1:], &env{
		stdin:      os.Stdin,
		stdout:     os.Stdout,
		stderr:     os.Stderr,
		getenv:     os.Getenv,
		isTerminal: isTerminal,
		now:        time.Now,
		getwd:      os.Getwd,
		openURL:    openURL,
		keyring:    kr,
	})
}

func isTerminal(f any) bool {
	file, ok := f.(*os.File)
	return ok && term.IsTerminal(int(file.Fd())) //nolint:gosec // a file descriptor fits in an int
}

// run runs the command and returns its exit code: 0 on success, 2 for a
// mistake in flags or arguments, 1 for any other error.
func run(args []string, e *env) int {
	a := &app{env: e}
	a.ss = &usecase.Session{
		Getenv:         e.getenv,
		Now:            e.now,
		BeforeUpdate:   e.beforeUpdate,
		Keyring:        e.keyring,
		HTTPClient:     e.httpClient,
		Run:            e.run,
		Progress:       newProgress(e.stderr, time.Now),
		Warn:           func(m string) { _, _ = fmt.Fprintln(e.stderr, "warning: "+m) },
		BackgroundSync: true,
	}
	root := a.rootCmd()
	root.SetArgs(args)
	root.SetIn(e.stdin)
	root.SetOut(e.stdout)
	root.SetErr(e.stderr)
	err := root.Execute()
	if err == nil {
		return 0
	}
	if errors.Is(err, errFindings) {
		return 1 // the findings are printed already
	}
	writeErr(e.stderr, err)
	var ue usageError
	var pe prompt.UsageError
	if errors.As(err, &ue) || errors.As(err, &pe) || usecase.KindOf(err) == usecase.KindUsage || usecase.KindOf(err) == usecase.KindAmbiguous || strings.HasPrefix(err.Error(), "unknown command") {
		return 2
	}
	return 1
}

// writeErr prints err as hedgehug: <reason>, with the details of the errors
// that carry a list on the lines after it.
func writeErr(w io.Writer, err error) {
	_, _ = fmt.Fprintln(w, "hedgehug: "+usecase.Describe(err))
}

// openURL opens u in the browser of the OS and returns when the command that
// opens it has ended. Its own output is not shown.
func openURL(u string) error {
	browser.Stdout, browser.Stderr = io.Discard, io.Discard // not into the output of the CLI
	return browser.OpenURL(u)
}
