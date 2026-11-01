// Command hedgehug is a local-first task manager that uses git as its storage
// and sync layer, with a CLI and a GUI in a single binary.
package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/kukv/hedgehug/internal/auth"
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
	getwd          func() (string, error) // the current directory; nil means none, which skips the stages of hedgehug add that read it
	beforeUpdate   func()                 // for tests: runs before each write; nil in the binary
	keyring        auth.Keyring           // nil is the OS keychain
	httpClient     *http.Client           // nil is a client with a 30-second timeout
	// run runs a command (gh) and gives its output; nil runs the command.
	run func(name string, args ...string) (stdout, stderr []byte, err error)
	// openURL opens a URL in the browser; nil means no browser can be opened.
	openURL func(url string) error
}

func init() {
	// On Windows cobra lists every process on each run, about 40ms, to tell
	// whether Explorer started the command and if so to ask for cmd.exe.
	// hedgehug is run from a shell; started from Explorer it prints its
	// usage as it does with no arguments.
	cobra.MousetrapHelpText = ""
}

func main() {
	os.Exit(run(os.Args[1:], &env{
		stdin:      os.Stdin,
		stdout:     os.Stdout,
		stderr:     os.Stderr,
		getenv:     os.Getenv,
		isTerminal: isTerminal,
		now:        time.Now,
		getwd:      os.Getwd,
		openURL:    openURL,
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
		Getenv:       e.getenv,
		Now:          e.now,
		BeforeUpdate: e.beforeUpdate,
		Keyring:      e.keyring,
		HTTPClient:   e.httpClient,
		Run:          e.run,
		Warn:         func(m string) { _, _ = fmt.Fprintln(e.stderr, "warning: "+m) },
		AfterWrite: func(s *store.Store, en config.StoreEntry) {
			if !ops.SyncsInBackground(en) {
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
	if errors.As(err, &ue) || ops.KindOf(err) == ops.KindUsage || ops.KindOf(err) == ops.KindAmbiguous || strings.HasPrefix(err.Error(), "unknown command") {
		return 2
	}
	return 1
}

// writeErr prints err as hedgehug: <reason>, with the details of the errors
// that carry a list on the lines after it.
func writeErr(w io.Writer, err error) {
	_, _ = fmt.Fprintln(w, "hedgehug: "+describe(err))
}

// openURL opens u in the browser of the OS.
func openURL(u string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u) //nolint:gosec // G204: the command is fixed; u is the verification URL the host gave
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u) //nolint:gosec // G204: the command is fixed; u is the verification URL the host gave
	default:
		cmd = exec.Command("xdg-open", u) //nolint:gosec // G204: the command is fixed; u is the verification URL the host gave
	}
	return cmd.Start()
}
