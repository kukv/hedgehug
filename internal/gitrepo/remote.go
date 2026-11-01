package gitrepo

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	format "github.com/go-git/go-git/v5/plumbing/format/config"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

// ErrRejected means the remote has commits this clone does not.
var ErrRejected = errors.New("the remote rejected the push: it has commits this clone does not")

// InitBare creates a bare repository in dir, with main as its default branch.
func InitBare(dir string) error {
	_, err := git.PlainInitWithOptions(dir, &git.PlainInitOptions{
		Bare:        true,
		InitOptions: git.InitOptions{DefaultBranch: plumbing.NewBranchReferenceName("main")},
	})
	if err != nil {
		return fmt.Errorf("init bare %s: %w", dir, err)
	}
	return nil
}

// remoteName names the i-th remote: origin, then mirror1, mirror2...
func remoteName(i int) string {
	if i == 0 {
		return "origin"
	}
	return "mirror" + strconv.Itoa(i)
}

// isManagedRemote reports whether name is one SetRemotes writes.
func isManagedRemote(name string) bool {
	if name == "origin" {
		return true
	}
	n, ok := strings.CutPrefix(name, "mirror")
	if !ok {
		return false
	}
	i, err := strconv.Atoi(n)
	return err == nil && i >= 1 && remoteName(i) == name
}

func remoteConfig(name, url string) *config.RemoteConfig {
	return &config.RemoteConfig{
		Name:  name,
		URLs:  []string{url},
		Fetch: []config.RefSpec{config.RefSpec("+refs/heads/main:refs/remotes/" + name + "/main")},
	}
}

// SetRemotes writes urls to .git/config as origin, mirror1... and removes the
// origin and mirror remotes it no longer lists. Other remotes are kept.
func (r *Repo) SetRemotes(urls []string) error {
	cfg, err := r.r.Config()
	if err != nil {
		return err
	}
	keep := make(map[string]bool, len(urls))
	for i, u := range urls {
		name := remoteName(i)
		keep[name] = true
		cfg.Remotes[name] = remoteConfig(name, u)
	}
	for name := range cfg.Remotes {
		if isManagedRemote(name) && !keep[name] {
			delete(cfg.Remotes, name)
		}
	}
	return r.r.SetConfig(cfg)
}

// Remotes returns the names of the configured remotes in the order origin,
// mirror1...; other remotes are not listed.
func (r *Repo) Remotes() []string {
	cfg, err := r.r.Config()
	if err != nil {
		return nil
	}
	var names []string
	for i := 0; ; i++ {
		name := remoteName(i)
		if _, ok := cfg.Remotes[name]; !ok {
			return names
		}
		names = append(names, name)
	}
}

func trackingRef(remote string) plumbing.ReferenceName {
	return plumbing.NewRemoteReferenceName(remote, "main")
}

// Fetch updates refs/remotes/<remote>/main and returns its hash, or the zero
// hash when the remote is empty.
func (r *Repo) Fetch(remote string) (plumbing.Hash, error) {
	u, err := r.remoteURL(remote)
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("fetch %s: %w", remote, err)
	}
	to, auth, err := r.conn.endpoint(u)
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("fetch %s: %w", remote, err)
	}
	err = r.r.Fetch(&git.FetchOptions{RemoteName: remote, RemoteURL: to, Auth: auth})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) && !errors.Is(err, transport.ErrEmptyRemoteRepository) {
		return plumbing.ZeroHash, fmt.Errorf("fetch %s: %w", remote, err)
	}
	return r.RemoteHead(remote)
}

// RemoteHead returns the hash of refs/remotes/<remote>/main as of the last
// fetch or push; zero when nothing was fetched or the remote is empty.
func (r *Repo) RemoteHead(remote string) (plumbing.Hash, error) {
	ref, err := r.r.Reference(trackingRef(remote), true)
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return plumbing.ZeroHash, nil
	}
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return ref.Hash(), nil
}

// beforePush runs once Push has read the commit it sends; tests replace it to
// commit meanwhile.
var beforePush = func() {}

// Push sends main to the remote as a fast-forward. It returns ErrRejected when
// the remote is not an ancestor of main, and nil when there is nothing to send.
// The commit main is at when Push starts is sent, and becomes
// refs/remotes/<remote>/main; a commit made during the push is not taken as
// being on the remote.
func (r *Repo) Push(remote string) error { return r.push(remote, false, plumbing.ZeroHash) }

// push sends main to the remote; force sends it even when the remote is not
// its ancestor.
func (r *Repo) push(remote string, force bool, lease plumbing.Hash) error {
	h, err := r.Head()
	if err != nil {
		return err
	}
	if h.IsZero() {
		return errors.New("push: the repository has no commit")
	}
	u, err := r.remoteURL(remote)
	if err != nil {
		return fmt.Errorf("push %s: %w", remote, err)
	}
	to, auth, err := r.conn.endpoint(u)
	if err != nil {
		return fmt.Errorf("push %s: %w", remote, err)
	}
	beforePush()
	spec := h.String() + ":refs/heads/main"
	if force {
		spec = "+" + spec
	}
	opts := &git.PushOptions{
		RemoteName: remote,
		RemoteURL:  to,
		Auth:       auth,
		RefSpecs:   []config.RefSpec{config.RefSpec(spec)},
		Force:      force,
	}
	if force && !lease.IsZero() {
		// go-git checks a lease for a ref source only, not for a hash
		// source; the caller holds the store lock, so main is h.
		opts.RefSpecs = []config.RefSpec{config.RefSpec("+" + mainRef.String() + ":refs/heads/main")}
		opts.ForceWithLease = &git.ForceWithLease{RefName: mainRef, Hash: lease}
	}
	err = r.r.Push(opts)
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) && (errors.Is(err, git.ErrNonFastForwardUpdate) || strings.Contains(err.Error(), "non-fast-forward")) {
		return ErrRejected
	}
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) && pushDenied(err, isLocal(to)) {
		return fmt.Errorf("push %s: %w: %w", remote, ErrPushDenied, err)
	}
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return fmt.Errorf("push %s: %w", remote, err)
	}
	return r.SetTracking(remote, h)
}

