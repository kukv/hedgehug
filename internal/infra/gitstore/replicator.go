package gitstore

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/kukv/hedgehug/internal/app/port"
)

// maxSendTries は、送信が他の人の更新と重なったときに、受け取りからやり直す最大の回数(06「同期のエラー」)。
const maxSendTries = 3

// Replicator は、gitstore の Store を remote と合わせる(port.Replicator)。
type Replicator struct {
	stores *Stores
	// syncing は、Store ごとの mutex(*sync.Mutex)。同じ Store の Sync・SyncSave を 1 つずつにする(決定 G1-10)。
	syncing sync.Map

	// Repair は、合わせたあと、送る前に呼ぶ修復(決定 C3a-2・C3a-3)。cmd が置く。nil なら呼ばない。Mirror では使わない。
	Repair port.RepairFunc
	// Judge は、同期の最後に、受け取った保存・修復の保存・合わせを渡す判定の差し込み口(決定 C4b1-2・C4b2-2)。Judged は判定した保存の記録。
	// cmd が置く。どちらかが nil なら判定しない。
	Judge  port.JudgeFunc
	Judged port.JudgedSaves
	// OnSkew は、git の通信の応答の Date で測った時計のずれ(Date − このマシンの時計)を受け取る(決定 C5d-3)。nil なら測らない。
	OnSkew func(time.Duration)

	// beforeSend は、送る直前に呼ぶ。テストで、送信が他の人の更新と重なる場面を作るためのもの。
	beforeSend func() error
	// duringFetch・duringSend は、Sync の受け取り・送りの通信の直前に(ロックの外で)呼ぶ。テストで、通信のあいだに保存できることを確かめるためのもの。
	duringFetch, duringSend func()
}

func NewReplicator(s *Stores) *Replicator { return &Replicator{stores: s} }

// Sync は、受け取り、合わせ、修復し、送る。Store のロックを持つのは、合わせ、修復し、判定するあいだだけで、
// 受け取りと送りの通信のあいだは持たない(決定 G1-10。そのあいだも画面から保存できる)。
func (r *Replicator) Sync(ctx context.Context, id port.StoreID, remote port.Remote) (port.SyncResult, error) {
	defer r.hold(id)()
	ctx = withSkew(ctx, r.OnSkew, r.stores.clock.Now)
	s, unlock, err := r.begin(id, remote)
	if err != nil {
		return port.SyncResult{}, err
	}
	s.baseline(ctx)
	unlock()
	// 2 回目からの試みが失敗しても(断る理由のエラーでも、ErrSendConflict と同じく)、前の試みで作った
	// 合わせたコミットはこのマシンに残る。それは受け取ったものを正しく合わせたもので、次の同期で続ける。
	// 修復が失敗したときも(最初の試みでも)、合わせたコミットと、それまでに書いた修復は残る。合わせはまだ
	// 送っていないので、次の同期がもう一度修復を呼ぶ(決定 C3a-4)。
	for range maxSendTries {
		res, err := s.once(ctx)
		if errors.Is(err, errRejected) {
			continue
		}
		if errors.Is(err, port.ErrRemoteRewritten) {
			err = errors.Join(err, s.locked(s.restoreSeen))
		}
		return res, err
	}
	return port.SyncResult{}, port.ErrSendConflict
}

// hold は、Store ごとの mutex を取り、放す関数を返す。受け取りは remote の位置の ref を書き換えるので、
// 同じ Store の通信どうしは重ねない(決定 G1-10)。
func (r *Replicator) hold(id port.StoreID) (release func()) {
	v, _ := r.syncing.LoadOrStore(id, &sync.Mutex{})
	mu, _ := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
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
		r: r, id: id, repo: repo, root: root, link: newLink(repo, remote), branch: head.Name(), start: head.Hash(),
		seen: parseSeen(remote.Seen), receiveOnly: remote.ReceiveOnly,
	}, unlock, nil
}

// syncRun は、1 回の Sync の状態。start は、同期を始めたときの自分の先端(受け取った数を数える基準)。
type syncRun struct {
	r      *Replicator
	id     port.StoreID
	repo   *git.Repository
	root   string
	link   link
	branch plumbing.ReferenceName
	start  plumbing.Hash

	// seen は、前の同期で同じ remote から受け取った位置(ゼロなら確かめない)。receiveOnly なら送らない。
	seen        plumbing.Hash
	receiveOnly bool
}

// locked は、Store のロックを取り、途中で止まった保存の跡を戻してから fn を呼ぶ。
func (s syncRun) locked(fn func() error) error {
	unlock, err := open(s.root)
	if err != nil {
		return err
	}
	defer unlock()
	return fn()
}

