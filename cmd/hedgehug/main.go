// Command hedgehug is a local-first task manager that uses git as its storage
// and sync layer, with a CLI and a GUI in a single binary. Its body is
// internal/app/cli.
package main

import (
	"os"

	"github.com/kukv/hedgehug/internal/app/cli"
)

// version is what --version prints. GoReleaser sets it with -ldflags at
// release.
var version = "dev"

func main() {
	os.Exit(cli.Main(version))
}
