package replicate

import (
	"errors"
	"fmt"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/domain/model"
	"github.com/kukv/hedgehug/internal/app/storage/store"
	"github.com/kukv/hedgehug/internal/lib/gitrepo"
)

// ErrFormatDiverged means the two sides of a merge keep their store in
// different formats: the remote was converted to a newer format while this
// clone still has changes it has not pushed. hh store convert converts those
// changes, then syncs.
var ErrFormatDiverged = errors.New("the remote uses a newer store format and this clone has changes that are not pushed; run hh store convert")

// FormatAt is the store format of the commit: the format of its store.yaml.
func FormatAt(r *gitrepo.Repo, commit plumbing.Hash) (int, error) {
	tree, err := r.TreeOf(commit)
	if err != nil {
		return 0, err
	}
	b, err := r.ReadFile(tree, model.StoreMetaPath)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", model.StoreMetaPath, err)
	}
	m, err := model.DecodeStoreMeta(b)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", model.StoreMetaPath, err)
	}
	return m.Format, nil
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
		return fmt.Errorf("format %d here, %d there: %w", a, b, ErrFormatDiverged)
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
