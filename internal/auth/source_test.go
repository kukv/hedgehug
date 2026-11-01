package auth

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kukv/hedgehug/internal/config"
)

// memKeyring is a Keyring in memory, safe for goroutines.
type memKeyring struct {
	mu sync.Mutex
	m  map[string]string
}

func newMem() *memKeyring { return &memKeyring{m: map[string]string{}} }

func (k *memKeyring) Get(service, user string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	v, ok := k.m[service+"|"+user]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func (k *memKeyring) Set(service, user, secret string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.m[service+"|"+user] = secret
	return nil
}

func (k *memKeyring) Delete(service, user string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, ok := k.m[service+"|"+user]; !ok {
		return ErrNotFound
	}
	delete(k.m, service+"|"+user)
	return nil
}

func acct(kind, env string) config.Account {
	return config.Account{Name: "a", Host: "github.com", Login: "alice", Token: config.TokenSource{Kind: kind, Env: env}}
}

func TestTokenGh(t *testing.T) {
	var got []string
	opt := Options{Run: func(name string, args ...string) ([]byte, []byte, error) {
		got = append([]string{name}, args...)
		return []byte(" gho_abc \n"), nil, nil
	}}
	tok, err := Token(acct("gh", ""), opt)
	if err != nil || tok != "gho_abc" {
		t.Fatalf("Token = %q, %v", tok, err)
	}
	if strings.Join(got, " ") != "gh auth token --hostname github.com --user alice" {
		t.Errorf("ran %q", got)
	}
	opt.Run = func(string, ...string) ([]byte, []byte, error) {
		return nil, []byte("no oauth token"), errors.New("exit status 1")
	}
	if _, err := Token(acct("gh", ""), opt); err == nil || !strings.Contains(err.Error(), "gh auth login --hostname github.com") || !strings.Contains(err.Error(), "no oauth token") {
		t.Errorf("err = %v", err)
	}
}

func TestTokenEnv(t *testing.T) {
	opt := Options{Getenv: func(k string) string {
		if k == "HH_TOKEN" {
			return "t1"
		}
		return ""
	}}
	if tok, err := Token(acct("env", "HH_TOKEN"), opt); err != nil || tok != "t1" {
		t.Errorf("Token = %q, %v", tok, err)
	}
	if _, err := Token(acct("env", "NOPE"), opt); err == nil || !strings.Contains(err.Error(), "NOPE is not set") {
		t.Errorf("err = %v", err)
	}
}

func TestTokenKeychain(t *testing.T) {
	k := newMem()
	opt := Options{Keyring: k}
	if _, err := Token(acct("keychain", ""), opt); err == nil || !strings.Contains(err.Error(), "hedgehug auth login a --with-token") {
		t.Errorf("err = %v", err)
	}
	if err := SaveToken(acct("keychain", ""), "secret", k); err != nil {
		t.Fatal(err)
	}
	if k.m["hedgehug|github.com/alice"] != "secret" {
		t.Errorf("keyring = %v", k.m)
	}
	if tok, err := Token(acct("keychain", ""), opt); err != nil || tok != "secret" {
		t.Errorf("Token = %q, %v", tok, err)
	}
	if err := Delete(acct("keychain", ""), k); err != nil || len(k.m) != 0 {
		t.Errorf("Delete = %v, %v", err, k.m)
	}
}

func TestTokenNone(t *testing.T) {
	if _, err := Token(acct("", ""), Options{}); !errors.Is(err, ErrNoToken) {
		t.Errorf("err = %v, want ErrNoToken", err)
	}
}

func TestDeviceTokenRoundTrip(t *testing.T) {
	k := newMem()
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	dt := DeviceToken{AccessToken: "ghu_1", RefreshToken: "ghr_1", ExpiresAt: now.Add(8 * time.Hour), RefreshExpiresAt: now.Add(4000 * time.Hour)}
	if err := SaveDeviceToken(acct("device", ""), dt, k); err != nil {
		t.Fatal(err)
	}
	got, err := LoadDeviceToken(acct("device", ""), k)
	if err != nil || got != dt {
		t.Errorf("Load = %+v, %v", got, err)
	}
}

func TestClientID(t *testing.T) {
	if got := ClientID(config.Host{Host: "ghe", ClientID: "Iv23x"}); got != "Iv23x" {
		t.Errorf("override = %q", got)
	}
	if got := ClientID(config.Host{Host: "ghe"}); got != "" {
		t.Errorf("ghe without one = %q", got)
	}
	old := builtinClientID
	builtinClientID = "Iv23builtin"
	defer func() { builtinClientID = old }()
	if got := ClientID(config.Host{Host: "github.com"}); got != "Iv23builtin" {
		t.Errorf("github.com = %q", got)
	}
	if got := ClientID(config.Host{Host: "GitHub.com"}); got != "Iv23builtin" {
		t.Errorf("GitHub.com (case) = %q", got)
	}
}
