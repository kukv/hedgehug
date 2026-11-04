package usecase

import "github.com/kukv/hedgehug/internal/app/remote/auth"

// Keyring is where the secrets of the accounts are kept: the OS keychain, or
// a stand-in in a test. It has the methods of auth.Keyring.
type Keyring interface {
	Get(service, user string) (string, error)
	Set(service, user, secret string) error
	Delete(service, user string) error
}

// FileKeyring is a keyring kept in a JSON file at path, for the browser tests
// (the e2e build), which have no OS keychain to write.
func FileKeyring(path string) Keyring { return auth.FileKeyring(path) }
