package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kukv/hedgehug/internal/gitrepo"
)

func writeFile(t *testing.T, path, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(s), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestAuthor(t *testing.T) {
	old := goos
	goos = "linux"
	defer func() { goos = old }()

	t.Run("from ~/.gitconfig", func(t *testing.T) {
		h := t.TempDir()
		writeFile(t, filepath.Join(h, ".gitconfig"), "[user]\n\tname = Alice\n\temail = alice@example.com\n")
		got, err := Author(envOf(map[string]string{"HOME": h}))
		if err != nil {
			t.Fatal(err)
		}
		if want := (gitrepo.Signature{Name: "Alice", Email: "alice@example.com"}); got != want {
			t.Errorf("Author = %v, want %v", got, want)
		}
	})
	t.Run("~/.gitconfig wins over the XDG file", func(t *testing.T) {
		h, x := t.TempDir(), t.TempDir()
		writeFile(t, filepath.Join(x, "git", "config"), "[user]\n\tname = Xdg\n\temail = xdg@example.com\n")
		writeFile(t, filepath.Join(h, ".gitconfig"), "[user]\n\tname = Home\n")
		got, err := Author(envOf(map[string]string{"HOME": h, "XDG_CONFIG_HOME": x}))
		if err != nil {
			t.Fatal(err)
		}
		if want := (gitrepo.Signature{Name: "Home", Email: "xdg@example.com"}); got != want {
			t.Errorf("Author = %v, want %v", got, want)
		}
	})
	t.Run("~/.config/git/config when XDG is unset", func(t *testing.T) {
		h := t.TempDir()
		writeFile(t, filepath.Join(h, ".config", "git", "config"), "[user]\n\tname = C\n\temail = c@example.com\n")
		got, err := Author(envOf(map[string]string{"HOME": h}))
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "C" {
			t.Errorf("Author = %v", got)
		}
	})
	t.Run("GIT_CONFIG_GLOBAL replaces the others", func(t *testing.T) {
		h, d := t.TempDir(), t.TempDir()
		writeFile(t, filepath.Join(h, ".gitconfig"), "[user]\n\tname = Home\n\temail = home@example.com\n")
		g := filepath.Join(d, "gc")
		writeFile(t, g, "[user]\n\tname = G\n\temail = g@example.com\n")
		got, err := Author(envOf(map[string]string{"HOME": h, "GIT_CONFIG_GLOBAL": g}))
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "G" || got.Email != "g@example.com" {
			t.Errorf("Author = %v", got)
		}
	})
	t.Run("missing email", func(t *testing.T) {
		h := t.TempDir()
		writeFile(t, filepath.Join(h, ".gitconfig"), "[user]\n\tname = A\n")
		_, err := Author(envOf(map[string]string{"HOME": h}))
		if err == nil || !strings.Contains(err.Error(), "git config --global user.email") {
			t.Errorf("err = %v, want advice on user.email", err)
		}
	})
	t.Run("no file", func(t *testing.T) {
		_, err := Author(envOf(map[string]string{"HOME": t.TempDir()}))
		if err == nil || !strings.Contains(err.Error(), "git config --global user.name") {
			t.Errorf("err = %v, want advice on user.name", err)
		}
	})
}
