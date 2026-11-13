package gitstore

import (
	"context"
	"errors"
	"log/slog"
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
	s, unlock, err := r.begin(id, remote)
	if err != nil {
		return port.SyncResult{}, err
	}
	defer unlock()
	// 2 回目からの試みが失敗しても(断る理由のエラーでも、ErrSendConflict と同じく)、前の試みで作った
	// 合わせたコミットはこのマシンに残る。それは受け取ったものを正しく合わせたもので、次の同期で続ける。
	for range maxSendTries {
		res, err := s.once(ctx)
		if errors.Is(err, errRejected) {
			continue
		}
		if errors.Is(err, port.ErrRemoteRewritten) {
			err = errors.Join(err, s.restoreSeen())
		}
		return res, err
	}
	return port.SyncResult{}, port.ErrSendConflict
}

// begin は、Store のロックを取り、1 回の同期の状態を作る。unlock は呼ぶ側が呼ぶ。
func (r *Replicator) begin(id port.StoreID, remote port.Remote) (s syncRun, unlock func(), err error) {
	root, err := r.stores.dir(id)
	if err != nil {
		return syncRun{}, nil, err
	}
	// Read・Write と同じく、途中で止まった保存の跡を戻してから合わせる(跡を合わせたコミットに入れない)。
	unlock, err = open(root)
	if err != nil {
		return syncRun{}, nil, err
	}
	repo, err := git.PlainOpen(root)
	if err != nil {
		unlock()
		return syncRun{}, nil, err
	}
	head, err := repo.Head()
	if err != nil {
		unlock()
		return syncRun{}, nil, err
	}
	return syncRun{
		r: r, repo: repo, root: root, link: newLink(repo, remote), branch: head.Name(), start: head.Hash(),
		seen: parseSeen(remote.Seen), receiveOnly: remote.ReceiveOnly,
	}, unlock, nil
}

// syncRun は、1 回の Sync の状態。start は、同期を始めたときの自分の先端(受け取った数を数える基準)。
type syncRun struct {
	r      *Replicator
	repo   *git.Repository
	root   string
	link   link
	branch plumbing.ReferenceName
	start  plumbing.Hash

	// seen は、前の同期で同じ remote から受け取った位置(ゼロなら確かめない)。receiveOnly なら送らない。
	seen        plumbing.Hash
	receiveOnly bool
}

// once は、受け取り・合わせる・送るを 1 回行う。送信が拒否されたら errRejected。
func (s syncRun) once(ctx context.Context) (port.SyncResult, error) {
	theirs, err := s.receive(ctx)
	if err != nil {
		return port.SyncResult{}, err
	}
	if s.receiveOnly {
		return s.result(theirs, theirs, false)
	}
	head, err := s.push(ctx, theirs)
	if err != nil {
		return port.SyncResult{}, err
	}
	return s.result(theirs, head, true)
}

// receive は、remote から受け取り、自分の先端に取り込む。remote の先端 theirs を返す(空ならゼロ)。
func (s syncRun) receive(ctx context.Context) (plumbing.Hash, error) {
	theirs, err := s.link.fetch(ctx, s.repo, s.branch)
	if errors.Is(err, port.ErrNoCommonHistory) && !s.seen.IsZero() {
		return plumbing.ZeroHash, port.ErrRemoteRewritten // 前に受け取ったブランチが消えた(決定 C2-12)
	}
	if err != nil {
		return plumbing.ZeroHash, err
	}
	if err := s.checkSeen(theirs); err != nil {
		return plumbing.ZeroHash, err
	}
	if theirs.IsZero() {
		return theirs, nil
	}
	if err := checkRemoteStore(s.repo, theirs); err != nil {
		return plumbing.ZeroHash, err
	}
	return theirs, s.integrate(theirs)
}

