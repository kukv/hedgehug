package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	gitconfig "github.com/go-git/go-git/v5/plumbing/format/config"

	"github.com/kukv/hedgehug/internal/gitrepo"
)

// Author returns the author of commits: user.name and user.email of the
// user's global git config. GIT_CONFIG_GLOBAL names the only file to read
// when set; otherwise the XDG file and ~/.gitconfig are read in that order,
// the later winning, as git does. Accounts replace this in phase 3.
func Author(getenv func(string) string) (gitrepo.Signature, error) {
	var files []string
	if g := getenv("GIT_CONFIG_GLOBAL"); g != "" {
		files = []string{g}
	} else {
		h, err := home(getenv)
		if err != nil {
			return gitrepo.Signature{}, err
		}
		x := abs(getenv("XDG_CONFIG_HOME"))
		if x == "" {
			x = filepath.Join(h, ".config")
		}
		files = []string{filepath.Join(x, "git", "config"), filepath.Join(h, ".gitconfig")}
	}
	var sig gitrepo.Signature
	for _, f := range files {
		b, err := os.Open(f) //nolint:gosec // the path is the user's own git config
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return gitrepo.Signature{}, err
		}
		cfg := gitconfig.New()
		err = gitconfig.NewDecoder(b).Decode(cfg)
		_ = b.Close()
		if err != nil {
			return gitrepo.Signature{}, fmt.Errorf("%s: %w", f, err)
		}
		u := cfg.Section("user")
		if v := u.Option("name"); v != "" {
			sig.Name = v
		}
		if v := u.Option("email"); v != "" {
			sig.Email = v
		}
	}
	switch {
	case sig.Name == "":
		return sig, errors.New("the author is not known: set it with git config --global user.name <name>")
	case sig.Email == "":
		return sig, errors.New("the author is not known: set it with git config --global user.email <email>")
	}
	return sig, nil
}
