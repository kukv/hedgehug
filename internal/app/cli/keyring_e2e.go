//go:build e2e

package cli

import (
	"path/filepath"

	"github.com/kukv/hedgehug/internal/app/remote/auth"
)

// mainKeyring is the keyring of the E2E binary (go build -tags e2e): a file
// in HEDGEHUG_HOME, so that the browser tests neither need an OS keychain (CI
// has none) nor write the Keychain of the developer.
func mainKeyring(getenv func(string) string) auth.Keyring {
	home := getenv("HEDGEHUG_HOME")
	if home == "" {
		panic("the e2e build of hedgehug needs HEDGEHUG_HOME for its file keyring")
	}
	return auth.FileKeyring(filepath.Join(home, "e2e-keyring.json"))
}
