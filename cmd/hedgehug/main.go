// Command hedgehug is a local-first task manager that uses git as its storage
// and sync layer, with a CLI and a GUI in a single binary.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

// version is what --version prints. GoReleaser sets it with -ldflags at
// release build time.
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("hedgehug", flag.ContinueOnError)
	fs.SetOutput(stderr)
	showVersion := fs.Bool("version", false, "print the version and exit")
	if err := fs.Parse(args); err != nil {
		// Same exit codes as flag.ExitOnError: -h is a request, not a mistake.
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *showVersion {
		_, _ = fmt.Fprintln(stdout, "hedgehug "+version)
		return 0
	}
	fs.Usage()
	return 2
}
