//go:build !e2e

package cli

import "github.com/kukv/hedgehug/internal/app/remote/auth"

// mainKeyring is the keyring of the binary: nil, which is the OS keychain.
func mainKeyring(func(string) string) (auth.Keyring, error) { return nil, nil }
