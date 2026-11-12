package gitstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/port"
)

// maxSendTries は、送信が他の人の更新と重なったときに、受け取りからやり直す最大の回数(06「同期のエラー」)。
const maxSendTries = 3

// Replicator は、gitstore の Store を remote と合わせる(port.Replicator)。
type Replicator struct {
	stores *Stores

	// beforeSend は、送る直前に呼ぶ。テストで、送信が他の人の更新と重なる場面を作るためのもの。
	beforeSend func() error
}

func NewReplicator(s *Stores) *Replicator { return &Replicator{stores: s} }

// Sync は、受け取り、合わせ、送る。合わせているあいだは、Store のロックを持つ。
func (r *Replicator) Sync(ctx context.Context, id port.StoreID, remote port.Remote) (port.SyncResult, error) {
	root, err := r.stores.dir(id)
	if err != nil {
		return port.SyncResult{}, err
	}
	// Read・Write と同じく、途中で止まった保存の跡を戻してから合わせる(跡を合わせたコミットに入れない)。
	unlock, err := open(root)
	if err != nil {
		return port.SyncResult{}, err
	}
	defer unlock()
	repo, err := git.PlainOpen(root)
	if err != nil {
		return port.SyncResult{}, err
	}
	head, err := repo.Head()
	if err != nil {
		return port.SyncResult{}, err
	}
	s := syncRun{r: r, repo: repo, root: root, link: newLink(repo, remote), branch: head.Name(), start: head.Hash()}
	// 2 回目からの試みが失敗しても(断る理由のエラーでも、ErrSendConflict と同じく)、前の試みで作った
	// 合わせたコミットはこのマシンに残る。それは受け取ったものを正しく合わせたもので、次の同期で続ける。
	for range maxSendTries {
		res, err := s.once(ctx)
		if errors.Is(err, errRejected) {
			continue
		}
		return res, err
	}
	return port.SyncResult{}, port.ErrSendConflict
}

// syncRun は、1 回の Sync の状態。start は、同期を始めたときの自分の先端(受け取った数を数える基準)。
type syncRun struct {
	r      *Replicator
	repo   *git.Repository
	root   string
	link   link
	branch plumbing.ReferenceName
	start  plumbing.Hash
}

// once は、受け取り・合わせる・送るを 1 回行う。送信が拒否されたら errRejected。
func (s syncRun) once(ctx context.Context) (port.SyncResult, error) {
	theirs, err := s.link.fetch(ctx, s.repo, s.branch)
	if err != nil {
		return port.SyncResult{}, err
	}
	if !theirs.IsZero() {
		if err := checkRemoteStore(s.repo, theirs); err != nil {
			return port.SyncResult{}, err
		}
		if err := s.integrate(theirs); err != nil {
			return port.SyncResult{}, err
		}
	}
	if s.r.beforeSend != nil {
		if err := s.r.beforeSend(); err != nil {
			return port.SyncResult{}, err
		}
	}
	if err := s.link.send(ctx, s.branch, theirs); err != nil {
		return port.SyncResult{}, err
	}
	return s.result(theirs)
}

// integrate は、自分の先端に remote の先端 theirs を取り込む。
func (s syncRun) integrate(theirs plumbing.Hash) error {
	head, err := s.repo.Head()
	if err != nil {
		return err
	}
	ours := head.Hash()
	oc, err := s.repo.CommitObject(ours)
	if err != nil {
		return err
	}
	tc, err := s.repo.CommitObject(theirs)
	if err != nil {
		return err
	}
	if ours == theirs {
		return nil
	}
	if behind, err := tc.IsAncestor(oc); err != nil || behind {
		return err
	}
	wt, err := s.repo.Worktree()
	if err != nil {
		return err
	}
	if ahead, err := oc.IsAncestor(tc); err != nil {
		return err
	} else if ahead {
		return s.guarded(func() error { return wt.Reset(&git.ResetOptions{Commit: theirs, Mode: git.HardReset}) })
	}
	bases, err := oc.MergeBase(tc)
	if err != nil {
		return err
	}
	if len(bases) == 0 {
		return port.ErrNoCommonHistory
	}
	return s.guarded(func() error { return s.merge(wt, bases[0].Hash, ours, theirs) })
}

// guarded は、作業ツリーを変える fn のあいだ、保存の途中であることを示す印を置く。プロセスが落ちても、
// 次の操作が印を見て、いまのコミットに戻す(保存形式「技術構成」)。fn が失敗したときは印を残し、
// 次の操作に戻させる(戻すのは何度行っても同じ結果になる)。
func (s syncRun) guarded(fn func() error) error {
	if err := os.WriteFile(markerPath(s.root), nil, 0o600); err != nil {
		return err
	}
	if err := fn(); err != nil {
		return err
	}
	// 作業ツリーはコミットと同じ。印が消せなくても、次の操作がいまのコミットに戻すだけで、何も失われない。
	_ = os.Remove(markerPath(s.root))
	return nil
}

// merge は、3 つのツリーを合わせて作業ツリーに書き、2 つの親を持つコミットにする。失敗したら元に戻す。
func (s syncRun) merge(wt *git.Worktree, base, ours, theirs plumbing.Hash) error {
	trees := make([]tree, 3)
	for i, h := range []plumbing.Hash{base, ours, theirs} {
		t, err := readTree(s.repo, h)
		if err != nil {
			return err
		}
		trees[i] = t
	}
	merged, err := mergeTrees(trees[0], trees[1], trees[2])
	if err != nil {
		return err
	}
	created, err := writeTree(s.root, trees[1], merged)
	if err == nil {
		err = wt.AddWithOptions(&git.AddOptions{All: true})
	}
	if err == nil {
		sig := signature(botName, botEmail, s.r.stores.clock.Now())
		_, err = wt.Commit("remote と合わせる\n", &git.CommitOptions{Author: sig, Committer: sig, Parents: []plumbing.Hash{ours, theirs}})
	}
	if err != nil {
		errs := []error{err, wt.Reset(&git.ResetOptions{Commit: ours, Mode: git.HardReset})}
		for _, p := range created {
			errs = append(errs, os.Remove(filepath.Join(s.root, p)))
		}
		return errors.Join(errs...)
	}
	return nil
}

// result は、送った保存(remote の先端 theirs になかったもの)と、受け取った保存(始めたときの自分になかったもの)を数える。
func (s syncRun) result(theirs plumbing.Hash) (port.SyncResult, error) {
	head, err := s.repo.Head()
	if err != nil {
		return port.SyncResult{}, err
	}
	sent, err := countSaves(s.repo, head.Hash(), theirs)
	if err != nil {
		return port.SyncResult{}, err
	}
	received, err := countSaves(s.repo, theirs, s.start)
	if err != nil {
		return port.SyncResult{}, err
	}
	return port.SyncResult{Sent: sent, Received: received}, nil
}