func isLocal(u string) bool {
	_, ok := LocalPath(u)
	return ok
}

// Head returns the commit HEAD points at, or the zero hash when there is no
// commit yet.
func (r *Repo) Head() (plumbing.Hash, error) {
	ref, err := r.r.Head()
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return plumbing.ZeroHash, nil
	}
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return ref.Hash(), nil
}

// afterClone finishes a fresh clone; tests replace it to make the step fail.
var afterClone = func(r *Repo, url string) error { return r.SetRemotes([]string{url}) }

// Clone is CloneWith with no rewrite and no auth.
func Clone(url, dir string) (*Repo, error) { return CloneWith(url, dir, Conn{}) }

// CloneWith clones the main branch of url into dir through c; .git/config
// keeps url as given. dir must be missing or an empty directory; anything
// else is refused untouched. A failed clone removes what it created: the
// directory and its missing parents, or only its contents when dir existed.
func CloneWith(url, dir string, c Conn) (*Repo, error) {
	created, err := checkCloneTarget(dir)
	if err != nil {
		return nil, fmt.Errorf("clone %s: %w", url, err)
	}
	to, auth, err := c.endpoint(url)
	if err != nil {
		return nil, fmt.Errorf("clone %s: %w", url, err)
	}
	r, err := git.PlainClone(dir, false, &git.CloneOptions{
		URL:           to,
		Auth:          auth,
		RemoteName:    "origin",
		ReferenceName: plumbing.NewBranchReferenceName("main"),
		SingleBranch:  true,
	})
	if err == nil {
		repo := newRepo(dir, r)
		repo.conn = c
		if err = afterClone(repo, url); err == nil {
			return repo, nil
		}
	} else {
		err = fmt.Errorf("clone %s: %w", url, err)
	}
	removeCreated(dir, created)
	return nil, err
}

// checkCloneTarget fails unless dir is missing or an empty directory; a
// symlink is refused, so cleanup never follows it. When dir is missing it
// returns the topmost of dir and its parents that does not exist, the path the
// clone will create; it returns "" for an existing empty directory.
func checkCloneTarget(dir string) (created string, err error) {
	fi, err := os.Lstat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		created = dir
		for p := filepath.Dir(created); p != created; p = filepath.Dir(created) {
			if _, err := os.Lstat(p); !errors.Is(err, fs.ErrNotExist) {
				// Windows says "not found" for a path below a file, where Unix says
				// "not a directory"; either way the clone could not make dir.
				if pi, err := os.Stat(p); err == nil && !pi.IsDir() {
					return "", fmt.Errorf("%s: %s is not a directory", dir, p)
				}
				break
			}
			created = p
		}
		return created, nil
	}
	if err != nil {
		return "", fmt.Errorf("%s: %w", dir, err)
	}
	if !fi.IsDir() {
		return "", fmt.Errorf("%s exists and is not a directory", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("%s: %w", dir, err)
	}
	if len(entries) > 0 {
		return "", fmt.Errorf("%s is not empty", dir)
	}
	return "", nil
}

// removeCreated removes created, or the contents of dir when created is "".
func removeCreated(dir, created string) {
	if created != "" {
		_ = os.RemoveAll(created)
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		_ = os.RemoveAll(filepath.Join(dir, e.Name()))
	}
}

// MergeDriver is the command .git/config runs for the files .gitattributes
// marks merge=hedgehug. It names hedgehug, not a path to the binary, so a
// moved or temporary binary leaves nothing stale behind.
const MergeDriver = "hedgehug merge-driver %O %A %B %P"

// RegisterMergeDriver writes the hedgehug merge driver to .git/config. It
// leaves the file alone when the driver is already there.
func (r *Repo) RegisterMergeDriver() error {
	cfg, err := r.r.Config()
	if err != nil {
		return err
	}
	sub := cfg.Raw.Section("merge").Subsection("hedgehug")
	if mergeDriverSet(sub) {
		return nil
	}
	sub.SetOption("name", "Hedgehug field merge").SetOption("driver", MergeDriver)
	return r.r.SetConfig(cfg)
}

// MergeDriverRegistered reports whether .git/config has the hedgehug merge
// driver, as RegisterMergeDriver writes it.
func (r *Repo) MergeDriverRegistered() (bool, error) {
	cfg, err := r.r.Config()
	if err != nil {
		return false, err
	}
	return mergeDriverSet(cfg.Raw.Section("merge").Subsection("hedgehug")), nil
}

func mergeDriverSet(sub *format.Subsection) bool {
	return sub.Option("name") == "Hedgehug field merge" && sub.Option("driver") == MergeDriver
}
