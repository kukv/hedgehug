package secret

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/domain/account"
	"github.com/kukv/hedgehug/internal/domain/ghcall"
)

type fakeKeyring map[string]string

func (k fakeKeyring) Get(service, user string) (string, error) {
	if k == nil {
		return "", errors.New("no secret service on this machine")
	}
	v, ok := k[service+"|"+user]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func (k fakeKeyring) Set(service, user, value string) error {
	if k == nil {
		return errors.New("no secret service on this machine")
	}
	k[service+"|"+user] = value
	return nil
}

func (k fakeKeyring) Delete(service, user string) error {
	if k == nil {
		return errors.New("no secret service on this machine")
	}
	if _, ok := k[service+"|"+user]; !ok {
		return ErrNotFound
	}
	delete(k, service+"|"+user)
	return nil
}

func acc(m account.Method) account.Account {
	return account.Account{ID: "A1", Name: "work", Login: "taro", Method: m}
}

func TestGhToken(t *testing.T) {
	var gotArgs []string
	s := &Source{GhTimeout: time.Second, Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
		gotArgs = append([]string{name}, args...)
		return []byte(" gh-token \n"), nil
	}}
	tok, err := s.Token(context.Background(), acc(account.GhCLI))
	want := []string{"gh", "auth", "token", "--hostname", "github.com", "--user", "taro"}
	if err != nil || tok != "gh-token" || !slices.Equal(gotArgs, want) {
		t.Errorf("Token = %q, %v; args %v", tok, err, gotArgs)
	}
}

func TestGhFailures(t *testing.T) {
	missing := &Source{GhTimeout: time.Second, Run: func(context.Context, string, ...string) ([]byte, error) {
		return nil, errors.New("exit status 1: no oauth token for LEAKED-token")
	}}
	_, err := missing.Token(context.Background(), acc(account.GhCLI))
	if !ghcall.Is(err, ghcall.GhTokenMissing) || strings.Contains(err.Error(), "LEAKED-token") {
		t.Errorf("missing: %v", err)
	}
	empty := &Source{GhTimeout: time.Second, Run: func(context.Context, string, ...string) ([]byte, error) { return []byte("\n"), nil }}
	if _, err := empty.Token(context.Background(), acc(account.GhCLI)); !ghcall.Is(err, ghcall.GhTokenMissing) {
		t.Errorf("empty: %v", err)
	}
	hung := &Source{GhTimeout: 20 * time.Millisecond, Run: func(ctx context.Context, _ string, _ ...string) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	if _, err := hung.Token(context.Background(), acc(account.GhCLI)); !ghcall.Is(err, ghcall.GhTimeout) {
		t.Errorf("hung: %v", err)
	}
}

func TestEnvToken(t *testing.T) {
	env := map[string]string{"GH_TOKEN": " env-token "}
	s := &Source{Getenv: func(k string) string { return env[k] }}
	if tok, err := s.Token(context.Background(), acc(account.EnvVar)); err != nil || tok != "env-token" {
		t.Errorf("Token = %q, %v", tok, err)
	}
	env["GH_TOKEN"] = "  "
	if _, err := s.Token(context.Background(), acc(account.EnvVar)); !ghcall.Is(err, ghcall.EnvMissing) {
		t.Errorf("unset: %v", err)
	}
}

func TestKeychainToken(t *testing.T) {
	s := &Source{Keyring: fakeKeyring{"hedgehug|github.com/taro": "keychain-token"}}
	if tok, err := s.Token(context.Background(), acc(account.Keychain)); err != nil || tok != "keychain-token" {
		t.Errorf("Token = %q, %v", tok, err)
	}
	s.Keyring = fakeKeyring{"hedgehug|github.com/taro": " "}
	if _, err := s.Token(context.Background(), acc(account.Keychain)); !ghcall.Is(err, ghcall.KeychainMissing) {
		t.Errorf("blank: %v", err)
	}
	s.Keyring = fakeKeyring{}
	if _, err := s.Token(context.Background(), acc(account.Keychain)); !ghcall.Is(err, ghcall.KeychainMissing) {
		t.Errorf("absent: %v", err)
	}
	s.Keyring = fakeKeyring(nil)
	if _, err := s.Token(context.Background(), acc(account.Keychain)); !ghcall.Is(err, ghcall.KeychainUnavailable) {
		t.Errorf("no keychain: %v", err)
	}
	// デバイスフローが置いた JSON の項目は、access_token をトークンとして読む(D2 の計画の決定 D2-5)。
	s.Keyring = fakeKeyring{"hedgehug|github.com/taro": `{"access_token":"tok-device","refresh_token":"r"}`}
	if tok, err := s.Token(context.Background(), acc(account.Keychain)); err != nil || tok != "tok-device" {
		t.Errorf("device JSON: Token = %q, %v", tok, err)
	}
}

func TestDeviceFlowIsNotReadHere(t *testing.T) {
	if _, err := (&Source{}).Token(context.Background(), acc(account.DeviceFlow)); err == nil {
		t.Error("device flow token read in D1")
	}
}