// once は、受け取り・合わせる・修復する・送るを 1 回行う。送信が拒否されたら errRejected。通信はロックの外で行い、
// 送るのはロックの中で読んだ先端 mine まで(そのあとの保存は次の同期で送る)。判定と数え方も mine までにする。
func (s syncRun) once(ctx context.Context) (res port.SyncResult, err error) {
	if s.r.duringFetch != nil {
		s.r.duringFetch()
	}
	theirs, err := s.fetch(ctx)
	if err != nil {
		return port.SyncResult{}, err
	}
	var mine plumbing.Hash
	err = s.locked(func() error {
		if err := s.accept(theirs); err != nil {
			return err
		}
		if s.receiveOnly {
			res, err = s.finish(ctx, theirs, theirs, false)
			return err
		}
		// 修復は送る前に書く。送ったあとに書くと、相手のマシンが修復していない合わせた結果を受け取る(決定 C3a-2)。
		if err := s.repairUnsent(ctx, theirs); err != nil {
			return err
		}
		mine, err = s.head()
		return err
	})
	if err != nil || s.receiveOnly {
		return res, err
	}
	if s.r.duringSend != nil {
		s.r.duringSend()
	}
	if err := s.send(ctx, theirs, mine); err != nil {
		return port.SyncResult{}, err
	}
	err = s.locked(func() error {
		s.markSent(mine)
		res, err = s.finish(ctx, theirs, mine, true)
		return err
	})
	return res, err
}

// receive は、remote から受け取り、自分の先端に取り込む。remote の先端 theirs を返す(空ならゼロ)。
func (s syncRun) receive(ctx context.Context) (plumbing.Hash, error) {
	theirs, err := s.fetch(ctx)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return theirs, s.accept(theirs)
}

// fetch は、remote から受け取り、remote の先端 theirs を返す(空ならゼロ)。書くのは、中身で決まる名前の
// オブジェクトと remote の位置の ref だけで、作業ツリーと自分の先端には触らないので、ロックの外で呼んでよい。
func (s syncRun) fetch(ctx context.Context) (plumbing.Hash, error) {
	theirs, err := s.link.fetch(ctx, s.repo, s.branch)
	if errors.Is(err, port.ErrNoCommonHistory) && !s.seen.IsZero() {
		return plumbing.ZeroHash, port.ErrRemoteRewritten // 前に受け取ったブランチが消えた(決定 C2-12)
	}
	return theirs, err
}

// accept は、受け取った remote の先端 theirs を確かめ、自分の先端に取り込む。ロックの中で呼ぶ。
func (s syncRun) accept(theirs plumbing.Hash) error {
	if err := s.checkSeen(theirs); err != nil {
		return err
	}
	if theirs.IsZero() {
		return nil
	}
	if err := checkRemoteStore(s.repo, theirs); err != nil {
		return err
	}
	return s.integrate(theirs)
}

// push は、自分の先端を remote へ送り、送った先端を返す。error は送れなかったときだけ返す(送れたあとの
// 記録の失敗はログに残すだけ。SyncSave が送れた保存を捨てないため)。
func (s syncRun) push(ctx context.Context, theirs plumbing.Hash) (plumbing.Hash, error) {
	head, err := s.head()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	if err := s.send(ctx, theirs, head); err != nil {
		return plumbing.ZeroHash, err
	}
	s.markSent(head)
	return head, nil
}

// send は、mine を remote へ送る。
func (s syncRun) send(ctx context.Context, theirs, mine plumbing.Hash) error {
	if s.r.beforeSend != nil {
		if err := s.r.beforeSend(); err != nil {
			return err
		}
	}
	return s.link.send(ctx, s.branch, theirs, mine)
}

// markSent は、remote の位置の ref を送った先端 mine にする。未送信の数え方(Unsent)の基準を進める。
// 進められなくても、未送信の数が次の同期まで多く見えるだけ(次の受け取りが基準を進める)。
func (s syncRun) markSent(mine plumbing.Hash) {
	ref := plumbing.NewHashReference(plumbing.ReferenceName(remotePrefix+s.branch.Short()), mine)
	if err := s.repo.Storer.SetReference(ref); err != nil {
		slog.Warn("送ったあとに remote の位置を記録できませんでした", "store", s.root, "error", err)
	}
}

func (s syncRun) head() (plumbing.Hash, error) {
	head, err := s.repo.Head()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	return head.Hash(), nil
}
