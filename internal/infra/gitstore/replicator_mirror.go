package gitstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/port"
)

// mirrorPrefix は、ミラーから受け取ったブランチを置く場所。ミラーの URL ごとに分け、remote のもの
// (remotePrefix。未送信の数え方に使う)とは混ぜない。
const mirrorPrefix = "refs/hedgehug/mirror/"

func mirrorRefPrefix(url string) string {
	sum := sha256.Sum256([]byte(url))
	return mirrorPrefix + hex.EncodeToString(sum[:])[:12] + "/"
}

// Mirror は、Store の先端をミラーへ送る(保存形式「同期」の手順 7、決定 C2-15)。ミラーからは受け取って
// 合わせない。履歴は書き換えない(強制しない)。
func (r *Replicator) Mirror(ctx context.Context, id port.StoreID, mirror port.Remote) error {
	root, err := r.stores.dir(id)
	if err != nil {
		return err
	}
	repo, name, mine, err := readHead(root)
	if err != nil {
		return err
	}
	l := newLink(repo, mirror)
	theirs, err := l.fetchTo(ctx, repo, name, mirrorRefPrefix(mirror.URL))
	if errors.Is(err, port.ErrNoCommonHistory) {
		return port.ErrMirrorUnrelated
	}
	if err != nil {
		return err
	}
	if theirs == mine {
		return nil
	}
	if !theirs.IsZero() {
		if err := mirrorBehind(repo, theirs, mine); err != nil {
			return err
		}
	}
	err = l.send(ctx, name, theirs, mine)
	if errors.Is(err, errRejected) {
		return port.ErrMirrorAhead // 送るあいだに、ミラーが先に進んだ
	}
	return err
}

// readHead は、Store の先端のブランチ名とハッシュを読む。ロックを持つのは読むあいだだけで、
// ミラーとの通信(最大 60 秒)のあいだは持たない。ミラーへは、ここで読んだハッシュで合わせる。
func readHead(root string) (*git.Repository, plumbing.ReferenceName, plumbing.Hash, error) {
	unlock, err := open(root)
	if err != nil {
		return nil, "", plumbing.ZeroHash, err
	}
	defer unlock()
	repo, err := git.PlainOpen(root)
	if err != nil {
		return nil, "", plumbing.ZeroHash, err
	}
	head, err := repo.Head()
	if err != nil {
		return nil, "", plumbing.ZeroHash, err
	}
	return repo, head.Name(), head.Hash(), nil
}

// mirrorBehind は、ミラーの先端 theirs が自分の先端 mine の祖先なら nil。共通の祖先がなければ
// ErrMirrorUnrelated、あればミラーにだけ保存があるので ErrMirrorAhead。
func mirrorBehind(repo *git.Repository, theirs, mine plumbing.Hash) error {
	tc, err := repo.CommitObject(theirs)
	if err != nil {
		return err
	}
	mc, err := repo.CommitObject(mine)
	if err != nil {
		return err
	}
	if ok, err := tc.IsAncestor(mc); err != nil || ok {
		return err
	}
	bases, err := mc.MergeBase(tc)
	if err != nil {
		return err
	}
	if len(bases) == 0 {
		return port.ErrMirrorUnrelated
	}
	return port.ErrMirrorAhead
}

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

// restoreSeen は、止めた同期のあとで、受け取った位置の記録を s.seen に戻す(受け取りが書き換え後の先端に
// 動かしたので)。未送信の数が動かない(06「未送信の保存はそのまま残る」)。s.seen の保存が手元になければそのままにする。
func (s syncRun) restoreSeen() error {
	if _, err := s.repo.CommitObject(s.seen); err != nil {
		return nil
	}
	return s.repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(remotePrefix+s.branch.Short()), s.seen))
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
