package auth

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zalando/go-keyring"

	"github.com/kukv/hedgehug/internal/store"
)

func TestDeleteNotFoundIsNoError(t *testing.T) {
	t.Parallel()
	k := newMem()
	if err := Delete(acct("keychain", ""), k); err != nil {
		t.Errorf("Delete of nothing = %v", err)
	}
	if err := Delete(acct("keychain", ""), failKeyring{errors.New("locked")}); err == nil || !strings.Contains(err.Error(), "locked") {
		t.Errorf("Delete with a failing keychain = %v", err)
	}
}

// failKeyring fails every call with err.
type failKeyring struct{ err error }

func (f failKeyring) Get(string, string) (string, error) { return "", f.err }
func (f failKeyring) Set(string, string, string) error   { return f.err }
func (f failKeyring) Delete(string, string) error        { return f.err }

func TestBrokenDeviceTokenJSON(t *testing.T) {
	t.Parallel()
	k := newMem()
	a := acct("device", "")
	_ = k.Set(Service, Key(a), "{not json")
	if _, err := LoadDeviceToken(a, k); err == nil || !strings.Contains(err.Error(), "github.com/alice") {
		t.Errorf("LoadDeviceToken = %v", err)
	}
	if _, err := Token(a, Options{Keyring: k}); err == nil || !strings.Contains(err.Error(), "github.com/alice") {
		t.Errorf("Token = %v", err)
	}
}

func TestTokenUnknownKind(t *testing.T) {
	t.Parallel()
	if _, err := Token(acct("vault", ""), Options{}); err == nil || !strings.Contains(err.Error(), `unknown token source "vault"`) {
		t.Errorf("Token = %v", err)
	}
}

func TestDeviceNotLoggedInSaysWhatToRun(t *testing.T) {
	t.Parallel()
	_, err := Token(acct("device", ""), Options{Keyring: newMem()})
	if err == nil || !strings.Contains(err.Error(), "not logged in") || !strings.Contains(err.Error(), "hedgehug auth login a") {
		t.Errorf("Token = %v", err)
	}
}

func TestWrapKeyring(t *testing.T) {
	t.Parallel()
	if wrapKeyring(nil) != nil {
		t.Error("nil is not nil")
	}
	if !errors.Is(wrapKeyring(keyring.ErrNotFound), ErrNotFound) {
		t.Error("not found is not ErrNotFound")
	}
	err := wrapKeyring(errors.New("no secret service"))
	if err == nil || !strings.Contains(err.Error(), "no secret service") || !strings.Contains(err.Error(), "token: gh or token: env:<VAR>") {
		t.Errorf("wrapKeyring = %v", err)
	}
	if errors.Is(err, ErrNotFound) {
		t.Error("another error is ErrNotFound")
	}
}

func TestOptionsKeychain(t *testing.T) {
	t.Parallel()
	if _, ok := (Options{}).Keychain().(osKeyring); !ok {
		t.Errorf("nil Keyring gave %T, want the OS keychain", Options{}.Keychain())
	}
	k := newMem()
	if (Options{Keyring: k}).Keychain() != Keyring(k) {
		t.Error("Keyring is not what Keychain returns")
	}
}

// 直列: パッケージ変数 lockWait の差し替え
func TestWithLock(t *testing.T) {
	if err := WithLock(Options{}, func() error { return nil }); err == nil || !strings.Contains(err.Error(), "LockFile") {
		t.Errorf("no lock file = %v", err)
	}
	lock := filepath.Join(t.TempDir(), "auth.lock")
	ran := false
	if err := WithLock(Options{LockFile: lock}, func() error { ran = true; return nil }); err != nil || !ran {
		t.Errorf("WithLock = %v, ran %v", err, ran)
	}
	want := errors.New("from fn")
	if err := WithLock(Options{LockFile: lock}, func() error { return want }); !errors.Is(err, want) {
		t.Errorf("WithLock = %v, want fn's error", err)
	}
	old := lockWait
	lockWait = 5 * time.Millisecond
	t.Cleanup(func() { lockWait = old })
	var busy error
	if err := store.WithFileLock(lock, time.Second, func() error {
		busy = WithLock(Options{LockFile: lock}, func() error { return nil })
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(busy, store.ErrLocked) || !strings.Contains(busy.Error(), "another hedgehug") {
		t.Errorf("busy = %v", busy)
	}
}
