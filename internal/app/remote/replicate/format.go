package replicate

import (
	"errors"
	"fmt"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/domain/upgrade"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

// ErrFormatDiverged means the two sides of a merge keep their store in
// different formats: one was converted while the other still has changes
// that are not merged. hh store convert converts those changes, then syncs.
// The error is a *FormatDivergedError, which says on which side the newer
// format is.
var ErrFormatDiverged = errors.New("the remote and this clone keep the store in different formats; run hh store convert")

// FormatDivergedError is ErrFormatDiverged with the two formats.
type FormatDivergedError struct {
	Ours, Theirs int
}

// RemoteOlder reports that the remote is in the older format: the changes
// that are not merged are the remote's.
func (e *FormatDivergedError) RemoteOlder() bool { return e.Theirs < e.Ours }

func (e *FormatDivergedError) Error() string {
	if e.RemoteOlder() {
		return fmt.Sprintf("format %d here, %d there: the remote has changes in an older store format that are not merged; run hh store convert to convert the changes of the remote", e.Ours, e.Theirs)
	}
	return fmt.Sprintf("format %d here, %d there: the remote uses a newer store format and this clone has changes that are not pushed; run hh store convert", e.Ours, e.Theirs)
}

// Is makes the error ErrFormatDiverged.
func (e *FormatDivergedError) Is(target error) bool { return target == ErrFormatDiverged }

// FormatAt is the store format of the commit: the format of its store.yaml.
func FormatAt(r *gitrepo.Repo, commit plumbing.Hash) (int, error) {
	return upgrade.FormatOf(func(p string) ([]byte, error) { return r.ReadAt(commit, p) })
}

// checkFormats is ErrFormatDiverged when ours and theirs have different
// formats. A store.yaml that cannot be read is left to the merge, which
// treats it as any file that does not decode.
func checkFormats(r *gitrepo.Repo, ours, theirs plumbing.Hash) error {
	a, err := FormatAt(r, ours)
	if err != nil {
		return nil
	}
	b, err := FormatAt(r, theirs)
	if err != nil {
		return nil
	}
	if a != b {
		return &FormatDivergedError{Ours: a, Theirs: b}
	}
	return nil
}

// FetchOrigin fetches origin as a sync does, with the check for a rewritten
// history, and puts nothing into HEAD. It is for what has to know what origin
// has before it syncs.
func FetchOrigin(s *store.Store, opt Options) error {
	r := s.Repo()
	r.SetConn(opt.Conn)
	if err := r.SetRemotes(opt.Remotes); err != nil {
		return err
	}
	tracked, err := beginFetch(s)
	if err != nil {
		return err
	}
	if _, err := r.Fetch("origin"); err != nil {
		return withEnd(err, endFetch(s))
	}
	return withEnd(checkOrigin(s, tracked, opt), endFetch(s))
}

// FastForwardOrigin moves HEAD to the head of origin as the last fetch left
// it, which has to descend from HEAD. It is for hh store convert, in a clone
// that origin has converted and that has nothing to push.
func FastForwardOrigin(s *store.Store) error {
	r := s.Repo()
	head, err := r.Head()
	if err != nil {
		return err
	}
	remote, err := r.RemoteHead("origin")
	if err != nil {
		return err
	}
	return fastForward(s, head, remote)
}
