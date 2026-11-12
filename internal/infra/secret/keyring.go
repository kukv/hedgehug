package secret

import (
	"errors"

	"github.com/zalando/go-keyring"
)

// ErrNotFound は、キーチェーンに項目がないこと。ほかの失敗は、キーチェーンそのものが使えないこと。
var ErrNotFound = errors.New("not in the keychain")

// osKeyring は OS のキーチェーン(macOS のキーチェーン、Linux の Secret Service)。
type osKeyring struct{}

func (osKeyring) Get(service, user string) (string, error) {
	v, err := keyring.Get(service, user)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	return v, err
}

func (osKeyring) Set(service, user, value string) error { return keyring.Set(service, user, value) }

func (osKeyring) Delete(service, user string) error {
	err := keyring.Delete(service, user)
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNotFound
	}
	return err
}
