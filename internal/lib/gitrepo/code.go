package gitrepo

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

// ErrNoRepository means the directory is not in a git repository.
var ErrNoRepository = errors.New("not in a git repository")

// ErrNoCommit means the repository has no commit to start a branch at.
var ErrNoCommit = errors.New("the repository has no commit")

// Code is the repository the developer works in, as opposed to a store.
type Code struct {
	dir string
	r   *git.Repository
}

func openCodeRepo(dir string) (*git.Repository, error) {
	// A linked worktree's .git is a file that points into the .git of the main
	// repository, where the remotes and the branches are; HEAD is the
	// worktree's own.
	r, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{DetectDotGit: true, EnableDotGitCommonDir: true})
	if errors.Is(err, git.ErrRepositoryNotExists) {
		return nil, ErrNoRepository
	}
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", dir, err)
	}
	return r, nil
}

// OpenCode opens the repository at dir or above it.
func OpenCode(dir string) (*Code, error) {
	r, err := openCodeRepo(dir)
	if err != nil {
		return nil, err
	}
	return &Code{dir: dir, r: r}, nil
}

// Close lets go of the files the repository holds open.
func (c *Code) Close() error {
	if s, ok := c.r.Storer.(interface{ Close() error }); ok {
		return s.Close()
	}
	return nil
}

// Branch returns the branch HEAD is on; ok is false on a detached HEAD.
func (c *Code) Branch() (name string, ok bool, err error) {
	ref, err := c.r.Reference(plumbing.HEAD, false)
	if err != nil {
		return "", false, err
	}
	if ref.Type() != plumbing.SymbolicReference || !ref.Target().IsBranch() {
		return "", false, nil
	}
	return strings.TrimPrefix(string(ref.Target()), "refs/heads/"), true, nil
}

// HasBranch reports whether the branch name exists.
func (c *Code) HasBranch(name string) (bool, error) {
	_, err := c.r.Reference(plumbing.NewBranchReferenceName(name), false)
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return false, nil
	}
	return err == nil, err
}

// StartBranch makes the branch name at HEAD and moves HEAD to it. The index and
// the files stay as they are. It fails when the branch exists.
func (c *Code) StartBranch(name string) error {
	head, err := c.r.Reference(plumbing.HEAD, true)
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return ErrNoCommit
	}
	if err != nil {
		return err
	}
	ref := plumbing.NewBranchReferenceName(name)
	if ok, err := c.HasBranch(name); err != nil {
		return err
	} else if ok {
		return fmt.Errorf("branch %s already exists", name)
	}
	if err := c.r.Storer.SetReference(plumbing.NewHashReference(ref, head.Hash())); err != nil {
		return err
	}
	return c.r.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, ref))
}

// Branches returns the names of the local branches match accepts, sorted.
func (c *Code) Branches(match func(string) bool) ([]string, error) {
	it, err := c.r.Branches()
	if err != nil {
		return nil, err
	}
	defer it.Close()
	var out []string
	err = it.ForEach(func(ref *plumbing.Reference) error {
		if n := strings.TrimPrefix(string(ref.Name()), "refs/heads/"); match(n) {
			out = append(out, n)
		}
		return nil
	})
	slices.Sort(out)
	return out, err
}

// RemoteURL returns the URL of the remote name as .git/config has it; ok is
// false when there is no such remote.
func (c *Code) RemoteURL(name string) (string, bool, error) {
	rm, err := c.r.Remote(name)
	if errors.Is(err, git.ErrRemoteNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	urls := rm.Config().URLs
	if len(urls) == 0 {
		return "", false, nil
	}
	return urls[0], true, nil
}

// PushBranch sends the branch to the remote as a fast-forward, through conn.
// It returns ErrRejected when the remote has commits the branch does not, and
// ErrPushDenied when the remote refuses to be written; nothing to send is a
// success.
func (c *Code) PushBranch(remote, branch string, conn Conn) error {
	u, ok, err := c.RemoteURL(remote)
	if err != nil {
		return fmt.Errorf("push %s: %w", remote, err)
	}
	if !ok {
		return fmt.Errorf("push %s: no such remote", remote)
	}
	to, auth, err := conn.endpoint(u)
	if err != nil {
		return fmt.Errorf("push %s: %w", remote, err)
	}
	// As Repo.Push: when the remote refuses the push before it reads the pack,
	// go-git returns without waiting for the goroutine that writes the pack, so
	// the push goes through a Repository of its own.
	p, err := openCodeRepo(c.dir)
	if err != nil {
		return fmt.Errorf("push %s: %w", remote, err)
	}
	if s, ok := p.Storer.(interface{ Close() error }); ok {
		defer func() { _ = s.Close() }()
	}
	spec := "refs/heads/" + branch + ":refs/heads/" + branch
	err = p.Push(&git.PushOptions{RemoteName: remote, RemoteURL: to, Auth: auth, RefSpecs: []config.RefSpec{config.RefSpec(spec)}})
	switch {
	case err == nil || errors.Is(err, git.NoErrAlreadyUpToDate):
		return nil
	case errors.Is(err, git.ErrNonFastForwardUpdate) || strings.Contains(err.Error(), "non-fast-forward"):
		return ErrRejected
	case pushDenied(err, isLocal(to)):
		return fmt.Errorf("push %s: %w: %w", remote, ErrPushDenied, err)
	}
	return fmt.Errorf("push %s: %w", remote, err)
}
