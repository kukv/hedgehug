package auth

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// gh's standard error goes into the error, with the tokens it names hidden.
func TestTokenGhHidesTokensInStderr(t *testing.T) {
	secrets := []string{
		"gho_abc123", "ghp_DEF456", "ghs_789xyz", "ghu_Q1w2", "github_pat_11AAA_bbbCCC",
		"my_ghp_glued1", "tokenghu_glued2", "xgithub_pat_glued3",
	}
	stderr := "bad credentials " + strings.Join(secrets, " ") + " and token=" + secrets[0] + "\nkeep this words"
	opt := Options{Run: func(string, ...string) ([]byte, []byte, error) {
		return nil, []byte(stderr), errors.New("exit status 1")
	}}
	_, err := Token(acct("gh", ""), opt)
	if err == nil {
		t.Fatal("want an error")
	}
	for _, s := range secrets {
		if strings.Contains(err.Error(), s) {
			t.Errorf("the error shows %s: %v", s, err)
		}
	}
	if !strings.Contains(err.Error(), "***") || !strings.Contains(err.Error(), "bad credentials") || !strings.Contains(err.Error(), "keep this words") {
		t.Errorf("err = %v", err)
	}
}

// A token that is empty once trimmed is an error with what to do, whichever
// way it was got.
func TestTokenEmptyIsAnError(t *testing.T) {
	k := newMem()
	if err := k.Set(Service, "github.com/alice", " \n"); err != nil {
		t.Fatal(err)
	}
	kd := newMem()
	if err := SaveDeviceToken(acct("device", ""), DeviceToken{AccessToken: "  "}, kd); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		kind string
		env  string
		opt  Options
		want []string
	}{
		{
			"gh", "gh", "",
			Options{Run: func(string, ...string) ([]byte, []byte, error) { return []byte(" \n"), nil, nil }},
			[]string{"gh auth login --hostname github.com", "account a"},
		},
		{
			"env", "env", "HH",
			Options{Getenv: func(string) string { return " \t" }},
			[]string{"HH is not set"},
		},
		{
			"keychain", "keychain", "",
			Options{Keyring: k},
			[]string{"hedgehug auth login a --with-token"},
		},
		{
			"device", "device", "",
			Options{Keyring: kd},
			[]string{"hedgehug auth login a"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			tok, err := Token(acct(c.kind, c.env), c.opt)
			if err == nil {
				t.Fatalf("Token = %q, nil; want an error", tok)
			}
			for _, w := range c.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("err = %v; want it to say %q", err, w)
				}
			}
		})
	}
}

func TestTokenIsTrimmed(t *testing.T) {
	if tok, err := Token(acct("env", "HH"), Options{Getenv: func(string) string { return "  t1\n" }}); err != nil || tok != "t1" {
		t.Errorf("env = %q, %v", tok, err)
	}
	k := newMem()
	if err := k.Set(Service, "github.com/alice", " t2\n"); err != nil {
		t.Fatal(err)
	}
	if tok, err := Token(acct("keychain", ""), Options{Keyring: k}); err != nil || tok != "t2" {
		t.Errorf("keychain = %q, %v", tok, err)
	}
	kd := newMem()
	dt := DeviceToken{AccessToken: " t3 ", ExpiresAt: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)}
	if err := SaveDeviceToken(acct("device", ""), dt, kd); err != nil {
		t.Fatal(err)
	}
	now := func() time.Time { return time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC) }
	if tok, err := Token(acct("device", ""), Options{Keyring: kd, Now: now}); err != nil || tok != "t3" {
		t.Errorf("device = %q, %v", tok, err)
	}
}
