package usecase

import "github.com/kukv/hedgehug/internal/app/remote/auth"

// Keyring is where the tokens are kept: the OS keychain, or a stand-in. The
// entrances name it through usecase, so that they do not import remote.
type Keyring = auth.Keyring

// FileKeyring is a keyring kept in a JSON file at path, for the browser tests
// (the e2e build), which have no OS keychain to write.
func FileKeyring(path string) Keyring { return auth.FileKeyring(path) }
