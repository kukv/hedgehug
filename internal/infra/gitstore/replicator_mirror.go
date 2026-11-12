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
