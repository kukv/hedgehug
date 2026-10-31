// Command hedgehug is a local-first task manager that uses git as its storage
// and sync layer, with a CLI and a GUI in a single binary.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/kukv/hedgehug/internal/config"
	"github.com/kukv/hedgehug/internal/ops"
	"github.com/kukv/hedgehug/internal/store"
	"github.com/kukv/hedgehug/internal/sync"
)

// version is what --version prints. GoReleaser sets it with -ldflags at
// release build time.
var version = "dev"

// env is what one run of the command sees of the world outside.
type env struct {
	stdin          io.Reader
	stdout, stderr io.Writer
	getenv         func(string) string
	isTerminal     func(any) bool // reports whether a stream is a terminal
	now            func() time.Time
}

func main() {
	os.Exit(run(os.Args[1:], &env{
		stdin:      os.Stdin,
		stdout:     os.Stdout,
		stderr:     os.Stderr,
		getenv:     os.Getenv,
		isTerminal: isTerminal,
		now:        time.Now,
	}))
}

func isTerminal(f any) bool {
	file, ok := f.(*os.File)
	return ok && term.IsTerminal(int(file.Fd())) //nolint:gosec // a file descriptor fits in an int
}

// run runs the command and returns its exit code: 0 on success, 2 for a
// mistake in flags or arguments, 1 for any other error.
func run(args []string, e *env) int {
	a := &app{env: e}
	a.ss = &ops.Session{
		Getenv: e.getenv,
		Now:    e.now,
		Warn:   func(m string) { _, _ = fmt.Fprintln(e.stderr, "warning: "+m) },
		AfterWrite: func(s *store.Store, en config.StoreEntry) {
			if !syncsInBackground(en) {
				return
			}
			if err := sync.Start(s, en.Name, e.getenv); err != nil {
				_, _ = fmt.Fprintln(e.stderr, "warning: the background sync did not start:", err)
			}
		},
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
	if errors.As(err, &ue) || ops.KindOf(err) == ops.KindUsage || strings.HasPrefix(err.Error(), "unknown command") {
		return 2
	}
	return 1
}

// writeErr prints err as hedgehug: <reason>, with the details of the errors
// that carry a list on the lines after it.
func writeErr(w io.Writer, err error) {
	_, _ = fmt.Fprintln(w, "hedgehug: "+describe(err))
}
