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

// 直列: パッケージ変数 goos の差し替え
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

// 直列: パッケージ変数 goos の差し替えと t.Chdir
func TestAuthorInclude(t *testing.T) {
	old := goos
	goos = "linux"
	defer func() { goos = old }()
	author := func(t *testing.T, h, g string) (gitrepo.Signature, error) {
		t.Helper()
		return Author(envOf(map[string]string{"HOME": h, "GIT_CONFIG_GLOBAL": g}))
	}

	t.Run("relative to the including file", func(t *testing.T) {
		d := t.TempDir()
		g := filepath.Join(d, "gc")
		writeFile(t, g, "[include]\n\tpath = inc.conf\n")
		writeFile(t, filepath.Join(d, "inc.conf"), "[user]\n\tname = Inc\n\temail = inc@example.com\n")
		got, err := author(t, d, g)
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "Inc" || got.Email != "inc@example.com" {
			t.Errorf("Author = %v", got)
		}
	})
	t.Run("~/ is expanded", func(t *testing.T) {
		h, d := t.TempDir(), t.TempDir()
		g := filepath.Join(d, "gc")
		writeFile(t, g, "[include]\n\tpath = ~/x.conf\n")
		writeFile(t, filepath.Join(h, "x.conf"), "[user]\n\tname = X\n\temail = x@example.com\n")
		got, err := author(t, h, g)
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "X" {
			t.Errorf("Author = %v", got)
		}
	})
	t.Run("~/ is skipped without a home directory", func(t *testing.T) {
		d := t.TempDir()
		g := filepath.Join(d, "gc")
		writeFile(t, g, "[user]\n\tname = Base\n\temail = b@example.com\n[include]\n\tpath = ~/x.conf\n")
		t.Chdir(d)
		writeFile(t, filepath.Join(d, "x.conf"), "[user]\n\tname = Cwd\n")
		got, err := Author(envOf(map[string]string{"GIT_CONFIG_GLOBAL": g}))
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "Base" {
			t.Errorf("Name = %q, want Base", got.Name)
		}
	})
	t.Run("the later setting wins", func(t *testing.T) {
		d := t.TempDir()
		g := filepath.Join(d, "gc")
		writeFile(t, g, "[user]\n\tname = Before\n\temail = e@example.com\n[include]\n\tpath = inc.conf\n[user]\n\tname = After\n")
		writeFile(t, filepath.Join(d, "inc.conf"), "[user]\n\tname = Inc\n")
		got, err := author(t, d, g)
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "After" {
			t.Errorf("Name = %q, want After", got.Name)
		}
		writeFile(t, g, "[user]\n\tname = Before\n\temail = e@example.com\n[include]\n\tpath = inc.conf\n")
		got, err = author(t, d, g)
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "Inc" {
			t.Errorf("Name = %q, want Inc", got.Name)
		}
	})
	t.Run("a loop stops without an error", func(t *testing.T) {
		d := t.TempDir()
		g := filepath.Join(d, "gc")
		writeFile(t, g, "[user]\n\tname = Loop\n\temail = l@example.com\n[include]\n\tpath = gc\n")
		got, err := author(t, d, g)
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "Loop" {
			t.Errorf("Author = %v", got)
		}
	})
	t.Run("includeIf is not read", func(t *testing.T) {
		d := t.TempDir()
		g := filepath.Join(d, "gc")
		writeFile(t, g, "[user]\n\tname = Base\n\temail = b@example.com\n[includeIf \"gitdir:"+filepath.ToSlash(d)+"/\"]\n\tpath = inc.conf\n")
		writeFile(t, filepath.Join(d, "inc.conf"), "[user]\n\tname = Cond\n")
		got, err := author(t, d, g)
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "Base" {
			t.Errorf("Name = %q, want Base", got.Name)
		}
	})
}

func TestCommitAuthorFromAccount(t *testing.T) {
	t.Parallel()
	sig, err := CommitAuthor(Account{Host: "github.com", Login: "alice"}, true, func(string) string { return "" })
	if err != nil || sig != (gitrepo.Signature{Name: "alice", Email: "alice@users.noreply.github.com"}) {
		t.Errorf("CommitAuthor = %+v, %v", sig, err)
	}
}

func TestCommitAuthorWithoutAccountReadsGitConfig(t *testing.T) {
	t.Parallel()
	f := filepath.Join(t.TempDir(), "gitconfig")
	writeFile(t, f, "[user]\n\tname = Bob\n\temail = bob@example.com\n")
	env := func(k string) string {
		if k == "GIT_CONFIG_GLOBAL" {
			return f
		}
		return ""
	}
	sig, err := CommitAuthor(Account{}, false, env)
	if err != nil || sig != (gitrepo.Signature{Name: "Bob", Email: "bob@example.com"}) {
		t.Errorf("CommitAuthor = %+v, %v", sig, err)
	}
}
