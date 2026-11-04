package auth

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFileKeyringSetGetDelete(t *testing.T) {
	t.Parallel()
	k := FileKeyring(filepath.Join(t.TempDir(), "sub", "keyring.json"))

	if _, err := k.Get("svc", "u"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get with no file: err = %v, want ErrNotFound", err)
	}
	if err := k.Delete("svc", "u"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete with no file: err = %v, want ErrNotFound", err)
	}
	if err := k.Set("svc", "u", "one"); err != nil {
		t.Fatal(err)
	}
	if err := k.Set("svc", "other", "x"); err != nil {
		t.Fatal(err)
	}
	if got, err := k.Get("svc", "u"); err != nil || got != "one" {
		t.Fatalf("Get = %q, %v; want one", got, err)
	}
	if err := k.Set("svc", "u", "two"); err != nil {
		t.Fatal(err)
	}
	if got, err := k.Get("svc", "u"); err != nil || got != "two" {
		t.Fatalf("Get after overwrite = %q, %v; want two", got, err)
	}
	if err := k.Delete("svc", "u"); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Get("svc", "u"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after Delete: err = %v, want ErrNotFound", err)
	}
	if err := k.Delete("svc", "u"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second Delete: err = %v, want ErrNotFound", err)
	}
	if got, err := k.Get("svc", "other"); err != nil || got != "x" {
		t.Fatalf("other key = %q, %v; want x", got, err)
	}
}

func TestFileKeyringSharedBetweenValues(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "keyring.json")
	if err := FileKeyring(path).Set("svc", "u", "tok"); err != nil {
		t.Fatal(err)
	}
	if got, err := FileKeyring(path).Get("svc", "u"); err != nil || got != "tok" {
		t.Fatalf("Get through another value = %q, %v; want tok", got, err)
	}
	k := FileKeyring(path)
	if err := FileKeyring(path).Delete("svc", "u"); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Get("svc", "u"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after another value's Delete: err = %v, want ErrNotFound", err)
	}
}

func TestFileKeyringFileMode(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "keyring.json")
	if err := FileKeyring(path).Set("svc", "u", "tok"); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 600", got)
	}
}

func TestFileKeyringBrokenFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "keyring.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := FileKeyring(path).Get("svc", "u"); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("Get on a broken file: err = %v, want a parse error", err)
	}
}
