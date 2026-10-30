package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/gcfg/v2"

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
	h, _ := home(getenv) // only needed for ~/ in an include path
	var sig gitrepo.Signature
	for _, f := range files {
		if err := readGitConfig(f, h, 0, &sig); err != nil {
			return gitrepo.Signature{}, err
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

// maxIncludeDepth is how deep [include] files are followed; git stops at 10
// too, which also ends an include loop.
const maxIncludeDepth = 10

// readGitConfig reads user.name and user.email from the git config file f
// into sig, in the order they appear, following [include] path = ... where
// it stands. A missing file is skipped. [includeIf] is not read.
func readGitConfig(f, home string, depth int, sig *gitrepo.Signature) error {
	if f == "" || depth > maxIncludeDepth {
		return nil
	}
	r, err := os.Open(f) //nolint:gosec // the path is the user's own git config
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()
	err = gcfg.ReadWithCallback(r, func(section, sub, key, value string, blank bool) error {
		if sub != "" || blank {
			return nil
		}
		switch {
		case strings.EqualFold(section, "user") && strings.EqualFold(key, "name") && value != "":
			sig.Name = value
		case strings.EqualFold(section, "user") && strings.EqualFold(key, "email") && value != "":
			sig.Email = value
		case strings.EqualFold(section, "include") && strings.EqualFold(key, "path") && value != "":
			return readGitConfig(includePath(f, home, value), home, depth+1, sig)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("%s: %w", f, err)
	}
	return nil
}

// includePath resolves an include path: ~/ is the home directory, and a
// relative path is relative to the file that includes it.
func includePath(from, home, p string) string {
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		if home == "" {
			return "" // no home directory: the include is skipped
		}
		return filepath.Join(home, filepath.FromSlash(rest))
	}
	p = filepath.FromSlash(p)
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(filepath.Dir(from), p)
}
