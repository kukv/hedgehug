package gitrepo

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	git "github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage/memory"
)

// ErrPushDenied means the remote refused the push for want of the right to
// write: a 403, or a local repository that cannot be written.
var ErrPushDenied = errors.New("the remote denied the push")

// Conn is how a fetch, push or clone reaches a remote. Rewrite turns the
// URL of .git/config into the one to reach (url_rewrites); Auth gives the
// auth for the rewritten URL. Both may be nil.
type Conn struct {
	Rewrite func(url string) string
	Auth    func(url string) (transport.AuthMethod, error)
}

// endpoint is the URL to reach for u and its auth.
func (c Conn) endpoint(u string) (string, transport.AuthMethod, error) {
	if c.Rewrite != nil {
		u = c.Rewrite(u)
	}
	if c.Auth == nil {
		return u, nil, nil
	}
	a, err := c.Auth(u)
	return u, a, err
}

// SetConn makes the fetches and pushes of r go through c.
func (r *Repo) SetConn(c Conn) { r.conn = c }

// remoteURL is the URL of the remote name as .git/config has it.
func (r *Repo) remoteURL(name string) (string, error) {
	cfg, err := r.r.Config()
	if err != nil {
		return "", err
	}
	rc, ok := cfg.Remotes[name]
	if !ok || len(rc.URLs) == 0 {
		return "", fmt.Errorf("no remote %s", name)
	}
	return rc.URLs[0], nil
}

// pushDenied reports whether err is a refusal to write: a 403 from an
// http remote, or, for a local remote only, a permission error (go-git's
// in-process server gives it as text only). An ssh "Permission denied
// (publickey)" is a failed login, not a refusal to write.
func pushDenied(err error, local bool) bool {
	if errors.Is(err, transport.ErrAuthorizationFailed) {
		return true
	}
	return local && (errors.Is(err, fs.ErrPermission) || strings.Contains(strings.ToLower(err.Error()), "permission denied"))
}

// LocalPath returns the local path of a remote that is one: a path, or a
// file:// URL.
func LocalPath(u string) (string, bool) {
	if p, ok := strings.CutPrefix(u, "file://"); ok {
		if p == "" {
			return "", false
		}
		return filepath.FromSlash(localPath(p)), true
	}
	// A rooted path (/srv/x.git) is local on Windows too, as go-git takes it:
	// filepath.IsAbs wants a drive there.
	if filepath.IsAbs(u) || strings.HasPrefix(u, "/") || driveRE.MatchString("/"+u) {
		return u, true
	}
	return "", false
}

// IsHTTP reports whether u is an http or https URL.
func IsHTTP(u string) bool {
	p, err := url.Parse(u)
	return err == nil && (p.Scheme == "http" || p.Scheme == "https") && p.Host != ""
}

// Writable reports whether a file can be made and removed in dir/objects,
// which a push to the bare repository dir needs.
func Writable(dir string) bool {
	f, err := os.CreateTemp(filepath.Join(dir, "objects"), ".hedgehug-write-test-*") //nolint:gosec // G703: dir is the bare repository of a remote in config.yaml
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	return os.Remove(name) == nil //nolint:gosec // G703: the file made just above
}

// RemoteEmpty reports whether the remote at u, the URL as .git/config has
// it, has no ref at all, listing its refs through c (url_rewrites and auth,
// as for a fetch). A remote the server calls empty, and one that lists
// nothing, are both empty; a repository that is not there, or a failed
// connection, is an error.
func RemoteEmpty(u string, c Conn) (bool, error) {
	to, auth, err := c.endpoint(u)
	if err != nil {
		return false, fmt.Errorf("list the refs of %s: %w", u, err)
	}
	rm := git.NewRemote(memory.NewStorage(), &gitconfig.RemoteConfig{Name: "origin", URLs: []string{to}})
	refs, err := rm.List(&git.ListOptions{Auth: auth})
	if errors.Is(err, transport.ErrEmptyRemoteRepository) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("list the refs of %s: %w", u, err)
	}
	return len(refs) == 0, nil
}

// HeadHasFile reports whether the tree of HEAD of the repository at dir,
// bare or not, has the file p. A repository with no commit has none.
func HeadHasFile(dir, p string) (bool, error) {
	r, err := git.PlainOpen(dir)
	if err != nil {
		return false, fmt.Errorf("open %s: %w", dir, err)
	}
	ref, err := r.Head()
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	c, err := r.CommitObject(ref.Hash())
	if err != nil {
		return false, err
	}
	_, err = c.File(p)
	if errors.Is(err, object.ErrFileNotFound) {
		return false, nil
	}
	return err == nil, err
}