// push は、自分の先端を remote へ送り、送った先端を返す。error は送れなかったときだけ返す(送れたあとの
// 記録の失敗はログに残すだけ。SyncSave が送れた保存を捨てないため)。
func (s syncRun) push(ctx context.Context, theirs plumbing.Hash) (plumbing.Hash, error) {
	if s.r.beforeSend != nil {
		if err := s.r.beforeSend(); err != nil {
			return plumbing.ZeroHash, err
		}
	}
	head, err := s.repo.Head()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	if err := s.link.send(ctx, s.branch, theirs, head.Hash()); err != nil {
		return plumbing.ZeroHash, err
	}
	// 送ったので、remote の先端は自分の先端になった。未送信の数え方(Unsent)の基準を進める。
	// 進められなくても、未送信の数が次の同期まで多く見えるだけ(次の受け取りが基準を進める)。
	ref := plumbing.NewHashReference(plumbing.ReferenceName(remotePrefix+s.branch.Short()), head.Hash())
	if err := s.repo.Storer.SetReference(ref); err != nil {
		slog.Warn("送ったあとに remote の位置を記録できませんでした", "store", s.root, "error", err)
	}
	return head.Hash(), nil
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
		return s.guarded(func() error { return s.forward(wt, ours, theirs) })
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

// forward は、自分の先端を remote の先端 theirs へ進める。ハードリセットは使わない(go-git v5 のハードリセットは
// インデックスにないファイルも消すので、Store のものでないファイルまで消してしまう)。
func (s syncRun) forward(wt *git.Worktree, ours, theirs plumbing.Hash) error {
	from, err := readTree(s.repo, ours)
	if err != nil {
		return err
	}
	to, err := readTree(s.repo, theirs)
	if err != nil {
		return err
	}
	return s.apply(wt, ours, from, to, func() error {
		return s.repo.Storer.SetReference(plumbing.NewHashReference(s.branch, theirs))
	})
}

// merge は、3 つのツリーを合わせて作業ツリーに書き、2 つの親を持つコミットにする。
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
	return s.apply(wt, ours, trees[1], merged, func() error {
		sig := signature(botName, botEmail, s.r.stores.clock.Now())
		_, err := wt.Commit("remote と合わせる\n", &git.CommitOptions{Author: sig, Committer: sig, Parents: []plumbing.Hash{ours, theirs}})
		return err
	})
}

// apply は、作業ツリーを from から to にし、変えたファイルだけをインデックスに載せて、finish でブランチを動かす。
// 失敗したら ours に戻し、新しく作ったファイルを消す。
func (s syncRun) apply(wt *git.Worktree, ours plumbing.Hash, from, to tree, finish func() error) error {
	changes, err := writeTree(s.root, from, to)
	if err == nil {
		err = stage(wt, changes)
	}
	if err == nil {
		err = finish()
	}
	if err != nil {
		errs := []error{err, wt.Reset(&git.ResetOptions{Commit: ours, Mode: git.HardReset})}
		for _, p := range changes.created {
			errs = append(errs, os.Remove(filepath.Join(s.root, p)))
		}
		return errors.Join(errs...)
	}
	return nil
}

// result は、送った保存(remote の先端 theirs になかったもの)と、受け取った保存(始めたときの自分になかったもの)を
// 数える。now は同期のあとに remote が持つ先端(Seen になる)。sent が false なら、送った数は 0。
func (s syncRun) result(theirs, now plumbing.Hash, sent bool) (port.SyncResult, error) {
	var res port.SyncResult
	if !now.IsZero() {
		res.Seen = now.String()
	}
	if sent {
		head, err := s.repo.Head()
		if err != nil {
			return port.SyncResult{}, err
		}
		if res.Sent, err = countSaves(s.repo, head.Hash(), theirs); err != nil {
			return port.SyncResult{}, err
		}
	}
	received, err := countSaves(s.repo, theirs, s.start)
	if err != nil {
		return port.SyncResult{}, err
	}
	res.Received = received
	return res, nil
}
