// Command openapigen writes the OpenAPI document of the server.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/kukv/hedgehug/internal/server"
)

func main() {
	out := flag.String("o", "", "the file to write")
	flag.Parse()
	b, err := server.OpenAPIYAML()
	if err == nil {
		err = os.WriteFile(*out, b, 0o644) //nolint:gosec // a file of the repository
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "openapigen:", err)
		os.Exit(1)
	}
}
