package gitstore

import (
	"context"
	"errors"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/port"
)

// checkSeen は、remote の先端 theirs が、前に受け取った位置 s.seen の続きかを確かめる(保存形式「remote の
// 履歴が書き換えられたとき」の見つけ方)。続きでなければ port.ErrRemoteRewritten。位置がなければ確かめない。
func (s syncRun) checkSeen(theirs plumbing.Hash) error {
	if s.seen.IsZero() || s.seen == theirs {
		return nil
	}
	if theirs.IsZero() {
		return port.ErrRemoteRewritten // remote が空になった(決定 C2-12)
	}
	seen, err := s.repo.CommitObject(s.seen)
	if errors.Is(err, plumbing.ErrObjectNotFound) {
		return port.ErrRemoteRewritten
	}
	if err != nil {
		return err
	}
	tc, err := s.repo.CommitObject(theirs)
	if err != nil {
		return err
	}
	ok, err := seen.IsAncestor(tc)
	if err != nil {
		return err
	}
	if !ok {
		return port.ErrRemoteRewritten
	}
	return nil
}

// parseSeen は、Remote.Seen をハッシュにする。読めなければゼロ(確かめない)。
func parseSeen(s string) plumbing.Hash {
	if !plumbing.IsHash(s) {
		return plumbing.ZeroHash
	}
	return plumbing.NewHash(s)
}

// Unsent は、remote に送っていない保存の数(決定 C2-14)。前の同期で受け取った remote の先端から辿れない保存を
// 数える。一度も受け取っていなければ、すべての保存を数える。
func (r *Replicator) Unsent(_ context.Context, id port.StoreID) (int, error) {
	root, err := r.stores.dir(id)
	if err != nil {
		return 0, err
	}
	repo, err := git.PlainOpen(root)
	if err != nil {
		return 0, err
	}
	head, err := repo.Head()
	if err != nil {
		return 0, err
	}
	var theirs plumbing.Hash
	ref, err := repo.Reference(plumbing.ReferenceName(remotePrefix+head.Name().Short()), true)
	switch {
	case err == nil:
		theirs = ref.Hash()
	case !errors.Is(err, plumbing.ErrReferenceNotFound):
		return 0, err
	}
	return countSaves(repo, head.Hash(), theirs)
}
