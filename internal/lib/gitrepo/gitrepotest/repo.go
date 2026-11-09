package gitrepotest

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Bare makes an empty bare repository at dir, whose branch is main.
func Bare(t testing.TB, dir string) {
	t.Helper()
	_, err := git.PlainInitWithOptions(dir, &git.PlainInitOptions{
		Bare:        true,
		InitOptions: git.InitOptions{DefaultBranch: plumbing.NewBranchReferenceName("main")},
	})
	if err != nil {
		t.Fatalf("init bare %s: %v", dir, err)
	}
}

// MainHasFile reports whether the tree of the branch main of the repository
// at dir, bare or not, has the file rel, as gitrepo.MainHasFile does, for the
// tests of the layers that do not import gitrepo. A repository without main
// has none. It does not depend on HEAD, which a plain git init --bare leaves
// on master while a store is pushed to main.
func MainHasFile(dir, rel string) (bool, error) {
	r, err := git.PlainOpen(dir)
	if err != nil {
		return false, err
	}
	ref, err := r.Reference(plumbing.NewBranchReferenceName("main"), true)
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
	_, err = c.File(rel)
	if errors.Is(err, object.ErrFileNotFound) {
		return false, nil
	}
	return err == nil, err
}

// CommitFiles commits the files rels (slash-separated, from the root of the
// work tree at dir) as they are on disk, on top of HEAD: the author and the
// committer are name and email, at when, and the message is subject.
func CommitFiles(t testing.TB, dir, subject, name, email string, when time.Time, rels ...string) {
	t.Helper()
	r, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	w, err := r.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range rels {
		if _, err := w.Add(filepath.ToSlash(rel)); err != nil {
			t.Fatal(err)
		}
	}
	sig := &object.Signature{Name: name, Email: email, When: when}
	if _, err := w.Commit(subject+"\n", &git.CommitOptions{Author: sig, Committer: sig}); err != nil {
		t.Fatal(err)
	}
}

// CommitFilesOn is CommitFiles on top of the commit parent (a hash, as hex)
// rather than HEAD: the branch goes back to parent first, with the files as they
// are. It returns the hash of the new commit.
func CommitFilesOn(t testing.TB, dir, parent, subject, name, email string, when time.Time, rels ...string) string {
	t.Helper()
	r, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	w, err := r.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Reset(&git.ResetOptions{Commit: plumbing.NewHash(parent), Mode: git.SoftReset}); err != nil {
		t.Fatal(err)
	}
	CommitFiles(t, dir, subject, name, email, when, rels...)
	return HeadHash(t, dir)
}

// HeadHash is the hash of HEAD of the repository at dir, as hex.
func HeadHash(t testing.TB, dir string) string {
	t.Helper()
	r, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	return ref.Hash().String()
}

// Trailer is one "Key: Value" line at the end of a commit message.
type Trailer struct{ Key, Value string }

// Commit is one commit as Log reads it back.
type Commit struct {
	Hash     string
	Subject  string
	Trailers []Trailer
}

// Trailer returns the value of the first trailer named key.
func (c Commit) Trailer(key string) (string, bool) {
	for _, t := range c.Trailers {
		if t.Key == key {
			return t.Value, true
		}
	}
	return "", false
}

// TrailerValues returns the values of every trailer named key, in the order
// they are written.
func (c Commit) TrailerValues(key string) []string {
	var out []string
	for _, t := range c.Trailers {
		if t.Key == key {
			out = append(out, t.Value)
		}
	}
	return out
}

// Log is the first-parent log of HEAD of the repository at dir, newest
// first, capped at limit when it is above zero.
func Log(t testing.TB, dir string, limit int) []Commit {
	t.Helper()
	r, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := r.Head()
	if err != nil {
		t.Fatal(err)
	}
	var out []Commit
	for h := ref.Hash(); !h.IsZero() && (limit <= 0 || len(out) < limit); {
		c, err := r.CommitObject(h)
		if err != nil {
			t.Fatal(err)
		}
		subject, trailers := parseMessage(c.Message)
		out = append(out, Commit{Hash: c.Hash.String(), Subject: subject, Trailers: trailers})
		h = plumbing.ZeroHash
		if len(c.ParentHashes) > 0 {
			h = c.ParentHashes[0]
		}
	}
	return out
}

// parseMessage splits a commit message into its subject and the trailers:
// the "Key: Value" lines of the last paragraph, if every line of it is one
// and a blank line follows the subject.
func parseMessage(msg string) (string, []Trailer) {
	msg = strings.TrimRight(msg, "\n")
	subject, rest, found := strings.Cut(msg, "\n")
	if !found || !strings.HasPrefix(rest, "\n") {
		return subject, nil
	}
	paras := strings.Split(strings.TrimLeft(rest, "\n"), "\n\n")
	var trailers []Trailer
	for _, line := range strings.Split(paras[len(paras)-1], "\n") {
		k, v, ok := strings.Cut(line, ": ")
		if !ok || k == "" || strings.ContainsAny(k, " \t") {
			return subject, nil
		}
		trailers = append(trailers, Trailer{Key: k, Value: v})
	}
	return subject, trailers
}
