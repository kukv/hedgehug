// Package auth gets the token of an account: from gh, an environment
// variable, the keychain or the device flow of the Hedgehug GitHub App.
package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/kukv/hedgehug/internal/config"
)

// Service is the keychain service the tokens are kept under.
const Service = "hedgehug"

// ErrNoToken means the account has no token: in config.yaml.
var ErrNoToken = errors.New("the account has no token")

// builtinClientID is the client ID of the Hedgehug GitHub App on github.com.
// It is set once the App is registered (see the appendix of the phase 3
// spec); a client ID is not a secret.
var builtinClientID = ""

// ClientID is the client ID of the device flow on the host h: the one of
// hosts, or the built-in one on github.com.
func ClientID(h config.Host) string {
	if h.ClientID != "" {
		return h.ClientID
	}
	if h.Host == "github.com" {
		return builtinClientID
	}
	return ""
}

// Options is what getting a token needs from outside.
type Options struct {
	Getenv   func(string) string
	Run      func(name string, args ...string) (stdout, stderr []byte, err error) // nil runs the command
	Keyring  Keyring                                                              // nil is the OS keychain
	Now      func() time.Time
	Host     config.Host  // of the account, for the device flow
	HTTP     *http.Client // nil is a client with a 30-second timeout
	LockFile string       // auth.lock; a refresh holds it
	Sleep    func(time.Duration)
}

// Keychain is the keyring to use: Keyring, or the OS keychain when it is nil.
// The commands get the keyring from here too, so the choice is made once.
func (o Options) Keychain() Keyring {
	if o.Keyring == nil {
		return OSKeyring()
	}
	return o.Keyring
}

func (o Options) run(name string, args ...string) ([]byte, []byte, error) {
	if o.Run != nil {
		return o.Run(name, args...)
	}
	var out, errb bytes.Buffer
	cmd := exec.Command(name, args...) //nolint:gosec // G204: name is gh, and args are the host and login of an account in config.yaml
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	return out.Bytes(), errb.Bytes(), err
}

// Key is the keychain key of the account: <host>/<login>.
func Key(a config.Account) string { return a.Host + "/" + a.Login }

// Token returns the token of the account a, from where its token: says.
func Token(a config.Account, opt Options) (string, error) {
	switch a.Token.Kind {
	case "":
		return "", ErrNoToken
	case "gh":
		out, errb, err := opt.run("gh", "auth", "token", "--hostname", a.Host, "--user", a.Login)
		if err != nil {
			return "", fmt.Errorf("gh auth token for %s: %w: %s (run gh auth login --hostname %s, or change token: of account %s)",
				Key(a), err, strings.TrimSpace(string(errb)), a.Host, a.Name)
		}
		return strings.TrimSpace(string(out)), nil
	case "env":
		v := ""
		if opt.Getenv != nil {
			v = opt.Getenv(a.Token.Env)
		}
		if v == "" {
			return "", fmt.Errorf("%s is not set (the token of account %s)", a.Token.Env, a.Name)
		}
		return v, nil
	case "keychain":
		v, err := opt.Keychain().Get(Service, Key(a))
		if errors.Is(err, ErrNotFound) {
			return "", fmt.Errorf("the keychain has no token of %s; run hedgehug auth login %s --with-token", Key(a), a.Name)
		}
		return v, err
	case "device":
		return deviceToken(a, opt)
	}
	return "", fmt.Errorf("account %s: unknown token source %q", a.Name, a.Token.Kind)
}

// SaveToken keeps token in the keychain as the token of a.
func SaveToken(a config.Account, token string, k Keyring) error {
	return k.Set(Service, Key(a), token)
}

// Delete removes the token of a from the keychain; none there is no error.
func Delete(a config.Account, k Keyring) error {
	if err := k.Delete(Service, Key(a)); err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	return nil
}

// DeviceToken is what the device flow leaves in the keychain.
type DeviceToken struct {
	AccessToken      string    `json:"access_token"`
	RefreshToken     string    `json:"refresh_token"`
	ExpiresAt        time.Time `json:"expires_at"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
}

// SaveDeviceToken keeps t in the keychain as JSON.
func SaveDeviceToken(a config.Account, t DeviceToken, k Keyring) error {
	b, err := json.Marshal(t) //nolint:gosec // G117: the JSON goes only to the keychain
	if err != nil {
		return err
	}
	return k.Set(Service, Key(a), string(b))
}

// LoadDeviceToken reads what SaveDeviceToken kept.
func LoadDeviceToken(a config.Account, k Keyring) (DeviceToken, error) {
	var t DeviceToken
	v, err := k.Get(Service, Key(a))
	if err != nil {
		return t, err
	}
	if err := json.Unmarshal([]byte(v), &t); err != nil {
		return t, fmt.Errorf("the keychain entry of %s: %w", Key(a), err)
	}
	return t, nil
}

// deviceToken returns the access token of a device login. Task 5 adds the
// refresh; until then the token is used as it is.
func deviceToken(a config.Account, opt Options) (string, error) {
	t, err := LoadDeviceToken(a, opt.Keychain())
	if errors.Is(err, ErrNotFound) {
		return "", fmt.Errorf("%s is not logged in; run hedgehug auth login %s", Key(a), a.Name)
	}
	if err != nil {
		return "", err
	}
	return t.AccessToken, nil
}
