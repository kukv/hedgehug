package auth

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

// ErrNotFound means the keychain has nothing under the key.
var ErrNotFound = errors.New("not in the keychain")

// Keyring is the OS credential store, or a stand-in in tests.
type Keyring interface {
	Get(service, user string) (string, error)
	Set(service, user, secret string) error
	Delete(service, user string) error
}

// OSKeyring is the credential store of the OS: the macOS Keychain, the
// Windows Credential Manager or the Secret Service.
func OSKeyring() Keyring { return osKeyring{} }

type osKeyring struct{}

func (osKeyring) Get(service, user string) (string, error) {
	v, err := keyring.Get(service, user)
	return v, wrapKeyring(err)
}

func (osKeyring) Set(service, user, secret string) error {
	return wrapKeyring(keyring.Set(service, user, secret))
}

func (osKeyring) Delete(service, user string) error {
	return wrapKeyring(keyring.Delete(service, user))
}

// wrapKeyring turns go-keyring's not found into ErrNotFound, and adds a hint
// to the other errors: a machine with no Secret Service (WSL, say) fails here.
func wrapKeyring(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, keyring.ErrNotFound):
		return ErrNotFound
	default:
		return fmt.Errorf("the keychain: %w (where there is no keychain, use token: gh or token: env:<VAR>)", err)
	}
}
