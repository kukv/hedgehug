//go:build e2e

package cli

import (
	"errors"
	"path/filepath"

	"github.com/kukv/hedgehug/internal/app/usecase"
)

// mainKeyring is the keyring of the E2E binary (go build -tags e2e): a file
// in HEDGEHUG_HOME, so that the browser tests neither need an OS keychain (CI
// has none) nor write the Keychain of the developer.
func mainKeyring(getenv func(string) string) (usecase.Keyring, error) {
	home := getenv("HEDGEHUG_HOME")
	if home == "" {
		return nil, errors.New("the e2e build needs HEDGEHUG_HOME for its file keyring")
	}
	return usecase.FileKeyring(filepath.Join(home, "e2e-keyring.json")), nil
}
